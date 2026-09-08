//go:build linux && cgo

#define VK_USE_PLATFORM_XLIB_KHR
#include "bridge.h"

VkResult vkbCreateXlibSurface(VkInstance inst, void* display, uint64_t window,
                              VkSurfaceKHR* out) {
    VkXlibSurfaceCreateInfoKHR info = {0};
    info.sType = VK_STRUCTURE_TYPE_XLIB_SURFACE_CREATE_INFO_KHR;
    info.dpy = (Display*)display;
    info.window = (Window)window;
    return vkCreateXlibSurfaceKHR(inst, &info, NULL, out);
}
