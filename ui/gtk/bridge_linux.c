//go:build linux && cgo && gtk

#include "bridge.h"

#include <gtk/gtk.h>
#include <gdk/gdkx.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    GtkWidget *window;
    GtkWidget *overlay;
    GtkWidget *content;
    gboolean should_close;
    uintptr_t handle;
} GKGTKWindow;

typedef struct {
    GtkWidget *widget;
    GtkWidget *content;
    GtkWidget *control;
    GKGTKWindow *window;
    GPtrArray *items;
    GdkPixbuf *pixbuf;
    int kind;
    int scaling;
    int selected;
    int x;
    int y;
    int drag_x;
    int drag_y;
    double drag_root_x;
    double drag_root_y;
    gboolean running;
    gboolean movable;
    gboolean dragging;
    uintptr_t handle;
    gboolean suppress;
} GKGTKWidget;

static gboolean gkGTKInitialized;
static gboolean gkGTKSystemDark;

static gboolean gkGTKDelete(GtkWidget *widget, GdkEvent *event, gpointer data) {
    (void)widget;
    (void)event;
    ((GKGTKWindow *)data)->should_close = TRUE;
    return TRUE;
}

static void gkGTKApplyTheme(int theme) {
    GtkSettings *settings = gtk_settings_get_default();
    if (!settings) return;
    gboolean dark = theme == 2 || (theme == 0 && gkGTKSystemDark);
    g_object_set(settings, "gtk-application-prefer-dark-theme", dark, NULL);
}

void *gkGTKWindowCreate(const char *title, int width, int height,
                        uint32_t flags, int theme, uintptr_t handle,
                        char *error, size_t error_size) {
    if (!gkGTKInitialized) {
        gdk_set_allowed_backends("x11");
        if (!gtk_init_check(NULL, NULL)) {
            if (error && error_size) {
                snprintf(error, error_size, "GTK could not initialize an X11 display");
            }
            return NULL;
        }
        GtkSettings *settings = gtk_settings_get_default();
        if (settings) {
            g_object_get(settings, "gtk-application-prefer-dark-theme", &gkGTKSystemDark, NULL);
        }
        gkGTKInitialized = TRUE;
    }

    GKGTKWindow *native = calloc(1, sizeof(*native));
    if (!native) return NULL;
    native->window = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    native->overlay = gtk_overlay_new();
    native->handle = handle;
    gtk_container_add(GTK_CONTAINER(native->window), native->overlay);
    gtk_window_set_title(GTK_WINDOW(native->window), title ? title : "");
    gtk_window_set_default_size(GTK_WINDOW(native->window), width, height);
    gtk_window_set_resizable(GTK_WINDOW(native->window), (flags & 1u) != 0);
    gtk_window_set_decorated(GTK_WINDOW(native->window), (flags & (1u << 2)) == 0);
    gtk_window_set_keep_above(GTK_WINDOW(native->window), (flags & (1u << 4)) != 0);
    if (flags & (1u << 5)) {
        GdkScreen *screen = gtk_widget_get_screen(native->window);
        GdkVisual *visual = gdk_screen_get_rgba_visual(screen);
        if (visual) gtk_widget_set_visual(native->window, visual);
        gtk_widget_set_app_paintable(native->window, TRUE);
    }
    g_signal_connect(native->window, "delete-event", G_CALLBACK(gkGTKDelete), native);
    gkGTKApplyTheme(theme);
    gtk_widget_show(native->overlay);
    gtk_widget_realize(native->window);
    if (flags & (1u << 3)) gtk_window_maximize(GTK_WINDOW(native->window));
    if (!(flags & (1u << 1))) gtk_widget_show(native->window);
    return native;
}

void gkGTKWindowDestroy(void *pointer) {
    GKGTKWindow *native = pointer;
    if (!native) return;
    if (native->window) gtk_widget_destroy(native->window);
    free(native);
}

void gkGTKWindowPoll(void *pointer) {
    (void)pointer;
    while (g_main_context_iteration(NULL, FALSE)) {}
}

int gkGTKWindowShouldClose(void *pointer) {
    GKGTKWindow *native = pointer;
    return !native || native->should_close;
}

void gkGTKWindowSetShouldClose(void *pointer, int close) {
    if (pointer) ((GKGTKWindow *)pointer)->should_close = close != 0;
}

void gkGTKWindowSize(void *pointer, int *width, int *height) {
    GKGTKWindow *native = pointer;
    GtkAllocation allocation = {0};
    if (native) gtk_widget_get_allocation(native->overlay, &allocation);
    if (width) *width = allocation.width;
    if (height) *height = allocation.height;
}

void gkGTKWindowFramebufferSize(void *pointer, int *width, int *height) {
    GKGTKWindow *native = pointer;
    int logical_width = 0;
    int logical_height = 0;
    gkGTKWindowSize(pointer, &logical_width, &logical_height);
    int scale = native ? gtk_widget_get_scale_factor(native->window) : 1;
    if (width) *width = logical_width * scale;
    if (height) *height = logical_height * scale;
}

void gkGTKWindowSetTitle(void *pointer, const char *title) {
    GKGTKWindow *native = pointer;
    if (native) gtk_window_set_title(GTK_WINDOW(native->window), title ? title : "");
}

void gkGTKWindowSetTheme(void *pointer, int theme) {
    (void)pointer;
    gkGTKApplyTheme(theme);
}

static GdkWindow *gkGTKNativeWindow(GKGTKWindow *native) {
    if (!native) return NULL;
    gtk_widget_realize(native->window);
    return gtk_widget_get_window(native->window);
}

uintptr_t gkGTKWindowXID(void *pointer) {
    GdkWindow *window = gkGTKNativeWindow(pointer);
    return window && GDK_IS_X11_WINDOW(window) ? (uintptr_t)gdk_x11_window_get_xid(window) : 0;
}

