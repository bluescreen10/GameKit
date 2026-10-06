#ifndef GAMEKIT_UI_WIDGET_BRIDGE_H
#define GAMEKIT_UI_WIDGET_BRIDGE_H

#include <stdint.h>
#include <stddef.h>

enum {
    GK_UI_EVENT_CLICK = 1,
    GK_UI_EVENT_TEXT_CHANGE = 2,
    GK_UI_EVENT_VALUE_CHANGE = 3,
    GK_UI_EVENT_PANEL_MOVE = 4,
    GK_UI_EVENT_SECTION_CHANGE = 5,
    GK_UI_EVENT_TOGGLE_CHANGE = 6,
    GK_UI_EVENT_SELECTION_CHANGE = 7,
    GK_UI_EVENT_TAB_CHANGE = 8,
};

void *gkUICreateLayout(uintptr_t window);
void *gkUICreateLabel(uintptr_t window, const char *text);
void *gkUICreateButton(uintptr_t window, const char *text, uintptr_t handle);
void *gkUICreateTextField(uintptr_t window, uintptr_t handle);
void *gkUICreateTextArea(uintptr_t window, const char *text,
                         const char *placeholder, int readOnly, int wrap,
                         uintptr_t handle);
void *gkUICreateCheckBox(uintptr_t window, const char *text, int checked,
                         uintptr_t handle);
void *gkUICreateRadioGroup(uintptr_t window, uintptr_t handle);
void *gkUICreateSelect(uintptr_t window, uintptr_t handle);
void gkUISelectionAddItem(void *control, const char *text);
int gkUISelectionIndex(void *control);
void gkUISetSelectionIndex(void *control, int index);
void *gkUICreateSlider(uintptr_t window, double minimum, double maximum,
                       double value, double step, uintptr_t handle);
void *gkUICreateProgressBar(uintptr_t window, double value);
void *gkUICreateActivityIndicator(uintptr_t window, int running);
void *gkUICreateSeparator(uintptr_t window, int direction);
void *gkUICreateImage(uintptr_t window, int scaling);
void gkUIImageSet(void *control, const unsigned char *pixels,
                  int width, int height, int stride);
int gkUIImageScaling(void *control);
void gkUIImageSetScaling(void *control, int scaling);
void *gkUICreateLink(uintptr_t window, const char *text,
                     const char *url, uintptr_t handle);
char *gkUILinkURL(void *control);
void gkUISetLinkURL(void *control, const char *url);
void *gkUICreateTabs(uintptr_t window, uintptr_t handle);
int gkUITabsAdd(void *tabs, const char *title, void *content);
int gkUITabsSelected(void *tabs);
void gkUITabsSetSelected(void *tabs, int index);
void gkUITabsContentSize(void *tabs, int *width, int *height);
void *gkUICreateScrollView(uintptr_t window, int axes);
void gkUIScrollViewportSize(void *scroll, int *width, int *height);
void gkUIScrollSetContentSize(void *scroll, int width, int height);
void gkUIScrollOffset(void *scroll, int *x, int *y);
void gkUIScrollSetOffset(void *scroll, int x, int y);
void *gkUICreateCanvas(uintptr_t window);
void *gkUICreatePanel(uintptr_t window, const char *title, int movable,
                      uintptr_t handle);
void *gkUICreateSection(uintptr_t window, const char *title, int expanded,
                        uintptr_t handle);
uintptr_t gkUIControlHandle(void *control);

void gkUIDestroy(void *control);
void gkUIClearAction(void *control);
void gkUIAttachToWindow(uintptr_t window, void *control);
void gkUISetParent(void *control, void *parent);
void gkUIBringToFront(void *control);
void gkUISetFrame(void *control, int x, int y, int width, int height);
void gkUISetVisible(void *control, int visible);
void gkUISetEnabled(void *control, int enabled);
void gkUIPreferredSize(void *control, int *width, int *height);
void gkUIFramebufferSize(void *control, int *width, int *height);

char *gkUIText(void *control);
void gkUISetText(void *control, const char *text);
void gkUISetPlaceholder(void *control, const char *text);
char *gkUIPlaceholder(void *control);
int gkUIChecked(void *control);
void gkUISetChecked(void *control, int checked);
int gkUIReadOnly(void *control);
void gkUISetReadOnly(void *control, int readOnly);
double gkUISliderValue(void *control);
void gkUISetSliderValue(void *control, double value);
double gkUIProgressValue(void *control);
void gkUISetProgressValue(void *control, double value);
int gkUIActivityRunning(void *control);
void gkUISetActivityRunning(void *control, int running);
void gkUIPanelSetMovable(void *control, int movable);
void gkUIPanelPosition(void *control, int *x, int *y);
int gkUISectionExpanded(void *control);
void gkUISectionSetExpanded(void *control, int expanded);

char *gkUIOpenPanel(uintptr_t window, const char *title,
                    const char *directory, const char *filename,
                    const char *extensions, int multiple, int directories,
                    size_t *resultSize, int *cancelled);
char *gkUISavePanel(uintptr_t window, const char *title,
                    const char *directory, const char *filename,
                    const char *extensions, size_t *resultSize, int *cancelled);

extern void gkUIGoEvent(uintptr_t handle, int event);

#ifdef _WIN32
#include <windows.h>
int gkUIProcessMessage(HWND window, UINT message, WPARAM wparam,
                       LPARAM lparam, LRESULT *result);
void gkUIApplyTheme(HWND window, int dark);
#endif

#endif
