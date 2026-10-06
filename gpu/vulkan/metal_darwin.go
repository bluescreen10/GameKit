//go:build darwin && cgo

package vulkan

/*
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/metal"
)

// MetalSurfaceExtensions are the instance extensions needed for a Metal surface;
// pass them to New when creating an on-screen backend on macOS.
var MetalSurfaceExtensions = []string{"VK_KHR_surface", "VK_EXT_metal_surface"}

// CreateSurface creates a Vulkan Metal surface for a Cocoa window or view.
// The Vulkan instance must have been created with MetalSurfaceExtensions.
func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("vulkan: nil surface target")
	}
	native := target.NativeSurface()
	var layer uintptr
	switch native.Kind {
	case gpu.NativeSurfaceCocoaWindow:
		layer = metal.CocoaWindowMetalLayer(native.Handle)
	case gpu.NativeSurfaceCocoaView:
		layer = metal.CocoaViewMetalLayer(native.Handle)
	default:
		return 0, fmt.Errorf("vulkan: unsupported native surface kind %d", native.Kind)
	}
	if layer == 0 {
		return 0, errors.New("vulkan: surface target is closed or invalid")
	}
	var surface C.VkSurfaceKHR
	result := C.vkbCreateMetalSurface(b.instance, C.uintptr_t(layer), &surface)
	if result != C.VK_SUCCESS {
		return 0, fmt.Errorf("vulkan: vkCreateMetalSurfaceEXT failed (%d)", int(result))
	}
	return gpu.Surface(C.vkbSurfaceHandle(surface)), nil
}
