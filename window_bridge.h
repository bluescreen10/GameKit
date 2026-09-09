#ifndef GAMEKIT_WINDOW_BRIDGE_H
#define GAMEKIT_WINDOW_BRIDGE_H

#include <stddef.h>
#include <stdint.h>

enum {
    GK_WINDOW_RESIZABLE = 1u << 0,
    GK_WINDOW_HIDDEN = 1u << 1,
    GK_WINDOW_BORDERLESS = 1u << 2,
    GK_WINDOW_MAXIMIZED = 1u << 3,
    GK_WINDOW_ALWAYS_ON_TOP = 1u << 4,
    GK_WINDOW_TRANSPARENT = 1u << 5,
};

enum {
    GK_KEY_RELEASED = 0,
    GK_KEY_PRESSED = 1,
    GK_KEY_REPEAT = 2,
    GK_KEY_COUNT = 349,
};

/* Pointing-device buttons and their state. Numbering matches GLFW. */
enum {
    GK_POINTER_RELEASED = 0,
    GK_POINTER_PRESSED = 1,
    GK_POINTER_BUTTON_LEFT = 0,
    GK_POINTER_BUTTON_RIGHT = 1,
    GK_POINTER_BUTTON_MIDDLE = 2,
    GK_POINTER_BUTTON_COUNT = 8,
};

/* Modifier bits reported alongside key and button events. Matches GLFW. */
enum {
    GK_MOD_SHIFT = 1u << 0,
    GK_MOD_CONTROL = 1u << 1,
    GK_MOD_ALT = 1u << 2,
    GK_MOD_SUPER = 1u << 3,
    GK_MOD_CAPS_LOCK = 1u << 4,
    GK_MOD_NUM_LOCK = 1u << 5,
};

/* Cursor behaviour over a window. */
enum {
    GK_CURSOR_NORMAL = 0,
    GK_CURSOR_HIDDEN = 1,
    GK_CURSOR_DISABLED = 2,
};

void *gkWindowCreate(const char *title, int width, int height, uint32_t flags,
                     char *error, size_t errorSize);
void gkWindowDestroy(void *window);
void gkWindowPoll(void *window);
int gkWindowShouldClose(void *window);
void gkWindowSetShouldClose(void *window, int close);
void gkWindowSize(void *window, int *width, int *height);
void gkWindowFramebufferSize(void *window, int *width, int *height);
void gkWindowSetTitle(void *window, const char *title);
uintptr_t gkWindowNativeHandle(void *window);
void *gkWindowNativeDisplay(void *window);
void gkWindowCursorPosition(void *window, double *x, double *y);
/* Pointing device. */
void gkWindowGetScroll(void *window, double *x, double *y);
void gkWindowSetCursorMode(void *window, int mode);
int gkWindowGetCursorMode(void *window);

/*
 * Associate a window with its Go counterpart. The value is a runtime/cgo Handle, which
 * is how a Go value is referred to from C — C may not hold a Go pointer directly, and a
 * handle is an opaque integer the Go side can resolve back. Zero means "not set", which
 * cgo.Handle never produces.
 */
void gkWindowSetHandle(void *window, uintptr_t handle);

/*
 * Event dispatch.
 *
 * Rather than storing C function pointers per window, the platform layers call these
 * unconditionally and Go decides whether a callback is registered. That keeps the
 * platform code free of registration bookkeeping, and the cost — one cgo transition
 * per input event — is irrelevant at the handful of events a frame produces.
 *
 * Implemented in Go; see callbacks.go.
 */
extern void gkGoKeyEvent(uintptr_t handle, int key, int scancode, int action, int mods);
extern void gkGoCharEvent(uintptr_t handle, unsigned int codepoint);
extern void gkGoScrollEvent(uintptr_t handle, double x, double y);
extern void gkGoPointerButtonEvent(uintptr_t handle, int button, int action, int mods);

#endif
