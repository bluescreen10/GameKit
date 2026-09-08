//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#include "window_bridge.h"
#include <stdlib.h>
#include <string.h>
#include <pthread.h>

typedef struct GKWindowNative GKWindowNative;

@interface GKWindowDelegate : NSObject <NSWindowDelegate>
@property(assign) GKWindowNative *owner;
@end

struct GKWindowNative {
    NSWindow *window;
    GKWindowDelegate *delegate;
    int shouldClose;
    unsigned char keys[GK_KEY_COUNT];
};

@implementation GKWindowDelegate
- (BOOL)windowShouldClose:(NSWindow *)sender {
    (void)sender;
    self.owner->shouldClose = 1;
    return NO;
}
@end

static void gkSetError(char *error, size_t size, const char *message) {
    if (error && size) {
        strncpy(error, message, size - 1);
        error[size - 1] = '\0';
    }
}

static int gkMacKey(unsigned short code) {
    switch (code) {
        case 0: return 65; case 1: return 83; case 2: return 68; case 3: return 70;
        case 4: return 72; case 5: return 71; case 6: return 90; case 7: return 88;
        case 8: return 67; case 9: return 86; case 11: return 66; case 12: return 81;
        case 13: return 87; case 14: return 69; case 15: return 82; case 16: return 89;
        case 17: return 84; case 18: return 49; case 19: return 50; case 20: return 51;
        case 21: return 52; case 22: return 54; case 23: return 53; case 24: return 61;
        case 25: return 57; case 26: return 55; case 27: return 45; case 28: return 56;
        case 29: return 48; case 30: return 93; case 31: return 79; case 32: return 85;
        case 33: return 91; case 34: return 73; case 35: return 80; case 36: return 257;
        case 37: return 76; case 38: return 74; case 39: return 39; case 40: return 75;
        case 41: return 59; case 42: return 92; case 43: return 44; case 44: return 47;
        case 45: return 78; case 46: return 77; case 47: return 46; case 48: return 258;
        case 49: return 32; case 50: return 96; case 51: return 259; case 53: return 256;
        case 54: return 347; case 55: return 343; case 56: return 340; case 57: return 280;
        case 58: return 342; case 59: return 341; case 60: return 344; case 61: return 346;
        case 62: return 345; case 64: return 306; case 65: return 330; case 67: return 332;
        case 69: return 334; case 71: return 282; case 75: return 331; case 76: return 335;
        case 78: return 333; case 79: return 307; case 80: return 308; case 81: return 336;
        case 82: return 320; case 83: return 321; case 84: return 322; case 85: return 323;
        case 86: return 324; case 87: return 325; case 88: return 326; case 89: return 327;
        case 90: return 309; case 91: return 328; case 92: return 329; case 96: return 294;
        case 97: return 295; case 98: return 296; case 99: return 292; case 100: return 297;
        case 101: return 298; case 103: return 300; case 105: return 302; case 106: return 305;
        case 107: return 303; case 109: return 299; case 111: return 301; case 113: return 304;
        case 114: return 260; case 115: return 268; case 116: return 266; case 117: return 261;
        case 118: return 293; case 119: return 269; case 120: return 291; case 121: return 267;
        case 122: return 290; case 123: return 263; case 124: return 262; case 125: return 264;
        case 126: return 265; default: return -1;
    }
}

static NSEventModifierFlags gkMacModifierMask(unsigned short code) {
    switch (code) {
        case 54: case 55: return NSEventModifierFlagCommand;
        case 56: case 60: return NSEventModifierFlagShift;
        case 57: return NSEventModifierFlagCapsLock;
        case 58: case 61: return NSEventModifierFlagOption;
        case 59: case 62: return NSEventModifierFlagControl;
        default: return 0;
    }
}

static void gkHandleKeyEvent(GKWindowNative *native, NSEvent *event) {
    int key = gkMacKey([event keyCode]);
    if (!native || key < 0 || key >= GK_KEY_COUNT) return;
    switch ([event type]) {
        case NSEventTypeKeyDown:
            native->keys[key] = [event isARepeat] ? GK_KEY_REPEAT : GK_KEY_PRESSED;
            break;
        case NSEventTypeKeyUp:
            native->keys[key] = GK_KEY_RELEASED;
            break;
        case NSEventTypeFlagsChanged: {
            NSEventModifierFlags mask = gkMacModifierMask([event keyCode]);
            native->keys[key] = ([event modifierFlags] & mask) ? GK_KEY_PRESSED : GK_KEY_RELEASED;
            break;
        }
        default: break;
    }
}