uintptr_t gkGTKWindowXDisplay(void *pointer) {
    GdkWindow *window = gkGTKNativeWindow(pointer);
    if (!window || !GDK_IS_X11_WINDOW(window)) return 0;
    return (uintptr_t)gdk_x11_display_get_xdisplay(gdk_window_get_display(window));
}

static void gkGTKRemoveFromParent(GtkWidget *widget) {
    GtkWidget *parent = gtk_widget_get_parent(widget);
    if (parent && GTK_IS_CONTAINER(parent)) {
        gtk_container_remove(GTK_CONTAINER(parent), widget);
    }
}

void gkGTKWindowSetContent(void *window_pointer, void *widget_pointer) {
    GKGTKWindow *window = window_pointer;
    GKGTKWidget *widget = widget_pointer;
    if (!window || !widget) return;
    if (window->content && window->content != widget->widget) {
        gtk_widget_hide(window->content);
        gkGTKRemoveFromParent(window->content);
    }
    gkGTKRemoveFromParent(widget->widget);
    gtk_container_add(GTK_CONTAINER(window->overlay), widget->widget);
    window->content = widget->widget;
    gtk_widget_show(widget->widget);
}

static GKGTKWidget *gkGTKWrap(GtkWidget *widget, int kind, uintptr_t handle) {
    if (!widget) return NULL;
    GKGTKWidget *native = calloc(1, sizeof(*native));
    if (!native) {
        gtk_widget_destroy(widget);
        return NULL;
    }
    native->widget = g_object_ref_sink(widget);
    native->kind = kind;
    native->handle = handle;
    gtk_widget_show(widget);
    return native;
}

static void gkGTKSignal(GtkWidget *widget, gpointer data) {
    (void)widget;
    GKGTKWidget *native = data;
    if (!native->suppress && native->handle) {
        int event = native->kind == GK_GTK_WIDGET_BUTTON || native->kind == GK_GTK_WIDGET_LINK ? GK_GTK_EVENT_CLICK :
                    native->kind == GK_GTK_WIDGET_TEXT_FIELD || native->kind == GK_GTK_WIDGET_TEXT_AREA ? GK_GTK_EVENT_TEXT_CHANGE :
                    native->kind == GK_GTK_WIDGET_SLIDER ? GK_GTK_EVENT_VALUE_CHANGE :
                    native->kind == GK_GTK_WIDGET_CHECK_BOX ? GK_GTK_EVENT_TOGGLE_CHANGE :
                    native->kind == GK_GTK_WIDGET_RADIO_GROUP || native->kind == GK_GTK_WIDGET_SELECT ? GK_GTK_EVENT_SELECTION_CHANGE :
                    native->kind == GK_GTK_WIDGET_TABS ? GK_GTK_EVENT_TAB_CHANGE :
                    native->kind == GK_GTK_WIDGET_PANEL ? GK_GTK_EVENT_PANEL_MOVE :
                    GK_GTK_EVENT_SECTION_CHANGE;
        gkGTKGoWidgetEvent(native->handle, event);
    }
}

static void gkGTKRadioSignal(GtkToggleButton *button, gpointer data) {
    GKGTKWidget *native = data;
    if (!gtk_toggle_button_get_active(button)) return;
    for (guint i = 0; native->items && i < native->items->len; i++) {
        if (g_ptr_array_index(native->items, i) == GTK_WIDGET(button)) {
            native->selected = (int)i;
            break;
        }
    }
    gkGTKSignal(GTK_WIDGET(button), data);
}

static void gkGTKTabSignal(GtkNotebook *notebook, GtkWidget *page,
                           guint page_number, gpointer data) {
    (void)notebook;
    (void)page;
    (void)page_number;
    gkGTKSignal(NULL, data);
}

static gboolean gkGTKLinkSignal(GtkWidget *widget, gpointer data) {
    gkGTKSignal(widget, data);
    return TRUE;
}

static gboolean gkGTKImageDraw(GtkWidget *widget, cairo_t *context, gpointer data) {
    GKGTKWidget *native = data;
    if (!native->pixbuf) return FALSE;
    GtkAllocation allocation;
    gtk_widget_get_allocation(widget, &allocation);
    int image_width = gdk_pixbuf_get_width(native->pixbuf);
    int image_height = gdk_pixbuf_get_height(native->pixbuf);
    double sx = image_width ? (double)allocation.width / image_width : 1;
    double sy = image_height ? (double)allocation.height / image_height : 1;
    double scale_x = sx;
    double scale_y = sy;
    if (native->scaling != 2) {
        double scale = native->scaling == 1 ? MAX(sx, sy) : MIN(sx, sy);
        scale_x = scale;
        scale_y = scale;
    }
    double width = image_width * scale_x;
    double height = image_height * scale_y;
    cairo_save(context);
    cairo_translate(context, (allocation.width - width) / 2, (allocation.height - height) / 2);
    cairo_scale(context, scale_x, scale_y);
    gdk_cairo_set_source_pixbuf(context, native->pixbuf, 0, 0);
    cairo_paint(context);
    cairo_restore(context);
    return TRUE;
}

static void gkGTKFreePixels(guchar *pixels, gpointer data) {
    (void)data;
    g_free(pixels);
}

static void gkGTKSectionSignal(GObject *object, GParamSpec *spec, gpointer data) {
    (void)object;
    (void)spec;
    gkGTKSignal(NULL, data);
}

static void gkGTKPanelRaise(GKGTKWidget *native) {
    if (!native || native->kind != GK_GTK_WIDGET_PANEL || !native->window) return;
    gtk_overlay_reorder_overlay(GTK_OVERLAY(native->window->overlay), native->widget, -1);
}

static gboolean gkGTKPanelPress(GtkWidget *widget, GdkEventButton *event, gpointer data) {
    (void)widget;
    GKGTKWidget *native = data;
    if (!native->movable || event->button != GDK_BUTTON_PRIMARY) return FALSE;
    native->dragging = TRUE;
    native->drag_x = native->x;
    native->drag_y = native->y;
    native->drag_root_x = event->x_root;
    native->drag_root_y = event->y_root;
    gkGTKPanelRaise(native);
    return TRUE;
}

