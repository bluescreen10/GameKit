//go:build windows && cgo

package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// CreateWin32Surface creates a Vulkan surface for a Win32 HWND.
func (b *Backend) CreateWin32Surface(window uintptr) uintptr {
	var surface C.VkSurfaceKHR
	if result := C.vkbCreateWin32Surface(b.instance, C.uintptr_t(window), &surface); result != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: vkCreateWin32SurfaceKHR failed (%d)", int(result)))
	}
	return uintptr(unsafe.Pointer(surface))
}
