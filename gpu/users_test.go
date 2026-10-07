package gpu_test

import "testing"

// TestDeviceOutlivesAnotherUser: the backend is one instance shared by everything that
// uses it, so one user's Destroy must leave the device working for the rest — two
// renderers in one process, one of them closed. Afterwards the shared backend still
// runs a compute pipeline and reads back what it wrote.
func TestDeviceOutlivesAnotherUser(t *testing.T) {
	b := testBackend(t)
	if err := b.Init(); err != nil {
		t.Fatalf("second Init: %v", err)
	}
	b.Destroy()

	got := readComputeConstants(t, nil)
	want := shaderConstants{enabled: 0, offset: -3, count: 7, scale: 0.5}
	if got != want {
		t.Errorf("after another user's Destroy, constants = %+v, want %+v", got, want)
	}
}
