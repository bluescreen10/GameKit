//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include "widget_bridge.h"
#include <math.h>
#include <stdlib.h>
#include <string.h>

@interface GKUILayoutView : NSView
@end

@implementation GKUILayoutView
- (BOOL)isFlipped { return YES; }
@end

@interface GKUITextAreaView : NSScrollView <NSTextViewDelegate>
@property uintptr_t handle;
@property(retain) NSTextView *editor;
@property(copy) NSString *placeholder;
@end

@implementation GKUITextAreaView
- (instancetype)initWithText:(NSString *)text placeholder:(NSString *)placeholder
                     readOnly:(BOOL)readOnly wrap:(BOOL)wrap {
    self = [super initWithFrame:NSMakeRect(0, 0, 200, 96)];
    if (!self) return nil;
    self.hasVerticalScroller = YES;
    self.hasHorizontalScroller = !wrap;
    self.borderType = NSBezelBorder;
    self.editor = [[[NSTextView alloc] initWithFrame:NSZeroRect] autorelease];
    self.editor.string = text ?: @"";
    self.editor.editable = !readOnly;
    self.editor.delegate = self;
    self.editor.verticallyResizable = YES;
    self.editor.horizontallyResizable = !wrap;
    self.editor.textContainer.widthTracksTextView = wrap;
    self.documentView = self.editor;
    self.placeholder = placeholder ?: @"";
    return self;
}
- (void)dealloc {
    self.editor.delegate = nil;
    [_editor release];
    [_placeholder release];
    [super dealloc];
}
- (void)textDidChange:(NSNotification *)notification {
    (void)notification;
    if (self.handle) gkUIGoEvent(self.handle, GK_UI_EVENT_TEXT_CHANGE);
}
@end

@interface GKUIRadioGroupView : NSView
@property uintptr_t handle;
@property(retain) NSMutableArray<NSButton *> *buttons;
@property(nonatomic) NSInteger selectedIndex;
- (void)addItem:(NSString *)title;
@end

@implementation GKUIRadioGroupView
- (instancetype)init {
    self = [super initWithFrame:NSMakeRect(0, 0, 160, 24)];
    if (self) self.buttons = [NSMutableArray array];
    return self;
}
- (void)dealloc {
    [_buttons release];
    [super dealloc];
}
- (BOOL)isFlipped { return YES; }
- (void)addItem:(NSString *)title {
    NSButton *button = [NSButton radioButtonWithTitle:title ?: @"" target:self action:@selector(select:)];
    button.tag = self.buttons.count;
    [self.buttons addObject:button];
    [self addSubview:button];
    [self setNeedsLayout:YES];
}
- (void)layout {
    [super layout];
    CGFloat y = 0;
    for (NSButton *button in self.buttons) {
        button.frame = NSMakeRect(0, y, self.bounds.size.width, 22);
        y += 24;
    }
}
- (void)setSelectedIndex:(NSInteger)selectedIndex {
    _selectedIndex = selectedIndex;
    for (NSInteger i = 0; i < (NSInteger)self.buttons.count; i++) {
        self.buttons[i].state = i == selectedIndex ? NSControlStateValueOn : NSControlStateValueOff;
    }
}
- (void)select:(NSButton *)sender {
    self.selectedIndex = sender.tag;
    if (self.handle) gkUIGoEvent(self.handle, GK_UI_EVENT_SELECTION_CHANGE);
}
@end

@interface GKUILinkButton : NSButton
@property(copy) NSString *linkURL;
@end

@implementation GKUILinkButton
- (void)dealloc {
    [_linkURL release];
    [super dealloc];
}
@end

@interface GKUIActivityIndicator : NSProgressIndicator
@property BOOL running;
@end

@implementation GKUIActivityIndicator
@end

@interface GKUIImageView : NSView
@property(retain) NSImage *image;
@property NSInteger scaling;
@end

@implementation GKUIImageView
- (void)dealloc {
    [_image release];
    [super dealloc];
}
- (BOOL)isFlipped { return YES; }
- (NSSize)fittingSize { return self.image ? self.image.size : NSZeroSize; }
- (void)drawRect:(NSRect)dirtyRect {
    (void)dirtyRect;
    if (!self.image) return;
    NSSize source = self.image.size;
    NSRect destination = self.bounds;
    if (self.scaling != 2 && source.width > 0 && source.height > 0) {
        CGFloat sx = self.bounds.size.width / source.width;
        CGFloat sy = self.bounds.size.height / source.height;
        CGFloat scale = self.scaling == 1 ? MAX(sx, sy) : MIN(sx, sy);
        destination.size = NSMakeSize(source.width * scale, source.height * scale);
        destination.origin.x = (self.bounds.size.width - destination.size.width) / 2;
        destination.origin.y = (self.bounds.size.height - destination.size.height) / 2;
    }
    [NSGraphicsContext saveGraphicsState];
    NSRectClip(self.bounds);
    [self.image drawInRect:destination fromRect:NSZeroRect
                 operation:NSCompositingOperationSourceOver fraction:1
           respectFlipped:YES hints:nil];
    [NSGraphicsContext restoreGraphicsState];
}
@end

