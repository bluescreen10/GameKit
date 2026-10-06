package ui

import "errors"

var (
	// ErrCancelled reports that the user dismissed a dialog without choosing.
	ErrCancelled = errors.New("ui: dialog cancelled")
	// ErrClosed reports that an operation requires a live window or widget.
	ErrClosed = errors.New("ui: object is closed")
	// ErrNotSupported reports that a backend cannot provide an optional feature.
	ErrNotSupported = errors.New("ui: feature not supported")
)
