//go:build cgo && (darwin || linux || windows)

package gamekit

/*
#include "window_bridge.h"
*/
import "C"

import "github.com/bluescreen10/gamekit/pointer"

// GetPointerButton returns whether a pointing-device button is currently held. Call
// PollEvents once per frame to refresh it.
//
// This is level-triggered state, so a button pressed and released between two polls is
// missed entirely — register a PointerButtonCallback when every transition matters.
func (w *Window) GetPointerButton(button pointer.Button) pointer.ButtonAction {
	if native := w.nativePointer(); native != nil {
		return pointer.ButtonAction(C.gkWindowGetPointerButton(native, C.int(button)))
	}
	return pointer.Released
}

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

// SetCursorMode selects how the cursor behaves over this window.
func (w *Window) SetCursorMode(mode pointer.CursorMode) {
	if native := w.nativePointer(); native != nil {
		C.gkWindowSetCursorMode(native, C.int(mode))
	}
}

// CursorMode returns the window's current cursor mode.
func (w *Window) CursorMode() pointer.CursorMode {
	if native := w.nativePointer(); native != nil {
		return pointer.CursorMode(C.gkWindowGetCursorMode(native))
	}
	return pointer.CursorNormal
}

// DisableCursor hides the cursor and locks it to the window, so GetPointerPos reports
// unbounded virtual motion instead of a position inside the window.
//
// This is what a first-person camera needs: the pointer never reaches the edge of the
// display, so there is no limit on how far the view can turn. Shorthand for
// SetCursorMode(pointer.CursorDisabled).
func (w *Window) DisableCursor() { w.SetCursorMode(pointer.CursorDisabled) }

// EnableCursor restores the normal visible, free-moving cursor. Shorthand for
// SetCursorMode(pointer.CursorNormal).
func (w *Window) EnableCursor() { w.SetCursorMode(pointer.CursorNormal) }
