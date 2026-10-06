// Package window provides GameKit's built-in native window backend for ui.
//
// It uses Cocoa on macOS, Win32 on Windows, and X11 on Linux. Cocoa and Win32
// implement the native widget set. The built-in X11 path provides windows and
// input; applications that need native Linux widgets should use package gtk.
// The zero value of Backend is ready for use.
package window
