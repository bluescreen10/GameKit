//go:build !cgo || (!darwin && !linux && !windows)

package window

// GetPointerPos returns zeroes when native window support is unavailable.
func (w *Window) GetPointerPos() (x, y float64) { return 0, 0 }