static gboolean gkGTKPanelRelease(GtkWidget *widget, GdkEventButton *event, gpointer data) {
    (void)widget;
    GKGTKWidget *native = data;
    if (event->button != GDK_BUTTON_PRIMARY || !native->dragging) return FALSE;
    native->dragging = FALSE;
    return TRUE;
}

static gboolean gkGTKPanelMotion(GtkWidget *widget, GdkEventMotion *event, gpointer data) {
    (void)widget;
    GKGTKWidget *native = data;
    if (!native->movable || !native->dragging || !native->window) return FALSE;

    int x = native->drag_x + (int)(event->x_root - native->drag_root_x);
    int y = native->drag_y + (int)(event->y_root - native->drag_root_y);
    GtkAllocation overlay = {0};
    gtk_widget_get_allocation(native->window->overlay, &overlay);
    int maximum_x = MAX(0, overlay.width - gtk_widget_get_allocated_width(native->widget));
    int maximum_y = MAX(0, overlay.height - gtk_widget_get_allocated_height(native->widget));
    native->x = CLAMP(x, 0, maximum_x);
    native->y = CLAMP(y, 0, maximum_y);
    gtk_widget_set_margin_start(native->widget, native->x);
    gtk_widget_set_margin_top(native->widget, native->y);
    gkGTKSignal(NULL, native);
    return TRUE;
}

void *gkGTKCreateLayout(void) {
    return gkGTKWrap(gtk_fixed_new(), GK_GTK_WIDGET_LAYOUT, 0);
}

void *gkGTKCreateLabel(const char *text) {
    GtkWidget *label = gtk_label_new(text ? text : "");
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    return gkGTKWrap(label, GK_GTK_WIDGET_LABEL, 0);
}

void *gkGTKCreateButton(const char *text, uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_button_new_with_label(text ? text : ""),
                                    GK_GTK_WIDGET_BUTTON, handle);
    if (native) g_signal_connect(native->widget, "clicked", G_CALLBACK(gkGTKSignal), native);
    return native;
}

void *gkGTKCreateTextField(uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_entry_new(), GK_GTK_WIDGET_TEXT_FIELD, handle);
    if (native) g_signal_connect(native->widget, "changed", G_CALLBACK(gkGTKSignal), native);
    return native;
}

void *gkGTKCreateTextArea(const char *text, const char *placeholder,
                          int read_only, int wrap, uintptr_t handle) {
    GtkWidget *scroll = gtk_scrolled_window_new(NULL, NULL);
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll),
        wrap ? GTK_POLICY_NEVER : GTK_POLICY_AUTOMATIC, GTK_POLICY_AUTOMATIC);
    GtkWidget *editor = gtk_text_view_new();
    gtk_text_view_set_wrap_mode(GTK_TEXT_VIEW(editor), wrap ? GTK_WRAP_WORD_CHAR : GTK_WRAP_NONE);
    gtk_text_view_set_editable(GTK_TEXT_VIEW(editor), read_only == 0);
    GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(editor));
    gtk_text_buffer_set_text(buffer, text ? text : "", -1);
    gtk_container_add(GTK_CONTAINER(scroll), editor);
    gtk_widget_show(editor);
    GKGTKWidget *native = gkGTKWrap(scroll, GK_GTK_WIDGET_TEXT_AREA, handle);
    if (!native) return NULL;
    native->control = editor;
    g_object_set_data_full(G_OBJECT(editor), "gamekit-placeholder",
                           g_strdup(placeholder ? placeholder : ""), g_free);
    g_signal_connect(buffer, "changed", G_CALLBACK(gkGTKSignal), native);
    return native;
}

void *gkGTKCreateCheckBox(const char *text, int checked, uintptr_t handle) {
    GtkWidget *button = gtk_check_button_new_with_label(text ? text : "");
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(button), checked != 0);
    GKGTKWidget *native = gkGTKWrap(button, GK_GTK_WIDGET_CHECK_BOX, handle);
    if (native) g_signal_connect(button, "toggled", G_CALLBACK(gkGTKSignal), native);
    return native;
}

void *gkGTKCreateRadioGroup(uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_box_new(GTK_ORIENTATION_VERTICAL, 2),
                                    GK_GTK_WIDGET_RADIO_GROUP, handle);
    if (native) {
        native->items = g_ptr_array_new();
        native->selected = -1;
    }
    return native;
}

void *gkGTKCreateSelect(uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_combo_box_text_new(), GK_GTK_WIDGET_SELECT, handle);
    if (native) g_signal_connect(native->widget, "changed", G_CALLBACK(gkGTKSignal), native);
    return native;
}

void gkGTKSelectionAddItem(void *pointer, const char *text) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    if (native->kind == GK_GTK_WIDGET_SELECT) {
        gtk_combo_box_text_append_text(GTK_COMBO_BOX_TEXT(native->widget), text ? text : "");
    } else if (native->kind == GK_GTK_WIDGET_RADIO_GROUP) {
        GtkWidget *first = native->items->len ? g_ptr_array_index(native->items, 0) : NULL;
        GtkWidget *button = first ?
            gtk_radio_button_new_with_label_from_widget(GTK_RADIO_BUTTON(first), text ? text : "") :
            gtk_radio_button_new_with_label(NULL, text ? text : "");
        g_ptr_array_add(native->items, button);
        gtk_box_pack_start(GTK_BOX(native->widget), button, FALSE, FALSE, 0);
        gtk_widget_show(button);
        g_signal_connect(button, "toggled", G_CALLBACK(gkGTKRadioSignal), native);
    }
}

int gkGTKSelectionIndex(void *pointer) {
    GKGTKWidget *native = pointer;
    if (!native) return -1;
    if (native->kind == GK_GTK_WIDGET_SELECT) {
        return gtk_combo_box_get_active(GTK_COMBO_BOX(native->widget));
    }
    return native->kind == GK_GTK_WIDGET_RADIO_GROUP ? native->selected : -1;
}

