//go:build linux && cgo

#include "window_bridge.h"
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/XKBlib.h>
#include <X11/keysym.h>
#include <X11/Xutil.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    Display *display;
    Window window;
    Colormap colormap;
    Atom deleteWindow;
    Cursor blankCursor;
    int shouldClose;
    unsigned char keys[GK_KEY_COUNT];
    unsigned char buttons[GK_POINTER_BUTTON_COUNT];
    double scrollX, scrollY;
    int cursorMode;
    /* Virtual cursor position, accumulated from deltas while the cursor is disabled,
       plus the last real position the deltas are measured against. */
    double virtualX, virtualY;
    int lastX, lastY;
    int haveLast;
    /* Cached client size, refreshed from ConfigureNotify. Recentring the pointer needs
       the window centre on every motion event, and asking the server each time would
       be a round trip per event. */
    int width, height;
    /* runtime/cgo Handle for the Go *Window; see gkWindowSetHandle. */
    uintptr_t handle;
} GKWindowNative;

static void gkSetError(char *error, size_t size, const char *message) {
    if (error && size) {
        strncpy(error, message, size - 1);
        error[size - 1] = '\0';
    }
}

static void gkSetMotifDecorations(Display *display, Window window, int decorated) {
    struct {
        unsigned long flags, functions, decorations;
        long inputMode;
        unsigned long status;
    } hints = {2, 0, decorated ? 1ul : 0ul, 0, 0};
    Atom property = XInternAtom(display, "_MOTIF_WM_HINTS", False);
    XChangeProperty(display, window, property, property, 32, PropModeReplace,
                    (unsigned char *)&hints, 5);
}

static int gkX11Key(KeySym symbol) {
    if (symbol >= XK_a && symbol <= XK_z) return (int)(symbol - XK_a + 'A');
    if ((symbol >= XK_A && symbol <= XK_Z) || (symbol >= XK_0 && symbol <= XK_9)) return (int)symbol;
    switch (symbol) {
        case XK_space: return 32; case XK_apostrophe: return 39; case XK_comma: return 44;
        case XK_minus: return 45; case XK_period: return 46; case XK_slash: return 47;
        case XK_semicolon: return 59; case XK_equal: return 61; case XK_bracketleft: return 91;
        case XK_backslash: return 92; case XK_bracketright: return 93; case XK_grave: return 96;
        case XK_Escape: return 256; case XK_Return: return 257; case XK_Tab: return 258;
        case XK_BackSpace: return 259; case XK_Insert: return 260; case XK_Delete: return 261;
        case XK_Right: return 262; case XK_Left: return 263; case XK_Down: return 264;
        case XK_Up: return 265; case XK_Page_Up: return 266; case XK_Page_Down: return 267;
        case XK_Home: return 268; case XK_End: return 269; case XK_Caps_Lock: return 280;
        case XK_Scroll_Lock: return 281; case XK_Num_Lock: return 282; case XK_Print: return 283;
        case XK_Pause: return 284; case XK_KP_Decimal: return 330; case XK_KP_Divide: return 331;
        case XK_KP_Multiply: return 332; case XK_KP_Subtract: return 333; case XK_KP_Add: return 334;
        case XK_KP_Enter: return 335; case XK_KP_Equal: return 336; case XK_Shift_L: return 340;
        case XK_Control_L: return 341; case XK_Alt_L: return 342; case XK_Super_L: return 343;
        case XK_Shift_R: return 344; case XK_Control_R: return 345; case XK_Alt_R: return 346;
        case XK_Super_R: return 347; case XK_Menu: return 348; default: break;
    }
    if (symbol >= XK_F1 && symbol <= XK_F25) return 290 + (int)(symbol - XK_F1);
    if (symbol >= XK_KP_0 && symbol <= XK_KP_9) return 320 + (int)(symbol - XK_KP_0);
    return -1;
}

