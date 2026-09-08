//go:build darwin && cgo

package gamekit

import (
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

// TestCocoaPointerInitialState: a freshly created window reports nothing held and no
// scroll accumulated, which is what a caller diffing against the first frame assumes.
func TestCocoaPointerInitialState(t *testing.T) {
	if cocoaTestWindowError != nil {
		t.Fatal(cocoaTestWindowError)
	}
	for _, button := range []pointer.Button{pointer.ButtonLeft, pointer.ButtonRight, pointer.ButtonMiddle} {
		if action := cocoaTestWindow.GetPointerButton(button); action != pointer.Released {
			t.Errorf("button %v = %v on a new window, want Released", button, action)
		}
	}
	if x, y := cocoaTestWindow.GetScroll(); x != 0 || y != 0 {
		t.Errorf("scroll = (%v, %v) on a new window, want (0, 0)", x, y)
	}
	if mode := cocoaTestWindow.CursorMode(); mode != pointer.CursorNormal {
		t.Errorf("cursor mode = %v on a new window, want CursorNormal", mode)
	}
}

// TestCocoaPointerButtonOutOfRange: an index past the tracked buttons must report
// Released rather than reading off the end of the array.
func TestCocoaPointerButtonOutOfRange(t *testing.T) {
	if cocoaTestWindowError != nil {
		t.Fatal(cocoaTestWindowError)
	}
	for _, button := range []pointer.Button{-1, pointer.ButtonCount, pointer.ButtonCount + 100} {
		if action := cocoaTestWindow.GetPointerButton(button); action != pointer.Released {
			t.Errorf("out-of-range button %v = %v, want Released", button, action)
		}
	}
}

// TestCocoaCursorModeRoundTrips checks the mode actually reaches the native layer and
// comes back.
//
// CursorDisabled is deliberately not exercised: it calls
// CGAssociateMouseAndMouseCursorPosition(false), which detaches the pointer for the
// whole system, and a test that failed midway would leave the developer's mouse frozen.
// CursorHidden takes the same path through gkWindowSetCursorMode without that risk.
func TestCocoaCursorModeRoundTrips(t *testing.T) {
	if cocoaTestWindowError != nil {
		t.Fatal(cocoaTestWindowError)
	}
	defer cocoaTestWindow.EnableCursor()

	cocoaTestWindow.SetCursorMode(pointer.CursorHidden)
	if mode := cocoaTestWindow.CursorMode(); mode != pointer.CursorHidden {
		t.Fatalf("cursor mode = %v after SetCursorMode(CursorHidden)", mode)
	}
	cocoaTestWindow.EnableCursor()
	if mode := cocoaTestWindow.CursorMode(); mode != pointer.CursorNormal {
		t.Fatalf("cursor mode = %v after EnableCursor", mode)
	}
}

// TestCocoaCallbacksRegisterOnANativeWindow: callbacks must attach to and detach from a
// real native window, not just the zero value the portable tests use.
//
// It stops short of calling PollEvents, which would be the interesting half: on macOS
// that must run on the main thread (see Window's doc comment), and Go runs every test
// function on another goroutine, so polling here aborts the process inside AppKit
// rather than testing anything. Exercising a real event round trip needs a harness that
// drives the loop from TestMain.
func TestCocoaCallbacksRegisterOnANativeWindow(t *testing.T) {
	if cocoaTestWindowError != nil {
		t.Fatal(cocoaTestWindowError)
	}
	defer cocoaTestWindow.SetKeyCallback(nil)
	defer cocoaTestWindow.SetCharCallback(nil)
	defer cocoaTestWindow.SetScrollCallback(nil)
	defer cocoaTestWindow.SetPointerButtonCallback(nil)

	cocoaTestWindow.SetKeyCallback(func(*Window, keyboard.Key, int, keyboard.KeyAction, keyboard.ModifierKey) {})
	cocoaTestWindow.SetCharCallback(func(*Window, rune) {})
	cocoaTestWindow.SetScrollCallback(func(*Window, float64, float64) {})
	cocoaTestWindow.SetPointerButtonCallback(func(*Window, pointer.Button, pointer.ButtonAction, keyboard.ModifierKey) {})

	if prev := cocoaTestWindow.SetScrollCallback(nil); prev == nil {
		t.Fatal("the scroll callback did not survive registration on a native window")
	}
}
