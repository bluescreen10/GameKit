package window

import (
	"errors"
	"sync"
	"unsafe"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
	"github.com/bluescreen10/gamekit/ui"
)

// WindowOptions controls optional native-window behavior. Its zero value creates
// a visible, fixed-size, decorated window, like SDL_CreateWindow without flags.
type WindowOptions struct {
	Resizable              bool
	Hidden                 bool
	Borderless             bool
	Maximized              bool
	AlwaysOnTop            bool
	TransparentFramebuffer bool
	Theme                  ui.Theme
}

// Window owns a native operating-system window. Window and event methods must
// be called from the goroutine that called CreateWindow. On macOS that goroutine
// must be the program's main goroutine.
type Window struct {
	mu        sync.RWMutex
	native    unsafe.Pointer
	title     string
	destroyed bool

	// handle identifies this window to the C layer, which cannot hold a Go pointer.
	// Released by Destroy; see callbacks.go.
	handle uintptr

	// Event callbacks, all guarded by mu. See callbacks.go.
	keyCallback           KeyCallback
	charCallback          CharCallback
	scrollCallback        ScrollCallback
	pointerButtonCallback PointerButtonCallback

	// Level-triggered input state. Native event dispatch updates these before calling
	// user callbacks, so callbacks and polling always observe the same state.
	keys    [keyboard.KeyMenu + 1]keyboard.KeyAction
	buttons [pointer.ButtonCount]pointer.ButtonAction

	content ui.Widget
	panels  []ui.Panel
	theme   ui.Theme

	handlersMu sync.RWMutex
	nextID     uint64

	keyHandlers           map[uint64]ui.KeyHandler
	textHandlers          map[uint64]ui.TextHandler
	pointerButtonHandlers map[uint64]ui.PointerButtonHandler
	scrollHandlers        map[uint64]ui.ScrollHandler
}

var (
	windowsMu sync.RWMutex
	windows   = map[*Window]struct{}{}
)

// CreateWindow creates a native window without an OpenGL context. Its native
// handle is exposed through NativeSurface for GPU presentation.
func CreateWindow(title string, width, height int, opts *WindowOptions) (*Window, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("ui/window: window width and height must be positive")
	}
	var options WindowOptions
	if opts != nil {
		options = *opts
	}
	if err := options.Theme.Validate(); err != nil {
		return nil, err
	}
	native, err := createNativeWindow(title, width, height, options)
	if err != nil {
		return nil, err
	}
	w := &Window{native: native, title: title, theme: options.Theme}
	windowsMu.Lock()
	windows[w] = struct{}{}
	windowsMu.Unlock()
	// Give the C layer a way to name this window when it dispatches an event.
	w.handle = newWindowHandle(w)
	setNativeWindowHandle(native, w.handle)
	setNativeWindowTheme(native, options.Theme)
	w.connectInput()
	return w, nil
}

// Destroy closes the native window. It is safe to call more than once.
func (w *Window) Destroy() {
	if w == nil {
		return
	}
	if w.content != nil {
		w.content.Close()
		w.content = nil
	}
	for _, panel := range w.panels {
		panel.Close()
	}
	w.panels = nil
	w.mu.Lock()
	if w.destroyed {
		w.mu.Unlock()
		return
	}
	native := w.native
	handle := w.handle
	w.native = nil
	w.handle = 0
	w.destroyed = true
	w.mu.Unlock()

	windowsMu.Lock()
	delete(windows, w)
	windowsMu.Unlock()
	// Clear the handle before the native window goes away, so an event still in flight
	// resolves to nothing rather than to a freed window.
	setNativeWindowHandle(native, 0)
	deleteWindowHandle(handle)
	destroyNativeWindow(native)
}

// Close is equivalent to Destroy.
func (w *Window) Close() {
	w.Destroy()
}

// PollEvents processes pending operating-system events for this window.
func (w *Window) PollEvents() {
	if native := w.nativePointer(); native != nil {
		pollNativeWindow(native)
	}
	if w.content != nil {
		bounds := ui.Rect{Size: w.Size()}
		if w.content.Bounds() != bounds {
			w.content.SetBounds(bounds)
		}
	}
}

// PollEvents processes events for every live GameKit window.
func PollEvents() {
	windowsMu.RLock()
	snapshot := make([]*Window, 0, len(windows))
	for w := range windows {
		snapshot = append(snapshot, w)
	}
	windowsMu.RUnlock()
	for _, w := range snapshot {
		w.PollEvents()
	}
}

// ShouldClose reports whether the user requested that the window close.
func (w *Window) ShouldClose() bool {
	if native := w.nativePointer(); native != nil {
		return nativeWindowShouldClose(native)
	}
	return true
}

// SetShouldClose changes the close-request flag without destroying the window.
func (w *Window) SetShouldClose(close bool) {
	if native := w.nativePointer(); native != nil {
		setNativeWindowShouldClose(native, close)
	}
}

// GetSize returns the native window's drawable client size in logical pixels.
func (w *Window) GetSize() (width, height int) {
	if native := w.nativePointer(); native != nil {
		return nativeWindowSize(native)
	}
	return 0, 0
}

// GetFramebufferSize returns the drawable size in physical pixels.
func (w *Window) GetFramebufferSize() (width, height int) {
	if native := w.nativePointer(); native != nil {
		return nativeWindowFramebufferSize(native)
	}
	return 0, 0
}

// Size returns the client size in logical pixels.
func (w *Window) Size() ui.Size {
	width, height := w.GetSize()
	return ui.Size{Width: width, Height: height}
}

// FramebufferSize returns the drawable size in physical pixels.
func (w *Window) FramebufferSize() ui.Size {
	width, height := w.GetFramebufferSize()
	return ui.Size{Width: width, Height: height}
}

// SetTitle changes the native window title.
func (w *Window) SetTitle(title string) {
	if native := w.nativePointer(); native != nil {
		setNativeWindowTitle(native, title)
		w.mu.Lock()
		w.title = title
		w.mu.Unlock()
	}
}

// GetTitle returns the last title assigned through GameKit.
func (w *Window) GetTitle() string {
	if w == nil {
		return ""
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.title
}

// Title returns the last title assigned through GameKit.
func (w *Window) Title() string {
	return w.GetTitle()
}

// Theme returns the window's requested native appearance.
func (w *Window) Theme() ui.Theme {
	if w == nil {
		return ui.ThemeSystem
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.theme
}

// SetTheme changes the native appearance of the window and its controls.
func (w *Window) SetTheme(theme ui.Theme) error {
	if err := theme.Validate(); err != nil {
		return err
	}
	native := w.nativePointer()
	if native == nil {
		return ui.ErrClosed
	}
	setNativeWindowTheme(native, theme)
	w.mu.Lock()
	w.theme = theme
	w.mu.Unlock()
	return nil
}

// GetNativeHandle returns NSWindow*, X11 Window, or HWND depending on the platform.
// Prefer passing the window to gpu.Backend.CreateSurface unless integrating
// another native API.
func (w *Window) GetNativeHandle() uintptr {
	if native := w.nativePointer(); native != nil {
		return nativeWindowHandle(native)
	}
	return 0
}

// GetNativeDisplay returns the X11 Display* on Linux and nil on other platforms.
func (w *Window) GetNativeDisplay() unsafe.Pointer {
	if native := w.nativePointer(); native != nil {
		return nativeWindowDisplay(native)
	}
	return nil
}

func (w *Window) nativePointer() unsafe.Pointer {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.native
}
