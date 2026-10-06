//go:build windows && cgo

package window

/*
#include "widget_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"context"
	"strings"
	"unsafe"

	"github.com/bluescreen10/gamekit/ui"
)

// OpenFile displays a native open-file dialog parented to the window.
func (w *Window) OpenFile(ctx context.Context, options ui.OpenFileOptions) (string, error) {
	paths, err := w.openPanel(ctx, options, false, false)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// OpenFiles displays a native multiple-file dialog parented to the window.
func (w *Window) OpenFiles(ctx context.Context, options ui.OpenFileOptions) ([]string, error) {
	return w.openPanel(ctx, options, true, false)
}

// SaveFile displays a native save-file dialog parented to the window.
func (w *Window) SaveFile(ctx context.Context, options ui.SaveFileOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	title := C.CString(options.Title)
	directory := C.CString(options.Directory)
	filename := C.CString(options.Filename)
	extensions := C.CString(fileExtensions(options.Filters))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(directory))
	defer C.free(unsafe.Pointer(filename))
	defer C.free(unsafe.Pointer(extensions))

	var size C.size_t
	var cancelled C.int
	result := C.gkUISavePanel(
		w.nativeWindowHandle(),
		title,
		directory,
		filename,
		extensions,
		&size,
		&cancelled,
	)
	paths, err := dialogPaths(result, size, cancelled)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// SelectDirectory displays a native directory dialog parented to the window.
func (w *Window) SelectDirectory(ctx context.Context, options ui.DirectoryDialogOptions) (string, error) {
	paths, err := w.openPanel(ctx, ui.OpenFileOptions{
		Title:     options.Title,
		Directory: options.Directory,
	}, false, true)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

func (w *Window) openPanel(ctx context.Context, options ui.OpenFileOptions, multiple, directories bool) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	title := C.CString(options.Title)
	directory := C.CString(options.Directory)
	filename := C.CString(options.Filename)
	extensions := C.CString(fileExtensions(options.Filters))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(directory))
	defer C.free(unsafe.Pointer(filename))
	defer C.free(unsafe.Pointer(extensions))

	var size C.size_t
	var cancelled C.int
	result := C.gkUIOpenPanel(
		w.nativeWindowHandle(),
		title,
		directory,
		filename,
		extensions,
		boolInt(multiple),
		boolInt(directories),
		&size,
		&cancelled,
	)
	return dialogPaths(result, size, cancelled)
}

func dialogPaths(result *C.char, size C.size_t, cancelled C.int) ([]string, error) {
	if result != nil {
		defer C.free(unsafe.Pointer(result))
	}
	if cancelled != 0 {
		return nil, ui.ErrCancelled
	}
	if result == nil || size == 0 {
		return nil, ui.ErrNotSupported
	}

	data := C.GoBytes(unsafe.Pointer(result), C.int(size))
	parts := bytes.Split(data, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			paths = append(paths, string(part))
		}
	}
	if len(paths) == 0 {
		return nil, ui.ErrCancelled
	}
	return paths, nil
}

func fileExtensions(filters []ui.FileFilter) string {
	extensions := make([]string, 0)
	for _, filter := range filters {
		for _, pattern := range filter.Patterns {
			extension := strings.TrimPrefix(strings.TrimSpace(pattern), "*.")
			if extension != "" && extension != "*" {
				extensions = append(extensions, extension)
			}
		}
	}
	return strings.Join(extensions, ",")
}