void *gkWindowCreate(const char *title, int width, int height, uint32_t flags,
                     char *error, size_t errorSize) {
    Display *display = XOpenDisplay(NULL);
    if (!display) {
        gkSetError(error, errorSize, "could not open the X11 display");
        return NULL;
    }

    int screen = DefaultScreen(display);
    Window root = RootWindow(display, screen);
    XSetWindowAttributes attributes = {0};
    attributes.event_mask = StructureNotifyMask | FocusChangeMask | KeyPressMask |
                            KeyReleaseMask | ButtonPressMask | ButtonReleaseMask |
                            PointerMotionMask;
    unsigned long mask = CWEventMask;
    Visual *visual = DefaultVisual(display, screen);
    int depth = DefaultDepth(display, screen);

    Colormap colormap = 0;
    if (flags & GK_WINDOW_TRANSPARENT) {
        XVisualInfo info;
        if (!XMatchVisualInfo(display, screen, 32, TrueColor, &info)) {
            XCloseDisplay(display);
            gkSetError(error, errorSize, "the X11 display has no transparent 32-bit visual");
            return NULL;
        }
        visual = info.visual;
        depth = info.depth;
        colormap = XCreateColormap(display, root, visual, AllocNone);
        attributes.colormap = colormap;
        attributes.border_pixel = 0;
        attributes.background_pixel = 0;
        mask |= CWColormap | CWBorderPixel | CWBackPixel;
    }

    Window window = XCreateWindow(display, root, 0, 0, (unsigned int)width,
        (unsigned int)height, 0, depth, InputOutput, visual, mask, &attributes);
    if (!window) {
        if (colormap) XFreeColormap(display, colormap);
        XCloseDisplay(display);
        gkSetError(error, errorSize, "X11 could not create a window");
        return NULL;
    }

    GKWindowNative *native = calloc(1, sizeof(*native));
    if (!native) {
        XDestroyWindow(display, window);
        if (colormap) XFreeColormap(display, colormap);
        XCloseDisplay(display);
        gkSetError(error, errorSize, "out of memory");
        return NULL;
    }
    native->display = display;
    native->window = window;
    native->colormap = colormap;
    native->deleteWindow = XInternAtom(display, "WM_DELETE_WINDOW", False);
    Bool detectable;
    XkbSetDetectableAutoRepeat(display, True, &detectable);
    XSetWMProtocols(display, window, &native->deleteWindow, 1);
    XStoreName(display, window, title ? title : "");

    if (!(flags & GK_WINDOW_RESIZABLE)) {
        XSizeHints size = {0};
        size.flags = PMinSize | PMaxSize;
        size.min_width = size.max_width = width;
        size.min_height = size.max_height = height;
        XSetWMNormalHints(display, window, &size);
    }
    if (flags & GK_WINDOW_BORDERLESS) gkSetMotifDecorations(display, window, 0);

    Atom states[3];
    int stateCount = 0;
    if (flags & GK_WINDOW_MAXIMIZED) {
        states[stateCount++] = XInternAtom(display, "_NET_WM_STATE_MAXIMIZED_HORZ", False);
        states[stateCount++] = XInternAtom(display, "_NET_WM_STATE_MAXIMIZED_VERT", False);
    }
    if (flags & GK_WINDOW_ALWAYS_ON_TOP) {
        states[stateCount++] = XInternAtom(display, "_NET_WM_STATE_ABOVE", False);
    }
    if (stateCount) {
        Atom state = XInternAtom(display, "_NET_WM_STATE", False);
        XChangeProperty(display, window, state, XA_ATOM, 32, PropModeReplace,
                        (unsigned char *)states, stateCount);
    }
    if (!(flags & GK_WINDOW_HIDDEN)) XMapRaised(display, window);
    XFlush(display);
    return native;
}

void gkWindowDestroy(void *pointer) {
    if (!pointer) return;
    GKWindowNative *native = pointer;
    /* A pointer grab outlives the window and would lock up the desktop. */
    XUngrabPointer(native->display, CurrentTime);
    if (native->blankCursor) XFreeCursor(native->display, native->blankCursor);
    XDestroyWindow(native->display, native->window);
    if (native->colormap) XFreeColormap(native->display, native->colormap);
    XCloseDisplay(native->display);
    free(native);
}

/* Translate X11's modifier state mask into GameKit's portable bitmask. */
static int gkX11Mods(unsigned int state) {
    int mods = 0;
    if (state & ShiftMask) mods |= GK_MOD_SHIFT;
    if (state & ControlMask) mods |= GK_MOD_CONTROL;
    if (state & Mod1Mask) mods |= GK_MOD_ALT;
    if (state & Mod4Mask) mods |= GK_MOD_SUPER;
    if (state & LockMask) mods |= GK_MOD_CAPS_LOCK;
    if (state & Mod2Mask) mods |= GK_MOD_NUM_LOCK;
    return mods;
}

