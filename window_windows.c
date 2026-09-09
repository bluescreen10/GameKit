//go:build windows && cgo

#define UNICODE
#define _UNICODE
#include <windows.h>
#include <windowsx.h> /* GET_X_LPARAM / GET_Y_LPARAM */
#include "window_bridge.h"
#include <stdlib.h>
#include <string.h>

typedef struct {
    HWND window;
    int shouldClose;
    double scrollX, scrollY;
    int cursorMode;
    /* Virtual cursor position, accumulated from deltas while the cursor is disabled,
       plus the last real position the deltas are measured against. */
    double virtualX, virtualY;
    int lastX, lastY;
    int haveLast;
    /* A high surrogate awaiting its pair; WM_CHAR delivers UTF-16 one unit at a time. */
    unsigned int highSurrogate;
    /* runtime/cgo Handle for the Go *Window; see gkWindowSetHandle. */
    uintptr_t handle;
} GKWindowNative;

static const wchar_t *gkWindowClass = L"gamekitNativeWindow";
static ATOM gkWindowClassAtom;

static void gkSetError(char *error, size_t size, const char *message) {
    if (error && size) {
        strncpy(error, message, size - 1);
        error[size - 1] = '\0';
    }
}

static wchar_t *gkWide(const char *text) {
    int count = MultiByteToWideChar(CP_UTF8, 0, text ? text : "", -1, NULL, 0);
    wchar_t *wide = calloc((size_t)count, sizeof(wchar_t));
    if (wide) MultiByteToWideChar(CP_UTF8, 0, text ? text : "", -1, wide, count);
    return wide;
}

static int gkWinKey(WPARAM value, LPARAM details) {
    UINT key = (UINT)value;
    if (key == VK_SHIFT) {
        UINT scan = (UINT)((details >> 16) & 0xff);
        key = MapVirtualKeyW(scan, MAPVK_VSC_TO_VK_EX);
    } else if (key == VK_CONTROL) {
        key = (details & (1l << 24)) ? VK_RCONTROL : VK_LCONTROL;
    } else if (key == VK_MENU) {
        key = (details & (1l << 24)) ? VK_RMENU : VK_LMENU;
    }
    if ((key >= 'A' && key <= 'Z') || (key >= '0' && key <= '9')) return (int)key;
    if (key >= VK_F1 && key <= VK_F24) return 290 + (int)(key - VK_F1);
    if (key >= VK_NUMPAD0 && key <= VK_NUMPAD9) return 320 + (int)(key - VK_NUMPAD0);
    switch (key) {
        case VK_SPACE: return 32; case VK_OEM_7: return 39; case VK_OEM_COMMA: return 44;
        case VK_OEM_MINUS: return 45; case VK_OEM_PERIOD: return 46; case VK_OEM_2: return 47;
        case VK_OEM_1: return 59; case VK_OEM_PLUS: return 61; case VK_OEM_4: return 91;
        case VK_OEM_5: return 92; case VK_OEM_6: return 93; case VK_OEM_3: return 96;
        case VK_ESCAPE: return 256; case VK_RETURN: return (details & (1l << 24)) ? 335 : 257;
        case VK_TAB: return 258; case VK_BACK: return 259; case VK_INSERT: return 260;
        case VK_DELETE: return 261; case VK_RIGHT: return 262; case VK_LEFT: return 263;
        case VK_DOWN: return 264; case VK_UP: return 265; case VK_PRIOR: return 266;
        case VK_NEXT: return 267; case VK_HOME: return 268; case VK_END: return 269;
        case VK_CAPITAL: return 280; case VK_SCROLL: return 281; case VK_NUMLOCK: return 282;
        case VK_SNAPSHOT: return 283; case VK_PAUSE: return 284; case VK_DECIMAL: return 330;
        case VK_DIVIDE: return 331; case VK_MULTIPLY: return 332; case VK_SUBTRACT: return 333;
        case VK_ADD: return 334;
#ifdef VK_OEM_NEC_EQUAL
        case VK_OEM_NEC_EQUAL: return 336;
#endif
        case VK_LSHIFT: return 340;
        case VK_LCONTROL: return 341; case VK_LMENU: return 342; case VK_LWIN: return 343;
        case VK_RSHIFT: return 344; case VK_RCONTROL: return 345; case VK_RMENU: return 346;
        case VK_RWIN: return 347; case VK_APPS: return 348; default: return -1;
    }
}

/* Read the modifier keys into GameKit's portable bitmask. Win32 reports them as
   global key state rather than per-message, unlike X11 and Cocoa. */
