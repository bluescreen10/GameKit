//go:build darwin && cgo

package metal

/*
#include "view_surface.h"
*/
import "C"

import (
	"errors"
	"fmt"

	"github.com/bluescreen10/gamekit/gpu"
)

// CocoaViewMetalLayer installs and returns a CAMetalLayer on a Cocoa view.
// It is exported for the Vulkan backend's VK_EXT_metal_surface path.
func CocoaViewMetalLayer(view uintptr) uintptr {
	return uintptr(C.mbCocoaViewMetalLayer(C.uintptr_t(view)))
}

// CocoaWindowMetalLayer installs and returns a CAMetalLayer on a Cocoa window's
// content view. It is exported for the Vulkan backend's Metal surface path.
func CocoaWindowMetalLayer(window uintptr) uintptr {
	return uintptr(C.mbCocoaWindowMetalLayer(C.uintptr_t(window)))
}

// CreateSurface creates a CAMetalLayer presentation surface for a native target.
func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("metal: nil surface target")
	}
	native := target.NativeSurface()
	var layer uintptr
	switch native.Kind {
	case gpu.NativeSurfaceCocoaWindow:
		layer = CocoaWindowMetalLayer(native.Handle)
	case gpu.NativeSurfaceCocoaView:
		layer = CocoaViewMetalLayer(native.Handle)
	default:
		return 0, fmt.Errorf("metal: unsupported native surface kind %d", native.Kind)
	}
	if layer == 0 {
		return 0, errors.New("metal: surface target is closed or invalid")
	}
	return gpu.Surface(layer), nil
}
