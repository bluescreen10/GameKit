//go:build cgo && (darwin || linux || windows)

package gamekit

/*
#cgo darwin LDFLAGS: -framework Cocoa
#cgo linux LDFLAGS: -lX11
#cgo windows LDFLAGS: -luser32 -lgdi32
#include "window_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func createNativeWindow(title string, width, height int, hints WindowOptions) (unsafe.Pointer, error) {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	var flags C.uint32_t
	if hints.Resizable {
		flags |= C.GK_WINDOW_RESIZABLE
	}
	if hints.Hidden {
		flags |= C.GK_WINDOW_HIDDEN
	}
	if hints.Borderless {
		flags |= C.GK_WINDOW_BORDERLESS
	}
	if hints.Maximized {
		flags |= C.GK_WINDOW_MAXIMIZED
	}
	if hints.AlwaysOnTop {
		flags |= C.GK_WINDOW_ALWAYS_ON_TOP
	}
	if hints.TransparentFramebuffer {
		flags |= C.GK_WINDOW_TRANSPARENT
	}
	var message [1024]C.char
	native := C.gkWindowCreate(ctitle, C.int(width), C.int(height), flags, &message[0], C.size_t(len(message)))
	if native == nil {
		detail := C.GoString(&message[0])
		if detail == "" {
			detail = "native window creation failed"
		}
		return nil, fmt.Errorf("gamekit: %s", detail)
	}
	return native, nil
}

func destroyNativeWindow(native unsafe.Pointer)          { C.gkWindowDestroy(native) }
func pollNativeWindow(native unsafe.Pointer)             { C.gkWindowPoll(native) }
func nativeWindowShouldClose(native unsafe.Pointer) bool { return C.gkWindowShouldClose(native) != 0 }
func setNativeWindowShouldClose(native unsafe.Pointer, close bool) {
	value := C.int(0)
	if close {
		value = 1
	}
	C.gkWindowSetShouldClose(native, value)
}
func nativeWindowSize(native unsafe.Pointer) (int, int) {
	var width, height C.int
	C.gkWindowSize(native, &width, &height)
	return int(width), int(height)
}
func nativeWindowFramebufferSize(native unsafe.Pointer) (int, int) {
	var width, height C.int
	C.gkWindowFramebufferSize(native, &width, &height)
	return int(width), int(height)
}
func setNativeWindowTitle(native unsafe.Pointer, title string) {
	ctitle := C.CString(title)
	defer C.free(unsafe.Pointer(ctitle))
	C.gkWindowSetTitle(native, ctitle)
}
func nativeWindowHandle(native unsafe.Pointer) uintptr {
	return uintptr(C.gkWindowNativeHandle(native))
}
func nativeWindowDisplay(native unsafe.Pointer) unsafe.Pointer {
	return C.gkWindowNativeDisplay(native)
}

func setNativeWindowHandle(native unsafe.Pointer, handle uintptr) {
	C.gkWindowSetHandle(native, C.uintptr_t(handle))
}
