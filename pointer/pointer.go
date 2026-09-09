// Package pointer defines portable pointing-device buttons, actions and modifiers.
//
// "Pointer" rather than "mouse" because the device underneath may be a trackpad,
// trackball, stylus or touch surface — the same choice X11 (core pointer), Wayland
// (wl_pointer) and the W3C Pointer Events specification make. What GameKit reports is
// the abstract pointing device, whatever hardware is driving it.
//
// Values match the GLFW numbering so that code ported from GLFW, or layered over both,
// needs no translation table.
package pointer

// Button identifies a pointing-device button. Buttons beyond the named three are
// reported by index; a device with fewer simply never emits them.
type Button int

const (
	ButtonLeft   Button = 0
	ButtonRight  Button = 1
	ButtonMiddle Button = 2
	Button4      Button = 3
	Button5      Button = 4
	Button6      Button = 5
	Button7      Button = 6
	Button8      Button = 7

	// ButtonCount is the number of buttons GameKit tracks per window.
	ButtonCount = 8
)

// ButtonAction is a button's state, or the transition a callback is reporting.
type ButtonAction uint8

const (
	Released ButtonAction = 0
	Pressed  ButtonAction = 1
)

// Modifiers held during a pointer event are reported as keyboard.ModifierKey — they
// are keyboard state, and duplicating the type here would mean two spellings of the
// same bitmask.

// Mode selects how the pointer behaves over a window.
type Mode int

const (
	// CursorNormal shows the cursor and lets it leave the window.
	Normal Mode = iota

	// Hidden hides the pointer while it is over the window but otherwise leaves
	// it alone — it still moves, and can still leave.
	Hidden

	// Disabled hides the pointer and locks it to the window, reporting unbounded
	// virtual motion instead of a screen position. This is what a first-person camera
	// wants: the pointer never hits the edge of the display, so there is no limit to
	// how far the view can turn. Window.GetPointerPos then returns an accumulated
	// virtual position rather than a coordinate inside the window.
	Disabled
)
