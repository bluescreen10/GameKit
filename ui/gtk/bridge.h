#ifndef GAMEKIT_UI_GTK_BRIDGE_H
#define GAMEKIT_UI_GTK_BRIDGE_H

#include <stddef.h>
#include <stdint.h>

enum {
    GK_GTK_EVENT_CLICK = 1,
    GK_GTK_EVENT_TEXT_CHANGE = 2,
    GK_GTK_EVENT_VALUE_CHANGE = 3,
    GK_GTK_EVENT_SECTION_CHANGE = 4,
    GK_GTK_EVENT_TOGGLE_CHANGE = 5,
    GK_GTK_EVENT_SELECTION_CHANGE = 6,
    GK_GTK_EVENT_TAB_CHANGE = 7,
    GK_GTK_EVENT_PANEL_MOVE = 8,
};

enum {
    GK_GTK_WIDGET_LAYOUT = 1,
    GK_GTK_WIDGET_LABEL = 2,
    GK_GTK_WIDGET_BUTTON = 3,
    GK_GTK_WIDGET_TEXT_FIELD = 4,
    GK_GTK_WIDGET_SLIDER = 5,
    GK_GTK_WIDGET_CANVAS = 6,
    GK_GTK_WIDGET_SECTION = 7,
    GK_GTK_WIDGET_TEXT_AREA = 8,
    GK_GTK_WIDGET_CHECK_BOX = 9,
    GK_GTK_WIDGET_RADIO_GROUP = 10,
    GK_GTK_WIDGET_SELECT = 11,
    GK_GTK_WIDGET_PROGRESS = 12,
    GK_GTK_WIDGET_ACTIVITY = 13,
    GK_GTK_WIDGET_SEPARATOR = 14,
    GK_GTK_WIDGET_IMAGE = 15,
    GK_GTK_WIDGET_LINK = 16,
    GK_GTK_WIDGET_TABS = 17,
    GK_GTK_WIDGET_SCROLL_VIEW = 18,
    GK_GTK_WIDGET_PANEL = 19,
};

void *gkGTKWindowCreate(const char *title, int width, int height,
                        uint32_t flags, int theme, uintptr_t handle,
                        char *error, size_t error_size);
void gkGTKWindowDestroy(void *window);
void gkGTKWindowPoll(void *window);
int gkGTKWindowShouldClose(void *window);
void gkGTKWindowSetShouldClose(void *window, int close);
void gkGTKWindowSize(void *window, int *width, int *height);
void gkGTKWindowFramebufferSize(void *window, int *width, int *height);
void gkGTKWindowSetTitle(void *window, const char *title);
void gkGTKWindowSetTheme(void *window, int theme);
uintptr_t gkGTKWindowXID(void *window);
uintptr_t gkGTKWindowXDisplay(void *window);
void gkGTKWindowSetContent(void *window, void *widget);

void *gkGTKCreateLayout(void);
void *gkGTKCreateLabel(const char *text);
void *gkGTKCreateButton(const char *text, uintptr_t handle);
void *gkGTKCreateTextField(uintptr_t handle);
void *gkGTKCreateTextArea(const char *text, const char *placeholder,
                          int read_only, int wrap, uintptr_t handle);
void *gkGTKCreateCheckBox(const char *text, int checked, uintptr_t handle);
void *gkGTKCreateRadioGroup(uintptr_t handle);
void *gkGTKCreateSelect(uintptr_t handle);
void gkGTKSelectionAddItem(void *widget, const char *text);
int gkGTKSelectionIndex(void *widget);
void gkGTKSelectionSetIndex(void *widget, int index);
void *gkGTKCreateSlider(double minimum, double maximum, double value,
                        double step, uintptr_t handle);
void *gkGTKCreateProgressBar(double value);
void *gkGTKCreateActivityIndicator(int running);
void *gkGTKCreateSeparator(int direction);
void *gkGTKCreateImage(int scaling);
void gkGTKImageSet(void *widget, const unsigned char *pixels,
                   int width, int height, int stride);
void gkGTKImageSetScaling(void *widget, int scaling);
void *gkGTKCreateLink(const char *text, const char *url, uintptr_t handle);
char *gkGTKLinkURL(void *widget);
void gkGTKLinkSetURL(void *widget, const char *url);
void *gkGTKCreateTabs(uintptr_t handle);
int gkGTKTabsAdd(void *tabs, const char *title, void *content);
int gkGTKTabsSelected(void *tabs);
void gkGTKTabsSetSelected(void *tabs, int index);
void gkGTKTabsContentSize(void *tabs, int *width, int *height);
void *gkGTKCreateScrollView(int axes);
void gkGTKScrollViewportSize(void *scroll, int *width, int *height);
void gkGTKScrollSetContentSize(void *scroll, int width, int height);
void gkGTKScrollOffset(void *scroll, int *x, int *y);
void gkGTKScrollSetOffset(void *scroll, int x, int y);
void *gkGTKCreateCanvas(void);
void *gkGTKCreateSection(const char *title, int expanded, uintptr_t handle);
void *gkGTKCreatePanel(void *window, const char *title, int movable,
                       int x, int y, int width, int height, uintptr_t handle);

void gkGTKWidgetDestroy(void *widget);
void gkGTKWidgetSetParent(void *widget, void *parent);
void gkGTKWidgetUnparent(void *widget);
void gkGTKWidgetAttachToWindow(void *window, void *widget);
void gkGTKWidgetSetFrame(void *widget, int x, int y, int width, int height);
void gkGTKWidgetSetVisible(void *widget, int visible);
void gkGTKWidgetSetEnabled(void *widget, int enabled);
void gkGTKWidgetPreferredSize(void *widget, int *width, int *height);
char *gkGTKWidgetText(void *widget);
void gkGTKWidgetSetText(void *widget, const char *text);
char *gkGTKTextFieldPlaceholder(void *widget);
void gkGTKTextFieldSetPlaceholder(void *widget, const char *placeholder);
int gkGTKWidgetChecked(void *widget);
void gkGTKWidgetSetChecked(void *widget, int checked);
int gkGTKTextAreaReadOnly(void *widget);
void gkGTKTextAreaSetReadOnly(void *widget, int read_only);
double gkGTKSliderValue(void *widget);
void gkGTKSliderSetValue(void *widget, double value);
double gkGTKProgressValue(void *widget);
void gkGTKProgressSetValue(void *widget, double value);
int gkGTKActivityRunning(void *widget);
void gkGTKActivitySetRunning(void *widget, int running);
uintptr_t gkGTKCanvasXID(void *widget);
void gkGTKCanvasFramebufferSize(void *widget, int *width, int *height);
void gkGTKCanvasQueueDraw(void *widget);
int gkGTKSectionExpanded(void *widget);
void gkGTKSectionSetExpanded(void *widget, int expanded);
void gkGTKPanelSetMovable(void *widget, int movable);
void gkGTKPanelPosition(void *widget, int *x, int *y);
void gkGTKPanelBringToFront(void *widget);

char *gkGTKOpenDialog(void *window, const char *title,
                      const char *directory, const char *filename,
                      const char *extensions, int multiple, int directories,
                      size_t *result_size, int *cancelled);
char *gkGTKSaveDialog(void *window, const char *title,
                      const char *directory, const char *filename,
                      const char *extensions, int confirm_overwrite,
                      size_t *result_size, int *cancelled);

extern void gkGTKGoWidgetEvent(uintptr_t handle, int event);

#endif
