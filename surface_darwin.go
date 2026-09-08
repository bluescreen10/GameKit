//go:build darwin && cgo

package gamekit

import (
	"errors"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

type metalSurfaceBackend interface {
	CreateMetalSurface(unsafe.Pointer) uintptr
}

// CreateSurface creates the backend-specific Metal-layer surface for this window.
func (w *Window) CreateSurface(backend gpu.Backend) (uintptr, error) {
	native := w.nativePointer()
	if native == nil {
		return 0, errors.New("gamekit: window is destroyed")
	}
	surfacer, ok := backend.(metalSurfaceBackend)
	if !ok {
		return 0, errors.New("gamekit: backend cannot create a macOS window surface")
	}
	return surfacer.CreateMetalSurface(unsafe.Pointer(w.NativeHandle())), nil
}
