//go:build !cgo || (!darwin && !linux && !windows)

package gamekit

import (
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

// Callback types and registration exist on every platform so that code compiles
// unchanged where native windows are unavailable; the callbacks simply never fire.
type (
	KeyCallback           func(w *Window, key keyboard.Key, scancode int, action keyboard.KeyAction, mods keyboard.ModifierKey)
	CharCallback          func(w *Window, char rune)
	ScrollCallback        func(w *Window, x, y float64)
	PointerButtonCallback func(w *Window, button pointer.Button, action pointer.ButtonAction, mods keyboard.ModifierKey)
)

func (w *Window) SetKeyCallback(cb KeyCallback) KeyCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.keyCallback
	w.keyCallback = cb
	return previous
}

func (w *Window) SetCharCallback(cb CharCallback) CharCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.charCallback
	w.charCallback = cb
	return previous
}

func (w *Window) SetScrollCallback(cb ScrollCallback) ScrollCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.scrollCallback
	w.scrollCallback = cb
	return previous
}

func (w *Window) SetPointerButtonCallback(cb PointerButtonCallback) PointerButtonCallback {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	previous := w.pointerButtonCallback
	w.pointerButtonCallback = cb
	return previous
}

// GetScroll returns zeroes when native windows are unavailable.
func (w *Window) GetScroll() (x, y float64) { return 0, 0 }

// SetPointerMode does nothing when native windows are unavailable.
func (w *Window) SetPointerMode(pointer.Mode) {}

// GetPointerMode reports the normal cursor when native windows are unavailable.
func (w *Window) GetPointerMode() pointer.Mode { return pointer.Normal }

// DisablePointer does nothing when native windows are unavailable.
func (w *Window) DisablePointer() {}

// EnablePointer does nothing when native windows are unavailable.
func (w *Window) EnablePointer() {}

// Handles are meaningless without a C layer to hand them to, so these are no-ops:
// nothing ever dispatches an event here.
func newWindowHandle(*Window) uintptr { return 0 }

func deleteWindowHandle(uintptr) {}
