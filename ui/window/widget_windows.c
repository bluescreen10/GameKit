//go:build windows && cgo

#define UNICODE
#define _UNICODE
#define COBJMACROS
#include <windows.h>
#include <windowsx.h>
#include <commctrl.h>
#include <commdlg.h>
#include <shlobj.h>
#include <uxtheme.h>
#include "widget_bridge.h"
#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

enum {
    GK_UI_LAYOUT,
    GK_UI_LABEL,
    GK_UI_BUTTON,
    GK_UI_TEXT_FIELD,
    GK_UI_SLIDER,
    GK_UI_CANVAS,
    GK_UI_PANEL,
    GK_UI_SECTION,
    GK_UI_TEXT_AREA,
    GK_UI_CHECK_BOX,
    GK_UI_RADIO_GROUP,
    GK_UI_SELECT,
    GK_UI_PROGRESS,
    GK_UI_ACTIVITY,
    GK_UI_SEPARATOR,
    GK_UI_IMAGE,
    GK_UI_LINK,
    GK_UI_TABS,
    GK_UI_SCROLL_VIEW,
};

typedef struct GKUIControl GKUIControl;

struct GKUIControl {
    HWND window;
    HWND body;
    HWND header;
    GKUIControl *bodyControl;
    int kind;
    uintptr_t handle;
    wchar_t *text;
    wchar_t *placeholder;
    wchar_t *url;
    HWND *items;
    int itemCount;
    int itemCapacity;
    unsigned char *pixels;
    int imageWidth;
    int imageHeight;
    int imageStride;
    int scaling;
    int selectedIndex;
    int readOnly;
    int running;
    int suppress;
    double minimum;
    double maximum;
    double step;
    int sliderMaximum;
    int movable;
    int expanded;
    int dragging;
    int contentWidth;
    int contentHeight;
    int scrollX;
    int scrollY;
    POINT dragStart;
    RECT dragFrame;
};

static const wchar_t *gkUIClass = L"gamekitUIContainer";
static ATOM gkUIClassAtom;
static HBRUSH gkUIDarkBrush;
static HFONT gkUIFont;

static wchar_t *gkUIWide(const char *text) {
    int count = MultiByteToWideChar(CP_UTF8, 0, text ? text : "", -1, NULL, 0);
    if (count <= 0) return NULL;
    wchar_t *wide = calloc((size_t)count, sizeof(*wide));
    if (wide) MultiByteToWideChar(CP_UTF8, 0, text ? text : "", -1, wide, count);
    return wide;
}

static char *gkUIUTF8(const wchar_t *text) {
    if (!text) return _strdup("");
    int count = WideCharToMultiByte(CP_UTF8, 0, text, -1, NULL, 0, NULL, NULL);
    if (count <= 0) return _strdup("");
    char *utf8 = malloc((size_t)count);
    if (utf8) WideCharToMultiByte(CP_UTF8, 0, text, -1, utf8, count, NULL, NULL);
    return utf8;
}

static GKUIControl *gkUIFromWindow(HWND window) {
    return window ? (GKUIControl *)GetPropW(window, L"GameKitUIControl") : NULL;
}

static int gkUIDarkForWindow(HWND window) {
    HWND root = GetAncestor(window, GA_ROOT);
    return (int)(intptr_t)GetPropW(root, L"GameKitUIDark");
}

static void gkUISetControlTheme(HWND window, int dark) {
    if (!window) return;
    SetPropW(window, L"GameKitUIDark", (HANDLE)(intptr_t)dark);
    SetWindowTheme(window, dark ? L"DarkMode_Explorer" : L"Explorer", NULL);
    InvalidateRect(window, NULL, TRUE);
}

static BOOL CALLBACK gkUIThemeChild(HWND window, LPARAM dark) {
    gkUISetControlTheme(window, (int)dark);
    return TRUE;
}

void gkUIApplyTheme(HWND window, int dark) {
    if (!window) return;
    gkUISetControlTheme(window, dark);
    EnumChildWindows(window, gkUIThemeChild, (LPARAM)dark);
}

static void gkUISetFont(HWND window) {
    if (!gkUIFont) {
        NONCLIENTMETRICSW metrics = {0};
        metrics.cbSize = sizeof(metrics);
        if (SystemParametersInfoW(SPI_GETNONCLIENTMETRICS, sizeof(metrics), &metrics, 0)) {
            gkUIFont = CreateFontIndirectW(&metrics.lfMessageFont);
        }
    }
    SendMessageW(window, WM_SETFONT,
                 (WPARAM)(gkUIFont ? gkUIFont : GetStockObject(DEFAULT_GUI_FONT)), TRUE);
}

static void gkUIUpdateContainerFrame(GKUIControl *control) {
    if (!control || !control->window) return;
    RECT rect = {0};
    GetClientRect(control->window, &rect);
    int width = rect.right - rect.left;
    int height = rect.bottom - rect.top;
    if (control->kind == GK_UI_PANEL && control->body) {
        MoveWindow(control->body, 12, 32, max(0, width - 24), max(0, height - 44), TRUE);
    } else if (control->kind == GK_UI_SECTION) {
        if (control->header) MoveWindow(control->header, 0, 0, width, 28, TRUE);
        if (control->body) MoveWindow(control->body, 0, 28, width, max(0, height - 28), TRUE);
    } else if (control->kind == GK_UI_RADIO_GROUP) {
        for (int i = 0; i < control->itemCount; i++) {
            MoveWindow(control->items[i], 0, i * 24, width, 22, TRUE);
        }
    } else if (control->kind == GK_UI_TABS && control->body) {
        RECT content = rect;
        TabCtrl_AdjustRect(control->window, FALSE, &content);
        MoveWindow(control->body, content.left, content.top,
                   max(0, content.right - content.left),
                   max(0, content.bottom - content.top), TRUE);
    } else if (control->kind == GK_UI_SCROLL_VIEW && control->body) {
        int maximumX = max(0, control->contentWidth - width);
        int maximumY = max(0, control->contentHeight - height);
        control->scrollX = max(0, min(control->scrollX, maximumX));
        control->scrollY = max(0, min(control->scrollY, maximumY));
        SCROLLINFO horizontal = {sizeof(horizontal), SIF_RANGE | SIF_PAGE | SIF_POS};
        horizontal.nMin = 0;
        horizontal.nMax = max(0, control->contentWidth - 1);
        horizontal.nPage = (UINT)max(0, width);
        horizontal.nPos = control->scrollX;
        SetScrollInfo(control->window, SB_HORZ, &horizontal, TRUE);
        SCROLLINFO vertical = {sizeof(vertical), SIF_RANGE | SIF_PAGE | SIF_POS};
        vertical.nMin = 0;
        vertical.nMax = max(0, control->contentHeight - 1);
        vertical.nPage = (UINT)max(0, height);
        vertical.nPos = control->scrollY;
        SetScrollInfo(control->window, SB_VERT, &vertical, TRUE);
        MoveWindow(control->body, -control->scrollX, -control->scrollY,
                   max(width, control->contentWidth), max(height, control->contentHeight), TRUE);
    }
}

static void gkUIScrollMessage(GKUIControl *control, int bar, WPARAM wparam) {
    if (!control || control->kind != GK_UI_SCROLL_VIEW) return;
    SCROLLINFO info = {sizeof(info), SIF_ALL};
    GetScrollInfo(control->window, bar, &info);
    int position = info.nPos;
    switch (LOWORD(wparam)) {
    case SB_LINEUP: position -= 24; break;
    case SB_LINEDOWN: position += 24; break;
    case SB_PAGEUP: position -= (int)info.nPage; break;
    case SB_PAGEDOWN: position += (int)info.nPage; break;
    case SB_THUMBPOSITION:
    case SB_THUMBTRACK: position = info.nTrackPos; break;
    case SB_TOP: position = info.nMin; break;
    case SB_BOTTOM: position = info.nMax; break;
    default: return;
    }
    int maximum = max(info.nMin, info.nMax - (int)info.nPage + 1);
    position = max(info.nMin, min(position, maximum));
    if (bar == SB_HORZ) control->scrollX = position;
    else control->scrollY = position;
    gkUIUpdateContainerFrame(control);
}

