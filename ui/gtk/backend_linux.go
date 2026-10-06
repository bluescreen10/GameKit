//go:build linux && cgo && gtk

package gtk

/*
#cgo pkg-config: gtk+-3.0 gdk-x11-3.0
#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdimage "image"
	"image/draw"
	"math"
	"runtime/cgo"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

type window struct {
	mu          sync.RWMutex
	native      unsafe.Pointer
	title       string
	theme       ui.Theme
	size        ui.Size
	content     ui.Widget
	panels      []ui.Panel
	shouldClose bool
}

func createWindow(options ui.WindowOptions) (ui.Window, error) {
	if !options.Size.IsValid() {
		return nil, errors.New("ui/gtk: window size must be positive")
	}
	if err := options.Theme.Validate(); err != nil {
		return nil, err
	}
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))
	var flags C.uint32_t
	if options.Resizable {
		flags |= 1 << 0
	}
	if options.Hidden {
		flags |= 1 << 1
	}
	if options.Borderless {
		flags |= 1 << 2
	}
	if options.Maximized {
		flags |= 1 << 3
	}
	if options.AlwaysOnTop {
		flags |= 1 << 4
	}
	if options.TransparentFramebuffer {
		flags |= 1 << 5
	}
	var message [512]C.char
	native := C.gkGTKWindowCreate(
		title,
		C.int(options.Size.Width),
		C.int(options.Size.Height),
		flags,
		C.int(options.Theme),
		0,
		&message[0],
		C.size_t(len(message)),
	)
	if native == nil {
		detail := C.GoString(&message[0])
		if detail == "" {
			detail = "GTK window creation failed"
		}
		return nil, fmt.Errorf("ui/gtk: %s", detail)
	}
	return &window{
		native: native,
		title:  options.Title,
		theme:  options.Theme,
		size:   options.Size,
	}, nil
}

func (w *window) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.native == nil {
		w.mu.Unlock()
		return
	}
	native := w.native
	content := w.content
	panels := w.panels
	w.native = nil
	w.content = nil
	w.panels = nil
	w.mu.Unlock()
	if content != nil {
		content.Close()
	}
	for _, panel := range panels {
		panel.Close()
	}
	C.gkGTKWindowDestroy(native)
}

func (w *window) nativePointer() unsafe.Pointer {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.native
}

func (w *window) Title() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.title
}

func (w *window) SetTitle(title string) {
	native := w.nativePointer()
	if native == nil {
		return
	}
	value := C.CString(title)
	defer C.free(unsafe.Pointer(value))
	C.gkGTKWindowSetTitle(native, value)
	w.mu.Lock()
	w.title = title
	w.mu.Unlock()
}

func (w *window) Theme() ui.Theme {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.theme
}

func (w *window) SetTheme(theme ui.Theme) error {
	if err := theme.Validate(); err != nil {
		return err
	}
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkGTKWindowSetTheme(native, C.int(theme))
	w.mu.Lock()
	w.theme = theme
	w.mu.Unlock()
	return nil
}

func (w *window) Size() ui.Size {
	native := w.nativePointer()
	if native == nil {
		return ui.Size{}
	}
	var width, height C.int
	C.gkGTKWindowSize(native, &width, &height)
	size := ui.Size{Width: int(width), Height: int(height)}
	if !size.IsValid() || size == (ui.Size{Width: 1, Height: 1}) {
		w.mu.RLock()
		defer w.mu.RUnlock()
		return w.size
	}
	w.mu.Lock()
	w.size = size
	w.mu.Unlock()
	return size
}

func (w *window) FramebufferSize() ui.Size {
	native := w.nativePointer()
	if native == nil {
		return ui.Size{}
	}
	var width, height C.int
	C.gkGTKWindowFramebufferSize(native, &width, &height)
	return ui.Size{Width: int(width), Height: int(height)}
}

func (w *window) NativeSurface() gpu.NativeSurface {
	native := w.nativePointer()
	if native == nil {
		return gpu.NativeSurface{}
	}
	handle := uintptr(C.gkGTKWindowXID(native))
	display := uintptr(C.gkGTKWindowXDisplay(native))
	if handle == 0 || display == 0 {
		return gpu.NativeSurface{}
	}
	return gpu.NativeSurface{
		Kind:    gpu.NativeSurfaceXlibWindow,
		Handle:  handle,
		Display: display,
	}
}

func (w *window) ShouldClose() bool {
	native := w.nativePointer()
	return native == nil || C.gkGTKWindowShouldClose(native) != 0
}

func (w *window) SetShouldClose(close bool) {
	if native := w.nativePointer(); native != nil {
		C.gkGTKWindowSetShouldClose(native, boolInt(close))
	}
}

func (w *window) PollEvents() {
	native := w.nativePointer()
	if native == nil {
		return
	}
	C.gkGTKWindowPoll(native)
	w.mu.RLock()
	content := w.content
	w.mu.RUnlock()
	if content != nil {
		content.SetBounds(ui.Rect{Size: w.Size()})
	}
}

func (w *window) SetContent(content ui.Widget) error {
	child, ok := content.(nativeWidget)
	if !ok || child.base().window != w {
		return errors.New("ui/gtk: content belongs to another backend or window")
	}
	if _, floating := child.(*panel); floating {
		return errors.New("ui/gtk: floating panel cannot be window content")
	}
	if child.base().parent != nil {
		return errors.New("ui/gtk: content already has a parent")
	}
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkGTKWindowSetContent(native, child.base().nativePointer())
	w.mu.Lock()
	if w.content != nil && w.content != content {
		w.content.SetVisible(false)
	}
	w.content = content
	w.mu.Unlock()
	content.SetVisible(true)
	content.SetBounds(ui.Rect{Size: w.Size()})
	for _, floating := range w.panels {
		floating.BringToFront()
	}
	return nil
}

type nativeWidget interface {
	ui.Widget
	base() *widgetBase
}

type widgetBase struct {
	window *window
	native unsafe.Pointer
	handle uintptr

	mu      sync.RWMutex
	bounds  ui.Rect
	parent  nativeWidget
	visible bool
	enabled bool
	closed  bool
}

func newWidgetBase(window *window, native unsafe.Pointer) *widgetBase {
	return &widgetBase{window: window, native: native, visible: true, enabled: true}
}

func (w *widgetBase) base() *widgetBase { return w }

func (w *widgetBase) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	native := w.native
	handle := w.handle
	w.native = nil
	w.handle = 0
	w.closed = true
	w.mu.Unlock()
	if handle != 0 {
		cgo.Handle(handle).Delete()
	}
	C.gkGTKWidgetDestroy(native)
}

func (w *widgetBase) nativePointer() unsafe.Pointer {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.native
}

func (w *widgetBase) Bounds() ui.Rect {
	if w == nil {
		return ui.Rect{}
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.bounds
}

func (w *widgetBase) SetBounds(bounds ui.Rect) {
	if w == nil {
		return
	}
	bounds.Size.Width = max(0, bounds.Size.Width)
	bounds.Size.Height = max(0, bounds.Size.Height)
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.bounds = bounds
	native := w.native
	w.mu.Unlock()
	C.gkGTKWidgetSetFrame(native, C.int(bounds.Position.X), C.int(bounds.Position.Y),
		C.int(bounds.Size.Width), C.int(bounds.Size.Height))
}

func (w *widgetBase) setPositionFromNative(position ui.Point) {
	w.mu.Lock()
	if !w.closed {
		w.bounds.Position = position
	}
	w.mu.Unlock()
}

func (w *widgetBase) PreferredSize() ui.Size {
	native := w.nativePointer()
	if native == nil {
		return ui.Size{}
	}
	var width, height C.int
	C.gkGTKWidgetPreferredSize(native, &width, &height)
	return ui.Size{Width: int(width), Height: int(height)}
}

func (w *widgetBase) IsVisible() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.visible && !w.closed
}

func (w *widgetBase) SetVisible(visible bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.visible = visible
	native := w.native
	w.mu.Unlock()
	C.gkGTKWidgetSetVisible(native, boolInt(visible))
}

func (w *widgetBase) IsEnabled() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.enabled && !w.closed
}

func (w *widgetBase) SetEnabled(enabled bool) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.enabled = enabled
	native := w.native
	w.mu.Unlock()
	C.gkGTKWidgetSetEnabled(native, boolInt(enabled))
}

type listeners[T any] struct {
	mu       sync.RWMutex
	nextID   uint64
	handlers map[uint64]T
}

func (l *listeners[T]) add(handler T) ui.Unsubscribe {
	l.mu.Lock()
	l.nextID++
	id := l.nextID
	if l.handlers == nil {
		l.handlers = make(map[uint64]T)
	}
	l.handlers[id] = handler
	l.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.handlers, id)
			l.mu.Unlock()
		})
	}
}

func (l *listeners[T]) snapshot() []T {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]T, 0, len(l.handlers))
	for _, handler := range l.handlers {
		result = append(result, handler)
	}
	return result
}

type layout struct {
	*widgetBase
	mu   sync.RWMutex
	spec ui.LayoutSpec
}

func (l *layout) base() *widgetBase { return l.widgetBase }

func (l *layout) Close() {
	if l == nil {
		return
	}
	for _, child := range l.Children() {
		child.Close()
	}
	l.widgetBase.Close()
}

func (l *layout) SetBounds(bounds ui.Rect) {
	l.widgetBase.SetBounds(bounds)
	l.applyLayout(bounds.Size)
}

func (l *layout) PreferredSize() ui.Size {
	l.mu.RLock()
	spec := copyLayoutSpec(l.spec)
	l.mu.RUnlock()
	size, err := spec.PreferredSize()
	if err != nil {
		return ui.Size{}
	}
	return size
}

func (l *layout) Spec() ui.LayoutSpec {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return copyLayoutSpec(l.spec)
}

func (l *layout) SetSpec(spec ui.LayoutSpec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	children, err := l.validateChildren(spec)
	if err != nil {
		return err
	}
	l.mu.Lock()
	previous := layoutChildren(l.spec)
	l.spec = copyLayoutSpec(spec)
	l.mu.Unlock()
	for _, child := range previous {
		if containsWidget(children, child) {
			continue
		}
		child.base().parent = nil
		C.gkGTKWidgetUnparent(child.base().nativePointer())
	}
	for _, child := range children {
		child.base().parent = l
		C.gkGTKWidgetSetParent(child.base().nativePointer(), l.nativePointer())
	}
	l.applyLayout(l.Bounds().Size)
	return nil
}

func (l *layout) Children() []ui.Widget {
	l.mu.RLock()
	defer l.mu.RUnlock()
	children := layoutChildren(l.spec)
	result := make([]ui.Widget, len(children))
	for i, child := range children {
		result[i] = child
	}
	return result
}

func (l *layout) validateChildren(spec ui.LayoutSpec) ([]nativeWidget, error) {
	for _, item := range spec.Items {
		if item.Widget == nil {
			continue
		}
		if _, ok := item.Widget.(nativeWidget); !ok {
			return nil, fmt.Errorf("ui/gtk: layout child %T belongs to another backend", item.Widget)
		}
	}
	children := layoutChildren(spec)
	seen := make(map[*widgetBase]struct{}, len(children))
	for _, child := range children {
		base := child.base()
		if base.window != l.window {
			return nil, errors.New("ui/gtk: layout child belongs to another window")
		}
		if base.closed {
			return nil, ui.ErrClosed
		}
		if base.parent != nil && base.parent.base() != l.widgetBase {
			return nil, errors.New("ui/gtk: layout child already has a parent")
		}
		if _, exists := seen[base]; exists {
			return nil, errors.New("ui/gtk: layout contains the same child twice")
		}
		seen[base] = struct{}{}
		for ancestor := nativeWidget(l); ancestor != nil; ancestor = ancestor.base().parent {
			if ancestor.base() == base {
				return nil, errors.New("ui/gtk: layout cycle")
			}
		}
	}
	return children, nil
}

func (l *layout) applyLayout(size ui.Size) {
	l.mu.RLock()
	spec := copyLayoutSpec(l.spec)
	l.mu.RUnlock()
	positions, err := spec.Resolve(size)
	if err != nil {
		return
	}
	for i, item := range spec.Items {
		if item.Widget != nil {
			item.Widget.SetBounds(positions[i])
		}
	}
}

func layoutChildren(spec ui.LayoutSpec) []nativeWidget {
	children := make([]nativeWidget, 0, len(spec.Items))
	for _, item := range spec.Items {
		if item.Widget == nil {
			continue
		}
		child, ok := item.Widget.(nativeWidget)
		if ok {
			children = append(children, child)
		}
	}
	return children
}

func containsWidget(widgets []nativeWidget, target nativeWidget) bool {
	for _, widget := range widgets {
		if widget.base() == target.base() {
			return true
		}
	}
	return false
}

func copyLayoutSpec(spec ui.LayoutSpec) ui.LayoutSpec {
	spec.Items = append([]ui.LayoutItem(nil), spec.Items...)
	return spec
}

type label struct{ *widgetBase }

func (l *label) Text() string        { return nativeText(l.nativePointer()) }
func (l *label) SetText(text string) { setNativeText(l.nativePointer(), text) }

type button struct {
	*widgetBase
	clicks listeners[ui.ClickHandler]
}

func (b *button) Text() string        { return nativeText(b.nativePointer()) }
func (b *button) SetText(text string) { setNativeText(b.nativePointer(), text) }
func (b *button) OnClick(handler ui.ClickHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return b.clicks.add(handler)
}
func (b *button) nativeEvent(event int) {
	if event == int(C.GK_GTK_EVENT_CLICK) {
		for _, handler := range b.clicks.snapshot() {
			handler()
		}
	}
}

type textField struct {
	*widgetBase
	changes listeners[ui.TextChangeHandler]
}

type textArea struct {
	*widgetBase
	changes listeners[ui.TextChangeHandler]
}

func (t *textArea) Text() string        { return nativeText(t.nativePointer()) }
func (t *textArea) SetText(text string) { setNativeText(t.nativePointer(), text) }
func (t *textArea) Placeholder() string {
	native := t.nativePointer()
	if native == nil {
		return ""
	}
	value := C.gkGTKTextFieldPlaceholder(native)
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}
func (t *textArea) SetPlaceholder(placeholder string) {
	native := t.nativePointer()
	if native == nil {
		return
	}
	value := C.CString(placeholder)
	defer C.free(unsafe.Pointer(value))
	C.gkGTKTextFieldSetPlaceholder(native, value)
}
func (t *textArea) IsReadOnly() bool {
	native := t.nativePointer()
	return native != nil && C.gkGTKTextAreaReadOnly(native) != 0
}
func (t *textArea) SetReadOnly(readOnly bool) {
	if native := t.nativePointer(); native != nil {
		C.gkGTKTextAreaSetReadOnly(native, boolInt(readOnly))
	}
}
func (t *textArea) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}
func (t *textArea) nativeEvent(event int) {
	if event != int(C.GK_GTK_EVENT_TEXT_CHANGE) {
		return
	}
	text := t.Text()
	for _, handler := range t.changes.snapshot() {
		handler(text)
	}
}

type checkBox struct {
	*widgetBase
	changes listeners[ui.ToggleHandler]
}

func (c *checkBox) Text() string        { return nativeText(c.nativePointer()) }
func (c *checkBox) SetText(text string) { setNativeText(c.nativePointer(), text) }
func (c *checkBox) IsChecked() bool {
	native := c.nativePointer()
	return native != nil && C.gkGTKWidgetChecked(native) != 0
}
func (c *checkBox) SetChecked(checked bool) {
	if native := c.nativePointer(); native != nil {
		C.gkGTKWidgetSetChecked(native, boolInt(checked))
	}
}
func (c *checkBox) OnChange(handler ui.ToggleHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return c.changes.add(handler)
}
func (c *checkBox) nativeEvent(event int) {
	if event != int(C.GK_GTK_EVENT_TOGGLE_CHANGE) {
		return
	}
	checked := c.IsChecked()
	for _, handler := range c.changes.snapshot() {
		handler(checked)
	}
}

type selection struct {
	*widgetBase
	items   []string
	changes listeners[ui.SelectionChangeHandler]
}

func (s *selection) Items() []string { return append([]string(nil), s.items...) }
func (s *selection) SelectedIndex() int {
	native := s.nativePointer()
	if native == nil {
		return -1
	}
	return int(C.gkGTKSelectionIndex(native))
}
func (s *selection) SetSelectedIndex(index int) error {
	if index < 0 || index >= len(s.items) {
		return errors.New("ui: selection is out of range")
	}
	native := s.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkGTKSelectionSetIndex(native, C.int(index))
	return nil
}
func (s *selection) OnChange(handler ui.SelectionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}
func (s *selection) nativeEvent(event int) {
	if event != int(C.GK_GTK_EVENT_SELECTION_CHANGE) {
		return
	}
	index := s.SelectedIndex()
	for _, handler := range s.changes.snapshot() {
		handler(index)
	}
}

type radioGroup struct{ *selection }
type selectControl struct{ *selection }

func (t *textField) Text() string        { return nativeText(t.nativePointer()) }
func (t *textField) SetText(text string) { setNativeText(t.nativePointer(), text) }
func (t *textField) Placeholder() string {
	native := t.nativePointer()
	if native == nil {
		return ""
	}
	value := C.gkGTKTextFieldPlaceholder(native)
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}
func (t *textField) SetPlaceholder(placeholder string) {
	native := t.nativePointer()
	if native == nil {
		return
	}
	value := C.CString(placeholder)
	defer C.free(unsafe.Pointer(value))
	C.gkGTKTextFieldSetPlaceholder(native, value)
}
func (t *textField) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}
func (t *textField) nativeEvent(event int) {
	if event == int(C.GK_GTK_EVENT_TEXT_CHANGE) {
		text := t.Text()
		for _, handler := range t.changes.snapshot() {
			handler(text)
		}
	}
}

type slider struct {
	*widgetBase
	changes listeners[ui.ValueChangeHandler]
}

type progressBar struct{ *widgetBase }

func (p *progressBar) Value() float64 {
	native := p.nativePointer()
	if native == nil {
		return 0
	}
	return float64(C.gkGTKProgressValue(native))
}
func (p *progressBar) SetValue(value float64) {
	if native := p.nativePointer(); native != nil {
		C.gkGTKProgressSetValue(native, C.double(value))
	}
}

type activityIndicator struct{ *widgetBase }

func (a *activityIndicator) IsRunning() bool {
	native := a.nativePointer()
	return native != nil && C.gkGTKActivityRunning(native) != 0
}
func (a *activityIndicator) SetRunning(running bool) {
	if native := a.nativePointer(); native != nil {
		C.gkGTKActivitySetRunning(native, boolInt(running))
	}
}

type separator struct {
	*widgetBase
	direction ui.LayoutDirection
}

func (s *separator) Direction() ui.LayoutDirection { return s.direction }

type imageView struct {
	*widgetBase
	scaling ui.ImageScaling
}

func (i *imageView) SetImage(source stdimage.Image) error {
	if source == nil || source.Bounds().Empty() {
		return errors.New("ui: image source is required and must not be empty")
	}
	native := i.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	bounds := source.Bounds()
	rgba := stdimage.NewRGBA(stdimage.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgba, rgba.Bounds(), source, bounds.Min, draw.Src)
	C.gkGTKImageSet(native, (*C.uchar)(unsafe.Pointer(&rgba.Pix[0])),
		C.int(rgba.Rect.Dx()), C.int(rgba.Rect.Dy()), C.int(rgba.Stride))
	return nil
}
func (i *imageView) Scaling() ui.ImageScaling { return i.scaling }
func (i *imageView) SetScaling(scaling ui.ImageScaling) error {
	if scaling > ui.ImageScaleStretch {
		return errors.New("ui: invalid image scaling")
	}
	native := i.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkGTKImageSetScaling(native, C.int(scaling))
	i.scaling = scaling
	return nil
}

type link struct {
	*widgetBase
	clicks listeners[ui.ClickHandler]
}

func (l *link) Text() string        { return nativeText(l.nativePointer()) }
func (l *link) SetText(text string) { setNativeText(l.nativePointer(), text) }
func (l *link) URL() string {
	native := l.nativePointer()
	if native == nil {
		return ""
	}
	value := C.gkGTKLinkURL(native)
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}
func (l *link) SetURL(url string) {
	native := l.nativePointer()
	if native == nil {
		return
	}
	value := C.CString(url)
	defer C.free(unsafe.Pointer(value))
	C.gkGTKLinkSetURL(native, value)
}
func (l *link) OnClick(handler ui.ClickHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return l.clicks.add(handler)
}
func (l *link) nativeEvent(event int) {
	if event != int(C.GK_GTK_EVENT_CLICK) {
		return
	}
	for _, handler := range l.clicks.snapshot() {
		handler()
	}
}

type tabs struct {
	*widgetBase
	stateMu sync.RWMutex
	items   []ui.TabItem
	changes listeners[ui.SelectionChangeHandler]
}

type scrollView struct {
	*widgetBase
	content nativeWidget
	axes    ui.ScrollAxes
}

func (s *scrollView) Close() {
	if s == nil {
		return
	}
	if s.content != nil {
		s.content.Close()
		s.content = nil
	}
	s.widgetBase.Close()
}

func (s *scrollView) SetBounds(bounds ui.Rect) {
	s.widgetBase.SetBounds(bounds)
	s.layoutContent()
}

func (s *scrollView) PreferredSize() ui.Size {
	return ui.Size{Width: 320, Height: 240}
}

func (s *scrollView) Content() ui.Widget {
	if s == nil || s.content == nil {
		return nil
	}
	return s.content
}

func (s *scrollView) ScrollOffset() ui.Point {
	native := s.nativePointer()
	if native == nil {
		return ui.Point{}
	}
	var x, y C.int
	C.gkGTKScrollOffset(native, &x, &y)
	return ui.Point{X: int(x), Y: int(y)}
}

func (s *scrollView) SetScrollOffset(offset ui.Point) {
	if native := s.nativePointer(); native != nil {
		C.gkGTKScrollSetOffset(native, C.int(max(0, offset.X)), C.int(max(0, offset.Y)))
	}
}

func (s *scrollView) layoutContent() {
	native := s.nativePointer()
	if native == nil || s.content == nil {
		return
	}
	var width, height C.int
	C.gkGTKScrollViewportSize(native, &width, &height)
	viewport := ui.Size{Width: max(0, int(width)), Height: max(0, int(height))}
	if viewport.Width <= 1 {
		viewport.Width = s.Bounds().Size.Width
	}
	if viewport.Height <= 1 {
		viewport.Height = s.Bounds().Size.Height
	}
	preferred := s.content.PreferredSize()
	contentSize := viewport
	switch s.axes {
	case ui.ScrollVertical:
		contentSize.Height = max(viewport.Height, preferred.Height)
	case ui.ScrollHorizontal:
		contentSize.Width = max(viewport.Width, preferred.Width)
	case ui.ScrollBoth:
		contentSize.Width = max(viewport.Width, preferred.Width)
		contentSize.Height = max(viewport.Height, preferred.Height)
	}
	C.gkGTKScrollSetContentSize(native, C.int(contentSize.Width), C.int(contentSize.Height))
	s.content.SetBounds(ui.Rect{Size: contentSize})
}

func (t *tabs) Close() {
	if t == nil {
		return
	}
	for _, item := range t.Items() {
		item.Content.Close()
	}
	t.widgetBase.Close()
}
func (t *tabs) SetBounds(bounds ui.Rect) {
	t.widgetBase.SetBounds(bounds)
	t.layoutContent()
}
func (t *tabs) PreferredSize() ui.Size {
	size := ui.Size{Width: 320, Height: 240}
	for _, item := range t.Items() {
		preferred := item.Content.PreferredSize()
		size.Width = max(size.Width, preferred.Width+16)
		size.Height = max(size.Height, preferred.Height+40)
	}
	return size
}
func (t *tabs) Items() []ui.TabItem {
	t.stateMu.RLock()
	defer t.stateMu.RUnlock()
	return append([]ui.TabItem(nil), t.items...)
}
func (t *tabs) SelectedIndex() int {
	native := t.nativePointer()
	if native == nil {
		return -1
	}
	return int(C.gkGTKTabsSelected(native))
}
func (t *tabs) SetSelectedIndex(index int) error {
	if index < 0 || index >= len(t.Items()) {
		return errors.New("ui: tab selection is out of range")
	}
	native := t.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkGTKTabsSetSelected(native, C.int(index))
	t.layoutContent()
	return nil
}
func (t *tabs) OnChange(handler ui.SelectionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}
func (t *tabs) nativeEvent(event int) {
	if event != int(C.GK_GTK_EVENT_TAB_CHANGE) {
		return
	}
	index := t.SelectedIndex()
	t.layoutContent()
	for _, handler := range t.changes.snapshot() {
		handler(index)
	}
}
func (t *tabs) layoutContent() {
	native := t.nativePointer()
	if native == nil {
		return
	}
	var width, height C.int
	C.gkGTKTabsContentSize(native, &width, &height)
	selected := t.SelectedIndex()
	for index, item := range t.Items() {
		if index == selected {
			item.Content.SetBounds(ui.Rect{Size: ui.Size{Width: int(width), Height: int(height)}})
		}
	}
}

func (s *slider) Value() float64 {
	if native := s.nativePointer(); native != nil {
		return float64(C.gkGTKSliderValue(native))
	}
	return 0
}
func (s *slider) SetValue(value float64) {
	if native := s.nativePointer(); native != nil {
		C.gkGTKSliderSetValue(native, C.double(value))
	}
}
func (s *slider) OnChange(handler ui.ValueChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}
func (s *slider) nativeEvent(event int) {
	if event == int(C.GK_GTK_EVENT_VALUE_CHANGE) {
		value := s.Value()
		for _, handler := range s.changes.snapshot() {
			handler(value)
		}
	}
}

type canvas struct {
	*widgetBase
	resizes listeners[ui.ResizeHandler]
	redraw  atomic.Bool
}

func (c *canvas) SetBounds(bounds ui.Rect) {
	previous := c.Bounds().Size
	c.widgetBase.SetBounds(bounds)
	if previous != bounds.Size {
		for _, handler := range c.resizes.snapshot() {
			handler(bounds.Size)
		}
	}
}
func (c *canvas) NativeSurface() gpu.NativeSurface {
	native := c.nativePointer()
	if native == nil {
		return gpu.NativeSurface{}
	}
	handle := uintptr(C.gkGTKCanvasXID(native))
	windowSurface := c.window.NativeSurface()
	if handle == 0 || windowSurface.Display == 0 {
		return gpu.NativeSurface{}
	}
	return gpu.NativeSurface{Kind: gpu.NativeSurfaceXlibWindow, Handle: handle, Display: windowSurface.Display}
}
func (c *canvas) FramebufferSize() ui.Size {
	native := c.nativePointer()
	if native == nil {
		return ui.Size{}
	}
	var width, height C.int
	C.gkGTKCanvasFramebufferSize(native, &width, &height)
	return ui.Size{Width: int(width), Height: int(height)}
}
func (c *canvas) OnResize(handler ui.ResizeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return c.resizes.add(handler)
}
func (c *canvas) RequestRedraw() {
	c.redraw.Store(true)
	if native := c.nativePointer(); native != nil {
		C.gkGTKCanvasQueueDraw(native)
	}
}
func (c *canvas) TakeRedrawRequest() bool { return c.redraw.Swap(false) }

const panelHeaderHeight = 32

type panel struct {
	*widgetBase

	stateMu sync.RWMutex
	content nativeWidget
	movable bool
	moves   listeners[ui.MoveHandler]
}

func (p *panel) Close() {
	if p == nil {
		return
	}
	p.stateMu.Lock()
	content := p.content
	p.content = nil
	p.stateMu.Unlock()
	if content != nil {
		content.Close()
	}
	p.widgetBase.Close()
}

func (p *panel) SetBounds(bounds ui.Rect) {
	p.widgetBase.SetBounds(bounds)
	p.layoutContent()
}

func (p *panel) PreferredSize() ui.Size {
	p.stateMu.RLock()
	content := p.content
	p.stateMu.RUnlock()
	if content == nil {
		return ui.Size{Width: 240, Height: 160}
	}
	size := content.PreferredSize()
	size.Width = max(240, size.Width+24)
	size.Height = max(160, size.Height+panelHeaderHeight+12)
	return size
}

func (p *panel) Title() string {
	return nativeText(p.nativePointer())
}

func (p *panel) SetTitle(title string) {
	setNativeText(p.nativePointer(), title)
}

func (p *panel) Content() ui.Widget {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	if p.content == nil {
		return nil
	}
	return p.content
}

func (p *panel) SetContent(content ui.Widget) error {
	child, err := validateContainerChild(p, content)
	if err != nil {
		return err
	}

	p.stateMu.Lock()
	previous := p.content
	if previous != nil && previous.base() != child.base() {
		previous.base().parent = nil
		C.gkGTKWidgetUnparent(previous.base().nativePointer())
		previous.SetVisible(false)
	}
	p.content = child
	p.stateMu.Unlock()

	child.base().parent = p
	C.gkGTKWidgetSetParent(child.base().nativePointer(), p.nativePointer())
	child.SetVisible(true)
	p.layoutContent()
	return nil
}

func (p *panel) IsMovable() bool {
	p.stateMu.RLock()
	defer p.stateMu.RUnlock()
	return p.movable
}

func (p *panel) SetMovable(movable bool) {
	p.stateMu.Lock()
	p.movable = movable
	p.stateMu.Unlock()
	if native := p.nativePointer(); native != nil {
		C.gkGTKPanelSetMovable(native, boolInt(movable))
	}
}

func (p *panel) BringToFront() {
	if native := p.nativePointer(); native != nil {
		C.gkGTKPanelBringToFront(native)
	}
}

func (p *panel) OnMove(handler ui.MoveHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return p.moves.add(handler)
}

func (p *panel) nativeEvent(event int) {
	if event != int(C.GK_GTK_EVENT_PANEL_MOVE) {
		return
	}
	native := p.nativePointer()
	if native == nil {
		return
	}
	var x, y C.int
	C.gkGTKPanelPosition(native, &x, &y)
	position := ui.Point{X: int(x), Y: int(y)}
	p.setPositionFromNative(position)
	for _, handler := range p.moves.snapshot() {
		handler(position)
	}
}

func (p *panel) layoutContent() {
	p.stateMu.RLock()
	content := p.content
	p.stateMu.RUnlock()
	if content == nil {
		return
	}
	size := p.Bounds().Size
	content.SetBounds(ui.Rect{Size: ui.Size{
		Width:  max(0, size.Width-24),
		Height: max(0, size.Height-panelHeaderHeight-12),
	}})
}

type section struct {
	*widgetBase
	stateMu  sync.RWMutex
	content  nativeWidget
	expanded bool
	changes  listeners[ui.ExpansionChangeHandler]
}

func (s *section) Close() {
	if s == nil {
		return
	}
	s.stateMu.Lock()
	content := s.content
	s.content = nil
	s.stateMu.Unlock()
	if content != nil {
		content.Close()
	}
	s.widgetBase.Close()
}
func (s *section) SetBounds(bounds ui.Rect) {
	s.widgetBase.SetBounds(bounds)
	if content := s.Content(); content != nil && s.IsExpanded() {
		content.SetBounds(ui.Rect{Size: ui.Size{Width: bounds.Size.Width, Height: max(0, bounds.Size.Height-28)}})
	}
}
func (s *section) PreferredSize() ui.Size {
	if !s.IsExpanded() {
		return ui.Size{Width: 120, Height: 28}
	}
	content := s.Content()
	if content == nil {
		return ui.Size{Width: 120, Height: 28}
	}
	size := content.PreferredSize()
	return ui.Size{Width: max(120, size.Width), Height: size.Height + 28}
}
func (s *section) Title() string         { return nativeText(s.nativePointer()) }
func (s *section) SetTitle(title string) { setNativeText(s.nativePointer(), title) }
func (s *section) Content() ui.Widget {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if s.content == nil {
		return nil
	}
	return s.content
}
func (s *section) IsExpanded() bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.expanded
}
func (s *section) SetExpanded(expanded bool) {
	s.stateMu.Lock()
	if s.expanded == expanded {
		s.stateMu.Unlock()
		return
	}
	s.expanded = expanded
	s.stateMu.Unlock()
	if native := s.nativePointer(); native != nil {
		C.gkGTKSectionSetExpanded(native, boolInt(expanded))
	}
	s.relayout()
}
func (s *section) OnChange(handler ui.ExpansionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}
func (s *section) nativeEvent(event int) {
	if event != int(C.GK_GTK_EVENT_SECTION_CHANGE) {
		return
	}
	expanded := C.gkGTKSectionExpanded(s.nativePointer()) != 0
	s.stateMu.Lock()
	s.expanded = expanded
	s.stateMu.Unlock()
	s.relayout()
	for _, handler := range s.changes.snapshot() {
		handler(expanded)
	}
}
func (s *section) relayout() {
	if parent, ok := s.parent.(*layout); ok {
		parent.applyLayout(parent.Bounds().Size)
	}
	s.SetBounds(s.Bounds())
}

func validateContainerChild(container nativeWidget, content ui.Widget) (nativeWidget, error) {
	child, ok := content.(nativeWidget)
	if !ok {
		return nil, errors.New("ui/gtk: container content belongs to another backend")
	}
	if _, floating := child.(*panel); floating {
		return nil, errors.New("ui/gtk: a floating panel cannot be container content")
	}
	base := child.base()
	owner := container.base()
	if base.window != owner.window {
		return nil, errors.New("ui/gtk: container content belongs to another window")
	}
	if base.closed {
		return nil, ui.ErrClosed
	}
	if base.parent != nil && base.parent.base() != owner {
		return nil, errors.New("ui/gtk: container content already has a parent")
	}
	for ancestor := container; ancestor != nil; ancestor = ancestor.base().parent {
		if base == ancestor.base() {
			return nil, errors.New("ui/gtk: container cycle")
		}
	}
	return child, nil
}

type nativeEventReceiver interface{ nativeEvent(int) }

//export gkGTKGoWidgetEvent
func gkGTKGoWidgetEvent(handle C.uintptr_t, event C.int) {
	if handle == 0 {
		return
	}
	if receiver, ok := cgo.Handle(handle).Value().(nativeEventReceiver); ok {
		receiver.nativeEvent(int(event))
	}
}

func (w *window) CreateLayout(spec ui.LayoutSpec) (ui.Layout, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	native := C.gkGTKCreateLayout()
	if native == nil {
		return nil, errors.New("ui/gtk: could not create layout")
	}
	result := &layout{widgetBase: newWidgetBase(w, native)}
	if err := result.SetSpec(spec); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (w *window) CreateLabel(text string) (ui.Label, error) {
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	native := C.gkGTKCreateLabel(value)
	if native == nil {
		return nil, errors.New("ui/gtk: could not create label")
	}
	return &label{newWidgetBase(w, native)}, nil
}

func (w *window) CreateButton(text string) (ui.Button, error) {
	result := &button{}
	handle := cgo.NewHandle(result)
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	native := C.gkGTKCreateButton(value, C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create button")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

func (w *window) CreateTextField() (ui.TextField, error) {
	result := &textField{}
	handle := cgo.NewHandle(result)
	native := C.gkGTKCreateTextField(C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create text field")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

func (w *window) CreateTextArea(options ui.TextAreaOptions) (ui.TextArea, error) {
	result := &textArea{}
	handle := cgo.NewHandle(result)
	text := C.CString(options.Text)
	placeholder := C.CString(options.Placeholder)
	defer C.free(unsafe.Pointer(text))
	defer C.free(unsafe.Pointer(placeholder))
	native := C.gkGTKCreateTextArea(text, placeholder, boolInt(options.ReadOnly),
		boolInt(options.Wrap), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create text area")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

func (w *window) CreateCheckBox(text string, checked bool) (ui.CheckBox, error) {
	result := &checkBox{}
	handle := cgo.NewHandle(result)
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	native := C.gkGTKCreateCheckBox(value, boolInt(checked), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create check box")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

func (w *window) CreateRadioGroup(options ui.RadioGroupOptions) (ui.RadioGroup, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &radioGroup{selection: &selection{items: append([]string(nil), options.Items...)}}
	handle := cgo.NewHandle(result)
	native := C.gkGTKCreateRadioGroup(C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create radio group")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	addSelectionItems(native, options.Items)
	C.gkGTKSelectionSetIndex(native, C.int(options.SelectedIndex))
	return result, nil
}

func (w *window) CreateSelect(options ui.SelectOptions) (ui.Select, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &selectControl{selection: &selection{items: append([]string(nil), options.Items...)}}
	handle := cgo.NewHandle(result)
	native := C.gkGTKCreateSelect(C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create select control")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	addSelectionItems(native, options.Items)
	C.gkGTKSelectionSetIndex(native, C.int(options.SelectedIndex))
	return result, nil
}

func addSelectionItems(native unsafe.Pointer, items []string) {
	for _, item := range items {
		value := C.CString(item)
		C.gkGTKSelectionAddItem(native, value)
		C.free(unsafe.Pointer(value))
	}
}

func (w *window) CreateSlider(options ui.SliderOptions) (ui.Slider, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &slider{}
	handle := cgo.NewHandle(result)
	native := C.gkGTKCreateSlider(C.double(options.Minimum), C.double(options.Maximum),
		C.double(options.Value), C.double(options.Step), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create slider")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

func (w *window) CreateProgressBar(value float64) (ui.ProgressBar, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return nil, errors.New("ui: progress value must be between zero and one")
	}
	native := C.gkGTKCreateProgressBar(C.double(value))
	if native == nil {
		return nil, errors.New("ui/gtk: could not create progress bar")
	}
	return &progressBar{widgetBase: newWidgetBase(w, native)}, nil
}

func (w *window) CreateActivityIndicator(running bool) (ui.ActivityIndicator, error) {
	native := C.gkGTKCreateActivityIndicator(boolInt(running))
	if native == nil {
		return nil, errors.New("ui/gtk: could not create activity indicator")
	}
	return &activityIndicator{widgetBase: newWidgetBase(w, native)}, nil
}

func (w *window) CreateSeparator(direction ui.LayoutDirection) (ui.Separator, error) {
	if direction != ui.Horizontal && direction != ui.Vertical {
		return nil, errors.New("ui: invalid separator direction")
	}
	native := C.gkGTKCreateSeparator(C.int(direction))
	if native == nil {
		return nil, errors.New("ui/gtk: could not create separator")
	}
	return &separator{widgetBase: newWidgetBase(w, native), direction: direction}, nil
}

func (w *window) CreateImage(options ui.ImageOptions) (ui.ImageView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	native := C.gkGTKCreateImage(C.int(options.Scaling))
	if native == nil {
		return nil, errors.New("ui/gtk: could not create image")
	}
	result := &imageView{widgetBase: newWidgetBase(w, native), scaling: options.Scaling}
	if err := result.SetImage(options.Source); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

func (w *window) CreateLink(text, url string) (ui.Link, error) {
	result := &link{}
	handle := cgo.NewHandle(result)
	label := C.CString(text)
	target := C.CString(url)
	defer C.free(unsafe.Pointer(label))
	defer C.free(unsafe.Pointer(target))
	native := C.gkGTKCreateLink(label, target, C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create link")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

func (w *window) CreateTabs(options ui.TabsOptions) (ui.Tabs, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &tabs{}
	handle := cgo.NewHandle(result)
	native := C.gkGTKCreateTabs(C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create tabs")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	seen := make(map[*widgetBase]struct{}, len(options.Items))
	for _, item := range options.Items {
		child, ok := item.Content.(nativeWidget)
		if !ok || child.base().window != w || child.base().parent != nil || child.base().closed {
			result.Close()
			return nil, errors.New("ui/gtk: tab content belongs to another backend, window, or parent")
		}
		if _, exists := seen[child.base()]; exists {
			result.Close()
			return nil, errors.New("ui/gtk: tabs contain the same content more than once")
		}
		seen[child.base()] = struct{}{}
		child.base().parent = result
		result.items = append(result.items, item)
		title := C.CString(item.Title)
		added := C.gkGTKTabsAdd(native, title, child.base().nativePointer()) != 0
		C.free(unsafe.Pointer(title))
		if !added {
			result.Close()
			return nil, errors.New("ui/gtk: could not add tab")
		}
	}
	C.gkGTKTabsSetSelected(native, C.int(options.SelectedIndex))
	result.layoutContent()
	return result, nil
}

func (w *window) CreateScrollView(options ui.ScrollViewOptions) (ui.ScrollView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	content, ok := options.Content.(nativeWidget)
	if !ok || content.base().window != w || content.base().parent != nil || content.base().closed {
		return nil, errors.New("ui/gtk: scroll content belongs to another backend, window, or parent")
	}
	native := C.gkGTKCreateScrollView(C.int(options.Axes))
	if native == nil {
		return nil, errors.New("ui/gtk: could not create scroll view")
	}
	result := &scrollView{
		widgetBase: newWidgetBase(w, native),
		content:    content,
		axes:       options.Axes,
	}
	content.base().parent = result
	C.gkGTKWidgetSetParent(content.base().nativePointer(), native)
	result.layoutContent()
	return result, nil
}

func (w *window) CreateCanvas() (ui.Canvas, error) {
	native := C.gkGTKCreateCanvas()
	if native == nil {
		return nil, errors.New("ui/gtk: could not create canvas")
	}
	return &canvas{widgetBase: newWidgetBase(w, native)}, nil
}

func (w *window) CreateSection(options ui.SectionOptions) (ui.Section, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	content, ok := options.Content.(nativeWidget)
	if !ok || content.base().window != w || content.base().parent != nil {
		return nil, errors.New("ui/gtk: section content belongs to another backend, window, or parent")
	}
	result := &section{expanded: options.Expanded}
	handle := cgo.NewHandle(result)
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))
	native := C.gkGTKCreateSection(title, boolInt(options.Expanded), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create section")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	result.content = content
	content.base().parent = result
	C.gkGTKWidgetSetParent(content.base().nativePointer(), native)
	return result, nil
}

func (w *window) CreatePanel(options ui.PanelOptions) (ui.Panel, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if w.nativePointer() == nil {
		return nil, ui.ErrClosed
	}

	result := &panel{movable: !options.Fixed}
	handle := cgo.NewHandle(result)
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))
	native := C.gkGTKCreatePanel(
		w.nativePointer(),
		title,
		boolInt(!options.Fixed),
		C.int(options.Position.X),
		C.int(options.Position.Y),
		C.int(options.Size.Width),
		C.int(options.Size.Height),
		C.uintptr_t(handle),
	)
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/gtk: could not create panel")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	result.SetBounds(ui.Rect{Position: options.Position, Size: options.Size})
	if err := result.SetContent(options.Content); err != nil {
		result.Close()
		return nil, err
	}
	w.mu.Lock()
	w.panels = append(w.panels, result)
	w.mu.Unlock()
	result.BringToFront()
	return result, nil
}

func (w *window) OpenFile(ctx context.Context, options ui.OpenFileOptions) (string, error) {
	paths, err := w.openDialog(ctx, options, false, false)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

func (w *window) OpenFiles(ctx context.Context, options ui.OpenFileOptions) ([]string, error) {
	return w.openDialog(ctx, options, true, false)
}

func (w *window) SaveFile(ctx context.Context, options ui.SaveFileOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	native := w.nativePointer()
	if native == nil {
		return "", ui.ErrClosed
	}
	title := C.CString(options.Title)
	directory := C.CString(options.Directory)
	filename := C.CString(options.Filename)
	extensions := C.CString(fileExtensions(options.Filters))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(directory))
	defer C.free(unsafe.Pointer(filename))
	defer C.free(unsafe.Pointer(extensions))

	var size C.size_t
	var cancelled C.int
	result := C.gkGTKSaveDialog(
		native,
		title,
		directory,
		filename,
		extensions,
		boolInt(options.ConfirmOverwrite),
		&size,
		&cancelled,
	)
	paths, err := dialogPaths(result, size, cancelled)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

func (w *window) SelectDirectory(ctx context.Context, options ui.DirectoryDialogOptions) (string, error) {
	paths, err := w.openDialog(ctx, ui.OpenFileOptions{
		Title:     options.Title,
		Directory: options.Directory,
	}, false, true)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

func (w *window) openDialog(ctx context.Context, options ui.OpenFileOptions, multiple, directories bool) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	native := w.nativePointer()
	if native == nil {
		return nil, ui.ErrClosed
	}
	title := C.CString(options.Title)
	directory := C.CString(options.Directory)
	filename := C.CString(options.Filename)
	extensions := C.CString(fileExtensions(options.Filters))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(directory))
	defer C.free(unsafe.Pointer(filename))
	defer C.free(unsafe.Pointer(extensions))

	var size C.size_t
	var cancelled C.int
	result := C.gkGTKOpenDialog(
		native,
		title,
		directory,
		filename,
		extensions,
		boolInt(multiple),
		boolInt(directories),
		&size,
		&cancelled,
	)
	return dialogPaths(result, size, cancelled)
}

func dialogPaths(result *C.char, size C.size_t, cancelled C.int) ([]string, error) {
	if result != nil {
		defer C.free(unsafe.Pointer(result))
	}
	if cancelled != 0 {
		return nil, ui.ErrCancelled
	}
	if result == nil || size == 0 {
		return nil, ui.ErrNotSupported
	}

	data := C.GoBytes(unsafe.Pointer(result), C.int(size))
	parts := bytes.Split(data, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			paths = append(paths, string(part))
		}
	}
	if len(paths) == 0 {
		return nil, ui.ErrCancelled
	}
	return paths, nil
}

func fileExtensions(filters []ui.FileFilter) string {
	extensions := make([]string, 0)
	for _, filter := range filters {
		for _, pattern := range filter.Patterns {
			extension := strings.TrimPrefix(strings.TrimSpace(pattern), "*.")
			if extension != "" && extension != "*" {
				extensions = append(extensions, extension)
			}
		}
	}
	return strings.Join(extensions, ",")
}

func (w *window) KeyState(key keyboard.Key) keyboard.KeyAction { return keyboard.KeyReleased }
func (w *window) PointerButtonState(button pointer.Button) pointer.ButtonAction {
	return pointer.Released
}
func (w *window) PointerPosition() (x, y float64)              { return 0, 0 }
func (w *window) ScrollOffset() (x, y float64)                 { return 0, 0 }
func (w *window) PointerMode() pointer.Mode                    { return pointer.Normal }
func (w *window) SetPointerMode(mode pointer.Mode)             {}
func (w *window) OnKey(handler ui.KeyHandler) ui.Unsubscribe   { return noOpUnsubscribe }
func (w *window) OnText(handler ui.TextHandler) ui.Unsubscribe { return noOpUnsubscribe }
func (w *window) OnPointerButton(handler ui.PointerButtonHandler) ui.Unsubscribe {
	return noOpUnsubscribe
}
func (w *window) OnScroll(handler ui.ScrollHandler) ui.Unsubscribe { return noOpUnsubscribe }

func nativeText(native unsafe.Pointer) string {
	if native == nil {
		return ""
	}
	value := C.gkGTKWidgetText(native)
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}

func setNativeText(native unsafe.Pointer, text string) {
	if native == nil {
		return
	}
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	C.gkGTKWidgetSetText(native, value)
}

func boolInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func noOpUnsubscribe() {}

var _ ui.Window = (*window)(nil)
var _ ui.Layout = (*layout)(nil)
var _ ui.Label = (*label)(nil)
var _ ui.Button = (*button)(nil)
var _ ui.TextField = (*textField)(nil)
var _ ui.Slider = (*slider)(nil)
var _ ui.Canvas = (*canvas)(nil)
var _ ui.Panel = (*panel)(nil)
var _ ui.Section = (*section)(nil)
