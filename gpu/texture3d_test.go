package gpu_test

import (
	"slices"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

// TestTexture3DRoundTripsThroughBufferCopies uploads a volume whose every byte differs
// and reads it back: a 3D texture's copy covers its whole depth, in both directions.
func TestTexture3DRoundTripsThroughBufferCopies(t *testing.T) {
	b := testBackend(t)

	const width, height, depth = 4, 3, 2
	const size = width * height * depth * 4
	tex := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture3D, Width: width, Height: height, Depth: depth,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled | gpu.TextureTransfer, Label: "volume"})
	upload := b.Alloc(size, gpu.MemoryHost, "upload")
	want := unsafe.Slice((*byte)(upload.Ptr), size)
	for i := range want {
		want[i] = byte(i)
	}
	readback := b.Alloc(size, gpu.MemoryHost, "readback")

	cmd := b.Begin()
	cmd.CopyBufferToTexture(tex, 0, 0, upload, 0)
	cmd.Barrier(gpu.StageTransfer, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, tex, 0, 0)
	b.Wait(b.Submit(cmd))

	if got := unsafe.Slice((*byte)(readback.Ptr), size); !slices.Equal(got, want) {
		t.Errorf("read back %v, want %v", got, want)
	}
}

// sampled3DRoot matches Root in sampled3d.frag: textured.vert's root, then the depth
// to sample at.
type sampled3DRoot struct {
	verts uint64
	tex   uint32
	samp  uint32
	w     float32
}

// TestTexture3DSampledInShader samples each depth of a 2x2x2 volume across a
// fullscreen quad, through the heap's texture3D declaration: the four quadrants must
// read the four voxels of the depth asked for.
func TestTexture3DSampledInShader(t *testing.T) {
	b := testBackend(t)

	// Depth 0: red, green / blue, white. Depth 1: cyan, magenta / yellow, black.
	voxels := []byte{
		255, 0, 0, 255, 0, 255, 0, 255,
		0, 0, 255, 255, 255, 255, 255, 255,
		0, 255, 255, 255, 255, 0, 255, 255,
		255, 255, 0, 255, 0, 0, 0, 255,
	}
	tex := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture3D, Width: 2, Height: 2, Depth: 2,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled | gpu.TextureTransfer, Label: "volume"})
	staging := b.Alloc(uint64(len(voxels)), gpu.MemoryHost, "staging")
	copy(unsafe.Slice((*byte)(staging.Ptr), len(voxels)), voxels)
	samp := b.CreateSampler(gpu.SamplerDescriptor{AddressU: gpu.AddressClamp, AddressV: gpu.AddressClamp, AddressW: gpu.AddressClamp})

	const size = 128
	color := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture2D, Width: size, Height: size,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureRenderTarget | gpu.TextureTransfer})
	verts := []texVertex{
		{-1, -1, 0, 0}, {1, -1, 1, 0}, {1, 1, 1, 1},
		{-1, -1, 0, 0}, {1, 1, 1, 1}, {-1, 1, 0, 1},
	}
	vbuf := b.Alloc(uint64(len(verts))*16, gpu.MemoryHost, "verts")
	copy(unsafe.Slice((*texVertex)(vbuf.Ptr), len(verts)), verts)
	pipe := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: texturedVert, FragmentShader: sampled3DFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		CullMode: gpu.CullNone,
	})

	q := size / 4
	for _, depth := range []struct {
		w    float32
		want [4][3]byte // top-left, top-right, bottom-left, bottom-right
	}{
		{0.25, [4][3]byte{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 255}}},
		{0.75, [4][3]byte{{0, 255, 255}, {255, 0, 255}, {255, 255, 0}, {0, 0, 0}}},
	} {
		root := b.Alloc(64, gpu.MemoryHost, "root")
		*(*sampled3DRoot)(root.Ptr) = sampled3DRoot{verts: vbuf.Addr, tex: tex.Index, samp: samp.Index, w: depth.w}
		readback := b.Alloc(size*size*4, gpu.MemoryHost, "readback")

		cmd := b.Begin()
		cmd.CopyBufferToTexture(tex, 0, 0, staging, 0)
		cmd.Barrier(gpu.StageTransfer, gpu.StageFragment, 0)
		cmd.BeginRenderPass(gpu.RenderTargets{
			Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0.5, 0.5, 0.5, 1}}},
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
		for i, at := range [4][2]int{{q, q}, {3 * q, q}, {q, 3 * q}, {3 * q, 3 * q}} {
			o := (at[1]*size + at[0]) * 4
			if got := [3]byte{px[o], px[o+1], px[o+2]}; got != depth.want[i] {
				t.Errorf("w = %v, quadrant %d = %v, want %v", depth.w, i, got, depth.want[i])
			}
		}
	}
}

// TestStorageImage3DWrittenByCompute writes every voxel of a 3D storage image with its
// own coordinates, through the heap's image3D declaration, and reads the volume back.
func TestStorageImage3DWrittenByCompute(t *testing.T) {
	b := testBackend(t)

	const n = 4
	tex := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture3D, Width: n, Height: n, Depth: n,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureStorage | gpu.TextureTransfer, Label: "volume"})
	readback := b.Alloc(n*n*n*4, gpu.MemoryHost, "readback")
	comp := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: storageWrite3D, Label: "storage-write-3d"})

	cmd := b.Begin()
	cmd.SetPipeline(comp)
	data := storageWriteData{image: tex.Index, size: n}
	cmd.Dispatch(utils.ToBytes(&data), 1, 1, 1)
	cmd.Barrier(gpu.StageCompute, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, tex, 0, 0)
	b.Wait(b.Submit(cmd))

	got := unsafe.Slice((*byte)(readback.Ptr), n*n*n*4)
	for z := range n {
		for y := range n {
			for x := range n {
				i := ((z*n+y)*n + x) * 4
				if voxel, want := [4]byte(got[i:i+4]), [4]byte{byte(x), byte(y), byte(z), 255}; voxel != want {
					t.Errorf("voxel (%d,%d,%d) = %v, want %v", x, y, z, voxel, want)
				}
			}
		}
	}
}