static void gkUIToggleSection(GKUIControl *control, int notify) {
    if (!control || control->kind != GK_UI_SECTION) return;
    control->expanded = !control->expanded;
    if (control->body) ShowWindow(control->body, control->expanded ? SW_SHOW : SW_HIDE);
    if (control->header) {
        size_t length = wcslen(control->text ? control->text : L"") + 4;
        wchar_t *title = calloc(length, sizeof(*title));
        if (title) {
            swprintf(title, length, L"%ls  %ls", control->expanded ? L"\x25BE" : L"\x25B8",
                     control->text ? control->text : L"");
            SetWindowTextW(control->header, title);
            free(title);
        }
    }
    if (notify && control->handle) {
        gkUIGoEvent(control->handle, GK_UI_EVENT_SECTION_CHANGE);
    }
}

int gkUIProcessMessage(HWND window, UINT message, WPARAM wparam,
                       LPARAM lparam, LRESULT *result) {
    if (message == WM_COMMAND && lparam) {
        HWND child = (HWND)lparam;
        GKUIControl *control = gkUIFromWindow(child);
        int code = HIWORD(wparam);
        if (control && control->handle) {
            if ((control->kind == GK_UI_BUTTON || control->kind == GK_UI_LINK) &&
                code == BN_CLICKED) {
                gkUIGoEvent(control->handle, GK_UI_EVENT_CLICK);
                if (result) *result = 0;
                return 1;
            }
            if ((control->kind == GK_UI_TEXT_FIELD || control->kind == GK_UI_TEXT_AREA) &&
                code == EN_CHANGE && !control->suppress) {
                gkUIGoEvent(control->handle, GK_UI_EVENT_TEXT_CHANGE);
                if (result) *result = 0;
                return 1;
            }
            if (control->kind == GK_UI_SECTION && child == control->header && code == BN_CLICKED) {
                gkUIToggleSection(control, 1);
                if (result) *result = 0;
                return 1;
            }
            if (control->kind == GK_UI_CHECK_BOX && code == BN_CLICKED) {
                gkUIGoEvent(control->handle, GK_UI_EVENT_TOGGLE_CHANGE);
                if (result) *result = 0;
                return 1;
            }
            if (control->kind == GK_UI_RADIO_GROUP && code == BN_CLICKED) {
                int selected = GetDlgCtrlID(child) - 1000;
                if (selected >= 0 && selected < control->itemCount) {
                    control->selectedIndex = selected;
                    for (int i = 0; i < control->itemCount; i++) {
                        SendMessageW(control->items[i], BM_SETCHECK,
                                     i == selected ? BST_CHECKED : BST_UNCHECKED, 0);
                    }
                    gkUIGoEvent(control->handle, GK_UI_EVENT_SELECTION_CHANGE);
                }
                if (result) *result = 0;
                return 1;
            }
            if (control->kind == GK_UI_SELECT && code == CBN_SELCHANGE) {
                gkUIGoEvent(control->handle, GK_UI_EVENT_SELECTION_CHANGE);
                if (result) *result = 0;
                return 1;
            }
        }
    } else if (message == WM_NOTIFY && lparam) {
        NMHDR *header = (NMHDR *)lparam;
        GKUIControl *control = gkUIFromWindow(header->hwndFrom);
        if (control && control->handle && control->kind == GK_UI_LINK &&
            (header->code == NM_CLICK || header->code == NM_RETURN)) {
            gkUIGoEvent(control->handle, GK_UI_EVENT_CLICK);
            if (result) *result = 0;
            return 1;
        }
        if (control && control->handle && control->kind == GK_UI_TABS &&
            header->code == TCN_SELCHANGE) {
            gkUIGoEvent(control->handle, GK_UI_EVENT_TAB_CHANGE);
            if (result) *result = 0;
            return 1;
        }
    } else if (message == WM_HSCROLL && lparam) {
        GKUIControl *control = gkUIFromWindow((HWND)lparam);
        if (control && control->kind == GK_UI_SLIDER && control->handle) {
            gkUIGoEvent(control->handle, GK_UI_EVENT_VALUE_CHANGE);
            if (result) *result = 0;
            return 1;
        }
    } else if (message == WM_CTLCOLORSTATIC || message == WM_CTLCOLOREDIT) {
        HDC dc = (HDC)wparam;
        if (gkUIDarkForWindow(window)) {
            SetTextColor(dc, RGB(238, 238, 238));
            SetBkColor(dc, RGB(32, 32, 32));
            if (!gkUIDarkBrush) gkUIDarkBrush = CreateSolidBrush(RGB(32, 32, 32));
            if (result) *result = (LRESULT)gkUIDarkBrush;
            return 1;
        }
        SetTextColor(dc, GetSysColor(COLOR_WINDOWTEXT));
        SetBkColor(dc, GetSysColor(COLOR_WINDOW));
        if (result) *result = (LRESULT)GetSysColorBrush(COLOR_WINDOW);
        return 1;
    }
    return 0;
}