@interface GKUITabsView : NSTabView <NSTabViewDelegate>
@property uintptr_t handle;
@end

@implementation GKUITabsView
- (instancetype)init {
    self = [super initWithFrame:NSMakeRect(0, 0, 320, 240)];
    if (self) self.delegate = self;
    return self;
}
- (void)dealloc {
    self.delegate = nil;
    [super dealloc];
}
- (void)tabView:(NSTabView *)tabView didSelectTabViewItem:(NSTabViewItem *)item {
    (void)tabView;
    (void)item;
    if (self.handle) gkUIGoEvent(self.handle, GK_UI_EVENT_TAB_CHANGE);
}
@end

@protocol GKUIContainer <NSObject>
- (NSView *)gkContentView;
@end

@interface GKUIScrollView : NSScrollView <GKUIContainer>
@property(retain) GKUILayoutView *bodyView;
@end

@implementation GKUIScrollView
- (instancetype)initWithAxes:(NSInteger)axes {
    self = [super initWithFrame:NSMakeRect(0, 0, 320, 240)];
    if (!self) return nil;
    self.hasVerticalScroller = axes == 0 || axes == 2;
    self.hasHorizontalScroller = axes == 1 || axes == 2;
    self.autohidesScrollers = YES;
    self.borderType = NSBezelBorder;
    self.drawsBackground = YES;
    self.bodyView = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    self.documentView = self.bodyView;
    return self;
}
- (void)dealloc {
    [_bodyView release];
    [super dealloc];
}
- (NSView *)gkContentView { return self.bodyView; }
@end

@interface GKUIFloatingPanelView : NSView <GKUIContainer>
@property uintptr_t handle;
@property BOOL movable;
@property(retain) NSTextField *titleLabel;
@property(retain) GKUILayoutView *bodyView;
@property NSPoint dragStart;
@property NSRect dragFrame;
- (instancetype)initWithTitle:(NSString *)title movable:(BOOL)movable;
@end

@implementation GKUIFloatingPanelView
- (instancetype)initWithTitle:(NSString *)title movable:(BOOL)movable {
    self = [super initWithFrame:NSMakeRect(0, 0, 240, 160)];
    if (!self) return nil;
    self.movable = movable;
    self.wantsLayer = YES;
    self.layer.cornerRadius = 8;
    self.layer.masksToBounds = YES;
    self.titleLabel = [NSTextField labelWithString:title ?: @""];
    self.titleLabel.font = [NSFont boldSystemFontOfSize:NSFont.systemFontSize];
    self.bodyView = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    [self addSubview:self.titleLabel];
    [self addSubview:self.bodyView];
    return self;
}
- (void)dealloc {
    [_titleLabel release];
    [_bodyView release];
    [super dealloc];
}
- (BOOL)isFlipped { return YES; }
- (NSView *)gkContentView { return self.bodyView; }
- (void)layout {
    [super layout];
    CGFloat width = self.bounds.size.width;
    CGFloat height = self.bounds.size.height;
    self.titleLabel.frame = NSMakeRect(12, 7, MAX(0, width - 24), 18);
    self.bodyView.frame = NSMakeRect(12, 32, MAX(0, width - 24), MAX(0, height - 44));
}
- (void)drawRect:(NSRect)dirtyRect {
    [[NSColor.windowBackgroundColor colorWithAlphaComponent:0.96] setFill];
    NSRectFill(dirtyRect);
    [NSColor.separatorColor setFill];
    NSRectFill(NSMakeRect(0, 31, self.bounds.size.width, 1));
}
- (NSView *)hitTest:(NSPoint)point {
    NSView *hit = [super hitTest:point];
    if (!hit) return nil;
    NSPoint localPoint = [self convertPoint:point fromView:self.superview];
    if (localPoint.y < 32) return self;
    return hit;
}
- (void)mouseDown:(NSEvent *)event {
    if (!self.movable) return;
    [self.superview addSubview:self positioned:NSWindowAbove relativeTo:nil];
    self.dragStart = event.locationInWindow;
    self.dragFrame = self.frame;
}
- (void)mouseDragged:(NSEvent *)event {
    if (!self.movable || !self.superview) return;
    NSPoint current = event.locationInWindow;
    NSRect frame = self.dragFrame;
    frame.origin.x += current.x - self.dragStart.x;
    frame.origin.y += current.y - self.dragStart.y;
    NSRect bounds = self.superview.bounds;
    frame.origin.x = MAX(0, MIN(frame.origin.x, MAX(0, bounds.size.width - frame.size.width)));
    frame.origin.y = MAX(0, MIN(frame.origin.y, MAX(0, bounds.size.height - frame.size.height)));
    self.frame = frame;
    if (self.handle) gkUIGoEvent(self.handle, GK_UI_EVENT_PANEL_MOVE);
}
@end

