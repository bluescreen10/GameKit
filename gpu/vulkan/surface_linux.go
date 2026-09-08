//go:build linux && cgo

package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// CreateXlibSurface creates a Vulkan surface for an X11 Display and Window.
func (b *Backend) CreateXlibSurface(display unsafe.Pointer, window uintptr) uintptr {
	var surface C.VkSurfaceKHR
	if result := C.vkbCreateXlibSurface(b.instance, display, C.uint64_t(window), &surface); result != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: vkCreateXlibSurfaceKHR failed (%d)", int(result)))
	}
	return uintptr(unsafe.Pointer(surface))
}