static LRESULT CALLBACK gkUIContainerProc(HWND window, UINT message,
                                           WPARAM wparam, LPARAM lparam) {
    GKUIControl *control = gkUIFromWindow(window);
    if (message == WM_NCCREATE) {
        CREATESTRUCTW *create = (CREATESTRUCTW *)lparam;
        control = (GKUIControl *)create->lpCreateParams;
        control->window = window;
        SetPropW(window, L"GameKitUIControl", control);
    }
    LRESULT result = 0;
    if (gkUIProcessMessage(window, message, wparam, lparam, &result)) return result;
    if (message == WM_SIZE) {
        gkUIUpdateContainerFrame(control);
    } else if ((message == WM_HSCROLL || message == WM_VSCROLL) &&
               control && control->kind == GK_UI_SCROLL_VIEW && !lparam) {
        gkUIScrollMessage(control, message == WM_HSCROLL ? SB_HORZ : SB_VERT, wparam);
        return 0;
    } else if (message == WM_MOUSEWHEEL && control &&
               control->kind == GK_UI_SCROLL_VIEW) {
        int steps = GET_WHEEL_DELTA_WPARAM(wparam) / WHEEL_DELTA;
        int bar = (GET_KEYSTATE_WPARAM(wparam) & MK_SHIFT) ? SB_HORZ : SB_VERT;
        if (bar == SB_VERT && !(GetWindowLongPtrW(window, GWL_STYLE) & WS_VSCROLL)) {
            bar = SB_HORZ;
        }
        SCROLLINFO info = {sizeof(info), SIF_ALL};
        GetScrollInfo(window, bar, &info);
        int maximum = max(info.nMin, info.nMax - (int)info.nPage + 1);
        int position = max(info.nMin, min(info.nPos - steps * 72, maximum));
        if (bar == SB_HORZ) control->scrollX = position;
        else control->scrollY = position;
        gkUIUpdateContainerFrame(control);
        return 0;
    } else if (message == WM_LBUTTONDOWN && control && control->kind == GK_UI_PANEL &&
               control->movable && GET_Y_LPARAM(lparam) < 32) {
        control->dragging = 1;
        GetCursorPos(&control->dragStart);
        GetWindowRect(window, &control->dragFrame);
        HWND parent = GetParent(window);
        MapWindowPoints(HWND_DESKTOP, parent, (POINT *)&control->dragFrame, 2);
        SetCapture(window);
        SetWindowPos(window, HWND_TOP, 0, 0, 0, 0,
                     SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
        return 0;
    } else if (message == WM_MOUSEMOVE && control && control->dragging) {
        HWND parent = GetParent(window);
        RECT parentRect = {0};
        RECT frame = control->dragFrame;
        GetClientRect(parent, &parentRect);
        int width = frame.right - frame.left;
        int height = frame.bottom - frame.top;
        POINT cursor;
        GetCursorPos(&cursor);
        int x = frame.left + cursor.x - control->dragStart.x;
        int y = frame.top + cursor.y - control->dragStart.y;
        x = max(0, min(x, max(0, parentRect.right - width)));
        y = max(0, min(y, max(0, parentRect.bottom - height)));
        SetWindowPos(window, HWND_TOP, x, y, 0, 0, SWP_NOSIZE | SWP_NOACTIVATE);
        if (control->handle) gkUIGoEvent(control->handle, GK_UI_EVENT_PANEL_MOVE);
        return 0;
    } else if (message == WM_LBUTTONUP && control && control->dragging) {
        control->dragging = 0;
        ReleaseCapture();
        return 0;
    } else if (message == WM_CAPTURECHANGED && control) {
        control->dragging = 0;
    } else if (message == WM_ERASEBKGND && control) {
        RECT rect;
        GetClientRect(window, &rect);
        int dark = gkUIDarkForWindow(window);
        HBRUSH brush = dark ? (gkUIDarkBrush ? gkUIDarkBrush :
            (gkUIDarkBrush = CreateSolidBrush(RGB(32, 32, 32)))) : GetSysColorBrush(COLOR_WINDOW);
        FillRect((HDC)wparam, &rect, brush);
        return 1;
    } else if (message == WM_PAINT && control && control->kind == GK_UI_IMAGE) {
        PAINTSTRUCT paint;
        HDC dc = BeginPaint(window, &paint);
        RECT bounds;
        GetClientRect(window, &bounds);
        FillRect(dc, &bounds, GetSysColorBrush(COLOR_WINDOW));
        if (control->pixels && control->imageWidth > 0 && control->imageHeight > 0) {
            int availableWidth = bounds.right - bounds.left;
            int availableHeight = bounds.bottom - bounds.top;
            int drawWidth = availableWidth;
            int drawHeight = availableHeight;
            int x = 0;
            int y = 0;
            if (control->scaling != 2) {
                double sx = (double)availableWidth / control->imageWidth;
                double sy = (double)availableHeight / control->imageHeight;
                double scale = control->scaling == 1 ? max(sx, sy) : min(sx, sy);
                drawWidth = (int)lround(control->imageWidth * scale);
                drawHeight = (int)lround(control->imageHeight * scale);
                x = (availableWidth - drawWidth) / 2;
                y = (availableHeight - drawHeight) / 2;
            }
            BITMAPINFO info = {0};
            info.bmiHeader.biSize = sizeof(info.bmiHeader);
            info.bmiHeader.biWidth = control->imageWidth;
            info.bmiHeader.biHeight = -control->imageHeight;
            info.bmiHeader.biPlanes = 1;
            info.bmiHeader.biBitCount = 32;
            info.bmiHeader.biCompression = BI_RGB;
            SetStretchBltMode(dc, HALFTONE);
            StretchDIBits(dc, x, y, drawWidth, drawHeight, 0, 0,
                          control->imageWidth, control->imageHeight,
                          control->pixels, &info, DIB_RGB_COLORS, SRCCOPY);
        }
        EndPaint(window, &paint);
        return 0;
    } else if (message == WM_PAINT && control && control->kind == GK_UI_PANEL) {
        PAINTSTRUCT paint;
        HDC dc = BeginPaint(window, &paint);
        RECT rect;
        GetClientRect(window, &rect);
        int dark = gkUIDarkForWindow(window);
        SetBkMode(dc, TRANSPARENT);
        SetTextColor(dc, dark ? RGB(238, 238, 238) : GetSysColor(COLOR_WINDOWTEXT));
        SelectObject(dc, gkUIFont ? gkUIFont : GetStockObject(DEFAULT_GUI_FONT));
        RECT title = {12, 7, max(12, rect.right - 12), 27};
        DrawTextW(dc, control->text ? control->text : L"", -1, &title,
                  DT_SINGLELINE | DT_VCENTER | DT_END_ELLIPSIS);
        HPEN pen = CreatePen(PS_SOLID, 1, dark ? RGB(72, 72, 72) : GetSysColor(COLOR_3DSHADOW));
        HGDIOBJ previous = SelectObject(dc, pen);
        MoveToEx(dc, 0, 31, NULL);
        LineTo(dc, rect.right, 31);
        SelectObject(dc, previous);
        DeleteObject(pen);
        EndPaint(window, &paint);
        return 0;
    } else if (message == WM_NCDESTROY) {
        RemovePropW(window, L"GameKitUIControl");
    }
    return DefWindowProcW(window, message, wparam, lparam);
}

static int gkUIRegisterClass(void) {
    if (gkUIClassAtom) return 1;
    WNDCLASSEXW cls = {0};
    cls.cbSize = sizeof(cls);
    cls.style = CS_HREDRAW | CS_VREDRAW | CS_OWNDC;
    cls.lpfnWndProc = gkUIContainerProc;
    cls.hInstance = GetModuleHandleW(NULL);
    cls.hCursor = LoadCursorW(NULL, IDC_ARROW);
    cls.hbrBackground = GetSysColorBrush(COLOR_WINDOW);
    cls.lpszClassName = gkUIClass;
    gkUIClassAtom = RegisterClassExW(&cls);
    return gkUIClassAtom != 0 || GetLastError() == ERROR_CLASS_ALREADY_EXISTS;
}

static GKUIControl *gkUICreateControl(uintptr_t parent, int kind,
                                      const wchar_t *className, const wchar_t *text,
                                      DWORD style, DWORD exStyle) {
    GKUIControl *control = calloc(1, sizeof(*control));
    if (!control) return NULL;
    control->kind = kind;
    control->text = _wcsdup(text ? text : L"");
    HWND window = CreateWindowExW(exStyle, className, text ? text : L"", style | WS_CHILD | WS_VISIBLE,
                                  0, 0, 0, 0, (HWND)parent, NULL, GetModuleHandleW(NULL),
                                  className == gkUIClass ? control : NULL);
    if (!window) {
        free(control->text);
        free(control);
        return NULL;
    }
    control->window = window;
    SetPropW(window, L"GameKitUIControl", control);
    gkUISetFont(window);
    gkUISetControlTheme(window, gkUIDarkForWindow((HWND)parent));
    return control;
}

static GKUIControl *gkUICreateContainer(uintptr_t parent, int kind) {
    if (!gkUIRegisterClass()) return NULL;
    return gkUICreateControl(parent, kind, gkUIClass, L"", WS_CLIPCHILDREN | WS_CLIPSIBLINGS, 0);
}

void *gkUICreateLayout(uintptr_t window) {
    return gkUICreateContainer(window, GK_UI_LAYOUT);
}

void *gkUICreateLabel(uintptr_t window, const char *text) {
    wchar_t *wide = gkUIWide(text);
    GKUIControl *control = gkUICreateControl(window, GK_UI_LABEL, L"STATIC", wide,
                                             SS_LEFT | SS_NOPREFIX, 0);
    free(wide);
    return control;
}

void *gkUICreateButton(uintptr_t window, const char *text, uintptr_t handle) {
    wchar_t *wide = gkUIWide(text);
    GKUIControl *control = gkUICreateControl(window, GK_UI_BUTTON, L"BUTTON", wide,
                                             BS_PUSHBUTTON | WS_TABSTOP, 0);
    free(wide);
    if (control) control->handle = handle;
    return control;
}

void *gkUICreateTextField(uintptr_t window, uintptr_t handle) {
    GKUIControl *control = gkUICreateControl(window, GK_UI_TEXT_FIELD, L"EDIT", L"",
                                             ES_AUTOHSCROLL | WS_TABSTOP | WS_BORDER,
                                             WS_EX_CLIENTEDGE);
    if (control) control->handle = handle;
    return control;
}

void *gkUICreateTextArea(uintptr_t window, const char *text,
                         const char *placeholder, int readOnly, int wrap,
                         uintptr_t handle) {
    wchar_t *wide = gkUIWide(text);
    DWORD style = ES_MULTILINE | ES_AUTOVSCROLL | ES_WANTRETURN | WS_TABSTOP |
                  WS_BORDER | WS_VSCROLL;
    if (!wrap) style |= ES_AUTOHSCROLL | WS_HSCROLL;
    if (readOnly) style |= ES_READONLY;
    GKUIControl *control = gkUICreateControl(window, GK_UI_TEXT_AREA, L"EDIT", wide,
                                             style, WS_EX_CLIENTEDGE);
    free(wide);
    if (!control) return NULL;
    control->handle = handle;
    control->readOnly = readOnly != 0;
    wchar_t *hint = gkUIWide(placeholder);
    control->placeholder = hint;
    if (hint) SendMessageW(control->window, EM_SETCUEBANNER, TRUE, (LPARAM)hint);
    return control;
}

void *gkUICreateCheckBox(uintptr_t window, const char *text, int checked,
                         uintptr_t handle) {
    wchar_t *wide = gkUIWide(text);
    GKUIControl *control = gkUICreateControl(window, GK_UI_CHECK_BOX, L"BUTTON", wide,
                                             BS_AUTOCHECKBOX | WS_TABSTOP, 0);
    free(wide);
    if (control) {
        control->handle = handle;
        SendMessageW(control->window, BM_SETCHECK,
                     checked ? BST_CHECKED : BST_UNCHECKED, 0);
    }
    return control;
}

void *gkUICreateRadioGroup(uintptr_t window, uintptr_t handle) {
    GKUIControl *control = gkUICreateContainer(window, GK_UI_RADIO_GROUP);
    if (control) {
        control->handle = handle;
        control->selectedIndex = -1;
    }
    return control;
}

void *gkUICreateSelect(uintptr_t window, uintptr_t handle) {
    GKUIControl *control = gkUICreateControl(window, GK_UI_SELECT, L"COMBOBOX", L"",
                                             CBS_DROPDOWNLIST | CBS_HASSTRINGS |
                                             WS_TABSTOP | WS_VSCROLL, 0);
    if (control) control->handle = handle;
    return control;
}

static int gkUIAppendWindow(GKUIControl *control, HWND item) {
    if (control->itemCount == control->itemCapacity) {
        int capacity = control->itemCapacity ? control->itemCapacity * 2 : 4;
        HWND *items = realloc(control->items, (size_t)capacity * sizeof(*items));
        if (!items) return 0;
        control->items = items;
        control->itemCapacity = capacity;
    }
    control->items[control->itemCount++] = item;
    return 1;
}

void gkUISelectionAddItem(void *pointer, const char *text) {
    GKUIControl *control = pointer;
    if (!control) return;
    wchar_t *wide = gkUIWide(text);
    if (!wide) return;
    if (control->kind == GK_UI_SELECT) {
        SendMessageW(control->window, CB_ADDSTRING, 0, (LPARAM)wide);
    } else if (control->kind == GK_UI_RADIO_GROUP) {
        int index = control->itemCount;
        HWND button = CreateWindowExW(0, L"BUTTON", wide,
            WS_CHILD | WS_VISIBLE | WS_TABSTOP | BS_AUTORADIOBUTTON,
            0, index * 24, 0, 22, control->window,
            (HMENU)(intptr_t)(1000 + index), GetModuleHandleW(NULL), NULL);
        if (button && gkUIAppendWindow(control, button)) {
            SetPropW(button, L"GameKitUIControl", control);
            gkUISetFont(button);
            gkUISetControlTheme(button, gkUIDarkForWindow(control->window));
            gkUIUpdateContainerFrame(control);
        } else if (button) {
            DestroyWindow(button);
        }
    }
    free(wide);
}

int gkUISelectionIndex(void *pointer) {
    GKUIControl *control = pointer;
    if (!control) return -1;
    if (control->kind == GK_UI_SELECT) {
        return (int)SendMessageW(control->window, CB_GETCURSEL, 0, 0);
    }
    return control->kind == GK_UI_RADIO_GROUP ? control->selectedIndex : -1;
}

void gkUISetSelectionIndex(void *pointer, int index) {
    GKUIControl *control = pointer;
    if (!control) return;
    if (control->kind == GK_UI_SELECT) {
        SendMessageW(control->window, CB_SETCURSEL, index, 0);
    } else if (control->kind == GK_UI_RADIO_GROUP &&
               index >= 0 && index < control->itemCount) {
        control->selectedIndex = index;
        for (int i = 0; i < control->itemCount; i++) {
            SendMessageW(control->items[i], BM_SETCHECK,
                         i == index ? BST_CHECKED : BST_UNCHECKED, 0);
        }
    }
}

void *gkUICreateSlider(uintptr_t window, double minimum, double maximum,
                       double value, double step, uintptr_t handle) {
    INITCOMMONCONTROLSEX common = {sizeof(common), ICC_BAR_CLASSES};
    InitCommonControlsEx(&common);
    GKUIControl *control = gkUICreateControl(window, GK_UI_SLIDER, TRACKBAR_CLASSW, L"",
                                             TBS_HORZ | TBS_AUTOTICKS | WS_TABSTOP, 0);
    if (!control) return NULL;
    control->handle = handle;
    control->minimum = minimum;
    control->maximum = maximum;
    control->step = step;
    double range = maximum - minimum;
    int positions = step > 0 ? (int)floor(range / step + 0.5) : 10000;
    control->sliderMaximum = max(1, min(positions, 1000000));
    SendMessageW(control->window, TBM_SETRANGEMIN, FALSE, 0);
    SendMessageW(control->window, TBM_SETRANGEMAX, FALSE, control->sliderMaximum);
    int position = (int)llround((value - minimum) / range * control->sliderMaximum);
    SendMessageW(control->window, TBM_SETPOS, TRUE, position);
    return control;
}

void *gkUICreateProgressBar(uintptr_t window, double value) {
    INITCOMMONCONTROLSEX common = {sizeof(common), ICC_PROGRESS_CLASS};
    InitCommonControlsEx(&common);
    GKUIControl *control = gkUICreateControl(window, GK_UI_PROGRESS,
                                             PROGRESS_CLASSW, L"", 0, 0);
    if (!control) return NULL;
    SendMessageW(control->window, PBM_SETRANGE32, 0, 10000);
    SendMessageW(control->window, PBM_SETPOS, (WPARAM)lround(value * 10000), 0);
    return control;
}

void *gkUICreateActivityIndicator(uintptr_t window, int running) {
    INITCOMMONCONTROLSEX common = {sizeof(common), ICC_PROGRESS_CLASS};
    InitCommonControlsEx(&common);
    GKUIControl *control = gkUICreateControl(window, GK_UI_ACTIVITY,
                                             PROGRESS_CLASSW, L"", PBS_MARQUEE, 0);
    if (!control) return NULL;
    control->running = running != 0;
    SendMessageW(control->window, PBM_SETMARQUEE, running != 0, 30);
    return control;
}

void *gkUICreateSeparator(uintptr_t window, int direction) {
    DWORD style = direction == 0 ? SS_ETCHEDHORZ : SS_ETCHEDVERT;
    GKUIControl *control = gkUICreateControl(window, GK_UI_SEPARATOR,
                                             L"STATIC", L"", style, 0);
    if (control) control->selectedIndex = direction;
    return control;
}

void *gkUICreateImage(uintptr_t window, int scaling) {
    GKUIControl *control = gkUICreateContainer(window, GK_UI_IMAGE);
    if (control) control->scaling = scaling;
    return control;
}

void gkUIImageSet(void *pointer, const unsigned char *pixels,
                  int width, int height, int stride) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_IMAGE || !pixels ||
        width <= 0 || height <= 0 || stride < width * 4) return;
    unsigned char *copy = malloc((size_t)width * height * 4);
    if (!copy) return;
    for (int y = 0; y < height; y++) {
        const unsigned char *source = pixels + y * stride;
        unsigned char *destination = copy + (size_t)y * width * 4;
        for (int x = 0; x < width; x++) {
            destination[x * 4 + 0] = source[x * 4 + 2];
            destination[x * 4 + 1] = source[x * 4 + 1];
            destination[x * 4 + 2] = source[x * 4 + 0];
            destination[x * 4 + 3] = source[x * 4 + 3];
        }
    }
    free(control->pixels);
    control->pixels = copy;
    control->imageWidth = width;
    control->imageHeight = height;
    control->imageStride = width * 4;
    InvalidateRect(control->window, NULL, TRUE);
}