@interface GKUISectionView : NSView <GKUIContainer>
@property uintptr_t handle;
@property BOOL expanded;
@property(copy) NSString *sectionTitle;
@property(retain) NSButton *header;
@property(retain) GKUILayoutView *bodyView;
- (instancetype)initWithTitle:(NSString *)title expanded:(BOOL)expanded;
- (void)setSectionExpanded:(BOOL)expanded;
@end

@implementation GKUISectionView
- (instancetype)initWithTitle:(NSString *)title expanded:(BOOL)expanded {
    self = [super initWithFrame:NSMakeRect(0, 0, 160, 28)];
    if (!self) return nil;
    self.sectionTitle = title ?: @"";
    self.header = [NSButton buttonWithTitle:@"" target:self action:@selector(toggle:)];
    self.header.bezelStyle = NSBezelStyleAccessoryBarAction;
    self.header.alignment = NSTextAlignmentLeft;
    self.bodyView = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    [self addSubview:self.header];
    [self addSubview:self.bodyView];
    [self setSectionExpanded:expanded];
    return self;
}
- (void)dealloc {
    [_sectionTitle release];
    [_header release];
    [_bodyView release];
    [super dealloc];
}
- (BOOL)isFlipped { return YES; }
- (NSView *)gkContentView { return self.bodyView; }
- (void)layout {
    [super layout];
    self.header.frame = NSMakeRect(0, 0, self.bounds.size.width, 28);
    self.bodyView.frame = NSMakeRect(0, 28, self.bounds.size.width,
                                     MAX(0, self.bounds.size.height - 28));
}
- (void)updateHeader {
    NSString *marker = self.expanded ? @"▾" : @"▸";
    self.header.title = [NSString stringWithFormat:@"%@  %@", marker, self.sectionTitle ?: @""];
}
- (void)setSectionExpanded:(BOOL)expanded {
    _expanded = expanded;
    self.bodyView.hidden = !expanded;
    [self updateHeader];
}
- (void)toggle:(id)sender {
    (void)sender;
    [self setSectionExpanded:!self.expanded];
    if (self.handle) gkUIGoEvent(self.handle, GK_UI_EVENT_SECTION_CHANGE);
}
@end

@interface GKUIAction : NSObject <NSTextFieldDelegate>
@property uintptr_t handle;
@property int event;
- (void)send:(id)sender;
@end

@implementation GKUIAction
- (void)send:(id)sender {
    (void)sender;
    if (self.handle) gkUIGoEvent(self.handle, self.event);
}
- (void)controlTextDidChange:(NSNotification *)notification {
    (void)notification;
    if (self.handle) gkUIGoEvent(self.handle, GK_UI_EVENT_TEXT_CHANGE);
}
@end

static char gkUIActionKey;

static NSWindow *gkUIWindow(uintptr_t window) {
    return (NSWindow *)window;
}

static NSView *gkUIView(void *control) {
    return (NSView *)control;
}

uintptr_t gkUIControlHandle(void *control) {
    return (uintptr_t)control;
}

static void gkUIAddToWindow(uintptr_t window, NSView *view) {
    [gkUIWindow(window).contentView addSubview:view];
}

static void gkUIInstallAction(NSControl *control, uintptr_t handle, int event) {
    GKUIAction *action = [GKUIAction new];
    action.handle = handle;
    action.event = event;
    objc_setAssociatedObject(control, &gkUIActionKey, action, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    if ([control isKindOfClass:NSTextField.class] && event == GK_UI_EVENT_TEXT_CHANGE) {
        ((NSTextField *)control).delegate = action;
    } else {
        control.target = action;
        control.action = @selector(send:);
    }
    [action release];
}

void *gkUICreateLayout(uintptr_t window) {
    GKUILayoutView *view = [GKUILayoutView new];
    gkUIAddToWindow(window, view);
    return view;
}

void *gkUICreateLabel(uintptr_t window, const char *text) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSTextField *label = [[NSTextField labelWithString:value] retain];
    gkUIAddToWindow(window, label);
    return label;
}

void *gkUICreateButton(uintptr_t window, const char *text, uintptr_t handle) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSButton *button = [[NSButton buttonWithTitle:value target:nil action:nil] retain];
    gkUIInstallAction(button, handle, GK_UI_EVENT_CLICK);
    gkUIAddToWindow(window, button);
    return button;
}

void *gkUICreateTextField(uintptr_t window, uintptr_t handle) {
    NSTextField *field = [[NSTextField alloc] init];
    gkUIInstallAction(field, handle, GK_UI_EVENT_TEXT_CHANGE);
    gkUIAddToWindow(window, field);
    return field;
}

void *gkUICreateTextArea(uintptr_t window, const char *text,
                         const char *placeholder, int readOnly, int wrap,
                         uintptr_t handle) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSString *hint = placeholder ? [NSString stringWithUTF8String:placeholder] : @"";
    GKUITextAreaView *area = [[GKUITextAreaView alloc] initWithText:value placeholder:hint
                                                          readOnly:readOnly != 0 wrap:wrap != 0];
    area.handle = handle;
    gkUIAddToWindow(window, area);
    return area;
}

