//go:build darwin && cgo

package gamekit

import "runtime"

// Cocoa requires window creation and event dispatch on the process main thread.
func init() { runtime.LockOSThread() }