int gkUIImageScaling(void *pointer) {
    GKUIControl *control = pointer;
    return control && control->kind == GK_UI_IMAGE ? control->scaling : 0;
}

void gkUIImageSetScaling(void *pointer, int scaling) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_IMAGE) return;
    control->scaling = scaling;
    InvalidateRect(control->window, NULL, TRUE);
}

static void gkUIUpdateLinkText(GKUIControl *control) {
    if (!control || control->kind != GK_UI_LINK) return;
    const wchar_t *text = control->text ? control->text : L"";
    const wchar_t *url = control->url ? control->url : L"";
    size_t size = wcslen(text) + wcslen(url) + 32;
    wchar_t *markup = calloc(size, sizeof(*markup));
    if (!markup) return;
    swprintf(markup, size, L"<a href=\"%ls\">%ls</a>", url, text);
    SetWindowTextW(control->window, markup);
    free(markup);
}

void *gkUICreateLink(uintptr_t window, const char *text,
                     const char *url, uintptr_t handle) {
    INITCOMMONCONTROLSEX common = {sizeof(common), ICC_LINK_CLASS};
    InitCommonControlsEx(&common);
    GKUIControl *control = gkUICreateControl(window, GK_UI_LINK,
                                             WC_LINK, L"", WS_TABSTOP, 0);
    if (!control) return NULL;
    free(control->text);
    control->text = gkUIWide(text);
    control->url = gkUIWide(url);
    control->handle = handle;
    gkUIUpdateLinkText(control);
    return control;
}

