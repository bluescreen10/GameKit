//go:build !cgo || (!darwin && !linux && !windows)

package gamekit

import (
	"errors"
	"unsafe"
)

func createNativeWindow(string, int, int, WindowHints) (unsafe.Pointer, error) {
	return nil, errors.New("gamekit: native windows require cgo on a supported platform")
}
func destroyNativeWindow(unsafe.Pointer)                    {}
func pollNativeWindow(unsafe.Pointer)                       {}
func nativeWindowShouldClose(unsafe.Pointer) bool           { return true }
func setNativeWindowShouldClose(unsafe.Pointer, bool)       {}
func nativeWindowSize(unsafe.Pointer) (int, int)            { return 0, 0 }
func nativeWindowFramebufferSize(unsafe.Pointer) (int, int) { return 0, 0 }
func setNativeWindowTitle(unsafe.Pointer, string)           {}
func nativeWindowHandle(unsafe.Pointer) uintptr             { return 0 }
func nativeWindowDisplay(unsafe.Pointer) unsafe.Pointer     { return nil }
