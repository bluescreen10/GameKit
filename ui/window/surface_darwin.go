//go:build darwin && cgo

package window

/*
#include "window_bridge.h"
*/
import "C"

import "github.com/bluescreen10/gamekit/gpu"

// NativeSurface describes this window to a GPU backend.
func (w *Window) NativeSurface() gpu.NativeSurface {
	native := w.nativePointer()
	if native == nil {
		return gpu.NativeSurface{}
	}
	return gpu.NativeSurface{
		Kind:   gpu.NativeSurfaceCocoaWindow,
		Handle: uintptr(C.gkWindowNativeHandle(native)),
	}
}
