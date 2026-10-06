// Package gtk provides a GTK 3 implementation of ui.Backend for Linux.
//
// Build applications with the gtk tag and install GTK 3 development files:
//
//	go build -tags gtk
//
// The backend implements GameKit's native widget set and targets GTK's X11
// implementation so Window and Canvas can be passed directly to the Vulkan
// backend as Xlib surface targets.
package gtk
