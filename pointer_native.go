//go:build cgo && (darwin || linux || windows)

package gamekit

/*
#include "window_bridge.h"
*/
import "C"

import "github.com/bluescreen10/gamekit/pointer"

// GetScroll returns the scroll offsets accumulated since the window was created.
//
// This is a running total rather than a per-frame delta, matching GLFW: a caller that
// wants the delta remembers the previous value and subtracts. Totals compose safely
// when several consumers read the same window; a delta would be consumed by whichever
// one read it first.
func (w *Window) GetScroll() (x, y float64) {
	if native := w.nativePointer(); native != nil {
		var nativeX, nativeY C.double
		C.gkWindowGetScroll(native, &nativeX, &nativeY)
		return float64(nativeX), float64(nativeY)
	}
	return 0, 0
}

// SetPointerMode selects how the cursor behaves over this window.
func (w *Window) SetPointerMode(mode pointer.Mode) {
	if native := w.nativePointer(); native != nil {
		C.gkWindowSetCursorMode(native, C.int(mode))
	}
}

// GetPointerMode returns the window's current cursor mode.
func (w *Window) GetPointerMode() pointer.Mode {
	if native := w.nativePointer(); native != nil {
		return pointer.Mode(C.gkWindowGetCursorMode(native))
	}
	return pointer.Normal
}

// DisablePointer hides the cursor and locks it to the window, so GetPointerPos reports
// unbounded virtual motion instead of a position inside the window.
//
// This is what a first-person camera needs: the pointer never reaches the edge of the
// display, so there is no limit on how far the view can turn. Shorthand for
// SetPointerMode(pointer.Disabled).
func (w *Window) DisablePointer() { w.SetPointerMode(pointer.Disabled) }

// EnablePointer restores the normal visible, free-moving cursor. Shorthand for
// SetPointerMode(pointer.Normal).
func (w *Window) EnablePointer() { w.SetPointerMode(pointer.Normal) }
