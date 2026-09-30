package gpu_test

import (
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

// storageWriteData matches PC in storage_write.comp.
type storageWriteData struct {
	image, size uint32
	_           [2]uint32
}

// TestStorageImageWrittenThenSampled writes four coloured quadrants into a texture from
// a compute shader, through the storage array, then samples it across a fullscreen
// quad through the sampled array — with the texture's one index both times. It proves
// that a storage write reaches the texture, that the two arrays agree on the index, and
// that a barrier from compute to fragment is all the ordering it takes.
func TestStorageImageWrittenThenSampled(t *testing.T) {
	b := testBackend(t)

	const imageSize = 64
	image := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture2D, Width: imageSize, Height: imageSize,
		Format: gpu.FormatRGBA16F, Usage: gpu.TextureSampled | gpu.TextureStorage, Label: "storage"})
	samp := b.CreateSampler(gpu.SamplerDescriptor{AddressU: gpu.AddressClamp, AddressV: gpu.AddressClamp})

	const size = 128
	color := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture2D, Width: size, Height: size,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureRenderTarget | gpu.TextureTransfer})

	verts := []texVertex{
		{-1, -1, 0, 0}, {1, -1, 1, 0}, {1, 1, 1, 1},
		{-1, -1, 0, 0}, {1, 1, 1, 1}, {-1, 1, 0, 1},
	}
	vbuf := b.Alloc(uint64(len(verts))*16, gpu.MemoryHost, "verts")
	copy(unsafe.Slice((*texVertex)(vbuf.Ptr), len(verts)), verts)
	root := b.Alloc(64, gpu.MemoryHost, "root")
	*(*texRoot)(root.Ptr) = texRoot{verts: vbuf.Addr, tex: image.Index, samp: samp.Index}

	comp := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: storageWrite, Label: "storage-write"})
	pipe := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: texturedVert, FragmentShader: texturedFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		CullMode: gpu.CullNone,
	})

	readback := b.Alloc(size*size*4, gpu.MemoryHost, "readback")
	cmd := b.Begin()
	cmd.SetPipeline(comp)
	data := storageWriteData{image: image.Index, size: imageSize}
	cmd.Dispatch(utils.ToBytes(&data), imageSize/8, imageSize/8, 1)
	cmd.Barrier(gpu.StageCompute, gpu.StageFragment, 0)
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
	})
	cmd.SetPipeline(pipe)
	rootAddr := root.Addr
	cmd.SetViewport(0, 0, size, size, 0, 1)
	cmd.SetScissor(0, 0, size, size)
	cmd.Draw(utils.ToBytes(&rootAddr), 6, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, color, 0, 0)
	b.Wait(b.Submit(cmd))

	px := unsafe.Slice((*byte)(readback.Ptr), size*size*4)
	q := size / 4
	for _, probe := range []struct {
		x, y int
		want [3]byte
		name string
	}{
		{q, q, [3]byte{255, 0, 0}, "top-left red"},
		{3 * q, q, [3]byte{0, 255, 0}, "top-right green"},
		{q, 3 * q, [3]byte{0, 0, 255}, "bottom-left blue"},
		{3 * q, 3 * q, [3]byte{255, 255, 255}, "bottom-right white"},
	} {
		i := (probe.y*size + probe.x) * 4
		if got := [3]byte{px[i], px[i+1], px[i+2]}; got != probe.want {
			t.Errorf("%s quadrant = %v, want %v", probe.name, got, probe.want)
		}
	}
}
