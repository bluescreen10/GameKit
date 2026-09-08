//go:build windows && cgo

#define UNICODE
#define _UNICODE
#include <windows.h>
#include "window_bridge.h"
#include <stdlib.h>
#include <string.h>

typedef struct {
    HWND window;
    int shouldClose;
    unsigned char keys[GK_KEY_COUNT];
} GKWindowNative;

static const wchar_t *gkWindowClass = L"GameKitNativeWindow";
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
            if (message == WM_KEYUP || message == WM_SYSKEYUP) {
                native->keys[key] = GK_KEY_RELEASED;
            } else {
                native->keys[key] = (lparam & (1l << 30)) ? GK_KEY_REPEAT : GK_KEY_PRESSED;
            }
        }
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
    POINT point = {0};
    GetCursorPos(&point);
    ScreenToClient(native->window, &point);
    if (x) *x = (double)point.x;
    if (y) *y = (double)point.y;
}

int gkWindowGetKey(void *pointer, int key) {
    if (!pointer || key < 0 || key >= GK_KEY_COUNT) return GK_KEY_RELEASED;
    return ((GKWindowNative *)pointer)->keys[key];
}