void *gkUICreateCheckBox(uintptr_t window, const char *text, int checked,
                         uintptr_t handle) {
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSButton *button = [[NSButton checkboxWithTitle:value target:nil action:nil] retain];
    button.state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    gkUIInstallAction(button, handle, GK_UI_EVENT_TOGGLE_CHANGE);
    gkUIAddToWindow(window, button);
    return button;
}

void *gkUICreateRadioGroup(uintptr_t window, uintptr_t handle) {
    GKUIRadioGroupView *group = [GKUIRadioGroupView new];
    group.handle = handle;
    gkUIAddToWindow(window, group);
    return group;
}

void *gkUICreateSelect(uintptr_t window, uintptr_t handle) {
    NSPopUpButton *select = [[NSPopUpButton alloc] initWithFrame:NSZeroRect pullsDown:NO];
    gkUIInstallAction(select, handle, GK_UI_EVENT_SELECTION_CHANGE);
    gkUIAddToWindow(window, select);
    return select;
}

void gkUISelectionAddItem(void *control, const char *text) {
    if (!control) return;
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        [(GKUIRadioGroupView *)view addItem:value];
    } else if ([view isKindOfClass:NSPopUpButton.class]) {
        [(NSPopUpButton *)view addItemWithTitle:value];
    }
}

int gkUISelectionIndex(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        return (int)((GKUIRadioGroupView *)view).selectedIndex;
    }
    if ([view isKindOfClass:NSPopUpButton.class]) {
        return (int)((NSPopUpButton *)view).indexOfSelectedItem;
    }
    return -1;
}

void gkUISetSelectionIndex(void *control, int index) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        ((GKUIRadioGroupView *)view).selectedIndex = index;
    } else if ([view isKindOfClass:NSPopUpButton.class]) {
        [(NSPopUpButton *)view selectItemAtIndex:index];
    }
}

void *gkUICreateSlider(uintptr_t window, double minimum, double maximum,
                       double value, double step, uintptr_t handle) {
    NSSlider *slider = [[NSSlider sliderWithValue:value minValue:minimum
        maxValue:maximum target:nil action:nil] retain];
    if (step > 0) {
        slider.numberOfTickMarks = (NSInteger)floor((maximum - minimum) / step) + 1;
        slider.allowsTickMarkValuesOnly = YES;
    }
    gkUIInstallAction(slider, handle, GK_UI_EVENT_VALUE_CHANGE);
    gkUIAddToWindow(window, slider);
    return slider;
}

void *gkUICreateProgressBar(uintptr_t window, double value) {
    NSProgressIndicator *progress = [[NSProgressIndicator alloc] initWithFrame:NSZeroRect];
    progress.indeterminate = NO;
    progress.minValue = 0;
    progress.maxValue = 1;
    progress.doubleValue = value;
    gkUIAddToWindow(window, progress);
    return progress;
}

void *gkUICreateActivityIndicator(uintptr_t window, int running) {
    GKUIActivityIndicator *activity = [[GKUIActivityIndicator alloc] initWithFrame:NSZeroRect];
    activity.indeterminate = YES;
    activity.style = NSProgressIndicatorStyleSpinning;
    activity.running = running != 0;
    if (running) [activity startAnimation:nil];
    gkUIAddToWindow(window, activity);
    return activity;
}

void *gkUICreateSeparator(uintptr_t window, int direction) {
    NSBox *separator = [[NSBox alloc] initWithFrame:NSZeroRect];
    separator.boxType = NSBoxSeparator;
    (void)direction;
    gkUIAddToWindow(window, separator);
    return separator;
}

void *gkUICreateImage(uintptr_t window, int scaling) {
    GKUIImageView *image = [[GKUIImageView alloc] initWithFrame:NSZeroRect];
    image.scaling = scaling;
    gkUIAddToWindow(window, image);
    gkUIImageSetScaling(image, scaling);
    return image;
}

void gkUIImageSet(void *control, const unsigned char *pixels,
                  int width, int height, int stride) {
    if (!control || !pixels || width <= 0 || height <= 0) return;
    NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc]
        initWithBitmapDataPlanes:NULL pixelsWide:width pixelsHigh:height
        bitsPerSample:8 samplesPerPixel:4 hasAlpha:YES isPlanar:NO
        colorSpaceName:NSCalibratedRGBColorSpace bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
        bytesPerRow:width * 4 bitsPerPixel:32];
    if (!bitmap) return;
    for (int y = 0; y < height; y++) {
        memcpy(bitmap.bitmapData + y * bitmap.bytesPerRow, pixels + y * stride, (size_t)width * 4);
    }
    NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(width, height)];
    [image addRepresentation:bitmap];
    ((GKUIImageView *)gkUIView(control)).image = image;
    [image release];
    [bitmap release];
}

