//go:build !cgo || (!darwin && !linux && !windows)

package gamekit

import "github.com/bluescreen10/gamekit/keyboard"

// GetKey returns KeyReleased when native window support is unavailable.
func (w *Window) GetKey(key keyboard.Key) keyboard.KeyAction {
	return keyboard.KeyReleased
}
