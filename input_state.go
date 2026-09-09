package gamekit

import (
	"github.com/bluescreen10/gamekit/keyboard"
	"github.com/bluescreen10/gamekit/pointer"
)

// GetKey reports whether key is currently held. Repeat is an event-stream detail;
// polling continues to report KeyPressed until the release event arrives.
func (w *Window) GetKey(key keyboard.Key) keyboard.KeyAction {
	if w == nil || key < 0 || key >= keyboard.Key(keyboard.KeyMenu+1) {
		return keyboard.KeyReleased
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.destroyed {
		return keyboard.KeyReleased
	}
	return w.keys[key]
}

// GetPointerButton reports whether button is currently held.
func (w *Window) GetPointerButton(button pointer.Button) pointer.ButtonAction {
	if w == nil || button < 0 || button >= pointer.ButtonCount {
		return pointer.Released
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.destroyed {
		return pointer.Released
	}
	return w.buttons[button]
}

func (w *Window) dispatchKey(key keyboard.Key, scancode int, action keyboard.KeyAction, mods keyboard.ModifierKey) {
	w.mu.Lock()
	if key >= 0 && key < keyboard.Key(keyboard.KeyMenu+1) {
		if action == keyboard.KeyReleased {
			w.keys[key] = keyboard.KeyReleased
		} else {
			// A repeated key-down remains pressed for polling. If a backend reports
			// repeated downs as ordinary presses, normalize subsequent ones here.
			if action == keyboard.KeyPressed && w.keys[key] == keyboard.KeyPressed {
				action = keyboard.KeyRepeat
			}
			w.keys[key] = keyboard.KeyPressed
		}
	}
	cb := w.keyCallback
	w.mu.Unlock()
	if cb != nil {
		cb(w, key, scancode, action, mods)
	}
}

func (w *Window) dispatchPointerButton(button pointer.Button, action pointer.ButtonAction, mods keyboard.ModifierKey) {
	w.mu.Lock()
	if button >= 0 && button < pointer.ButtonCount {
		w.buttons[button] = action
	}
	cb := w.pointerButtonCallback
	w.mu.Unlock()
	if cb != nil {
		cb(w, button, action, mods)
	}
}
