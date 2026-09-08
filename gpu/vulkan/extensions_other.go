//go:build !darwin && !linux && !windows && cgo

package vulkan

// defaultInstanceExtensions is empty on platforms without a built-in surface
// extension set (attach surface extensions explicitly via New).
func defaultInstanceExtensions() []string { return nil }
