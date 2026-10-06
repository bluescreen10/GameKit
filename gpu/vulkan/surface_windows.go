//go:build windows && cgo

package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"

	"github.com/bluescreen10/gamekit/gpu"
)

// CreateSurface creates a Vulkan Win32 surface for a native target.
func (b *Backend) CreateSurface(target gpu.SurfaceTarget) (gpu.Surface, error) {
	if target == nil {
		return 0, errors.New("vulkan: nil surface target")
	}
	native := target.NativeSurface()
	if native.Kind != gpu.NativeSurfaceWin32Window {
		return 0, fmt.Errorf("vulkan: unsupported native surface kind %d", native.Kind)
	}
	if native.Handle == 0 {
		return 0, errors.New("vulkan: surface target is closed or invalid")
	}
	var surface C.VkSurfaceKHR
	result := C.vkbCreateWin32Surface(b.instance, C.uintptr_t(native.Handle), &surface)
	if result != C.VK_SUCCESS {
		return 0, fmt.Errorf("vulkan: vkCreateWin32SurfaceKHR failed (%d)", int(result))
	}
	return gpu.Surface(C.vkbSurfaceHandle(surface)), nil
}
