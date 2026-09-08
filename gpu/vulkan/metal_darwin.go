//go:build darwin && cgo

package vulkan

/*
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/bluescreen10/GameKit/gpu/metal"
)

// MetalSurfaceExtensions are the instance extensions needed for a Metal surface;
// pass them to New when creating an on-screen backend on macOS.
var MetalSurfaceExtensions = []string{"VK_KHR_surface", "VK_EXT_metal_surface"}

// CreateMetalSurface makes a VkSurfaceKHR from a Cocoa NSWindow, returning it as
// a uintptr for CreateSwapchain. The
// window's content view is made CAMetalLayer-backed. Instance must have been
// created with MetalSurfaceExtensions.
func (b *Backend) CreateMetalSurface(nsWindow unsafe.Pointer) uintptr {
	var surf C.VkSurfaceKHR
	layer := metal.CocoaMetalLayer(nsWindow)
	if r := C.vkbCreateMetalSurface(b.instance, unsafe.Pointer(layer), &surf); r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: vkCreateMetalSurfaceEXT failed (%d)", int(r)))
	}
	return uintptr(unsafe.Pointer(surf))
}
