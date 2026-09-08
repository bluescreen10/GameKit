//go:build windows && cgo

#define VK_USE_PLATFORM_WIN32_KHR
#include "bridge.h"

VkResult vkbCreateWin32Surface(VkInstance inst, uintptr_t window, VkSurfaceKHR* out) {
    VkWin32SurfaceCreateInfoKHR info = {0};
    info.sType = VK_STRUCTURE_TYPE_WIN32_SURFACE_CREATE_INFO_KHR;
    info.hinstance = GetModuleHandleW(NULL);
    info.hwnd = (HWND)window;
    return vkCreateWin32SurfaceKHR(inst, &info, NULL, out);
}
