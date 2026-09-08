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
int gkWindowGetKey(void *window, int key);

#endif