void *gkUICreateTabs(uintptr_t window, uintptr_t handle) {
    INITCOMMONCONTROLSEX common = {sizeof(common), ICC_TAB_CLASSES};
    InitCommonControlsEx(&common);
    GKUIControl *control = gkUICreateControl(window, GK_UI_TABS,
                                             WC_TABCONTROLW, L"",
                                             WS_TABSTOP | WS_CLIPCHILDREN, 0);
    if (!control) return NULL;
    control->handle = handle;
    control->selectedIndex = 0;
    GKUIControl *body = gkUICreateContainer((uintptr_t)control->window, GK_UI_LAYOUT);
    if (!body) {
        DestroyWindow(control->window);
        free(control->text);
        free(control);
        return NULL;
    }
    control->body = body->window;
    control->bodyControl = body;
    gkUIUpdateContainerFrame(control);
    return control;
}

int gkUITabsAdd(void *tabsPointer, const char *title, void *contentPointer) {
    GKUIControl *tabs = tabsPointer;
    GKUIControl *content = contentPointer;
    if (!tabs || tabs->kind != GK_UI_TABS || !content) return 0;
    wchar_t *wide = gkUIWide(title);
    if (!wide) return 0;
    TCITEMW item = {0};
    item.mask = TCIF_TEXT;
    item.pszText = wide;
    int index = TabCtrl_GetItemCount(tabs->window);
    int added = TabCtrl_InsertItem(tabs->window, index, &item) >= 0;
    free(wide);
    if (!added || !gkUIAppendWindow(tabs, content->window)) return 0;
    SetParent(content->window, tabs->body);
    ShowWindow(content->window, index == tabs->selectedIndex ? SW_SHOW : SW_HIDE);
    return 1;
}

int gkUITabsSelected(void *pointer) {
    GKUIControl *tabs = pointer;
    return tabs && tabs->kind == GK_UI_TABS ? TabCtrl_GetCurSel(tabs->window) : -1;
}

void gkUITabsSetSelected(void *pointer, int index) {
    GKUIControl *tabs = pointer;
    if (!tabs || tabs->kind != GK_UI_TABS || index < 0 || index >= tabs->itemCount) return;
    tabs->selectedIndex = index;
    TabCtrl_SetCurSel(tabs->window, index);
    for (int i = 0; i < tabs->itemCount; i++) {
        ShowWindow(tabs->items[i], i == index ? SW_SHOW : SW_HIDE);
    }
}

void gkUITabsContentSize(void *pointer, int *width, int *height) {
    GKUIControl *tabs = pointer;
    RECT rect = {0};
    if (tabs && tabs->body) GetClientRect(tabs->body, &rect);
    if (width) *width = rect.right - rect.left;
    if (height) *height = rect.bottom - rect.top;
}

void *gkUICreateScrollView(uintptr_t window, int axes) {
    GKUIControl *control = gkUICreateContainer(window, GK_UI_SCROLL_VIEW);
    if (!control) return NULL;
    LONG_PTR style = GetWindowLongPtrW(control->window, GWL_STYLE);
    if (axes == 0 || axes == 2) style |= WS_VSCROLL;
    if (axes == 1 || axes == 2) style |= WS_HSCROLL;
    SetWindowLongPtrW(control->window, GWL_STYLE, style);
    SetWindowLongPtrW(control->window, GWL_EXSTYLE,
                      GetWindowLongPtrW(control->window, GWL_EXSTYLE) | WS_EX_CLIENTEDGE);
    GKUIControl *body = gkUICreateContainer((uintptr_t)control->window, GK_UI_LAYOUT);
    if (!body) {
        DestroyWindow(control->window);
        free(control->text);
        free(control);
        return NULL;
    }
    control->body = body->window;
    control->bodyControl = body;
    SetWindowPos(control->window, NULL, 0, 0, 0, 0,
                 SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
    return control;
}

void gkUIScrollViewportSize(void *pointer, int *width, int *height) {
    GKUIControl *control = pointer;
    RECT rect = {0};
    if (control && control->kind == GK_UI_SCROLL_VIEW) GetClientRect(control->window, &rect);
    if (width) *width = rect.right - rect.left;
    if (height) *height = rect.bottom - rect.top;
}

void gkUIScrollSetContentSize(void *pointer, int width, int height) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_SCROLL_VIEW) return;
    control->contentWidth = max(0, width);
    control->contentHeight = max(0, height);
    gkUIUpdateContainerFrame(control);
}

void gkUIScrollOffset(void *pointer, int *x, int *y) {
    GKUIControl *control = pointer;
    if (x) *x = control && control->kind == GK_UI_SCROLL_VIEW ? control->scrollX : 0;
    if (y) *y = control && control->kind == GK_UI_SCROLL_VIEW ? control->scrollY : 0;
}