/* A 1x1 fully transparent cursor, which is how X11 hides one. */
static Cursor gkX11BlankCursor(Display *display, Window window) {
    static const char bits[] = {0};
    XColor black = {0};
    Pixmap pixmap = XCreateBitmapFromData(display, window, bits, 1, 1);
    if (!pixmap) return None;
    Cursor cursor = XCreatePixmapCursor(display, pixmap, pixmap, &black, &black, 0, 0);
    XFreePixmap(display, pixmap);
    return cursor;
}

static void gkX11RefreshSize(GKWindowNative *native) {
    XWindowAttributes attributes;
    XGetWindowAttributes(native->display, native->window, &attributes);
    native->width = attributes.width;
    native->height = attributes.height;
}

static void gkX11WarpToCentre(GKWindowNative *native) {
    if (native->width <= 0 || native->height <= 0) gkX11RefreshSize(native);
    int cx = native->width / 2;
    int cy = native->height / 2;
    XWarpPointer(native->display, None, native->window, 0, 0, 0, 0, cx, cy);
    /* The warp itself arrives as a MotionNotify at the centre; recording the centre as
       the last position makes that event's delta zero, so it contributes nothing. */
    native->lastX = cx;
    native->lastY = cy;
    native->haveLast = 1;
}

static void gkApplyCursorMode(GKWindowNative *native) {
    if (!native) return;
    if (native->cursorMode == GK_CURSOR_NORMAL) {
        XUndefineCursor(native->display, native->window);
        XUngrabPointer(native->display, CurrentTime);
    } else {
        if (!native->blankCursor) {
            native->blankCursor = gkX11BlankCursor(native->display, native->window);
        }
        if (native->blankCursor) {
            XDefineCursor(native->display, native->window, native->blankCursor);
        }
        if (native->cursorMode == GK_CURSOR_DISABLED) {
            /* Grabbing confines the pointer to the window, so it cannot reach a screen
               edge and stop generating motion. Recentring below does the rest. */
            XGrabPointer(native->display, native->window, True,
                         ButtonPressMask | ButtonReleaseMask | PointerMotionMask,
                         GrabModeAsync, GrabModeAsync, native->window, None, CurrentTime);
            gkX11WarpToCentre(native);
        } else {
            XUngrabPointer(native->display, CurrentTime);
        }
    }
    XFlush(native->display);
}

void gkWindowPoll(void *pointer) {
    if (!pointer) return;
    GKWindowNative *native = pointer;
    while (XPending(native->display)) {
        XEvent event;
        XNextEvent(native->display, &event);
        if (event.type == ClientMessage &&
            (Atom)event.xclient.data.l[0] == native->deleteWindow) {
            native->shouldClose = 1;
        } else if (event.type == DestroyNotify) {
            native->shouldClose = 1;
        } else if (event.type == ConfigureNotify) {
            native->width = event.xconfigure.width;
            native->height = event.xconfigure.height;
        } else if (event.type == KeyPress || event.type == KeyRelease) {
            int key = gkX11Key(XLookupKeysym(&event.xkey, 0));
            int mods = gkX11Mods(event.xkey.state);
            if (key >= 0 && key < GK_KEY_COUNT) {
                int action;
                if (event.type == KeyRelease) {
                    action = GK_KEY_RELEASED;
                } else {
                    action = native->keys[key] == GK_KEY_RELEASED ? GK_KEY_PRESSED : GK_KEY_REPEAT;
                }
                native->keys[key] = (unsigned char)action;
                gkGoKeyEvent(native->handle, key, (int)event.xkey.keycode, action, mods);
            }
            /* Text is resolved separately: XLookupString applies the keyboard layout,
               shift state and any compose sequence, none of which the keysym above
               carries. */
            if (event.type == KeyPress) {
                char buffer[32];
                KeySym ignored;
                int n = XLookupString(&event.xkey, buffer, (int)sizeof(buffer), &ignored, NULL);
                for (int i = 0; i < n; i++) {
                    unsigned char c = (unsigned char)buffer[i];
                    if (c < 0x20 || c == 0x7F) continue; /* control codes are not text */
                    gkGoCharEvent(native->handle, c);
                }
            }
        } else if (event.type == ButtonPress || event.type == ButtonRelease) {
            int mods = gkX11Mods(event.xbutton.state);
            unsigned int b = event.xbutton.button;
            /* X11 reports the wheel as buttons 4-7 rather than as an axis. Turn those
               back into scroll and never surface them as buttons. */
            if (b >= 4 && b <= 7) {
                if (event.type == ButtonPress) {
                    double dx = 0, dy = 0;
                    if (b == 4) dy = 1;
                    else if (b == 5) dy = -1;
                    else if (b == 6) dx = -1;
                    else dx = 1;
                    native->scrollX += dx;
                    native->scrollY += dy;
                    gkGoScrollEvent(native->handle, dx, dy);
                }
            } else {
                /* X11 numbers left/middle/right as 1/2/3; GameKit uses 0/1/2 with
                   middle last. */
                int button = -1;
                if (b == 1) button = GK_POINTER_BUTTON_LEFT;
                else if (b == 2) button = GK_POINTER_BUTTON_MIDDLE;
                else if (b == 3) button = GK_POINTER_BUTTON_RIGHT;
                else if (b >= 8) button = (int)b - 5; /* 8,9 -> 3,4 */
                if (button >= 0 && button < GK_POINTER_BUTTON_COUNT) {
                    int action = event.type == ButtonPress ? GK_POINTER_PRESSED : GK_POINTER_RELEASED;
                    native->buttons[button] = (unsigned char)action;
                    gkGoPointerButtonEvent(native->handle, button, action, mods);
                }
            }
        } else if (event.type == MotionNotify) {
            if (native->cursorMode == GK_CURSOR_DISABLED) {
                if (native->haveLast) {
                    native->virtualX += event.xmotion.x - native->lastX;
                    native->virtualY += event.xmotion.y - native->lastY;
                }
                native->lastX = event.xmotion.x;
                native->lastY = event.xmotion.y;
                native->haveLast = 1;
                /* Recentre once the pointer drifts far enough that it might reach the
                   window edge, where motion would stop. */
                if (native->width > 0 && native->height > 0) {
                    int cx = native->width / 2, cy = native->height / 2;
                    if (abs(event.xmotion.x - cx) > native->width / 4 ||
                        abs(event.xmotion.y - cy) > native->height / 4) {
                        gkX11WarpToCentre(native);
                    }
                }
            }
        }
    }
}

