//go:build linux && cgo

package vulkan

func defaultInstanceExtensions() []string {
	return []string{"VK_KHR_surface", "VK_KHR_xlib_surface"}
}
