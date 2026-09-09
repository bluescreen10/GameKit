//go:build cgo && (darwin || linux || windows)

package gamekit

/*
#include "window_bridge.h"
*/
import "C"

import (
	"runtime/cgo"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

// Event callbacks, in the GLFW shape: register one per window, and it is invoked from
// PollEvents on the goroutine that called it. A nil callback removes the previous one.
//
// Callbacks report things polling cannot: the characters a keystroke actually produced
// (which depend on layout and dead keys), auto-repeat at the rate the OS is configured
// for, scroll deltas, and events that begin and end inside a single frame.
type (
	// KeyCallback receives physical key transitions. scancode is the raw
	// platform-specific code, for keys with no portable identity.
	KeyCallback func(w *Window, key keyboard.Key, scancode int, action keyboard.KeyAction, mods keyboard.ModifierKey)

	// CharCallback receives text, already mapped through the user's keyboard layout.
	// This cannot be reconstructed from KeyCallback: layout, shift states and dead
	// keys all sit between a keystroke and the character it produces.
	CharCallback func(w *Window, char rune)

	// ScrollCallback receives scroll deltas. A trackpad reports fractional values and
	// both axes; a wheel mouse typically reports whole steps on y only.
	ScrollCallback func(w *Window, x, y float64)

	// PointerButtonCallback receives pointing-device button transitions.
	PointerButtonCallback func(w *Window, button pointer.Button, action pointer.ButtonAction, mods keyboard.ModifierKey)
)

// SetKeyCallback registers cb for physical key transitions, replacing any previous
// callback, and returns the one it replaced.
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

// SetCharCallback registers cb for typed text, replacing any previous callback, and
// returns the one it replaced.
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

// SetScrollCallback registers cb for scroll deltas, replacing any previous callback,
// and returns the one it replaced.
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

// SetPointerButtonCallback registers cb for pointing-device buttons, replacing any
// previous callback, and returns the one it replaced.
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

// windowFromHandle resolves the cgo.Handle the C layer was given back to its *Window.
//
// A handle rather than the native pointer, for two reasons: C may not store a Go
// pointer at all, and handles are never reused, so one can never come to name a
// different window the way a recycled address could.
//
// Note that Handle.Value panics on a handle that has been deleted — it does not return
// nil. Destroy therefore clears the C-side handle to zero BEFORE releasing the Go one,
// so anything still dispatching sees zero and stops here. That ordering is what makes
// this safe, not any tolerance for stale handles.
func windowFromHandle(h C.uintptr_t) *Window {
	if h == 0 {
		// Either the window is mid-construction and has no handle yet, or it has been
		// destroyed and its handle was cleared.
		return nil
	}
	w, _ := cgo.Handle(h).Value().(*Window)
	return w
}

// The C layer calls these for every event, whether or not a callback is registered —
// see the note in window_bridge.h. Key and button dispatch update Window's polling
// state first. Every path releases the lock before invoking user code, so callbacks
// are free to call back into the window.

//export gkGoKeyEvent
func gkGoKeyEvent(h C.uintptr_t, key, scancode, action, mods C.int) {
	w := windowFromHandle(h)
	if w == nil {
		return
	}
	w.dispatchKey(keyboard.Key(key), int(scancode), keyboard.KeyAction(action), keyboard.ModifierKey(mods))
}

//export gkGoCharEvent
func gkGoCharEvent(h C.uintptr_t, codepoint C.uint) {
	w := windowFromHandle(h)
	if w == nil {
		return
	}
	w.mu.RLock()
	cb := w.charCallback
	w.mu.RUnlock()
	if cb != nil {
		cb(w, rune(codepoint))
	}
}

//export gkGoScrollEvent
func gkGoScrollEvent(h C.uintptr_t, x, y C.double) {
	w := windowFromHandle(h)
	if w == nil {
		return
	}
	w.mu.RLock()
	cb := w.scrollCallback
	w.mu.RUnlock()
	if cb != nil {
		cb(w, float64(x), float64(y))
	}
}

//export gkGoPointerButtonEvent
func gkGoPointerButtonEvent(h C.uintptr_t, button, action, mods C.int) {
	w := windowFromHandle(h)
	if w == nil {
		return
	}
	w.dispatchPointerButton(pointer.Button(button), pointer.ButtonAction(action), keyboard.ModifierKey(mods))
}

// newWindowHandle allocates the cgo.Handle naming w to the C layer, and
// deleteWindowHandle releases it. Split out so window.go stays free of cgo.
func newWindowHandle(w *Window) uintptr { return uintptr(cgo.NewHandle(w)) }

func deleteWindowHandle(h uintptr) {
	if h != 0 {
		cgo.Handle(h).Delete()
	}
}
