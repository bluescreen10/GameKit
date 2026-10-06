package gtk

import "github.com/bluescreen10/gamekit/ui"

// Backend creates GTK windows. Its zero value is ready for use.
type Backend struct{}

// CreateWindow creates a GTK top-level window.
func (b *Backend) CreateWindow(options ui.WindowOptions) (ui.Window, error) {
	return createWindow(options)
}

var _ ui.Backend = (*Backend)(nil)