void gkWindowSetHandle(void *pointer, uintptr_t handle) {
    if (pointer) ((GKWindowNative *)pointer)->handle = handle;
}

int gkWindowGetPointerButton(void *pointer, int button) {
    if (!pointer || button < 0 || button >= GK_POINTER_BUTTON_COUNT) return GK_POINTER_RELEASED;
    return ((GKWindowNative *)pointer)->buttons[button];
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

int gkWindowShouldClose(void *pointer) {
    return pointer ? ((GKWindowNative *)pointer)->shouldClose : 1;
}

void gkWindowSetShouldClose(void *pointer, int close) {
    if (pointer) ((GKWindowNative *)pointer)->shouldClose = close != 0;
}

void gkWindowSize(void *pointer, int *width, int *height) {
    GKWindowNative *native = pointer;
    XWindowAttributes attributes;
    XGetWindowAttributes(native->display, native->window, &attributes);
    if (width) *width = attributes.width;
    if (height) *height = attributes.height;
}

void gkWindowFramebufferSize(void *pointer, int *width, int *height) {
    gkWindowSize(pointer, width, height);
}

void gkWindowSetTitle(void *pointer, const char *title) {
    GKWindowNative *native = pointer;
    XStoreName(native->display, native->window, title ? title : "");
    XFlush(native->display);
}

uintptr_t gkWindowNativeHandle(void *pointer) {
    return pointer ? (uintptr_t)((GKWindowNative *)pointer)->window : 0;
}

void *gkWindowNativeDisplay(void *pointer) {
    return pointer ? ((GKWindowNative *)pointer)->display : NULL;
}

void gkWindowCursorPosition(void *pointer, double *x, double *y) {
    GKWindowNative *native = pointer;
    if (native->cursorMode == GK_CURSOR_DISABLED) {
        /* The pointer is grabbed and recentred; report accumulated motion instead. */
        if (x) *x = native->virtualX;
        if (y) *y = native->virtualY;
        return;
    }
    Window root, child;
    int rootX, rootY, windowX = 0, windowY = 0;
    unsigned int mask;
    XQueryPointer(native->display, native->window, &root, &child,
                  &rootX, &rootY, &windowX, &windowY, &mask);
    if (x) *x = (double)windowX;
    if (y) *y = (double)windowY;
}

int gkWindowGetKey(void *pointer, int key) {
    if (!pointer || key < 0 || key >= GK_KEY_COUNT) return GK_KEY_RELEASED;
    return ((GKWindowNative *)pointer)->keys[key];
}
