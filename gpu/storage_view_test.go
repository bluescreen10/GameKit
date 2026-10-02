package gpu_test

import (
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

// TestStorageViewWritesOneMip writes the second mip of a writable texture through a view
// of that mip alone, the way a compute shader fills one level of a mip chain, and reads
// that mip back: four quadrants, red, green, blue and white.
func TestStorageViewWritesOneMip(t *testing.T) {
	b := testBackend(t)

	const side = 8 // mip 1 is 4x4
	tex := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture2D, Width: side, Height: side, Mips: 2,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled | gpu.TextureStorage | gpu.TextureTransfer, Label: "mips"})
	view := b.TextureView(tex, gpu.Texture2D, 1, 1, 0, 1)
	readback := b.Alloc(4*4*4, gpu.MemoryHost, "readback")
	comp := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: storageWrite, Label: "storage-write"})

	cmd := b.Begin()
	cmd.SetPipeline(comp)
	data := storageWriteData{image: view.Index, size: side / 2}
	cmd.Dispatch(utils.ToBytes(&data), 1, 1, 1)
	cmd.Barrier(gpu.StageCompute, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, tex, 1, 0)
	b.Wait(b.Submit(cmd))

	px := unsafe.Slice((*byte)(readback.Ptr), 4*4*4)
	for _, probe := range []struct {
		x, y int
		want [4]byte
		name string
	}{
		{0, 0, [4]byte{255, 0, 0, 255}, "top-left red"},
		{3, 0, [4]byte{0, 255, 0, 255}, "top-right green"},
		{0, 3, [4]byte{0, 0, 255, 255}, "bottom-left blue"},
		{3, 3, [4]byte{255, 255, 255, 255}, "bottom-right white"},
	} {
		i := (probe.y*4 + probe.x) * 4
		if got := [4]byte(px[i : i+4]); got != probe.want {
			t.Errorf("mip 1 %s = %v, want %v", probe.name, got, probe.want)
		}
	}
}
