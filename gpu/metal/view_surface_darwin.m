//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <QuartzCore/CAMetalLayer.h>
#include "view_surface.h"

uintptr_t mbCocoaViewMetalLayer(uintptr_t pointer) {
    NSView *view = (NSView *)pointer;
    if (!view) return 0;
    if (![view.layer isKindOfClass:CAMetalLayer.class]) {
        view.wantsLayer = YES;
        view.layer = [CAMetalLayer layer];
    }
    CAMetalLayer *layer = (CAMetalLayer *)view.layer;
    CGFloat scale = view.window ? view.window.backingScaleFactor : 1;
    layer.contentsScale = scale;
    layer.drawableSize = CGSizeMake(view.bounds.size.width * scale,
                                    view.bounds.size.height * scale);
    return (uintptr_t)layer;
}

uintptr_t mbCocoaWindowMetalLayer(uintptr_t pointer) {
    NSWindow *window = (NSWindow *)pointer;
    return window ? mbCocoaViewMetalLayer((uintptr_t)window.contentView) : 0;
}