void gkUIScrollSetOffset(void *pointer, int x, int y) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_SCROLL_VIEW) return;
    control->scrollX = max(0, x);
    control->scrollY = max(0, y);
    gkUIUpdateContainerFrame(control);
}

void *gkUICreateCanvas(uintptr_t window) {
    return gkUICreateContainer(window, GK_UI_CANVAS);
}

void *gkUICreatePanel(uintptr_t window, const char *title, int movable,
                      uintptr_t handle) {
    GKUIControl *control = gkUICreateContainer(window, GK_UI_PANEL);
    if (!control) return NULL;
    SetWindowLongPtrW(control->window, GWL_EXSTYLE,
                      GetWindowLongPtrW(control->window, GWL_EXSTYLE) | WS_EX_CLIENTEDGE);
    SetWindowPos(control->window, NULL, 0, 0, 0, 0,
                 SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
    free(control->text);
    control->text = gkUIWide(title);
    control->movable = movable != 0;
    control->handle = handle;
    GKUIControl *body = gkUICreateContainer((uintptr_t)control->window, GK_UI_LAYOUT);
    if (!body) {
        DestroyWindow(control->window);
        free(control->text);
        free(control);
        return NULL;
    }
    control->body = body->window;
    control->bodyControl = body;
    return control;
}

void *gkUICreateSection(uintptr_t window, const char *title, int expanded,
                        uintptr_t handle) {
    GKUIControl *control = gkUICreateContainer(window, GK_UI_SECTION);
    if (!control) return NULL;
    free(control->text);
    control->text = gkUIWide(title);
    control->expanded = expanded != 0;
    control->handle = handle;
    control->header = CreateWindowExW(0, L"BUTTON", L"", WS_CHILD | WS_VISIBLE | BS_PUSHBUTTON,
                                      0, 0, 0, 0, control->window, NULL,
                                      GetModuleHandleW(NULL), NULL);
    GKUIControl *body = gkUICreateContainer((uintptr_t)control->window, GK_UI_LAYOUT);
    if (!control->header || !body) {
        if (body) {
            DestroyWindow(body->window);
            free(body->text);
            free(body);
        }
        DestroyWindow(control->window);
        free(control->text);
        free(control);
        return NULL;
    }
    control->body = body->window;
    control->bodyControl = body;
    SetPropW(control->header, L"GameKitUIControl", control);
    gkUISetFont(control->header);
    gkUISetControlTheme(control->header, gkUIDarkForWindow((HWND)window));
    control->expanded = !control->expanded;
    gkUIToggleSection(control, 0);
    gkUIUpdateContainerFrame(control);
    return control;
}

uintptr_t gkUIControlHandle(void *pointer) {
    GKUIControl *control = pointer;
    return control && IsWindow(control->window) ? (uintptr_t)control->window : 0;
}

void gkUIDestroy(void *pointer) {
    GKUIControl *control = pointer;
    if (!control) return;
    if (control->window && IsWindow(control->window)) DestroyWindow(control->window);
    if (control->bodyControl) {
        free(control->bodyControl->text);
        free(control->bodyControl->placeholder);
        free(control->bodyControl);
    }
    free(control->text);
    free(control->placeholder);
    free(control->url);
    free(control->items);
    free(control->pixels);
    free(control);
}

void gkUIClearAction(void *pointer) {
    GKUIControl *control = pointer;
    if (control) control->handle = 0;
}

void gkUIAttachToWindow(uintptr_t window, void *pointer) {
    GKUIControl *control = pointer;
    if (control && control->window) SetParent(control->window, (HWND)window);
}

void gkUISetParent(void *pointer, void *parentPointer) {
    GKUIControl *control = pointer;
    GKUIControl *parent = parentPointer;
    if (!control || !parent) return;
    HWND destination = parent->body ? parent->body : parent->window;
    SetParent(control->window, destination);
}

void gkUIBringToFront(void *pointer) {
    GKUIControl *control = pointer;
    if (control && control->window) {
        SetWindowPos(control->window, HWND_TOP, 0, 0, 0, 0,
                     SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    }
}

void gkUISetFrame(void *pointer, int x, int y, int width, int height) {
    GKUIControl *control = pointer;
    if (!control || !control->window) return;
    int nativeHeight = control->kind == GK_UI_SELECT ? max(200, height) : height;
    width = max(0, width);
    nativeHeight = max(0, nativeHeight);
    RECT current = {0};
    GetWindowRect(control->window, &current);
    MapWindowPoints(HWND_DESKTOP, GetParent(control->window), (POINT *)&current, 2);
    if (current.left == x && current.top == y &&
        current.right - current.left == width &&
        current.bottom - current.top == nativeHeight) return;
    MoveWindow(control->window, x, y, width, nativeHeight, TRUE);
    gkUIUpdateContainerFrame(control);
}

void gkUISetVisible(void *pointer, int visible) {
    GKUIControl *control = pointer;
    if (control && control->window) ShowWindow(control->window, visible ? SW_SHOW : SW_HIDE);
}

static BOOL CALLBACK gkUIEnableChild(HWND window, LPARAM enabled) {
    EnableWindow(window, (BOOL)enabled);
    return TRUE;
}

void gkUISetEnabled(void *pointer, int enabled) {
    GKUIControl *control = pointer;
    if (!control || !control->window) return;
    EnableWindow(control->window, enabled != 0);
    EnumChildWindows(control->window, gkUIEnableChild, (LPARAM)(enabled != 0));
}

void gkUIPreferredSize(void *pointer, int *width, int *height) {
    GKUIControl *control = pointer;
    int resultWidth = 0;
    int resultHeight = 0;
    if (control && control->window) {
        if (control->kind == GK_UI_LABEL || control->kind == GK_UI_BUTTON ||
            control->kind == GK_UI_CHECK_BOX || control->kind == GK_UI_LINK) {
            HDC dc = GetDC(control->window);
            HGDIOBJ previous = SelectObject(dc, (HGDIOBJ)SendMessageW(control->window, WM_GETFONT, 0, 0));
            RECT rect = {0};
            DrawTextW(dc, control->text ? control->text : L"", -1, &rect,
                      DT_CALCRECT | DT_SINGLELINE | DT_NOPREFIX);
            SelectObject(dc, previous);
            ReleaseDC(control->window, dc);
            resultWidth = rect.right - rect.left;
            resultHeight = rect.bottom - rect.top;
            if (control->kind == GK_UI_BUTTON) {
                resultWidth = max(75, resultWidth + 28);
                resultHeight = max(24, resultHeight + 10);
            } else if (control->kind == GK_UI_CHECK_BOX) {
                resultWidth += 24;
                resultHeight = max(20, resultHeight);
            }
        } else if (control->kind == GK_UI_TEXT_FIELD) {
            resultWidth = 120;
            resultHeight = 24;
        } else if (control->kind == GK_UI_TEXT_AREA) {
            resultWidth = 200;
            resultHeight = 96;
        } else if (control->kind == GK_UI_SLIDER) {
            resultWidth = 120;
            resultHeight = 30;
        } else if (control->kind == GK_UI_RADIO_GROUP) {
            resultWidth = 140;
            resultHeight = max(24, control->itemCount * 24);
        } else if (control->kind == GK_UI_SELECT) {
            resultWidth = 140;
            resultHeight = 28;
        } else if (control->kind == GK_UI_PROGRESS) {
            resultWidth = 140;
            resultHeight = 18;
        } else if (control->kind == GK_UI_ACTIVITY) {
            resultWidth = 80;
            resultHeight = 18;
        } else if (control->kind == GK_UI_SEPARATOR) {
            resultWidth = control->selectedIndex == 0 ? 80 : 2;
            resultHeight = control->selectedIndex == 0 ? 2 : 80;
        } else if (control->kind == GK_UI_IMAGE) {
            resultWidth = control->imageWidth;
            resultHeight = control->imageHeight;
        } else if (control->kind == GK_UI_TABS) {
            resultWidth = 320;
            resultHeight = 240;
        } else if (control->kind == GK_UI_SCROLL_VIEW) {
            resultWidth = 320;
            resultHeight = 240;
        }
    }
    if (width) *width = resultWidth;
    if (height) *height = resultHeight;
}

void gkUIFramebufferSize(void *pointer, int *width, int *height) {
    GKUIControl *control = pointer;
    RECT rect = {0};
    if (control && control->window) GetClientRect(control->window, &rect);
    if (width) *width = rect.right - rect.left;
    if (height) *height = rect.bottom - rect.top;
}

char *gkUIText(void *pointer) {
    GKUIControl *control = pointer;
    if (!control) return _strdup("");
    if (control->kind == GK_UI_TEXT_FIELD || control->kind == GK_UI_TEXT_AREA) {
        int length = GetWindowTextLengthW(control->window);
        wchar_t *text = calloc((size_t)length + 1, sizeof(*text));
        if (!text) return NULL;
        GetWindowTextW(control->window, text, length + 1);
        char *result = gkUIUTF8(text);
        free(text);
        return result;
    }
    return gkUIUTF8(control->text);
}

void gkUISetText(void *pointer, const char *text) {
    GKUIControl *control = pointer;
    if (!control) return;
    wchar_t *wide = gkUIWide(text);
    if (!wide) return;
    free(control->text);
    control->text = _wcsdup(wide);
    control->suppress = 1;
    if (control->kind == GK_UI_LINK) {
        gkUIUpdateLinkText(control);
    } else if (control->kind == GK_UI_SECTION) {
        control->expanded = !control->expanded;
        gkUIToggleSection(control, 0);
    } else if (control->kind == GK_UI_PANEL) {
        InvalidateRect(control->window, NULL, TRUE);
    } else {
        SetWindowTextW(control->window, wide);
    }
    control->suppress = 0;
    free(wide);
}

void gkUISetPlaceholder(void *pointer, const char *text) {
    GKUIControl *control = pointer;
    if (!control || (control->kind != GK_UI_TEXT_FIELD &&
                     control->kind != GK_UI_TEXT_AREA)) return;
    wchar_t *wide = gkUIWide(text);
    if (!wide) return;
    free(control->placeholder);
    control->placeholder = wide;
    SendMessageW(control->window, EM_SETCUEBANNER, TRUE, (LPARAM)control->placeholder);
}

char *gkUIPlaceholder(void *pointer) {
    GKUIControl *control = pointer;
    return gkUIUTF8(control ? control->placeholder : L"");
}

int gkUIChecked(void *pointer) {
    GKUIControl *control = pointer;
    return control && control->kind == GK_UI_CHECK_BOX &&
           SendMessageW(control->window, BM_GETCHECK, 0, 0) == BST_CHECKED;
}

void gkUISetChecked(void *pointer, int checked) {
    GKUIControl *control = pointer;
    if (control && control->kind == GK_UI_CHECK_BOX) {
        SendMessageW(control->window, BM_SETCHECK,
                     checked ? BST_CHECKED : BST_UNCHECKED, 0);
    }
}

int gkUIReadOnly(void *pointer) {
    GKUIControl *control = pointer;
    return control && control->kind == GK_UI_TEXT_AREA ? control->readOnly : 0;
}

void gkUISetReadOnly(void *pointer, int readOnly) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_TEXT_AREA) return;
    control->readOnly = readOnly != 0;
    SendMessageW(control->window, EM_SETREADONLY, readOnly != 0, 0);
}