static int gkWinMods(void) {
    int mods = 0;
    if (GetKeyState(VK_SHIFT) & 0x8000) mods |= GK_MOD_SHIFT;
    if (GetKeyState(VK_CONTROL) & 0x8000) mods |= GK_MOD_CONTROL;
    if (GetKeyState(VK_MENU) & 0x8000) mods |= GK_MOD_ALT;
    if ((GetKeyState(VK_LWIN) | GetKeyState(VK_RWIN)) & 0x8000) mods |= GK_MOD_SUPER;
    if (GetKeyState(VK_CAPITAL) & 1) mods |= GK_MOD_CAPS_LOCK;
    if (GetKeyState(VK_NUMLOCK) & 1) mods |= GK_MOD_NUM_LOCK;
    return mods;
}

static void gkWinCentre(GKWindowNative *native, int *outX, int *outY) {
    RECT rect;
    GetClientRect(native->window, &rect);
    int cx = (rect.right - rect.left) / 2;
    int cy = (rect.bottom - rect.top) / 2;
    if (outX) *outX = cx;
    if (outY) *outY = cy;
}

static void gkWinWarpToCentre(GKWindowNative *native) {
    int cx, cy;
    gkWinCentre(native, &cx, &cy);
    POINT point = {cx, cy};
    ClientToScreen(native->window, &point);
    SetCursorPos(point.x, point.y);
    native->lastX = cx;
    native->lastY = cy;
    native->haveLast = 1;
}

static void gkApplyCursorMode(GKWindowNative *native) {
    if (!native) return;
    if (native->cursorMode == GK_CURSOR_NORMAL) {
        ClipCursor(NULL);
        ReleaseCapture();
        while (ShowCursor(TRUE) < 0) {}
    } else {
        while (ShowCursor(FALSE) >= 0) {}
        if (native->cursorMode == GK_CURSOR_DISABLED) {
            /* Confine the cursor to the window so it cannot reach a screen edge and
               stop producing motion; recentring below does the rest. */
            RECT rect;
            GetClientRect(native->window, &rect);
            POINT topLeft = {rect.left, rect.top};
            POINT bottomRight = {rect.right, rect.bottom};
            ClientToScreen(native->window, &topLeft);
            ClientToScreen(native->window, &bottomRight);
            RECT screen = {topLeft.x, topLeft.y, bottomRight.x, bottomRight.y};
            ClipCursor(&screen);
            SetCapture(native->window);
            gkWinWarpToCentre(native);
        } else {
            ClipCursor(NULL);
            ReleaseCapture();
        }
    }
}

