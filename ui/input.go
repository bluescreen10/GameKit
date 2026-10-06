package ui

import (
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

// KeyEvent is one physical-key transition. Scancode is platform-specific and
// is useful for keys without a portable identity.
type KeyEvent struct {
	Key       keyboard.Key
	Scancode  int
	Action    keyboard.KeyAction
	Modifiers keyboard.ModifierKey
}

// PointerButtonEvent is one pointing-device button transition.
type PointerButtonEvent struct {
	Button    pointer.Button
	Action    pointer.ButtonAction
	Modifiers keyboard.ModifierKey
}

// ScrollEvent is one high-resolution scroll event.
type ScrollEvent struct {
	X float64
	Y float64
}

// KeyHandler receives physical-key transitions.
type KeyHandler func(event KeyEvent)

// TextHandler receives text after the operating system applies the active
// keyboard layout, modifiers, and dead keys.
type TextHandler func(char rune)

// PointerButtonHandler receives pointing-device button transitions.
type PointerButtonHandler func(event PointerButtonEvent)

// ScrollHandler receives high-resolution scrolling on both axes.
type ScrollHandler func(event ScrollEvent)

// Unsubscribe removes an event handler. It is safe to call more than once.
type Unsubscribe func()

// Input provides polling and event callbacks for keyboard and pointing
// devices. ScrollOffset is a running total, so several consumers can read it
// without consuming one another's input.
type Input interface {
	KeyState(key keyboard.Key) keyboard.KeyAction
	PointerButtonState(button pointer.Button) pointer.ButtonAction
	PointerPosition() (x, y float64)
	ScrollOffset() (x, y float64)

	PointerMode() pointer.Mode
	SetPointerMode(mode pointer.Mode)

	OnKey(handler KeyHandler) Unsubscribe
	OnText(handler TextHandler) Unsubscribe
	OnPointerButton(handler PointerButtonHandler) Unsubscribe
	OnScroll(handler ScrollHandler) Unsubscribe
}
