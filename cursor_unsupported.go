//go:build !cgo || (!darwin && !linux && !windows)

package gamekit

// GetCursorPos returns zeroes when native window support is unavailable.
func (w *Window) GetCursorPos() (x, y float64) { return 0, 0 }