int gkUIImageScaling(void *control) {
    GKUIImageView *view = control ? (GKUIImageView *)gkUIView(control) : nil;
    return view ? (int)view.scaling : 0;
}

void gkUIImageSetScaling(void *control, int scaling) {
    GKUIImageView *view = control ? (GKUIImageView *)gkUIView(control) : nil;
    if (!view) return;
    view.scaling = scaling;
    [view setNeedsDisplay:YES];
}

static void gkUIStyleLink(GKUILinkButton *link) {
    NSString *title = link.title ?: @"";
    NSDictionary *attributes = @{
        NSForegroundColorAttributeName: NSColor.linkColor,
        NSUnderlineStyleAttributeName: @(NSUnderlineStyleSingle),
    };
    link.attributedTitle = [[[NSAttributedString alloc] initWithString:title
        attributes:attributes] autorelease];
    link.bordered = NO;
    link.alignment = NSTextAlignmentLeft;
}

void *gkUICreateLink(uintptr_t window, const char *text,
                     const char *url, uintptr_t handle) {
    NSString *title = text ? [NSString stringWithUTF8String:text] : @"";
    GKUILinkButton *link = [[GKUILinkButton alloc] initWithFrame:NSZeroRect];
    link.title = title;
    link.linkURL = url ? [NSString stringWithUTF8String:url] : @"";
    gkUIStyleLink(link);
    gkUIInstallAction(link, handle, GK_UI_EVENT_CLICK);
    gkUIAddToWindow(window, link);
    return link;
}

void *gkUICreateTabs(uintptr_t window, uintptr_t handle) {
    GKUITabsView *tabs = [GKUITabsView new];
    tabs.handle = handle;
    gkUIAddToWindow(window, tabs);
    return tabs;
}

int gkUITabsAdd(void *tabsPointer, const char *title, void *content) {
    if (!tabsPointer || !content) return 0;
    GKUITabsView *tabs = (GKUITabsView *)gkUIView(tabsPointer);
    NSString *label = title ? [NSString stringWithUTF8String:title] : @"";
    NSTabViewItem *item = [[[NSTabViewItem alloc] initWithIdentifier:nil] autorelease];
    item.label = label;
    GKUILayoutView *page = [[[GKUILayoutView alloc] initWithFrame:NSZeroRect] autorelease];
    item.view = page;
    [tabs addTabViewItem:item];
    [gkUIView(content) removeFromSuperview];
    [page addSubview:gkUIView(content)];
    return 1;
}

int gkUITabsSelected(void *tabsPointer) {
    GKUITabsView *tabs = tabsPointer ? (GKUITabsView *)gkUIView(tabsPointer) : nil;
    return tabs ? (int)[tabs indexOfTabViewItem:tabs.selectedTabViewItem] : -1;
}

void gkUITabsSetSelected(void *tabsPointer, int index) {
    GKUITabsView *tabs = tabsPointer ? (GKUITabsView *)gkUIView(tabsPointer) : nil;
    if (tabs && index >= 0 && index < (int)tabs.numberOfTabViewItems) {
        uintptr_t handle = tabs.handle;
        tabs.handle = 0;
        [tabs selectTabViewItemAtIndex:index];
        tabs.handle = handle;
    }
}

void gkUITabsContentSize(void *tabsPointer, int *width, int *height) {
    GKUITabsView *tabs = tabsPointer ? (GKUITabsView *)gkUIView(tabsPointer) : nil;
    NSSize size = tabs.selectedTabViewItem.view.bounds.size;
    if (width) *width = (int)floor(size.width);
    if (height) *height = (int)floor(size.height);
}

void *gkUICreateScrollView(uintptr_t window, int axes) {
    GKUIScrollView *scroll = [[GKUIScrollView alloc] initWithAxes:axes];
    gkUIAddToWindow(window, scroll);
    return scroll;
}

void gkUIScrollViewportSize(void *pointer, int *width, int *height) {
    GKUIScrollView *scroll = pointer ? (GKUIScrollView *)gkUIView(pointer) : nil;
    NSSize size = scroll ? scroll.contentSize : NSZeroSize;
    if (width) *width = (int)floor(size.width);
    if (height) *height = (int)floor(size.height);
}

void gkUIScrollSetContentSize(void *pointer, int width, int height) {
    GKUIScrollView *scroll = pointer ? (GKUIScrollView *)gkUIView(pointer) : nil;
    if (!scroll) return;
    [scroll.bodyView setFrameSize:NSMakeSize(MAX(0, width), MAX(0, height))];
    [scroll reflectScrolledClipView:scroll.contentView];
}

void gkUIScrollOffset(void *pointer, int *x, int *y) {
    GKUIScrollView *scroll = pointer ? (GKUIScrollView *)gkUIView(pointer) : nil;
    NSPoint offset = scroll ? scroll.documentVisibleRect.origin : NSZeroPoint;
    if (x) *x = (int)floor(offset.x);
    if (y) *y = (int)floor(offset.y);
}

