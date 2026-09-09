package gpu_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
)

// Nil data is covered end to end by the Metal backend test, which draws with a
// pipeline whose shaders take no parameters. It is not covered here because a
// dispatch with no pipeline bound segfaults inside the driver rather than being
// rejected, so a standalone "does nil crash?" test would need a full compute
// pipeline to say anything about nil at all.

// TestMaxDataSizeIsUsable: every backend must report a limit big enough to be worth
// having. The Vulkan spec floor is 128 bytes, so anything below that means the query
// failed rather than found a small device.
func TestMaxDataSizeIsUsable(t *testing.T) {
	if !gpu.HasBackend() {
		t.Skip("no gpu backend registered")
	}
	b := gpu.Instance(nil)
	if err := b.Init(); err != nil {
		t.Skipf("device unavailable: %v", err)
	}
	got := b.GetMaxDataSize()
	if got < 128 {
		t.Fatalf("GetMaxDataSize = %d, below Vulkan's guaranteed 128-byte minimum — "+
			"the limit was probably not queried", got)
	}
	t.Logf("GetMaxDataSize = %d", got)
}
