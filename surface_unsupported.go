//go:build !cgo || (!darwin && !linux && !windows)

package gamekit

import (
	"errors"

	"github.com/bluescreen10/gamekit/gpu"
)

func (w *Window) CreateSurface(gpu.Backend) (uintptr, error) {
	return 0, errors.New("gamekit: window surfaces require cgo on a supported platform")
}
