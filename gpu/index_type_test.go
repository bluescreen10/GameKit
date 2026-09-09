package gpu_test

import (
	"bytes"
	"math"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

// TestIndexTypesAgree renders the same cube twice — once through a 32-bit index buffer
// and once through a 16-bit one — and requires the two frames to be identical.
//
// Comparing against the other index width, rather than against a fixed expectation, is
// what makes this test worth having. The index type reaches the GPU by two different
// routes: Vulkan binds it to the index buffer and takes firstIndex in elements, while
// Metal passes it per draw and wants a BYTE offset, which has to be scaled by the index
// width. A wrong enum or an unscaled offset reorders or corrupts the triangles, and the
// frames diverge; nothing else in the suite would notice, since every other test uses
// 32-bit indices exclusively.
func TestIndexTypesAgree(t *testing.T) {
	b := testBackend(t)

	const size = 128

	corners := [8][3]float32{
		{-0.5, -0.5, -0.5}, {0.5, -0.5, -0.5}, {0.5, 0.5, -0.5}, {-0.5, 0.5, -0.5},
		{-0.5, -0.5, 0.5}, {0.5, -0.5, 0.5}, {0.5, 0.5, 0.5}, {-0.5, 0.5, 0.5},
	}
	indices := []uint32{
		0, 1, 2, 0, 2, 3,
		4, 6, 5, 4, 7, 6,
		0, 4, 5, 0, 5, 1,
		3, 2, 6, 3, 6, 7,
		0, 3, 7, 0, 7, 4,
		1, 5, 6, 1, 6, 2,
	}

	vb := b.Alloc(uint64(len(corners))*24, gpu.MemoryHost, "verts")
	vs := unsafe.Slice((*vertex3D)(vb.Ptr), len(corners))
	for i, c := range corners {
		vs[i] = vertex3D{c[0], c[1], c[2], c[0] + 0.5, c[1] + 0.5, c[2] + 0.5}
	}

	// The same indices, in both widths. Every value here fits in 16 bits, so the two
	// buffers describe exactly the same geometry.
	ib32 := b.Alloc(uint64(len(indices))*4, gpu.MemoryHost, "indices32")
	copy(unsafe.Slice((*uint32)(ib32.Ptr), len(indices)), indices)

	ib16 := b.Alloc(uint64(len(indices))*2, gpu.MemoryHost, "indices16")
	dst16 := unsafe.Slice((*uint16)(ib16.Ptr), len(indices))
	for i, v := range indices {
		dst16[i] = uint16(v)
	}

	proj := perspectiveRH(float32(45*math.Pi/180), 1, 0.1, 100)
	view := lookAtRH(vec3{2, 2, 3}, vec3{0, 0, 0}, vec3{0, 1, 0})

	rb := b.Alloc(128, gpu.MemoryHost, "root")
	root := (*meshRoot)(rb.Ptr)
	root.mvp = mul4x4(proj, view)
	root.verts = vb.Addr

	pipe := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: meshVert, FragmentShader: meshFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		DepthFormat: gpu.FormatDepth32F, DepthTest: true, DepthWrite: true, DepthCompare: gpu.CompareLess,
		CullMode: gpu.CullNone, Label: "mesh",
	})

	render := func(ib gpu.Buffer, it gpu.IndexType, firstIndex, count uint32) []byte {
		color := b.CreateTexture(gpu.TextureDescriptor{
			Kind: gpu.Texture2D, Width: size, Height: size, Format: gpu.FormatRGBA8Unorm,
			Usage: gpu.TextureRenderTarget | gpu.TextureTransfer, Label: "color",
		})
		depth := b.CreateTexture(gpu.TextureDescriptor{
			Kind: gpu.Texture2D, Width: size, Height: size, Format: gpu.FormatDepth32F,
			Usage: gpu.TextureDepth, Label: "depth",
		})
		readback := b.Alloc(size*size*4, gpu.MemoryHost, "readback")

		cmd := b.Begin()
		cmd.BeginRenderPass(gpu.RenderTargets{
			Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
			Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: 1.0},
		})
		cmd.SetPipeline(pipe)
		rootAddr := rb.Addr
		cmd.SetViewport(0, 0, size, size, 0, 1)
		cmd.SetScissor(0, 0, size, size)
		cmd.DrawIndexed(utils.ToBytes(&rootAddr), ib, it, count, 1, firstIndex, 0, 0)
		cmd.EndRenderPass()
		cmd.CopyTextureToBuffer(readback, color, 0, 0)
		b.Wait(b.Submit(cmd))

		out := make([]byte, size*size*4)
		copy(out, unsafe.Slice((*byte)(readback.Ptr), size*size*4))
		return out
	}

	got32 := render(ib32, gpu.IndexUint32, 0, uint32(len(indices)))
	got16 := render(ib16, gpu.IndexUint16, 0, uint32(len(indices)))

	// Guard against both frames being empty, which would make the comparison vacuous.
	lit := 0
	for i := 0; i < len(got32); i += 4 {
		if got32[i] != 0 || got32[i+1] != 0 || got32[i+2] != 0 {
			lit++
		}
	}
	if frac := float64(lit) / float64(size*size); frac < 0.1 {
		t.Fatalf("the 32-bit reference frame is nearly empty (%.1f%% lit) — nothing was drawn", frac*100)
	}

	compare := func(what string, a, b []byte) {
		t.Helper()
		if bytes.Equal(a, b) {
			return
		}
		diff := 0
		for i := range a {
			if a[i] != b[i] {
				diff++
			}
		}
		t.Fatalf("%s: 16-bit indices produced a different image than 32-bit — %d of %d bytes differ",
			what, diff, len(a))
	}
	compare("whole buffer", got32, got16)

	// Again from a non-zero firstIndex, which is what actually exercises the byte
	// offset. At firstIndex 0 the offset is zero for both widths, so a hardcoded
	// 4-byte stride would slip through the comparison above untouched.
	const first, count = 6, 24
	compare("firstIndex=6",
		render(ib32, gpu.IndexUint32, first, count),
		render(ib16, gpu.IndexUint16, first, count))
}

// TestIndexTypeSize pins the widths the byte-offset arithmetic depends on.
func TestIndexTypeSize(t *testing.T) {
	if got := gpu.IndexUint16.Size(); got != 2 {
		t.Errorf("IndexUint16.Size() = %d, want 2", got)
	}
	if got := gpu.IndexUint32.Size(); got != 4 {
		t.Errorf("IndexUint32.Size() = %d, want 4", got)
	}
	// The zero value must be the 32-bit type: it is what every existing index buffer
	// uses, so a defaulted field reads them correctly rather than halving the stride.
	var zero gpu.IndexType
	if zero != gpu.IndexUint32 {
		t.Errorf("the zero IndexType is not IndexUint32")
	}
}
