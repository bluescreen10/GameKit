//go:build darwin && cgo

package gamekit

import (
	"math"
	"os"
	"testing"

	"github.com/bluescreen10/gamekit/keyboard"
)

var cocoaTestWindow *Window
var cocoaTestWindowError error

func TestMain(m *testing.M) {
	cocoaTestWindow, cocoaTestWindowError = CreateWindow("GameKit test", 320, 200, &WindowOptions{Hidden: true, Resizable: true})
	code := m.Run()
	if cocoaTestWindow != nil {
		cocoaTestWindow.Destroy()
	}
	os.Exit(code)
}

func TestNativeCocoaWindow(t *testing.T) {
	if cocoaTestWindowError != nil {
		t.Fatal(cocoaTestWindowError)
	}
	if cocoaTestWindow.NativeHandle() == 0 {
		t.Fatal("native NSWindow handle is missing")
	}
	if width, height := cocoaTestWindow.Size(); width != 320 || height != 200 {
		t.Fatalf("window size = %dx%d, want 320x200", width, height)
	}
	x, y := cocoaTestWindow.GetCursorPos()
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		t.Fatalf("cursor position = (%v, %v), want finite coordinates", x, y)
	}
	if action := cocoaTestWindow.GetKey(keyboard.KeyEsc); action != keyboard.KeyReleased {
		t.Fatalf("initial escape action = %v, want KeyReleased", action)
	}
	cocoaTestWindow.SetShouldClose(true)
	if !cocoaTestWindow.ShouldClose() {
		t.Fatal("SetShouldClose(true) was ignored")
	}
}
