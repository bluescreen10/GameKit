//go:build (!darwin && !windows) || !cgo

package window

import (
	"context"

	"github.com/bluescreen10/gamekit/ui"
)

// OpenFile displays a native open-file dialog parented to the window.
func (w *Window) OpenFile(ctx context.Context, options ui.OpenFileOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", ui.ErrNotSupported
}

// OpenFiles displays a native multiple-file dialog parented to the window.
func (w *Window) OpenFiles(ctx context.Context, options ui.OpenFileOptions) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ui.ErrNotSupported
}

// SaveFile displays a native save-file dialog parented to the window.
func (w *Window) SaveFile(ctx context.Context, options ui.SaveFileOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", ui.ErrNotSupported
}

// SelectDirectory displays a native directory dialog parented to the window.
func (w *Window) SelectDirectory(ctx context.Context, options ui.DirectoryDialogOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", ui.ErrNotSupported
}
