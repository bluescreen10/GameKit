//go:build linux && cgo

package gamekit

import (
	"errors"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

type xlibSurfaceBackend interface {
	CreateXlibSurface(unsafe.Pointer, uintptr) uintptr
}

// CreateSurface creates a Vulkan Xlib surface for this window.
func (w *Window) CreateSurface(backend gpu.Backend) (uintptr, error) {
	if w.nativePointer() == nil {
		return 0, errors.New("gamekit: window is destroyed")
	}
	surfacer, ok := backend.(xlibSurfaceBackend)
	if !ok {
		return 0, errors.New("gamekit: backend cannot create an X11 window surface")
	}
	return surfacer.CreateXlibSurface(w.GetNativeDisplay(), w.GetNativeHandle()), nil
}