void *gkWindowCreate(const char *title, int width, int height, uint32_t flags,
                     char *error, size_t errorSize) {
    @autoreleasepool {
        if (!pthread_main_np()) {
            gkSetError(error, errorSize, "Cocoa windows must be created on the main thread");
            return NULL;
        }
        NSApplication *app = [NSApplication sharedApplication];
        [app setActivationPolicy:NSApplicationActivationPolicyRegular];
        [app finishLaunching];

        NSWindowStyleMask style = NSWindowStyleMaskBorderless;
        if (!(flags & GK_WINDOW_BORDERLESS)) {
            style = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                    NSWindowStyleMaskMiniaturizable;
            if (flags & GK_WINDOW_RESIZABLE) style |= NSWindowStyleMaskResizable;
        }

        NSRect rect = NSMakeRect(0, 0, width, height);
        NSWindow *window = [[NSWindow alloc] initWithContentRect:rect
            styleMask:style backing:NSBackingStoreBuffered defer:NO];
        if (!window) {
            gkSetError(error, errorSize, "Cocoa could not create NSWindow");
            return NULL;
        }

        GKWindowNative *native = calloc(1, sizeof(*native));
        if (!native) {
            [window release];
            gkSetError(error, errorSize, "out of memory");
            return NULL;
        }
        native->window = window;
        native->delegate = [[GKWindowDelegate alloc] init];
        native->delegate.owner = native;
        [window setDelegate:native->delegate];
        [window setReleasedWhenClosed:NO];
        [window setTitle:[NSString stringWithUTF8String:title ? title : ""]];
        [window center];

        if (flags & GK_WINDOW_ALWAYS_ON_TOP) [window setLevel:NSFloatingWindowLevel];
        if (flags & GK_WINDOW_TRANSPARENT) {
            [window setOpaque:NO];
            [window setBackgroundColor:[NSColor clearColor]];
        }
        if (!(flags & GK_WINDOW_HIDDEN)) {
            [window makeKeyAndOrderFront:nil];
            [app activateIgnoringOtherApps:YES];
        }
        if (flags & GK_WINDOW_MAXIMIZED) [window zoom:nil];
        return native;
    }
}

void gkWindowDestroy(void *pointer) {
    if (!pointer) return;
    @autoreleasepool {
        GKWindowNative *native = pointer;
        [native->window setDelegate:nil];
        [native->window orderOut:nil];
        [native->window close];
        [native->delegate release];
        [native->window release];
        free(native);
    }
}

void gkWindowPoll(void *pointer) {
    (void)pointer;
    @autoreleasepool {
        NSEvent *event;
        while ((event = [NSApp nextEventMatchingMask:NSEventMaskAny
            untilDate:[NSDate distantPast] inMode:NSDefaultRunLoopMode dequeue:YES])) {
            GKWindowDelegate *delegate = (GKWindowDelegate *)[[event window] delegate];
            if ([delegate isKindOfClass:[GKWindowDelegate class]]) {
                gkHandleKeyEvent(delegate.owner, event);
            }
            [NSApp sendEvent:event];
        }
        [NSApp updateWindows];
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
    NSRect rect = [native->window contentRectForFrameRect:[native->window frame]];
    if (width) *width = (int)rect.size.width;
    if (height) *height = (int)rect.size.height;
}

void gkWindowFramebufferSize(void *pointer, int *width, int *height) {
    GKWindowNative *native = pointer;
    NSRect logical = [[native->window contentView] bounds];
    NSRect pixels = [[native->window contentView] convertRectToBacking:logical];
    if (width) *width = (int)pixels.size.width;
    if (height) *height = (int)pixels.size.height;
}

void gkWindowSetTitle(void *pointer, const char *title) {
    GKWindowNative *native = pointer;
    [native->window setTitle:[NSString stringWithUTF8String:title ? title : ""]];
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
    NSView *content = [native->window contentView];
    NSPoint point = [content convertPoint:[native->window mouseLocationOutsideOfEventStream]
                                 fromView:nil];
    NSRect bounds = [content bounds];
    if (x) *x = point.x;
    if (y) *y = bounds.size.height - point.y;
}

int gkWindowGetKey(void *pointer, int key) {
    if (!pointer || key < 0 || key >= GK_KEY_COUNT) return GK_KEY_RELEASED;
    return ((GKWindowNative *)pointer)->keys[key];
}