void gkGTKSelectionSetIndex(void *pointer, int index) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    native->suppress = TRUE;
    if (native->kind == GK_GTK_WIDGET_SELECT) {
        gtk_combo_box_set_active(GTK_COMBO_BOX(native->widget), index);
    } else if (native->kind == GK_GTK_WIDGET_RADIO_GROUP && native->items &&
               index >= 0 && index < (int)native->items->len) {
        native->selected = index;
        gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(g_ptr_array_index(native->items, index)), TRUE);
    }
    native->suppress = FALSE;
}

void *gkGTKCreateSlider(double minimum, double maximum, double value,
                        double step, uintptr_t handle) {
    double increment = step > 0 ? step : (maximum - minimum) / 100.0;
    GtkWidget *scale = gtk_scale_new_with_range(GTK_ORIENTATION_HORIZONTAL,
                                                minimum, maximum, increment);
    gtk_range_set_value(GTK_RANGE(scale), value);
    gtk_scale_set_draw_value(GTK_SCALE(scale), FALSE);
    GKGTKWidget *native = gkGTKWrap(scale, GK_GTK_WIDGET_SLIDER, handle);
    if (native) g_signal_connect(native->widget, "value-changed", G_CALLBACK(gkGTKSignal), native);
    return native;
}

void *gkGTKCreateProgressBar(double value) {
    GtkWidget *progress = gtk_progress_bar_new();
    gtk_progress_bar_set_fraction(GTK_PROGRESS_BAR(progress), CLAMP(value, 0, 1));
    return gkGTKWrap(progress, GK_GTK_WIDGET_PROGRESS, 0);
}

void *gkGTKCreateActivityIndicator(int running) {
    GtkWidget *spinner = gtk_spinner_new();
    if (running) gtk_spinner_start(GTK_SPINNER(spinner));
    GKGTKWidget *native = gkGTKWrap(spinner, GK_GTK_WIDGET_ACTIVITY, 0);
    if (native) native->running = running != 0;
    return native;
}

void *gkGTKCreateSeparator(int direction) {
    GtkWidget *separator = gtk_separator_new(direction == 0 ?
        GTK_ORIENTATION_HORIZONTAL : GTK_ORIENTATION_VERTICAL);
    return gkGTKWrap(separator, GK_GTK_WIDGET_SEPARATOR, 0);
}

void *gkGTKCreateImage(int scaling) {
    GtkWidget *area = gtk_drawing_area_new();
    GKGTKWidget *native = gkGTKWrap(area, GK_GTK_WIDGET_IMAGE, 0);
    if (native) {
        native->scaling = scaling;
        g_signal_connect(area, "draw", G_CALLBACK(gkGTKImageDraw), native);
    }
    return native;
}

void gkGTKImageSet(void *pointer, const unsigned char *pixels,
                   int width, int height, int stride) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_IMAGE || !pixels ||
        width <= 0 || height <= 0 || stride < width * 4) return;
    gsize size = (gsize)stride * height;
    guchar *copy = g_malloc(size);
    memcpy(copy, pixels, size);
    GdkPixbuf *pixbuf = gdk_pixbuf_new_from_data(copy, GDK_COLORSPACE_RGB, TRUE, 8,
        width, height, stride, gkGTKFreePixels, NULL);
    if (!pixbuf) {
        g_free(copy);
        return;
    }
    if (native->pixbuf) g_object_unref(native->pixbuf);
    native->pixbuf = pixbuf;
    gtk_widget_queue_draw(native->widget);
}

void gkGTKImageSetScaling(void *pointer, int scaling) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_IMAGE) return;
    native->scaling = scaling;
    gtk_widget_queue_draw(native->widget);
}

void *gkGTKCreateLink(const char *text, const char *url, uintptr_t handle) {
    GtkWidget *link = gtk_link_button_new_with_label(url ? url : "", text ? text : "");
    GKGTKWidget *native = gkGTKWrap(link, GK_GTK_WIDGET_LINK, handle);
    if (native) g_signal_connect(link, "activate-link", G_CALLBACK(gkGTKLinkSignal), native);
    return native;
}

char *gkGTKLinkURL(void *pointer) {
    GKGTKWidget *native = pointer;
    const char *url = native && native->kind == GK_GTK_WIDGET_LINK ?
        gtk_link_button_get_uri(GTK_LINK_BUTTON(native->widget)) : "";
    return strdup(url ? url : "");
}

void gkGTKLinkSetURL(void *pointer, const char *url) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_LINK) {
        gtk_link_button_set_uri(GTK_LINK_BUTTON(native->widget), url ? url : "");
    }
}

void *gkGTKCreateTabs(uintptr_t handle) {
    GKGTKWidget *native = gkGTKWrap(gtk_notebook_new(), GK_GTK_WIDGET_TABS, handle);
    if (native) g_signal_connect(native->widget, "switch-page", G_CALLBACK(gkGTKTabSignal), native);
    return native;
}

int gkGTKTabsAdd(void *tabs_pointer, const char *title, void *content_pointer) {
    GKGTKWidget *tabs = tabs_pointer;
    GKGTKWidget *content = content_pointer;
    if (!tabs || tabs->kind != GK_GTK_WIDGET_TABS || !content) return 0;
    gkGTKRemoveFromParent(content->widget);
    GtkWidget *label = gtk_label_new(title ? title : "");
    gtk_notebook_append_page(GTK_NOTEBOOK(tabs->widget), content->widget, label);
    gtk_widget_show(label);
    gtk_widget_show(content->widget);
    return 1;
}

int gkGTKTabsSelected(void *pointer) {
    GKGTKWidget *tabs = pointer;
    return tabs && tabs->kind == GK_GTK_WIDGET_TABS ?
        gtk_notebook_get_current_page(GTK_NOTEBOOK(tabs->widget)) : -1;
}

void gkGTKTabsSetSelected(void *pointer, int index) {
    GKGTKWidget *tabs = pointer;
    if (!tabs || tabs->kind != GK_GTK_WIDGET_TABS) return;
    tabs->suppress = TRUE;
    gtk_notebook_set_current_page(GTK_NOTEBOOK(tabs->widget), index);
    tabs->suppress = FALSE;
}

