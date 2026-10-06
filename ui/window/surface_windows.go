//go:build windows && cgo

package window

import "github.com/bluescreen10/gamekit/gpu"

// NativeSurface describes this window to a GPU backend.
func (w *Window) NativeSurface() gpu.NativeSurface {
	if w.nativePointer() == nil {
		return gpu.NativeSurface{}
	}
	return gpu.NativeSurface{
		Kind:   gpu.NativeSurfaceWin32Window,
		Handle: w.GetNativeHandle(),
	}
}
