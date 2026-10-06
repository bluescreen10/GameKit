//go:build cgo && (darwin || windows)

package window

/*
#cgo darwin LDFLAGS: -framework QuartzCore
#include "widget_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	stdimage "image"
	"image/draw"
	"math"
	"runtime"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/ui"
)

type nativeWidget interface {
	ui.Widget
	base() *widgetBase
}

type widgetBase struct {
	window *Window
	native unsafe.Pointer
	handle uintptr

	mu      sync.RWMutex
	bounds  ui.Rect
	parent  nativeWidget
	visible bool
	enabled bool
	closed  bool
}

func newWidgetBase(window *Window, native unsafe.Pointer) *widgetBase {
	return &widgetBase{
		window:  window,
		native:  native,
		visible: true,
		enabled: true,
	}
}

func (w *widgetBase) base() *widgetBase {
	return w
}

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
		C.gkUIClearAction(native)
		cgo.Handle(handle).Delete()
	}
	C.gkUIDestroy(native)
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

	C.gkUISetFrame(
		native,
		C.int(bounds.Position.X),
		C.int(bounds.Position.Y),
		C.int(bounds.Size.Width),
		C.int(bounds.Size.Height),
	)
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
	C.gkUIPreferredSize(native, &width, &height)
	return ui.Size{Width: int(width), Height: int(height)}
}

func (w *widgetBase) IsVisible() bool {
	if w == nil {
		return false
	}
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

	C.gkUISetVisible(native, boolInt(visible))
}

func (w *widgetBase) IsEnabled() bool {
	if w == nil {
		return false
	}
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

	C.gkUISetEnabled(native, boolInt(enabled))
}

func (w *widgetBase) nativePointer() unsafe.Pointer {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.native
}

func boolInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
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

	handlers := make([]T, 0, len(l.handlers))
	for _, handler := range l.handlers {
		handlers = append(handlers, handler)
	}
	return handlers
}

type layout struct {
	*widgetBase

	mu   sync.RWMutex
	spec ui.LayoutSpec
}

func (l *layout) base() *widgetBase {
	return l.widgetBase
}

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
		base := child.base()
		base.parent = nil
		C.gkUIAttachToWindow(l.window.nativeWindowHandle(), base.nativePointer())
	}
	for _, child := range children {
		base := child.base()
		base.parent = l
		C.gkUISetParent(base.nativePointer(), l.nativePointer())
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
	children := make([]nativeWidget, 0, len(spec.Items))
	for _, item := range spec.Items {
		if item.Widget == nil {
			continue
		}
		child, ok := item.Widget.(nativeWidget)
		if !ok {
			return nil, errors.New("ui/window: layout child belongs to another backend")
		}
		children = append(children, child)
	}
	seen := make(map[*widgetBase]struct{}, len(children))
	for _, child := range children {
		base := child.base()
		if _, floating := child.(*panel); floating {
			return nil, errors.New("ui/window: floating panel cannot be a layout child")
		}
		if base.window != l.window {
			return nil, errors.New("ui/window: layout child belongs to another window")
		}
		if base.closed {
			return nil, ui.ErrClosed
		}
		if base.parent != nil && base.parent.base() != l.widgetBase {
			return nil, errors.New("ui/window: layout child already has a parent")
		}
		if _, exists := seen[base]; exists {
			return nil, errors.New("ui/window: layout contains the same child more than once")
		}
		seen[base] = struct{}{}
		for ancestor := nativeWidget(l); ancestor != nil; ancestor = ancestor.base().parent {
			if base == ancestor.base() {
				return nil, errors.New("ui/window: layout cycle")
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
		if !ok {
			continue
		}
		children = append(children, child)
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

type label struct {
	*widgetBase
}

func (l *label) Text() string {
	return nativeText(l.nativePointer())
}

func (l *label) SetText(text string) {
	setNativeText(l.nativePointer(), text)
}

type button struct {
	*widgetBase
	clicks listeners[ui.ClickHandler]
}

func (b *button) Text() string {
	return nativeText(b.nativePointer())
}

func (b *button) SetText(text string) {
	setNativeText(b.nativePointer(), text)
}

func (b *button) OnClick(handler ui.ClickHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return b.clicks.add(handler)
}

func (b *button) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_CLICK) {
		return
	}
	for _, handler := range b.clicks.snapshot() {
		handler()
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
	value := C.gkUIPlaceholder(native)
	if value == nil {
		return ""
	}
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
	C.gkUISetPlaceholder(native, value)
}
func (t *textArea) IsReadOnly() bool {
	native := t.nativePointer()
	return native != nil && C.gkUIReadOnly(native) != 0
}
func (t *textArea) SetReadOnly(readOnly bool) {
	if native := t.nativePointer(); native != nil {
		C.gkUISetReadOnly(native, boolInt(readOnly))
	}
}
func (t *textArea) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}
func (t *textArea) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_TEXT_CHANGE) {
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
	return native != nil && C.gkUIChecked(native) != 0
}
func (c *checkBox) SetChecked(checked bool) {
	if native := c.nativePointer(); native != nil {
		C.gkUISetChecked(native, boolInt(checked))
	}
}
func (c *checkBox) OnChange(handler ui.ToggleHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return c.changes.add(handler)
}
func (c *checkBox) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_TOGGLE_CHANGE) {
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
	return int(C.gkUISelectionIndex(native))
}
func (s *selection) SetSelectedIndex(index int) error {
	if index < 0 || index >= len(s.items) {
		return errors.New("ui: selection is out of range")
	}
	native := s.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkUISetSelectionIndex(native, C.int(index))
	return nil
}
func (s *selection) OnChange(handler ui.SelectionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}
func (s *selection) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_SELECTION_CHANGE) {
		return
	}
	index := s.SelectedIndex()
	for _, handler := range s.changes.snapshot() {
		handler(index)
	}
}

type radioGroup struct{ *selection }
type selectControl struct{ *selection }

func (t *textField) Text() string {
	return nativeText(t.nativePointer())
}

func (t *textField) SetText(text string) {
	setNativeText(t.nativePointer(), text)
}

func (t *textField) Placeholder() string {
	native := t.nativePointer()
	if native == nil {
		return ""
	}
	value := C.gkUIPlaceholder(native)
	if value == nil {
		return ""
	}
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
	C.gkUISetPlaceholder(native, value)
}

func (t *textField) OnChange(handler ui.TextChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return t.changes.add(handler)
}

func (t *textField) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_TEXT_CHANGE) {
		return
	}
	text := t.Text()
	for _, handler := range t.changes.snapshot() {
		handler(text)
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
	return float64(C.gkUIProgressValue(native))
}
func (p *progressBar) SetValue(value float64) {
	if native := p.nativePointer(); native != nil {
		C.gkUISetProgressValue(native, C.double(value))
	}
}

type activityIndicator struct{ *widgetBase }

func (a *activityIndicator) IsRunning() bool {
	native := a.nativePointer()
	return native != nil && C.gkUIActivityRunning(native) != 0
}
func (a *activityIndicator) SetRunning(running bool) {
	if native := a.nativePointer(); native != nil {
		C.gkUISetActivityRunning(native, boolInt(running))
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
	C.gkUIImageSet(native, (*C.uchar)(unsafe.Pointer(&rgba.Pix[0])),
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
	C.gkUIImageSetScaling(native, C.int(scaling))
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
	value := C.gkUILinkURL(native)
	if value == nil {
		return ""
	}
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
	C.gkUISetLinkURL(native, value)
}
func (l *link) OnClick(handler ui.ClickHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return l.clicks.add(handler)
}
func (l *link) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_CLICK) {
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
	C.gkUIScrollOffset(native, &x, &y)
	return ui.Point{X: int(x), Y: int(y)}
}

func (s *scrollView) SetScrollOffset(offset ui.Point) {
	if native := s.nativePointer(); native != nil {
		C.gkUIScrollSetOffset(native, C.int(max(0, offset.X)), C.int(max(0, offset.Y)))
	}
}

func (s *scrollView) layoutContent() {
	native := s.nativePointer()
	if native == nil || s.content == nil {
		return
	}
	var width, height C.int
	C.gkUIScrollViewportSize(native, &width, &height)
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
	C.gkUIScrollSetContentSize(native, C.int(contentSize.Width), C.int(contentSize.Height))
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
	t.stateMu.RLock()
	items := append([]ui.TabItem(nil), t.items...)
	t.stateMu.RUnlock()
	size := ui.Size{Width: 160, Height: 100}
	for _, item := range items {
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
	return int(C.gkUITabsSelected(native))
}
func (t *tabs) SetSelectedIndex(index int) error {
	t.stateMu.RLock()
	count := len(t.items)
	t.stateMu.RUnlock()
	if index < 0 || index >= count {
		return errors.New("ui: tab selection is out of range")
	}
	native := t.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	C.gkUITabsSetSelected(native, C.int(index))
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
	if event != int(C.GK_UI_EVENT_TAB_CHANGE) {
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
	C.gkUITabsContentSize(native, &width, &height)
	selected := t.SelectedIndex()
	for index, item := range t.Items() {
		item.Content.SetVisible(index == selected)
		if index == selected {
			item.Content.SetBounds(ui.Rect{Size: ui.Size{Width: int(width), Height: int(height)}})
		}
	}
}

func (s *slider) Value() float64 {
	native := s.nativePointer()
	if native == nil {
		return 0
	}
	return float64(C.gkUISliderValue(native))
}

func (s *slider) SetValue(value float64) {
	if native := s.nativePointer(); native != nil {
		C.gkUISetSliderValue(native, C.double(value))
	}
}

func (s *slider) OnChange(handler ui.ValueChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}

func (s *slider) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_VALUE_CHANGE) {
		return
	}
	value := s.Value()
	for _, handler := range s.changes.snapshot() {
		handler(value)
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
	if bounds.Size == previous {
		return
	}
	for _, handler := range c.resizes.snapshot() {
		handler(bounds.Size)
	}
}

func (c *canvas) NativeSurface() gpu.NativeSurface {
	native := c.nativePointer()
	if native == nil {
		return gpu.NativeSurface{}
	}
	kind := gpu.NativeSurfaceCocoaView
	if runtime.GOOS == "windows" {
		kind = gpu.NativeSurfaceWin32Window
	}
	return gpu.NativeSurface{
		Kind:   kind,
		Handle: uintptr(C.gkUIControlHandle(native)),
	}
}

func (c *canvas) FramebufferSize() ui.Size {
	native := c.nativePointer()
	if native == nil {
		return ui.Size{}
	}
	var width, height C.int
	C.gkUIFramebufferSize(native, &width, &height)
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
}

func (c *canvas) TakeRedrawRequest() bool {
	return c.redraw.Swap(false)
}

const (
	panelHeaderHeight   = 32
	sectionHeaderHeight = 28
)

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
		C.gkUIAttachToWindow(p.window.nativeWindowHandle(), previous.base().nativePointer())
		previous.SetVisible(false)
	}
	p.content = child
	p.stateMu.Unlock()

	child.base().parent = p
	C.gkUISetParent(child.base().nativePointer(), p.nativePointer())
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
		C.gkUIPanelSetMovable(native, boolInt(movable))
	}
}

func (p *panel) BringToFront() {
	if native := p.nativePointer(); native != nil {
		C.gkUIBringToFront(native)
	}
}

func (p *panel) OnMove(handler ui.MoveHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return p.moves.add(handler)
}

func (p *panel) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_PANEL_MOVE) {
		return
	}
	native := p.nativePointer()
	if native == nil {
		return
	}
	var x, y C.int
	C.gkUIPanelPosition(native, &x, &y)
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
	s.layoutContent()
}

func (s *section) PreferredSize() ui.Size {
	s.stateMu.RLock()
	content := s.content
	expanded := s.expanded
	s.stateMu.RUnlock()
	if content == nil || !expanded {
		return ui.Size{Width: 120, Height: sectionHeaderHeight}
	}
	size := content.PreferredSize()
	return ui.Size{
		Width:  max(120, size.Width),
		Height: sectionHeaderHeight + size.Height,
	}
}

func (s *section) Title() string {
	return nativeText(s.nativePointer())
}

func (s *section) SetTitle(title string) {
	setNativeText(s.nativePointer(), title)
}

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
		C.gkUISectionSetExpanded(native, boolInt(expanded))
	}
	s.relayoutParent()
}

func (s *section) OnChange(handler ui.ExpansionChangeHandler) ui.Unsubscribe {
	if handler == nil {
		return noOpUnsubscribe
	}
	return s.changes.add(handler)
}

func (s *section) nativeEvent(event int) {
	if event != int(C.GK_UI_EVENT_SECTION_CHANGE) {
		return
	}
	native := s.nativePointer()
	if native == nil {
		return
	}
	expanded := C.gkUISectionExpanded(native) != 0
	s.stateMu.Lock()
	s.expanded = expanded
	s.stateMu.Unlock()
	s.relayoutParent()
	for _, handler := range s.changes.snapshot() {
		handler(expanded)
	}
}

func (s *section) layoutContent() {
	s.stateMu.RLock()
	content := s.content
	expanded := s.expanded
	s.stateMu.RUnlock()
	if content == nil || !expanded {
		return
	}
	size := s.Bounds().Size
	content.SetBounds(ui.Rect{Size: ui.Size{
		Width:  size.Width,
		Height: max(0, size.Height-sectionHeaderHeight),
	}})
}

func (s *section) relayoutParent() {
	parent := s.base().parent
	if layout, ok := parent.(*layout); ok {
		layout.applyLayout(layout.Bounds().Size)
	}
	s.layoutContent()
}

func validateContainerChild(container nativeWidget, content ui.Widget) (nativeWidget, error) {
	child, ok := content.(nativeWidget)
	if !ok {
		return nil, errors.New("ui/window: container content belongs to another backend")
	}
	if _, floating := child.(*panel); floating {
		return nil, errors.New("ui/window: a floating panel cannot be container content")
	}
	base := child.base()
	owner := container.base()
	if base.window != owner.window {
		return nil, errors.New("ui/window: container content belongs to another window")
	}
	if base.closed {
		return nil, ui.ErrClosed
	}
	if base.parent != nil && base.parent.base() != owner {
		return nil, errors.New("ui/window: container content already has a parent")
	}
	for ancestor := container; ancestor != nil; ancestor = ancestor.base().parent {
		if base == ancestor.base() {
			return nil, errors.New("ui/window: container cycle")
		}
	}
	return child, nil
}

type nativeEventReceiver interface {
	nativeEvent(event int)
}

//export gkUIGoEvent
func gkUIGoEvent(handle C.uintptr_t, event C.int) {
	if handle == 0 {
		return
	}
	receiver, ok := cgo.Handle(handle).Value().(nativeEventReceiver)
	if ok {
		receiver.nativeEvent(int(event))
	}
}

// SetContent makes content fill the native window's content area.
func (w *Window) SetContent(content ui.Widget) error {
	widget, ok := content.(nativeWidget)
	if !ok || widget.base().window != w {
		return errors.New("ui/window: content belongs to another backend or window")
	}
	if widget.base().parent != nil {
		return errors.New("ui/window: content already belongs to a layout")
	}
	if _, floating := widget.(*panel); floating {
		return errors.New("ui/window: floating panel cannot be window content")
	}
	if w.content != nil && w.content != content {
		w.content.SetVisible(false)
	}

	C.gkUIAttachToWindow(w.nativeWindowHandle(), widget.base().nativePointer())
	w.content = content
	content.SetVisible(true)
	content.SetBounds(ui.Rect{Size: w.Size()})
	for _, floating := range w.panels {
		floating.BringToFront()
	}
	return nil
}

// CreateLayout creates a native layout owned by the window.
func (w *Window) CreateLayout(spec ui.LayoutSpec) (ui.Layout, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	native := C.gkUICreateLayout(w.nativeWindowHandle())
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create a layout")
	}
	result := &layout{widgetBase: newWidgetBase(w, native)}
	if err := result.SetSpec(spec); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

// CreateLabel creates a native label owned by the window.
func (w *Window) CreateLabel(text string) (ui.Label, error) {
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	native := C.gkUICreateLabel(w.nativeWindowHandle(), value)
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create a label")
	}
	return &label{widgetBase: newWidgetBase(w, native)}, nil
}

// CreateButton creates a native push button owned by the window.
func (w *Window) CreateButton(text string) (ui.Button, error) {
	result := &button{}
	handle := cgo.NewHandle(result)
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	native := C.gkUICreateButton(w.nativeWindowHandle(), value, C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a button")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

// CreateTextField creates a native single-line text editor owned by the window.
func (w *Window) CreateTextField() (ui.TextField, error) {
	result := &textField{}
	handle := cgo.NewHandle(result)
	native := C.gkUICreateTextField(w.nativeWindowHandle(), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a text field")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

// CreateTextArea creates a native multiline text editor owned by the window.
func (w *Window) CreateTextArea(options ui.TextAreaOptions) (ui.TextArea, error) {
	result := &textArea{}
	handle := cgo.NewHandle(result)
	text := C.CString(options.Text)
	placeholder := C.CString(options.Placeholder)
	defer C.free(unsafe.Pointer(text))
	defer C.free(unsafe.Pointer(placeholder))
	native := C.gkUICreateTextArea(w.nativeWindowHandle(), text, placeholder,
		boolInt(options.ReadOnly), boolInt(options.Wrap), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a text area")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

// CreateCheckBox creates a native check box owned by the window.
func (w *Window) CreateCheckBox(text string, checked bool) (ui.CheckBox, error) {
	result := &checkBox{}
	handle := cgo.NewHandle(result)
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	native := C.gkUICreateCheckBox(w.nativeWindowHandle(), value,
		boolInt(checked), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a check box")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

// CreateRadioGroup creates a native group of mutually exclusive choices.
func (w *Window) CreateRadioGroup(options ui.RadioGroupOptions) (ui.RadioGroup, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &radioGroup{selection: &selection{items: append([]string(nil), options.Items...)}}
	handle := cgo.NewHandle(result)
	native := C.gkUICreateRadioGroup(w.nativeWindowHandle(), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a radio group")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	addSelectionItems(native, options.Items)
	C.gkUISetSelectionIndex(native, C.int(options.SelectedIndex))
	return result, nil
}

// CreateSelect creates a native drop-down selection control.
func (w *Window) CreateSelect(options ui.SelectOptions) (ui.Select, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &selectControl{selection: &selection{items: append([]string(nil), options.Items...)}}
	handle := cgo.NewHandle(result)
	native := C.gkUICreateSelect(w.nativeWindowHandle(), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a select control")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	addSelectionItems(native, options.Items)
	C.gkUISetSelectionIndex(native, C.int(options.SelectedIndex))
	return result, nil
}

func addSelectionItems(native unsafe.Pointer, items []string) {
	for _, item := range items {
		value := C.CString(item)
		C.gkUISelectionAddItem(native, value)
		C.free(unsafe.Pointer(value))
	}
}

// CreateSlider creates a native slider owned by the window.
func (w *Window) CreateSlider(options ui.SliderOptions) (ui.Slider, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &slider{}
	handle := cgo.NewHandle(result)
	native := C.gkUICreateSlider(
		w.nativeWindowHandle(),
		C.double(options.Minimum),
		C.double(options.Maximum),
		C.double(options.Value),
		C.double(options.Step),
		C.uintptr_t(handle),
	)
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a slider")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

// CreateProgressBar creates a native determinate progress indicator.
func (w *Window) CreateProgressBar(value float64) (ui.ProgressBar, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return nil, errors.New("ui: progress value must be between zero and one")
	}
	native := C.gkUICreateProgressBar(w.nativeWindowHandle(), C.double(value))
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create a progress bar")
	}
	return &progressBar{widgetBase: newWidgetBase(w, native)}, nil
}

// CreateActivityIndicator creates a native indeterminate activity indicator.
func (w *Window) CreateActivityIndicator(running bool) (ui.ActivityIndicator, error) {
	native := C.gkUICreateActivityIndicator(w.nativeWindowHandle(), boolInt(running))
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create an activity indicator")
	}
	return &activityIndicator{widgetBase: newWidgetBase(w, native)}, nil
}

// CreateSeparator creates a native visual divider.
func (w *Window) CreateSeparator(direction ui.LayoutDirection) (ui.Separator, error) {
	if direction != ui.Horizontal && direction != ui.Vertical {
		return nil, errors.New("ui: invalid separator direction")
	}
	native := C.gkUICreateSeparator(w.nativeWindowHandle(), C.int(direction))
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create a separator")
	}
	return &separator{widgetBase: newWidgetBase(w, native), direction: direction}, nil
}

// CreateImage creates a native image view.
func (w *Window) CreateImage(options ui.ImageOptions) (ui.ImageView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	native := C.gkUICreateImage(w.nativeWindowHandle(), C.int(options.Scaling))
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create an image")
	}
	result := &imageView{widgetBase: newWidgetBase(w, native), scaling: options.Scaling}
	if err := result.SetImage(options.Source); err != nil {
		result.Close()
		return nil, err
	}
	return result, nil
}

// CreateLink creates a native hyperlink control.
func (w *Window) CreateLink(text, url string) (ui.Link, error) {
	result := &link{}
	handle := cgo.NewHandle(result)
	label := C.CString(text)
	target := C.CString(url)
	defer C.free(unsafe.Pointer(label))
	defer C.free(unsafe.Pointer(target))
	native := C.gkUICreateLink(w.nativeWindowHandle(), label, target, C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a link")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	return result, nil
}

// CreateTabs creates a native tab view.
func (w *Window) CreateTabs(options ui.TabsOptions) (ui.Tabs, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &tabs{}
	handle := cgo.NewHandle(result)
	native := C.gkUICreateTabs(w.nativeWindowHandle(), C.uintptr_t(handle))
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create tabs")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	seen := make(map[*widgetBase]struct{}, len(options.Items))
	for _, item := range options.Items {
		child, err := validateContainerChild(result, item.Content)
		if err != nil {
			result.Close()
			return nil, err
		}
		if _, exists := seen[child.base()]; exists {
			result.Close()
			return nil, errors.New("ui/window: tabs contain the same content more than once")
		}
		seen[child.base()] = struct{}{}
		child.base().parent = result
		result.items = append(result.items, item)
		title := C.CString(item.Title)
		added := C.gkUITabsAdd(native, title, child.base().nativePointer()) != 0
		C.free(unsafe.Pointer(title))
		if !added {
			result.Close()
			return nil, errors.New("ui/window: native backend could not add a tab")
		}
	}
	C.gkUITabsSetSelected(native, C.int(options.SelectedIndex))
	result.layoutContent()
	return result, nil
}

// CreateScrollView creates a native scrolling viewport owned by the window.
func (w *Window) CreateScrollView(options ui.ScrollViewOptions) (ui.ScrollView, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	result := &scrollView{axes: options.Axes}
	native := C.gkUICreateScrollView(w.nativeWindowHandle(), C.int(options.Axes))
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create a scroll view")
	}
	result.widgetBase = newWidgetBase(w, native)
	content, err := validateContainerChild(result, options.Content)
	if err != nil {
		result.Close()
		return nil, err
	}
	result.content = content
	content.base().parent = result
	C.gkUISetParent(content.base().nativePointer(), native)
	result.layoutContent()
	return result, nil
}

// CreateCanvas creates a GPU-presentable native control owned by the window.
func (w *Window) CreateCanvas() (ui.Canvas, error) {
	native := C.gkUICreateCanvas(w.nativeWindowHandle())
	if native == nil {
		return nil, errors.New("ui/window: native backend could not create a canvas")
	}
	return &canvas{widgetBase: newWidgetBase(w, native)}, nil
}

// CreatePanel creates a floating native panel owned by the window.
func (w *Window) CreatePanel(options ui.PanelOptions) (ui.Panel, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	result := &panel{movable: !options.Fixed}
	handle := cgo.NewHandle(result)
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))
	native := C.gkUICreatePanel(
		w.nativeWindowHandle(),
		title,
		boolInt(!options.Fixed),
		C.uintptr_t(handle),
	)
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a panel")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	result.SetBounds(ui.Rect{Position: options.Position, Size: options.Size})
	if err := result.SetContent(options.Content); err != nil {
		result.Close()
		return nil, err
	}
	w.panels = append(w.panels, result)
	result.BringToFront()
	return result, nil
}

// CreateSection creates a collapsible native section owned by the window.
func (w *Window) CreateSection(options ui.SectionOptions) (ui.Section, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}

	result := &section{expanded: options.Expanded}
	handle := cgo.NewHandle(result)
	title := C.CString(options.Title)
	defer C.free(unsafe.Pointer(title))
	native := C.gkUICreateSection(
		w.nativeWindowHandle(),
		title,
		boolInt(options.Expanded),
		C.uintptr_t(handle),
	)
	if native == nil {
		handle.Delete()
		return nil, errors.New("ui/window: native backend could not create a section")
	}
	result.widgetBase = newWidgetBase(w, native)
	result.handle = uintptr(handle)
	child, err := validateContainerChild(result, options.Content)
	if err != nil {
		result.Close()
		return nil, err
	}
	result.content = child
	child.base().parent = result
	C.gkUISetParent(child.base().nativePointer(), result.nativePointer())
	return result, nil
}

func (w *Window) nativeWindowHandle() C.uintptr_t {
	return C.uintptr_t(w.GetNativeHandle())
}

func nativeText(native unsafe.Pointer) string {
	if native == nil {
		return ""
	}
	value := C.gkUIText(native)
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}

func setNativeText(native unsafe.Pointer, text string) {
	if native == nil {
		return
	}
	value := C.CString(text)
	defer C.free(unsafe.Pointer(value))
	C.gkUISetText(native, value)
}

var _ ui.Layout = (*layout)(nil)
var _ ui.Label = (*label)(nil)
var _ ui.Button = (*button)(nil)
var _ ui.TextField = (*textField)(nil)
var _ ui.Slider = (*slider)(nil)
var _ ui.Canvas = (*canvas)(nil)
var _ ui.Panel = (*panel)(nil)
var _ ui.Section = (*section)(nil)