void gkGTKTabsContentSize(void *pointer, int *width, int *height) {
    GKGTKWidget *tabs = pointer;
    GtkAllocation allocation = {0};
    if (tabs && tabs->kind == GK_GTK_WIDGET_TABS) {
        int index = gtk_notebook_get_current_page(GTK_NOTEBOOK(tabs->widget));
        GtkWidget *page = gtk_notebook_get_nth_page(GTK_NOTEBOOK(tabs->widget), index);
        if (page) gtk_widget_get_allocation(page, &allocation);
    }
    if (width) *width = allocation.width;
    if (height) *height = allocation.height;
}

void *gkGTKCreateScrollView(int axes) {
    GtkWidget *scroll = gtk_scrolled_window_new(NULL, NULL);
    GtkPolicyType horizontal = axes == 1 || axes == 2 ? GTK_POLICY_AUTOMATIC : GTK_POLICY_NEVER;
    GtkPolicyType vertical = axes == 0 || axes == 2 ? GTK_POLICY_AUTOMATIC : GTK_POLICY_NEVER;
    gtk_scrolled_window_set_policy(GTK_SCROLLED_WINDOW(scroll), horizontal, vertical);
    GtkWidget *viewport = gtk_viewport_new(NULL, NULL);
    GtkWidget *content = gtk_fixed_new();
    gtk_container_add(GTK_CONTAINER(viewport), content);
    gtk_container_add(GTK_CONTAINER(scroll), viewport);
    gtk_widget_show(content);
    gtk_widget_show(viewport);
    GKGTKWidget *native = gkGTKWrap(scroll, GK_GTK_WIDGET_SCROLL_VIEW, 0);
    if (!native) return NULL;
    native->content = content;
    native->control = viewport;
    return native;
}

void gkGTKScrollViewportSize(void *pointer, int *width, int *height) {
    GKGTKWidget *native = pointer;
    GtkAllocation allocation = {0};
    if (native && native->kind == GK_GTK_WIDGET_SCROLL_VIEW && native->control) {
        gtk_widget_get_allocation(native->control, &allocation);
    }
    if (width) *width = allocation.width;
    if (height) *height = allocation.height;
}

void gkGTKScrollSetContentSize(void *pointer, int width, int height) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_SCROLL_VIEW || !native->content) return;
    gtk_widget_set_size_request(native->content, MAX(0, width), MAX(0, height));
}

void gkGTKScrollOffset(void *pointer, int *x, int *y) {
    GKGTKWidget *native = pointer;
    double horizontal = 0;
    double vertical = 0;
    if (native && native->kind == GK_GTK_WIDGET_SCROLL_VIEW) {
        horizontal = gtk_adjustment_get_value(
            gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(native->widget)));
        vertical = gtk_adjustment_get_value(
            gtk_scrolled_window_get_vadjustment(GTK_SCROLLED_WINDOW(native->widget)));
    }
    if (x) *x = (int)horizontal;
    if (y) *y = (int)vertical;
}

void gkGTKScrollSetOffset(void *pointer, int x, int y) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_SCROLL_VIEW) return;
    gtk_adjustment_set_value(
        gtk_scrolled_window_get_hadjustment(GTK_SCROLLED_WINDOW(native->widget)), MAX(0, x));
    gtk_adjustment_set_value(
        gtk_scrolled_window_get_vadjustment(GTK_SCROLLED_WINDOW(native->widget)), MAX(0, y));
}

void *gkGTKCreateCanvas(void) {
    GtkWidget *area = gtk_drawing_area_new();
    gtk_widget_set_has_window(area, TRUE);
    return gkGTKWrap(area, GK_GTK_WIDGET_CANVAS, 0);
}

void *gkGTKCreateSection(const char *title, int expanded, uintptr_t handle) {
    GtkWidget *expander = gtk_expander_new(title ? title : "");
    GKGTKWidget *native = gkGTKWrap(expander, GK_GTK_WIDGET_SECTION, handle);
    if (!native) return NULL;
    native->content = gtk_fixed_new();
    gtk_widget_show(native->content);
    gtk_container_add(GTK_CONTAINER(native->widget), native->content);
    gtk_expander_set_expanded(GTK_EXPANDER(native->widget), expanded != 0);
    g_signal_connect(native->widget, "notify::expanded", G_CALLBACK(gkGTKSectionSignal), native);
    return native;
}

void *gkGTKCreatePanel(void *window_pointer, const char *title, int movable,
                       int x, int y, int width, int height, uintptr_t handle) {
    GKGTKWindow *window = window_pointer;
    if (!window) return NULL;

    GtkWidget *frame = gtk_frame_new(NULL);
    GtkWidget *box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 0);
    GtkWidget *header = gtk_event_box_new();
    GtkWidget *label = gtk_label_new(title ? title : "");
    GtkWidget *content = gtk_fixed_new();
    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_widget_set_margin_start(label, 12);
    gtk_widget_set_margin_end(label, 12);
    gtk_widget_set_size_request(header, -1, 32);
    gtk_widget_set_margin_start(content, 12);
    gtk_widget_set_margin_end(content, 12);
    gtk_widget_set_margin_bottom(content, 12);
    gtk_container_add(GTK_CONTAINER(header), label);
    gtk_box_pack_start(GTK_BOX(box), header, FALSE, FALSE, 0);
    gtk_box_pack_start(GTK_BOX(box), content, TRUE, TRUE, 0);
    gtk_container_add(GTK_CONTAINER(frame), box);

    GtkStyleContext *style = gtk_widget_get_style_context(header);
    gtk_style_context_add_class(style, GTK_STYLE_CLASS_TITLEBAR);
    gtk_frame_set_shadow_type(GTK_FRAME(frame), GTK_SHADOW_OUT);

    GKGTKWidget *native = gkGTKWrap(frame, GK_GTK_WIDGET_PANEL, handle);
    if (!native) return NULL;
    native->window = window;
    native->content = content;
    native->control = label;
    native->movable = movable != 0;
    native->x = MAX(0, x);
    native->y = MAX(0, y);

    gtk_widget_add_events(header, GDK_BUTTON_PRESS_MASK | GDK_BUTTON_RELEASE_MASK |
                                  GDK_POINTER_MOTION_MASK);
    g_signal_connect(header, "button-press-event", G_CALLBACK(gkGTKPanelPress), native);
    g_signal_connect(header, "button-release-event", G_CALLBACK(gkGTKPanelRelease), native);
    g_signal_connect(header, "motion-notify-event", G_CALLBACK(gkGTKPanelMotion), native);

    gtk_widget_set_halign(frame, GTK_ALIGN_START);
    gtk_widget_set_valign(frame, GTK_ALIGN_START);
    gtk_widget_set_margin_start(frame, native->x);
    gtk_widget_set_margin_top(frame, native->y);
    gtk_widget_set_size_request(frame, MAX(0, width), MAX(0, height));
    gtk_overlay_add_overlay(GTK_OVERLAY(window->overlay), frame);
    gtk_overlay_set_overlay_pass_through(GTK_OVERLAY(window->overlay), frame, FALSE);
    gtk_widget_show_all(frame);
    gkGTKPanelRaise(native);
    return native;
}