static LRESULT CALLBACK gkWindowProc(HWND window, UINT message, WPARAM wparam, LPARAM lparam) {
    GKWindowNative *native = (GKWindowNative *)GetWindowLongPtrW(window, GWLP_USERDATA);
    if (message == WM_NCCREATE) {
        CREATESTRUCTW *create = (CREATESTRUCTW *)lparam;
        native = (GKWindowNative *)create->lpCreateParams;
        native->window = window;
        SetWindowLongPtrW(window, GWLP_USERDATA, (LONG_PTR)native);
    } else if (message == WM_CLOSE) {
        if (native) native->shouldClose = 1;
        return 0;
    } else if (message == WM_DESTROY) {
        if (native) native->shouldClose = 1;
        return 0;
    } else if (message == WM_KEYDOWN || message == WM_SYSKEYDOWN ||
               message == WM_KEYUP || message == WM_SYSKEYUP) {
        int key = gkWinKey(wparam, lparam);
        if (native && key >= 0 && key < GK_KEY_COUNT) {
            int action;
            if (message == WM_KEYUP || message == WM_SYSKEYUP) {
                action = GK_KEY_RELEASED;
            } else {
                action = (lparam & (1l << 30)) ? GK_KEY_REPEAT : GK_KEY_PRESSED;
            }
            gkGoKeyEvent(native->handle, key, (int)((lparam >> 16) & 0xFF), action, gkWinMods());
        }
    } else if (message == WM_CHAR || message == WM_SYSCHAR) {
        /* WM_CHAR carries text already resolved through the layout and any dead key,
           delivered as UTF-16 — so a non-BMP character arrives as two messages. */
        if (native) {
            unsigned int unit = (unsigned int)wparam;
            if (unit >= 0xD800 && unit <= 0xDBFF) {
                native->highSurrogate = unit;
            } else {
                unsigned int codepoint = unit;
                if (unit >= 0xDC00 && unit <= 0xDFFF && native->highSurrogate) {
                    codepoint = 0x10000 + ((native->highSurrogate - 0xD800) << 10) +
                                (unit - 0xDC00);
                }
                native->highSurrogate = 0;
                if (codepoint >= 0x20 && codepoint != 0x7F) {
                    gkGoCharEvent(native->handle, codepoint);
                }
            }
        }
        return 0;
    } else if (message == WM_LBUTTONDOWN || message == WM_LBUTTONUP ||
               message == WM_RBUTTONDOWN || message == WM_RBUTTONUP ||
               message == WM_MBUTTONDOWN || message == WM_MBUTTONUP ||
               message == WM_XBUTTONDOWN || message == WM_XBUTTONUP) {
        if (native) {
            int button = -1, action = GK_POINTER_RELEASED;
            switch (message) {
                case WM_LBUTTONDOWN: button = GK_POINTER_BUTTON_LEFT; action = GK_POINTER_PRESSED; break;
                case WM_LBUTTONUP:   button = GK_POINTER_BUTTON_LEFT; break;
                case WM_RBUTTONDOWN: button = GK_POINTER_BUTTON_RIGHT; action = GK_POINTER_PRESSED; break;
                case WM_RBUTTONUP:   button = GK_POINTER_BUTTON_RIGHT; break;
                case WM_MBUTTONDOWN: button = GK_POINTER_BUTTON_MIDDLE; action = GK_POINTER_PRESSED; break;
                case WM_MBUTTONUP:   button = GK_POINTER_BUTTON_MIDDLE; break;
                case WM_XBUTTONDOWN: action = GK_POINTER_PRESSED; /* fallthrough */
                case WM_XBUTTONUP:
                    button = GET_XBUTTON_WPARAM(wparam) == XBUTTON1 ? 3 : 4;
                    break;
            }
            if (button >= 0 && button < GK_POINTER_BUTTON_COUNT) {
                gkGoPointerButtonEvent(native->handle, button, action, gkWinMods());
            }
        }
        if (message == WM_XBUTTONDOWN || message == WM_XBUTTONUP) return TRUE;
        return 0;
    } else if (message == WM_MOUSEWHEEL || message == WM_MOUSEHWHEEL) {
        if (native) {
            double delta = (double)GET_WHEEL_DELTA_WPARAM(wparam) / (double)WHEEL_DELTA;
            double dx = 0, dy = 0;
            if (message == WM_MOUSEWHEEL) dy = delta;
            else dx = delta;
            native->scrollX += dx;
            native->scrollY += dy;
            gkGoScrollEvent(native->handle, dx, dy);
        }
        return 0;
    } else if (message == WM_MOUSEMOVE) {
        if (native && native->cursorMode == GK_CURSOR_DISABLED) {
            int x = GET_X_LPARAM(lparam);
            int y = GET_Y_LPARAM(lparam);
            if (native->haveLast) {
                native->virtualX += x - native->lastX;
                native->virtualY += y - native->lastY;
            }
            native->lastX = x;
            native->lastY = y;
            native->haveLast = 1;
            /* Recentre once the cursor drifts far enough that it might reach the edge
               of the window, where motion would stop. */
            RECT rect;
            GetClientRect(native->window, &rect);
            int cx = (rect.right - rect.left) / 2;
            int cy = (rect.bottom - rect.top) / 2;
            if (abs(x - cx) > (rect.right - rect.left) / 4 ||
                abs(y - cy) > (rect.bottom - rect.top) / 4) {
                gkWinWarpToCentre(native);
            }
        }
        return 0;
    }
    return DefWindowProcW(window, message, wparam, lparam);
}

static int gkRegisterWindowClass(void) {
    if (gkWindowClassAtom) return 1;
    WNDCLASSEXW cls = {0};
    cls.cbSize = sizeof(cls);
    cls.style = CS_HREDRAW | CS_VREDRAW | CS_OWNDC;
    cls.lpfnWndProc = gkWindowProc;
    cls.hInstance = GetModuleHandleW(NULL);
    cls.hCursor = LoadCursorW(NULL, IDC_ARROW);
    cls.lpszClassName = gkWindowClass;
    gkWindowClassAtom = RegisterClassExW(&cls);
    return gkWindowClassAtom != 0 || GetLastError() == ERROR_CLASS_ALREADY_EXISTS;
}

