//go:build !linux || !cgo || !gtk

package gtk

import "github.com/bluescreen10/gamekit/ui"

func createWindow(ui.WindowOptions) (ui.Window, error) {
	return nil, ui.ErrNotSupported
}
