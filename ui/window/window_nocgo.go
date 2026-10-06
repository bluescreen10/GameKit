//go:build !cgo || (!darwin && !linux && !windows)

package window

import (
	"errors"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

func createNativeWindow(string, int, int, WindowOptions) (unsafe.Pointer, error) {
	return nil, errors.New("ui/window: native windows require cgo on a supported platform")
}
func destroyNativeWindow(unsafe.Pointer)                    {}
func pollNativeWindow(unsafe.Pointer)                       {}
func nativeWindowShouldClose(unsafe.Pointer) bool           { return true }
func setNativeWindowShouldClose(unsafe.Pointer, bool)       {}
func nativeWindowSize(unsafe.Pointer) (int, int)            { return 0, 0 }
func nativeWindowFramebufferSize(unsafe.Pointer) (int, int) { return 0, 0 }
func setNativeWindowTitle(unsafe.Pointer, string)           {}
func setNativeWindowTheme(unsafe.Pointer, ui.Theme)         {}
func nativeWindowHandle(unsafe.Pointer) uintptr             { return 0 }
func nativeWindowDisplay(unsafe.Pointer) unsafe.Pointer     { return nil }

func setNativeWindowHandle(unsafe.Pointer, uintptr) {}
