//go:build cgo && (darwin || linux || windows)

package gamekit

import "testing"

// TestDispatchIgnoresTheZeroHandle covers the guard that makes destruction safe: the
// C layer's handle is cleared to zero before the Go handle is released, and every
// dispatcher has to treat zero as "no window" rather than resolving it.
//
// Resolving a released handle would panic — cgo.Handle.Value does not return nil for
// one — so this is the check standing between a destroyed window and a crash.
func TestDispatchIgnoresTheZeroHandle(t *testing.T) {
	if w := windowFromHandle(0); w != nil {
		t.Fatalf("handle 0 resolved to %v, want nil", w)
	}
}
