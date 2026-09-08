//go:build cgo && (darwin || linux || windows)

package gamekit

/*
#include "window_bridge.h"
*/
import "C"

import "github.com/bluescreen10/gamekit/keyboard"

// GetKey returns the most recent native action for key. Call PollEvents once per
// frame to update keyboard state and receive operating-system key-repeat events.
func (w *Window) GetKey(key keyboard.Key) keyboard.KeyAction {
	if native := w.nativePointer(); native != nil {
		return keyboard.KeyAction(C.gkWindowGetKey(native, C.int(key)))
	}
	return keyboard.KeyReleased
}
