//go:build windows && cgo

package vulkan

func defaultInstanceExtensions() []string {
	return []string{"VK_KHR_surface", "VK_KHR_win32_surface"}
}