void *gkWindowCreate(const char *title, int width, int height, uint32_t flags,
                     char *error, size_t errorSize) {
    SetProcessDPIAware();
    if (!gkRegisterWindowClass()) {
        gkSetError(error, errorSize, "Win32 could not register the window class");
        return NULL;
    }
    GKWindowNative *native = calloc(1, sizeof(*native));
    wchar_t *wideTitle = gkWide(title);
    if (!native || !wideTitle) {
        free(native);
        free(wideTitle);
        gkSetError(error, errorSize, "out of memory");
        return NULL;
    }

    DWORD style = WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX;
    if (flags & GK_WINDOW_BORDERLESS) style = WS_POPUP;
    else if (flags & GK_WINDOW_RESIZABLE) style |= WS_THICKFRAME | WS_MAXIMIZEBOX;
    DWORD exStyle = (flags & GK_WINDOW_ALWAYS_ON_TOP) ? WS_EX_TOPMOST : 0;
    if (flags & GK_WINDOW_TRANSPARENT) exStyle |= WS_EX_LAYERED;

    RECT rect = {0, 0, width, height};
    AdjustWindowRectEx(&rect, style, FALSE, exStyle);
    HWND window = CreateWindowExW(exStyle, gkWindowClass, wideTitle, style,
        CW_USEDEFAULT, CW_USEDEFAULT, rect.right - rect.left, rect.bottom - rect.top,
        NULL, NULL, GetModuleHandleW(NULL), native);
    free(wideTitle);
    if (!window) {
        free(native);
        gkSetError(error, errorSize, "Win32 could not create a window");
        return NULL;
    }
    if (flags & GK_WINDOW_TRANSPARENT) SetLayeredWindowAttributes(window, 0, 255, LWA_ALPHA);
    if (!(flags & GK_WINDOW_HIDDEN)) {
        ShowWindow(window, (flags & GK_WINDOW_MAXIMIZED) ? SW_MAXIMIZE : SW_SHOW);
        UpdateWindow(window);
    }
    return native;
}

void gkWindowDestroy(void *pointer) {
    if (!pointer) return;
    GKWindowNative *native = pointer;
    /* A clip region or hidden cursor outlives the window and would affect the whole
       desktop. */
    if (native->cursorMode != GK_CURSOR_NORMAL) {
        native->cursorMode = GK_CURSOR_NORMAL;
        gkApplyCursorMode(native);
    }
    if (native->window) DestroyWindow(native->window);
    free(native);
}

void gkWindowPoll(void *pointer) {
    (void)pointer;
    MSG message;
    while (PeekMessageW(&message, NULL, 0, 0, PM_REMOVE)) {
        TranslateMessage(&message);
        DispatchMessageW(&message);
    }
}

int gkWindowShouldClose(void *pointer) {
    return pointer ? ((GKWindowNative *)pointer)->shouldClose : 1;
}

void gkWindowSetShouldClose(void *pointer, int close) {
    if (pointer) ((GKWindowNative *)pointer)->shouldClose = close != 0;
}

void gkWindowSize(void *pointer, int *width, int *height) {
    RECT rect = {0};
    GetClientRect(((GKWindowNative *)pointer)->window, &rect);
    if (width) *width = rect.right - rect.left;
    if (height) *height = rect.bottom - rect.top;
}

void gkWindowFramebufferSize(void *pointer, int *width, int *height) {
    gkWindowSize(pointer, width, height);
}

void gkWindowSetTitle(void *pointer, const char *title) {
    wchar_t *wide = gkWide(title);
    if (wide) {
        SetWindowTextW(((GKWindowNative *)pointer)->window, wide);
        free(wide);
    }
}

uintptr_t gkWindowNativeHandle(void *pointer) {
    return pointer ? (uintptr_t)((GKWindowNative *)pointer)->window : 0;
}

void *gkWindowNativeDisplay(void *pointer) {
    (void)pointer;
    return NULL;
}

void gkWindowCursorPosition(void *pointer, double *x, double *y) {
    GKWindowNative *native = pointer;
    if (native->cursorMode == GK_CURSOR_DISABLED) {
        /* The cursor is clipped and recentred; report accumulated motion instead. */
        if (x) *x = native->virtualX;
        if (y) *y = native->virtualY;
        return;
    }
    POINT point = {0};
    GetCursorPos(&point);
    ScreenToClient(native->window, &point);
    if (x) *x = (double)point.x;
    if (y) *y = (double)point.y;
}

void gkWindowSetHandle(void *pointer, uintptr_t handle) {
    if (pointer) ((GKWindowNative *)pointer)->handle = handle;
}

void gkWindowGetScroll(void *pointer, double *x, double *y) {
    GKWindowNative *native = pointer;
    if (x) *x = native ? native->scrollX : 0.0;
    if (y) *y = native ? native->scrollY : 0.0;
}

void gkWindowSetCursorMode(void *pointer, int mode) {
    GKWindowNative *native = pointer;
    if (!native || native->cursorMode == mode) return;
    if (mode == GK_CURSOR_DISABLED) {
        double x = 0, y = 0;
        gkWindowCursorPosition(pointer, &x, &y);
        native->virtualX = x;
        native->virtualY = y;
        native->haveLast = 0;
    }
    native->cursorMode = mode;
    gkApplyCursorMode(native);
}

int gkWindowGetCursorMode(void *pointer) {
    return pointer ? ((GKWindowNative *)pointer)->cursorMode : GK_CURSOR_NORMAL;
}