double gkUISliderValue(void *pointer) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_SLIDER) return 0;
    int position = (int)SendMessageW(control->window, TBM_GETPOS, 0, 0);
    if (control->step > 0) {
        return min(control->maximum, control->minimum + position * control->step);
    }
    return control->minimum + (control->maximum - control->minimum) *
           position / control->sliderMaximum;
}

void gkUISetSliderValue(void *pointer, double value) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_SLIDER) return;
    value = max(control->minimum, min(value, control->maximum));
    double range = control->maximum - control->minimum;
    int position = control->step > 0 ? (int)llround((value - control->minimum) / control->step) :
                   (int)llround((value - control->minimum) / range * control->sliderMaximum);
    SendMessageW(control->window, TBM_SETPOS, TRUE, position);
}

double gkUIProgressValue(void *pointer) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_PROGRESS) return 0;
    return (double)SendMessageW(control->window, PBM_GETPOS, 0, 0) / 10000.0;
}

void gkUISetProgressValue(void *pointer, double value) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_PROGRESS) return;
    value = max(0, min(value, 1));
    SendMessageW(control->window, PBM_SETPOS, (WPARAM)lround(value * 10000), 0);
}

int gkUIActivityRunning(void *pointer) {
    GKUIControl *control = pointer;
    return control && control->kind == GK_UI_ACTIVITY ? control->running : 0;
}

void gkUISetActivityRunning(void *pointer, int running) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_ACTIVITY) return;
    control->running = running != 0;
    SendMessageW(control->window, PBM_SETMARQUEE, control->running, 30);
}

char *gkUILinkURL(void *pointer) {
    GKUIControl *control = pointer;
    return gkUIUTF8(control && control->kind == GK_UI_LINK ? control->url : L"");
}

void gkUISetLinkURL(void *pointer, const char *url) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_LINK) return;
    wchar_t *wide = gkUIWide(url);
    if (!wide) return;
    free(control->url);
    control->url = wide;
    gkUIUpdateLinkText(control);
}

void gkUIPanelSetMovable(void *pointer, int movable) {
    GKUIControl *control = pointer;
    if (control && control->kind == GK_UI_PANEL) control->movable = movable != 0;
}

void gkUIPanelPosition(void *pointer, int *x, int *y) {
    GKUIControl *control = pointer;
    RECT rect = {0};
    if (control && control->window) {
        GetWindowRect(control->window, &rect);
        MapWindowPoints(HWND_DESKTOP, GetParent(control->window), (POINT *)&rect, 2);
    }
    if (x) *x = rect.left;
    if (y) *y = rect.top;
}

int gkUISectionExpanded(void *pointer) {
    GKUIControl *control = pointer;
    return control && control->kind == GK_UI_SECTION ? control->expanded : 0;
}

void gkUISectionSetExpanded(void *pointer, int expanded) {
    GKUIControl *control = pointer;
    if (!control || control->kind != GK_UI_SECTION || control->expanded == (expanded != 0)) return;
    gkUIToggleSection(control, 0);
}

static wchar_t *gkUIFileFilter(const char *extensions) {
    wchar_t *wide = gkUIWide(extensions);
    if (!wide || !wide[0]) {
        free(wide);
        wchar_t *filter = calloc(20, sizeof(*filter));
        if (filter) memcpy(filter, L"All files\0*.*\0\0", 15 * sizeof(*filter));
        return filter;
    }
    for (wchar_t *next = wide; *next; next++) {
        if (*next == L',') *next = L';';
    }
    size_t patternLength = wcslen(wide) * 2 + 3;
    wchar_t *pattern = calloc(patternLength, sizeof(*pattern));
    if (!pattern) {
        free(wide);
        return NULL;
    }
    wchar_t *out = pattern;
    wchar_t *part = wide;
    while (part && *part) {
        wchar_t *separator = wcschr(part, L';');
        if (separator) *separator = 0;
        if (out != pattern) *out++ = L';';
        *out++ = L'*';
        *out++ = L'.';
        size_t length = wcslen(part);
        memcpy(out, part, length * sizeof(*out));
        out += length;
        part = separator ? separator + 1 : NULL;
    }
    *out = 0;
    const wchar_t label[] = L"Supported files";
    size_t total = _countof(label) + wcslen(pattern) + 1 + 15;
    wchar_t *filter = calloc(total, sizeof(*filter));
    if (filter) {
        wchar_t *next = filter;
        memcpy(next, label, sizeof(label));
        next += _countof(label);
        size_t length = wcslen(pattern) + 1;
        memcpy(next, pattern, length * sizeof(*next));
        next += length;
        memcpy(next, L"All files\0*.*\0\0", 15 * sizeof(*next));
    }
    free(pattern);
    free(wide);
    return filter;
}

