// Package gamekit provides compatibility aliases for the native window API.
// New code should use github.com/bluescreen10/gamekit/ui and ui/window.
package gamekit

import "github.com/bluescreen10/gamekit/ui/window"

type (
	// WindowOptions controls optional native-window behavior.
	WindowOptions = window.WindowOptions
	// Window owns a native operating-system window.
	Window = window.Window

	// KeyCallback receives physical key transitions.
	KeyCallback = window.KeyCallback
	// CharCallback receives text mapped through the user's keyboard layout.
	CharCallback = window.CharCallback
	// ScrollCallback receives high-resolution scrolling on both axes.
	ScrollCallback = window.ScrollCallback
	// PointerButtonCallback receives pointing-device button transitions.
	PointerButtonCallback = window.PointerButtonCallback
)

// CreateWindow creates a native window. Deprecated: use ui/window.Backend.
func CreateWindow(title string, width, height int, options *WindowOptions) (*Window, error) {
	return window.CreateWindow(title, width, height, options)
}

// PollEvents processes events for every live native window.
// Deprecated: call Window.PollEvents.
func PollEvents() {
	window.PollEvents()
}