void gkGTKWidgetDestroy(void *pointer) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    native->handle = 0;
    gkGTKRemoveFromParent(native->widget);
    gtk_widget_destroy(native->widget);
    g_object_unref(native->widget);
    if (native->items) g_ptr_array_free(native->items, TRUE);
    if (native->pixbuf) g_object_unref(native->pixbuf);
    free(native);
}

void gkGTKWidgetUnparent(void *pointer) {
    GKGTKWidget *native = pointer;
    if (native) gkGTKRemoveFromParent(native->widget);
}

void gkGTKWidgetSetParent(void *widget_pointer, void *parent_pointer) {
    GKGTKWidget *widget = widget_pointer;
    GKGTKWidget *parent = parent_pointer;
    if (!widget || !parent) return;
    gkGTKRemoveFromParent(widget->widget);
    GtkWidget *container = parent->content ? parent->content : parent->widget;
    if (GTK_IS_FIXED(container)) {
        gtk_fixed_put(GTK_FIXED(container), widget->widget, 0, 0);
    } else if (GTK_IS_CONTAINER(container)) {
        gtk_container_add(GTK_CONTAINER(container), widget->widget);
    }
    gtk_widget_show(widget->widget);
}

void gkGTKWidgetAttachToWindow(void *window, void *widget) {
    gkGTKWindowSetContent(window, widget);
}

void gkGTKWidgetSetFrame(void *pointer, int x, int y, int width, int height) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    if (native->kind == GK_GTK_WIDGET_PANEL) {
        native->x = MAX(0, x);
        native->y = MAX(0, y);
        gtk_widget_set_margin_start(native->widget, native->x);
        gtk_widget_set_margin_top(native->widget, native->y);
    }
    GtkWidget *parent = gtk_widget_get_parent(native->widget);
    if (parent && GTK_IS_FIXED(parent)) gtk_fixed_move(GTK_FIXED(parent), native->widget, x, y);
    gtk_widget_set_size_request(native->widget, MAX(0, width), MAX(0, height));
}

void gkGTKWidgetSetVisible(void *pointer, int visible) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    if (visible) gtk_widget_show(native->widget); else gtk_widget_hide(native->widget);
}

void gkGTKWidgetSetEnabled(void *pointer, int enabled) {
    GKGTKWidget *native = pointer;
    if (native) gtk_widget_set_sensitive(native->widget, enabled != 0);
}

void gkGTKWidgetPreferredSize(void *pointer, int *width, int *height) {
    GKGTKWidget *native = pointer;
    GtkRequisition minimum = {0};
    GtkRequisition natural = {0};
    if (native) gtk_widget_get_preferred_size(native->widget, &minimum, &natural);
    if (native && native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        natural.width = MAX(natural.width, 200);
        natural.height = MAX(natural.height, 96);
    }
    if (native && native->kind == GK_GTK_WIDGET_IMAGE && native->pixbuf) {
        natural.width = gdk_pixbuf_get_width(native->pixbuf);
        natural.height = gdk_pixbuf_get_height(native->pixbuf);
    }
    if (native && native->kind == GK_GTK_WIDGET_SCROLL_VIEW) {
        natural.width = 320;
        natural.height = 240;
    }
    if (native && native->kind == GK_GTK_WIDGET_PANEL) {
        natural.width = MAX(natural.width, 240);
        natural.height = MAX(natural.height, 160);
    }
    if (width) *width = natural.width;
    if (height) *height = natural.height;
}

char *gkGTKWidgetText(void *pointer) {
    GKGTKWidget *native = pointer;
    const char *text = "";
    if (!native) return strdup(text);
    if (native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(native->control));
        GtkTextIter start, end;
        gtk_text_buffer_get_bounds(buffer, &start, &end);
        gchar *value = gtk_text_buffer_get_text(buffer, &start, &end, FALSE);
        char *result = strdup(value ? value : "");
        g_free(value);
        return result;
    }
    switch (native->kind) {
    case GK_GTK_WIDGET_LABEL: text = gtk_label_get_text(GTK_LABEL(native->widget)); break;
    case GK_GTK_WIDGET_BUTTON: text = gtk_button_get_label(GTK_BUTTON(native->widget)); break;
    case GK_GTK_WIDGET_TEXT_FIELD: text = gtk_entry_get_text(GTK_ENTRY(native->widget)); break;
    case GK_GTK_WIDGET_CHECK_BOX: text = gtk_button_get_label(GTK_BUTTON(native->widget)); break;
    case GK_GTK_WIDGET_LINK: text = gtk_button_get_label(GTK_BUTTON(native->widget)); break;
    case GK_GTK_WIDGET_SECTION: text = gtk_expander_get_label(GTK_EXPANDER(native->widget)); break;
    case GK_GTK_WIDGET_PANEL: text = gtk_label_get_text(GTK_LABEL(native->control)); break;
    }
    return strdup(text ? text : "");
}

