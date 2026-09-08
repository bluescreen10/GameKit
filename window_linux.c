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
    int shouldClose;
    unsigned char keys[GK_KEY_COUNT];
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
    XDestroyWindow(native->display, native->window);
    if (native->colormap) XFreeColormap(native->display, native->colormap);
    XCloseDisplay(native->display);
    free(native);
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
        } else if (event.type == KeyPress || event.type == KeyRelease) {
            int key = gkX11Key(XLookupKeysym(&event.xkey, 0));
            if (key >= 0 && key < GK_KEY_COUNT) {
                if (event.type == KeyRelease) native->keys[key] = GK_KEY_RELEASED;
                else native->keys[key] = native->keys[key] == GK_KEY_RELEASED
                    ? GK_KEY_PRESSED : GK_KEY_REPEAT;
            }
        }
    }
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