static char *gkUIPathList(wchar_t **paths, size_t count, size_t *resultSize) {
    size_t size = 0;
    char **values = calloc(count, sizeof(*values));
    if (!values) return NULL;
    for (size_t i = 0; i < count; i++) {
        values[i] = gkUIUTF8(paths[i]);
        if (values[i]) size += strlen(values[i]) + 1;
    }
    char *result = size ? malloc(size) : NULL;
    char *next = result;
    for (size_t i = 0; i < count; i++) {
        if (values[i]) {
            size_t length = strlen(values[i]) + 1;
            memcpy(next, values[i], length);
            next += length;
            free(values[i]);
        }
    }
    free(values);
    if (resultSize) *resultSize = size;
    return result;
}

static int CALLBACK gkUIBrowseCallback(HWND dialog, UINT message, LPARAM lparam, LPARAM data) {
    (void)lparam;
    if (message == BFFM_INITIALIZED && data) {
        SendMessageW(dialog, BFFM_SETSELECTIONW, TRUE, data);
    }
    return 0;
}

char *gkUIOpenPanel(uintptr_t window, const char *title,
                    const char *directory, const char *filename,
                    const char *extensions, int multiple, int directories,
                    size_t *resultSize, int *cancelled) {
    if (resultSize) *resultSize = 0;
    if (directories) {
        wchar_t path[MAX_PATH] = {0};
        wchar_t *wideTitle = gkUIWide(title);
        wchar_t *wideDirectory = gkUIWide(directory);
        BROWSEINFOW browse = {0};
        browse.hwndOwner = (HWND)window;
        browse.pszDisplayName = path;
        browse.lpszTitle = wideTitle;
        browse.ulFlags = BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE;
        browse.lpfn = gkUIBrowseCallback;
        browse.lParam = (LPARAM)wideDirectory;
        PIDLIST_ABSOLUTE item = SHBrowseForFolderW(&browse);
        free(wideTitle);
        free(wideDirectory);
        if (!item) {
            if (cancelled) *cancelled = 1;
            return NULL;
        }
        int ok = SHGetPathFromIDListW(item, path);
        CoTaskMemFree(item);
        if (!ok) {
            if (cancelled) *cancelled = 0;
            return NULL;
        }
        wchar_t *paths[] = {path};
        if (cancelled) *cancelled = 0;
        return gkUIPathList(paths, 1, resultSize);
    }

    const DWORD capacity = 65536;
    wchar_t *buffer = calloc(capacity, sizeof(*buffer));
    wchar_t *wideTitle = gkUIWide(title);
    wchar_t *wideDirectory = gkUIWide(directory);
    wchar_t *wideFilename = gkUIWide(filename);
    wchar_t *filter = gkUIFileFilter(extensions);
    if (!buffer || !filter) {
        free(buffer); free(wideTitle); free(wideDirectory); free(wideFilename); free(filter);
        if (cancelled) *cancelled = 0;
        return NULL;
    }
    if (wideFilename) wcsncpy(buffer, wideFilename, capacity - 1);
    OPENFILENAMEW open = {0};
    open.lStructSize = sizeof(open);
    open.hwndOwner = (HWND)window;
    open.lpstrFilter = filter;
    open.lpstrFile = buffer;
    open.nMaxFile = capacity;
    open.lpstrInitialDir = wideDirectory && wideDirectory[0] ? wideDirectory : NULL;
    open.lpstrTitle = wideTitle && wideTitle[0] ? wideTitle : NULL;
    open.Flags = OFN_EXPLORER | OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST |
                 OFN_HIDEREADONLY | (multiple ? OFN_ALLOWMULTISELECT : 0);
    int ok = GetOpenFileNameW(&open);
    free(wideTitle); free(wideDirectory); free(wideFilename); free(filter);
    if (!ok) {
        free(buffer);
        if (cancelled) *cancelled = CommDlgExtendedError() == 0;
        return NULL;
    }
    wchar_t *second = buffer + wcslen(buffer) + 1;
    char *result = NULL;
    if (!multiple || !*second) {
        wchar_t *paths[] = {buffer};
        result = gkUIPathList(paths, 1, resultSize);
    } else {
        size_t count = 0;
        for (wchar_t *next = second; *next; next += wcslen(next) + 1) count++;
        wchar_t **paths = calloc(count, sizeof(*paths));
        size_t directoryLength = wcslen(buffer);
        size_t i = 0;
        for (wchar_t *next = second; *next; next += wcslen(next) + 1) {
            size_t nameLength = wcslen(next);
            paths[i] = calloc(directoryLength + nameLength + 2, sizeof(**paths));
            if (paths[i]) {
                memcpy(paths[i], buffer, directoryLength * sizeof(**paths));
                paths[i][directoryLength] = L'\\';
                memcpy(paths[i] + directoryLength + 1, next,
                       (nameLength + 1) * sizeof(**paths));
            }
            i++;
        }
        result = gkUIPathList(paths, count, resultSize);
        for (i = 0; i < count; i++) free(paths[i]);
        free(paths);
    }
    free(buffer);
    if (cancelled) *cancelled = 0;
    return result;
}

char *gkUISavePanel(uintptr_t window, const char *title,
                    const char *directory, const char *filename,
                    const char *extensions, size_t *resultSize, int *cancelled) {
    if (resultSize) *resultSize = 0;
    const DWORD capacity = 32768;
    wchar_t *buffer = calloc(capacity, sizeof(*buffer));
    wchar_t *wideTitle = gkUIWide(title);
    wchar_t *wideDirectory = gkUIWide(directory);
    wchar_t *wideFilename = gkUIWide(filename);
    wchar_t *filter = gkUIFileFilter(extensions);
    if (!buffer || !filter) {
        free(buffer); free(wideTitle); free(wideDirectory); free(wideFilename); free(filter);
        if (cancelled) *cancelled = 0;
        return NULL;
    }
    if (wideFilename) wcsncpy(buffer, wideFilename, capacity - 1);
    OPENFILENAMEW save = {0};
    save.lStructSize = sizeof(save);
    save.hwndOwner = (HWND)window;
    save.lpstrFilter = filter;
    save.lpstrFile = buffer;
    save.nMaxFile = capacity;
    save.lpstrInitialDir = wideDirectory && wideDirectory[0] ? wideDirectory : NULL;
    save.lpstrTitle = wideTitle && wideTitle[0] ? wideTitle : NULL;
    save.Flags = OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT;
    int ok = GetSaveFileNameW(&save);
    free(wideTitle); free(wideDirectory); free(wideFilename); free(filter);
    if (!ok) {
        free(buffer);
        if (cancelled) *cancelled = CommDlgExtendedError() == 0;
        return NULL;
    }
    wchar_t *paths[] = {buffer};
    char *result = gkUIPathList(paths, 1, resultSize);
    free(buffer);
    if (cancelled) *cancelled = 0;
    return result;
}