void gkUIScrollSetOffset(void *pointer, int x, int y) {
    GKUIScrollView *scroll = pointer ? (GKUIScrollView *)gkUIView(pointer) : nil;
    if (!scroll) return;
    [scroll.contentView scrollToPoint:NSMakePoint(MAX(0, x), MAX(0, y))];
    [scroll reflectScrolledClipView:scroll.contentView];
}

void *gkUICreateCanvas(uintptr_t window) {
    GKUILayoutView *view = [GKUILayoutView new];
    view.wantsLayer = YES;
    gkUIAddToWindow(window, view);
    return view;
}

void *gkUICreatePanel(uintptr_t window, const char *title, int movable,
                      uintptr_t handle) {
    NSString *value = title ? [NSString stringWithUTF8String:title] : @"";
    GKUIFloatingPanelView *panel = [[GKUIFloatingPanelView alloc]
        initWithTitle:value movable:movable != 0];
    panel.handle = handle;
    gkUIAddToWindow(window, panel);
    return panel;
}

void *gkUICreateSection(uintptr_t window, const char *title, int expanded,
                        uintptr_t handle) {
    NSString *value = title ? [NSString stringWithUTF8String:title] : @"";
    GKUISectionView *section = [[GKUISectionView alloc]
        initWithTitle:value expanded:expanded != 0];
    section.handle = handle;
    gkUIAddToWindow(window, section);
    return section;
}

void gkUIDestroy(void *control) {
    if (!control) return;
    NSView *view = gkUIView(control);
    [view removeFromSuperview];
    [view release];
}

void gkUIClearAction(void *control) {
    if (!control) return;
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:GKUIFloatingPanelView.class]) {
        ((GKUIFloatingPanelView *)view).handle = 0;
    }
    if ([view isKindOfClass:GKUISectionView.class]) {
        GKUISectionView *section = (GKUISectionView *)view;
        section.handle = 0;
        section.header.target = nil;
        section.header.action = nil;
    }
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        GKUITextAreaView *area = (GKUITextAreaView *)view;
        area.handle = 0;
        area.editor.delegate = nil;
    }
    if ([view isKindOfClass:GKUIRadioGroupView.class]) {
        ((GKUIRadioGroupView *)view).handle = 0;
    }
    if ([view isKindOfClass:GKUITabsView.class]) {
        ((GKUITabsView *)view).handle = 0;
    }
    GKUIAction *action = objc_getAssociatedObject(view, &gkUIActionKey);
    action.handle = 0;
    if ([view isKindOfClass:NSTextField.class]) {
        ((NSTextField *)view).delegate = nil;
    }
    if ([view isKindOfClass:NSControl.class]) {
        ((NSControl *)view).target = nil;
        ((NSControl *)view).action = nil;
    }
    objc_setAssociatedObject(view, &gkUIActionKey, nil, OBJC_ASSOCIATION_ASSIGN);
}

void gkUIAttachToWindow(uintptr_t window, void *control) {
    if (!window || !control) return;
    gkUIAddToWindow(window, gkUIView(control));
}

void gkUISetParent(void *control, void *parent) {
    if (!control || !parent) return;
    NSView *parentView = gkUIView(parent);
    NSView *contentView = parentView;
    if ([parentView respondsToSelector:@selector(gkContentView)]) {
        contentView = [(id<GKUIContainer>)parentView gkContentView];
    }
    [contentView addSubview:gkUIView(control)];
}

void gkUIBringToFront(void *control) {
    if (!control) return;
    NSView *view = gkUIView(control);
    if (view.superview) {
        [view.superview addSubview:view positioned:NSWindowAbove relativeTo:nil];
    }
}

void gkUISetFrame(void *control, int x, int y, int width, int height) {
    if (!control) return;
    NSView *view = gkUIView(control);
    CGFloat nativeY = y;
    if (view.superview && !view.superview.isFlipped) {
        nativeY = view.superview.bounds.size.height - y - MAX(0, height);
    }
    view.frame = NSMakeRect(x, nativeY, MAX(0, width), MAX(0, height));
}

void gkUISetVisible(void *control, int visible) {
    if (control) gkUIView(control).hidden = !visible;
}

void gkUISetEnabled(void *control, int enabled) {
    if (!control) return;
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:NSControl.class]) ((NSControl *)view).enabled = enabled;
    for (NSView *child in view.subviews) {
        gkUISetEnabled(child, enabled);
    }
}

void gkUIPreferredSize(void *control, int *width, int *height) {
    NSSize size = NSZeroSize;
    if (control) {
        NSView *view = gkUIView(control);
        size = view.fittingSize;
        if ([view isKindOfClass:GKUITextAreaView.class]) size = NSMakeSize(200, 96);
        if ([view isKindOfClass:GKUIRadioGroupView.class]) {
            GKUIRadioGroupView *group = (GKUIRadioGroupView *)view;
            CGFloat maxWidth = 120;
            for (NSButton *button in group.buttons) maxWidth = MAX(maxWidth, button.fittingSize.width);
            size = NSMakeSize(maxWidth, MAX(24, group.buttons.count * 24));
        }
        if ([view isKindOfClass:GKUITabsView.class] ||
            [view isKindOfClass:GKUIScrollView.class]) size = NSMakeSize(320, 240);
    }
    if (width) *width = (int)ceil(size.width);
    if (height) *height = (int)ceil(size.height);
}

