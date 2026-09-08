//go:build windows && cgo

package gamekit

import (
	"errors"

	"github.com/bluescreen10/gamekit/gpu"
)

type win32SurfaceBackend interface {
	CreateWin32Surface(uintptr) uintptr
}

// CreateSurface creates a Vulkan Win32 surface for this window.
func (w *Window) CreateSurface(backend gpu.Backend) (uintptr, error) {
	if w.nativePointer() == nil {
		return 0, errors.New("gamekit: window is destroyed")
	}
	surfacer, ok := backend.(win32SurfaceBackend)
	if !ok {
		return 0, errors.New("gamekit: backend cannot create a Win32 window surface")
	}
	return surfacer.CreateWin32Surface(w.NativeHandle()), nil
}