void gkGTKWidgetSetText(void *pointer, const char *text) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    native->suppress = TRUE;
    if (native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        GtkTextBuffer *buffer = gtk_text_view_get_buffer(GTK_TEXT_VIEW(native->control));
        gtk_text_buffer_set_text(buffer, text ? text : "", -1);
    } else switch (native->kind) {
    case GK_GTK_WIDGET_LABEL: gtk_label_set_text(GTK_LABEL(native->widget), text ? text : ""); break;
    case GK_GTK_WIDGET_BUTTON: gtk_button_set_label(GTK_BUTTON(native->widget), text ? text : ""); break;
    case GK_GTK_WIDGET_TEXT_FIELD: gtk_entry_set_text(GTK_ENTRY(native->widget), text ? text : ""); break;
    case GK_GTK_WIDGET_CHECK_BOX: gtk_button_set_label(GTK_BUTTON(native->widget), text ? text : ""); break;
    case GK_GTK_WIDGET_LINK: gtk_button_set_label(GTK_BUTTON(native->widget), text ? text : ""); break;
    case GK_GTK_WIDGET_SECTION: gtk_expander_set_label(GTK_EXPANDER(native->widget), text ? text : ""); break;
    case GK_GTK_WIDGET_PANEL: gtk_label_set_text(GTK_LABEL(native->control), text ? text : ""); break;
    }
    native->suppress = FALSE;
}

char *gkGTKTextFieldPlaceholder(void *pointer) {
    GKGTKWidget *native = pointer;
    const char *text = NULL;
    if (native && native->kind == GK_GTK_WIDGET_TEXT_FIELD) {
        text = gtk_entry_get_placeholder_text(GTK_ENTRY(native->widget));
    } else if (native && native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        text = g_object_get_data(G_OBJECT(native->control), "gamekit-placeholder");
    }
    return strdup(text ? text : "");
}

void gkGTKTextFieldSetPlaceholder(void *pointer, const char *placeholder) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    if (native->kind == GK_GTK_WIDGET_TEXT_FIELD) {
        gtk_entry_set_placeholder_text(GTK_ENTRY(native->widget), placeholder ? placeholder : "");
    } else if (native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        g_object_set_data_full(G_OBJECT(native->control), "gamekit-placeholder",
                               g_strdup(placeholder ? placeholder : ""), g_free);
    }
}

int gkGTKWidgetChecked(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_CHECK_BOX &&
        gtk_toggle_button_get_active(GTK_TOGGLE_BUTTON(native->widget));
}

void gkGTKWidgetSetChecked(void *pointer, int checked) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_CHECK_BOX) return;
    native->suppress = TRUE;
    gtk_toggle_button_set_active(GTK_TOGGLE_BUTTON(native->widget), checked != 0);
    native->suppress = FALSE;
}

int gkGTKTextAreaReadOnly(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_TEXT_AREA &&
        !gtk_text_view_get_editable(GTK_TEXT_VIEW(native->control));
}

void gkGTKTextAreaSetReadOnly(void *pointer, int read_only) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_TEXT_AREA) {
        gtk_text_view_set_editable(GTK_TEXT_VIEW(native->control), read_only == 0);
    }
}

double gkGTKSliderValue(void *pointer) {
    GKGTKWidget *native = pointer;
    return native ? gtk_range_get_value(GTK_RANGE(native->widget)) : 0;
}

void gkGTKSliderSetValue(void *pointer, double value) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    native->suppress = TRUE;
    gtk_range_set_value(GTK_RANGE(native->widget), value);
    native->suppress = FALSE;
}

double gkGTKProgressValue(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_PROGRESS ?
        gtk_progress_bar_get_fraction(GTK_PROGRESS_BAR(native->widget)) : 0;
}

void gkGTKProgressSetValue(void *pointer, double value) {
    GKGTKWidget *native = pointer;
    if (native && native->kind == GK_GTK_WIDGET_PROGRESS) {
        gtk_progress_bar_set_fraction(GTK_PROGRESS_BAR(native->widget), CLAMP(value, 0, 1));
    }
}

int gkGTKActivityRunning(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && native->kind == GK_GTK_WIDGET_ACTIVITY ? native->running : 0;
}

void gkGTKActivitySetRunning(void *pointer, int running) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_ACTIVITY) return;
    native->running = running != 0;
    if (native->running) gtk_spinner_start(GTK_SPINNER(native->widget));
    else gtk_spinner_stop(GTK_SPINNER(native->widget));
}

uintptr_t gkGTKCanvasXID(void *pointer) {
    GKGTKWidget *native = pointer;
    if (!native) return 0;
    gtk_widget_realize(native->widget);
    GdkWindow *window = gtk_widget_get_window(native->widget);
    return window && GDK_IS_X11_WINDOW(window) ? (uintptr_t)gdk_x11_window_get_xid(window) : 0;
}

void gkGTKCanvasFramebufferSize(void *pointer, int *width, int *height) {
    GKGTKWidget *native = pointer;
    GtkAllocation allocation = {0};
    if (native) gtk_widget_get_allocation(native->widget, &allocation);
    int scale = native ? gtk_widget_get_scale_factor(native->widget) : 1;
    if (width) *width = allocation.width * scale;
    if (height) *height = allocation.height * scale;
}

void gkGTKCanvasQueueDraw(void *pointer) {
    GKGTKWidget *native = pointer;
    if (native) gtk_widget_queue_draw(native->widget);
}

int gkGTKSectionExpanded(void *pointer) {
    GKGTKWidget *native = pointer;
    return native && gtk_expander_get_expanded(GTK_EXPANDER(native->widget));
}