void gkUIFramebufferSize(void *control, int *width, int *height) {
    NSView *view = gkUIView(control);
    CGFloat scale = view.window ? view.window.backingScaleFactor : 1;
    if (width) *width = (int)ceil(view.bounds.size.width * scale);
    if (height) *height = (int)ceil(view.bounds.size.height * scale);
}

char *gkUIText(void *control) {
    if (!control) return strdup("");
    NSView *view = gkUIView(control);
    NSString *value = @"";
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        value = ((GKUITextAreaView *)view).editor.string;
    } else if ([view isKindOfClass:GKUIFloatingPanelView.class]) {
        value = ((GKUIFloatingPanelView *)view).titleLabel.stringValue;
    } else if ([view isKindOfClass:GKUISectionView.class]) {
        value = ((GKUISectionView *)view).sectionTitle;
    } else if ([view isKindOfClass:NSButton.class]) {
        value = ((NSButton *)view).title;
    } else if ([view respondsToSelector:@selector(stringValue)]) {
        value = [(id)view stringValue];
    }
    return strdup(value.UTF8String ?: "");
}

void gkUISetText(void *control, const char *text) {
    if (!control) return;
    NSView *view = gkUIView(control);
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        ((GKUITextAreaView *)view).editor.string = value;
    } else if ([view isKindOfClass:GKUIFloatingPanelView.class]) {
        ((GKUIFloatingPanelView *)view).titleLabel.stringValue = value;
    } else if ([view isKindOfClass:GKUISectionView.class]) {
        GKUISectionView *section = (GKUISectionView *)view;
        section.sectionTitle = value;
        [section updateHeader];
    } else if ([view isKindOfClass:GKUILinkButton.class]) {
        ((GKUILinkButton *)view).title = value;
        gkUIStyleLink((GKUILinkButton *)view);
    } else if ([view isKindOfClass:NSButton.class]) {
        ((NSButton *)view).title = value;
    } else if ([view respondsToSelector:@selector(setStringValue:)]) {
        [(id)view setStringValue:value];
    }
}

void gkUISetPlaceholder(void *control, const char *text) {
    if (!control) return;
    NSString *value = text ? [NSString stringWithUTF8String:text] : @"";
    NSView *view = gkUIView(control);
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        ((GKUITextAreaView *)view).placeholder = value;
    } else if ([view isKindOfClass:NSTextField.class]) {
        ((NSTextField *)view).placeholderString = value;
    }
}

char *gkUIPlaceholder(void *control) {
    if (!control) return strdup("");
    NSView *view = gkUIView(control);
    NSString *value = @"";
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        value = ((GKUITextAreaView *)view).placeholder ?: @"";
    } else if ([view isKindOfClass:NSTextField.class]) {
        value = ((NSTextField *)view).placeholderString ?: @"";
    }
    return strdup(value.UTF8String ?: "");
}

int gkUIChecked(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:NSButton.class] && ((NSButton *)view).state == NSControlStateValueOn;
}

void gkUISetChecked(void *control, int checked) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:NSButton.class]) {
        ((NSButton *)view).state = checked ? NSControlStateValueOn : NSControlStateValueOff;
    }
}

int gkUIReadOnly(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:GKUITextAreaView.class] && !((GKUITextAreaView *)view).editor.editable;
}

void gkUISetReadOnly(void *control, int readOnly) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUITextAreaView.class]) {
        ((GKUITextAreaView *)view).editor.editable = readOnly == 0;
    }
}

double gkUISliderValue(void *control) {
    if (!control || ![gkUIView(control) isKindOfClass:NSSlider.class]) return 0;
    return ((NSSlider *)gkUIView(control)).doubleValue;
}

void gkUISetSliderValue(void *control, double value) {
    if (control && [gkUIView(control) isKindOfClass:NSSlider.class]) {
        ((NSSlider *)gkUIView(control)).doubleValue = value;
    }
}

double gkUIProgressValue(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:NSProgressIndicator.class] ?
        ((NSProgressIndicator *)view).doubleValue : 0;
}

void gkUISetProgressValue(void *control, double value) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:NSProgressIndicator.class]) {
        ((NSProgressIndicator *)view).doubleValue = MAX(0, MIN(value, 1));
    }
}

int gkUIActivityRunning(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    return [view isKindOfClass:GKUIActivityIndicator.class] &&
        ((GKUIActivityIndicator *)view).running;
}

