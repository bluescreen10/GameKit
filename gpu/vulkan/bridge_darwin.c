//go:build darwin && cgo

#define VK_USE_PLATFORM_METAL_EXT
#include "bridge.h"

VkResult vkbCreateMetalSurface(VkInstance inst, void* layer, VkSurfaceKHR* out) {
    VkMetalSurfaceCreateInfoEXT ci = {0};
    ci.sType = VK_STRUCTURE_TYPE_METAL_SURFACE_CREATE_INFO_EXT;
    ci.pLayer = (const CAMetalLayer*)layer;
    return vkCreateMetalSurfaceEXT(inst, &ci, NULL, out);
}