void gkGTKSectionSetExpanded(void *pointer, int expanded) {
    GKGTKWidget *native = pointer;
    if (!native) return;
    native->suppress = TRUE;
    gtk_expander_set_expanded(GTK_EXPANDER(native->widget), expanded != 0);
    native->suppress = FALSE;
}

void gkGTKPanelSetMovable(void *pointer, int movable) {
    GKGTKWidget *native = pointer;
    if (!native || native->kind != GK_GTK_WIDGET_PANEL) return;
    native->movable = movable != 0;
    if (!native->movable) native->dragging = FALSE;
}

void gkGTKPanelPosition(void *pointer, int *x, int *y) {
    GKGTKWidget *native = pointer;
    if (x) *x = native && native->kind == GK_GTK_WIDGET_PANEL ? native->x : 0;
    if (y) *y = native && native->kind == GK_GTK_WIDGET_PANEL ? native->y : 0;
}

void gkGTKPanelBringToFront(void *pointer) {
    gkGTKPanelRaise(pointer);
}

static void gkGTKConfigureFileChooser(GtkFileChooser *chooser,
                                      const char *directory, const char *filename,
                                      const char *extensions, gboolean saving) {
    if (directory && directory[0]) {
        gtk_file_chooser_set_current_folder(chooser, directory);
    }
    if (filename && filename[0]) {
        if (saving) {
            gtk_file_chooser_set_current_name(chooser, filename);
        } else {
            char *folder = directory && directory[0] ? g_strdup(directory) :
                                                       gtk_file_chooser_get_current_folder(chooser);
            char *path = g_path_is_absolute(filename) ? g_strdup(filename) :
                         folder ? g_build_filename(folder, filename, NULL) : NULL;
            if (path) gtk_file_chooser_set_filename(chooser, path);
            g_free(path);
            g_free(folder);
        }
    }
    if (!extensions || !extensions[0]) return;

    GtkFileFilter *supported = gtk_file_filter_new();
    gtk_file_filter_set_name(supported, "Supported files");
    gchar **values = g_strsplit(extensions, ",", -1);
    for (gchar **next = values; next && *next; next++) {
        if (!(*next)[0]) continue;
        gchar *pattern = g_strdup_printf("*.%s", *next);
        gtk_file_filter_add_pattern(supported, pattern);
        g_free(pattern);
    }
    g_strfreev(values);
    gtk_file_chooser_add_filter(chooser, supported);

    GtkFileFilter *all = gtk_file_filter_new();
    gtk_file_filter_set_name(all, "All files");
    gtk_file_filter_add_pattern(all, "*");
    gtk_file_chooser_add_filter(chooser, all);
}

static char *gkGTKPackPaths(GSList *paths, size_t *result_size) {
    size_t size = 0;
    for (GSList *next = paths; next; next = next->next) {
        const char *path = next->data;
        if (path) size += strlen(path) + 1;
    }
    if (result_size) *result_size = size;
    if (!size) return NULL;

    char *result = malloc(size);
    if (!result) return NULL;
    char *next_result = result;
    for (GSList *next = paths; next; next = next->next) {
        const char *path = next->data;
        if (!path) continue;
        size_t length = strlen(path) + 1;
        memcpy(next_result, path, length);
        next_result += length;
    }
    return result;
}

char *gkGTKOpenDialog(void *window_pointer, const char *title,
                      const char *directory, const char *filename,
                      const char *extensions, int multiple, int directories,
                      size_t *result_size, int *cancelled) {
    GKGTKWindow *window = window_pointer;
    if (result_size) *result_size = 0;
    if (cancelled) *cancelled = 1;
    if (!window) return NULL;

    GtkFileChooserAction action = directories ? GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER :
                                                GTK_FILE_CHOOSER_ACTION_OPEN;
    GtkWidget *dialog = gtk_file_chooser_dialog_new(
        title && title[0] ? title : directories ? "Select Folder" : "Open File",
        GTK_WINDOW(window->window), action,
        "_Cancel", GTK_RESPONSE_CANCEL,
        "_Open", GTK_RESPONSE_ACCEPT,
        NULL);
    GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
    gtk_file_chooser_set_select_multiple(chooser, multiple != 0);
    gkGTKConfigureFileChooser(chooser, directory, filename, extensions, FALSE);

    char *result = NULL;
    if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
        GSList *paths = gtk_file_chooser_get_filenames(chooser);
        result = gkGTKPackPaths(paths, result_size);
        g_slist_free_full(paths, g_free);
        if (cancelled) *cancelled = result == NULL;
    }
    gtk_widget_destroy(dialog);
    return result;
}

char *gkGTKSaveDialog(void *window_pointer, const char *title,
                      const char *directory, const char *filename,
                      const char *extensions, int confirm_overwrite,
                      size_t *result_size, int *cancelled) {
    GKGTKWindow *window = window_pointer;
    if (result_size) *result_size = 0;
    if (cancelled) *cancelled = 1;
    if (!window) return NULL;

    GtkWidget *dialog = gtk_file_chooser_dialog_new(
        title && title[0] ? title : "Save File",
        GTK_WINDOW(window->window), GTK_FILE_CHOOSER_ACTION_SAVE,
        "_Cancel", GTK_RESPONSE_CANCEL,
        "_Save", GTK_RESPONSE_ACCEPT,
        NULL);
    GtkFileChooser *chooser = GTK_FILE_CHOOSER(dialog);
    gtk_file_chooser_set_do_overwrite_confirmation(chooser, confirm_overwrite != 0);
    gkGTKConfigureFileChooser(chooser, directory, filename, extensions, TRUE);

    char *result = NULL;
    if (gtk_dialog_run(GTK_DIALOG(dialog)) == GTK_RESPONSE_ACCEPT) {
        char *path = gtk_file_chooser_get_filename(chooser);
        GSList *paths = path ? g_slist_append(NULL, path) : NULL;
        result = gkGTKPackPaths(paths, result_size);
        g_slist_free_full(paths, g_free);
        if (cancelled) *cancelled = result == NULL;
    }
    gtk_widget_destroy(dialog);
    return result;
}