void gkUISetActivityRunning(void *control, int running) {
    NSView *view = control ? gkUIView(control) : nil;
    if (![view isKindOfClass:GKUIActivityIndicator.class]) return;
    GKUIActivityIndicator *activity = (GKUIActivityIndicator *)view;
    activity.running = running != 0;
    if (running) [activity startAnimation:nil];
    else [activity stopAnimation:nil];
}

char *gkUILinkURL(void *control) {
    NSView *view = control ? gkUIView(control) : nil;
    NSString *url = [view isKindOfClass:GKUILinkButton.class] ?
        ((GKUILinkButton *)view).linkURL : @"";
    return strdup(url.UTF8String ?: "");
}

void gkUISetLinkURL(void *control, const char *url) {
    NSView *view = control ? gkUIView(control) : nil;
    if ([view isKindOfClass:GKUILinkButton.class]) {
        ((GKUILinkButton *)view).linkURL = url ? [NSString stringWithUTF8String:url] : @"";
    }
}

void gkUIPanelSetMovable(void *control, int movable) {
    if (control && [gkUIView(control) isKindOfClass:GKUIFloatingPanelView.class]) {
        ((GKUIFloatingPanelView *)gkUIView(control)).movable = movable != 0;
    }
}

void gkUIPanelPosition(void *control, int *x, int *y) {
    NSView *view = control ? gkUIView(control) : nil;
    NSPoint position = view ? view.frame.origin : NSZeroPoint;
    if (view.superview && !view.superview.isFlipped) {
        position.y = view.superview.bounds.size.height - NSMaxY(view.frame);
    }
    if (x) *x = (int)round(position.x);
    if (y) *y = (int)round(position.y);
}

int gkUISectionExpanded(void *control) {
    if (!control || ![gkUIView(control) isKindOfClass:GKUISectionView.class]) return 0;
    return ((GKUISectionView *)gkUIView(control)).expanded;
}

void gkUISectionSetExpanded(void *control, int expanded) {
    if (control && [gkUIView(control) isKindOfClass:GKUISectionView.class]) {
        [(GKUISectionView *)gkUIView(control) setSectionExpanded:expanded != 0];
    }
}

static NSString *gkUIString(const char *value) {
    return value && value[0] ? [NSString stringWithUTF8String:value] : nil;
}

static NSArray<NSString *> *gkUIExtensions(const char *value) {
    NSString *list = gkUIString(value);
    if (!list) return nil;
    return [list componentsSeparatedByString:@","];
}

static void gkUIConfigurePanel(NSSavePanel *panel, const char *title,
                               const char *directory, const char *filename,
                               const char *extensions) {
    NSString *panelTitle = gkUIString(title);
    if (panelTitle) panel.title = panelTitle;
    NSString *initialDirectory = gkUIString(directory);
    if (initialDirectory) {
        panel.directoryURL = [NSURL fileURLWithPath:initialDirectory isDirectory:YES];
    }
    NSString *initialFilename = gkUIString(filename);
    if (initialFilename) panel.nameFieldStringValue = initialFilename;
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
    panel.allowedFileTypes = gkUIExtensions(extensions);
#pragma clang diagnostic pop
}

static char *gkUIPaths(NSArray<NSURL *> *urls, size_t *resultSize) {
    size_t size = 0;
    for (NSURL *url in urls) {
        size += strlen(url.path.fileSystemRepresentation) + 1;
    }
    if (resultSize) *resultSize = size;
    if (!size) return NULL;

    char *result = malloc(size);
    if (!result) return NULL;
    char *next = result;
    for (NSURL *url in urls) {
        const char *path = url.path.fileSystemRepresentation;
        size_t length = strlen(path) + 1;
        memcpy(next, path, length);
        next += length;
    }
    return result;
}

char *gkUIOpenPanel(uintptr_t window, const char *title,
                    const char *directory, const char *filename,
                    const char *extensions, int multiple, int directories,
                    size_t *resultSize, int *cancelled) {
    (void)window;
    NSOpenPanel *panel = [NSOpenPanel openPanel];
    gkUIConfigurePanel(panel, title, directory, filename, extensions);
    panel.allowsMultipleSelection = multiple;
    panel.canChooseDirectories = directories;
    panel.canChooseFiles = !directories;
    if ([panel runModal] != NSModalResponseOK) {
        if (cancelled) *cancelled = 1;
        if (resultSize) *resultSize = 0;
        return NULL;
    }
    if (cancelled) *cancelled = 0;
    return gkUIPaths(panel.URLs, resultSize);
}

char *gkUISavePanel(uintptr_t window, const char *title,
                    const char *directory, const char *filename,
                    const char *extensions, size_t *resultSize, int *cancelled) {
    (void)window;
    NSSavePanel *panel = [NSSavePanel savePanel];
    gkUIConfigurePanel(panel, title, directory, filename, extensions);
    if ([panel runModal] != NSModalResponseOK) {
        if (cancelled) *cancelled = 1;
        if (resultSize) *resultSize = 0;
        return NULL;
    }
    if (cancelled) *cancelled = 0;
    return gkUIPaths(@[panel.URL], resultSize);
}
