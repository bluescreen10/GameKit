package gpu_test

import (
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

const (
	msaaSize    = 64
	msaaSamples = 4
)

// whiteTriangleRoot fills a root with a white triangle whose slanted edges cross
// pixels partway, and returns the root's address.
func whiteTriangleRoot(b gpu.Backend) uint64 {
	vb := b.Alloc(256, gpu.MemoryHost, "verts")
	verts := unsafe.Slice((*vertex)(vb.Ptr), 3)
	verts[0] = vertex{-0.9, -0.9, 1, 1, 1}
	verts[1] = vertex{0.9, -0.7, 1, 1, 1}
	verts[2] = vertex{-0.1, 0.9, 1, 1, 1}

	rb := b.Alloc(64, gpu.MemoryHost, "root")
	root := (*rootData)(rb.Ptr)
	root.tint = [4]float32{1, 1, 1, 1}
	root.verts = vb.Addr
	return rb.Addr
}

// TestMultisampleColorResolve draws a white triangle on black into a 4x target and
// resolves it. Pixels the edges cross only partly hold some samples of each, so their
// resolved value lies strictly between black and white; a single-sample render has
// none.
func TestMultisampleColorResolve(t *testing.T) {
	b := testBackend(t)

	samples := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget, Samples: msaaSamples, Label: "msaa-color",
	})
	resolved := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget | gpu.TextureTransfer, Label: "resolved-color",
	})
	pipeline := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: triangleFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		Samples: msaaSamples, CullMode: gpu.CullNone, Label: "msaa-triangle",
	})
	rootAddr := whiteTriangleRoot(b)
	readback := b.Alloc(msaaSize*msaaSize*4, gpu.MemoryHost, "readback")

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{
			Texture: samples, Load: gpu.LoadClear, Store: gpu.StoreDontCare,
			Clear: [4]float32{0, 0, 0, 1}, ResolveTexture: resolved,
		}},
	})
	cmd.SetPipeline(pipeline)
	cmd.SetViewport(0, 0, msaaSize, msaaSize, 0, 1)
	cmd.SetScissor(0, 0, msaaSize, msaaSize)
	cmd.Draw(utils.ToBytes(&rootAddr), 3, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, resolved, 0, 0)
	b.Wait(b.Submit(cmd))

	pixels := unsafe.Slice((*byte)(readback.Ptr), msaaSize*msaaSize*4)
	var black, white, partial int
	for i := 0; i < len(pixels); i += 4 {
		switch red := pixels[i]; red {
		case 0:
			black++
		case 255:
			white++
		default:
			partial++
		}
	}
	t.Logf("resolved pixels: %d black, %d white, %d partly covered", black, white, partial)
	if black == 0 || white == 0 {
		t.Fatalf("resolved image has %d black and %d white pixels, want both: the triangle or the clear is missing", black, white)
	}
	if partial == 0 {
		t.Fatal("resolved image has no partly covered pixels, want the triangle's edges blended")
	}
}

// TestMultisampleDepthResolve clears a 4x depth buffer to 1, writes 0 under a triangle,
// and resolves it: each resolved pixel must hold its sample zero's depth, so the far
// corner reads 1 and the triangle's middle 0.
func TestMultisampleDepthResolve(t *testing.T) {
	b := testBackend(t)

	color := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget, Samples: msaaSamples, Label: "msaa-color",
	})
	samples := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatDepth32F,
		Usage: gpu.TextureDepth, Samples: msaaSamples, Label: "msaa-depth",
	})
	resolved := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatDepth32F,
		Usage: gpu.TextureDepth | gpu.TextureTransfer, Label: "resolved-depth",
	})
	pipeline := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: triangleFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		DepthFormat: gpu.FormatDepth32F, DepthTest: true, DepthWrite: true, DepthCompare: gpu.CompareAlways,
		Samples: msaaSamples, CullMode: gpu.CullNone, Label: "msaa-depth-triangle",
	})
	rootAddr := whiteTriangleRoot(b)
	readback := b.Alloc(msaaSize*msaaSize*4, gpu.MemoryHost, "readback")

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadClear, Store: gpu.StoreDontCare}},
		Depth: &gpu.DepthAttachment{
			Texture: samples, Load: gpu.LoadClear, Store: gpu.StoreDontCare, Clear: 1,
			ResolveTexture: resolved,
		},
	})
	cmd.SetPipeline(pipeline)
	cmd.SetViewport(0, 0, msaaSize, msaaSize, 0, 1)
	cmd.SetScissor(0, 0, msaaSize, msaaSize)
	cmd.Draw(utils.ToBytes(&rootAddr), 3, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageDepth, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, resolved, 0, 0)
	b.Wait(b.Submit(cmd))

	depths := unsafe.Slice((*float32)(readback.Ptr), msaaSize*msaaSize)
	depthAt := func(x, y int) float32 {
		return depths[y*msaaSize+x]
	}
	if got := depthAt(msaaSize-2, 1); got != 1 {
		t.Errorf("resolved depth at the empty corner = %v, want the clear depth 1", got)
	}
	if got := depthAt(msaaSize/2-4, msaaSize/2); got != 0 {
		t.Errorf("resolved depth inside the triangle = %v, want the triangle's depth 0", got)
	}
}

// TestMultisampleResolveInLaterPass writes 4x colour and depth in one pass and resolves
// both in a later pass that only loads them: a renderer drawing a scene over several
// passes resolves once they are all done. Every resolved pixel must hold what the first
// pass wrote.
func TestMultisampleResolveInLaterPass(t *testing.T) {
	b := testBackend(t)

	color := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget, Samples: msaaSamples, Label: "msaa-color",
	})
	depth := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatDepth32F,
		Usage: gpu.TextureDepth, Samples: msaaSamples, Label: "msaa-depth",
	})
	resolvedColor := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget | gpu.TextureTransfer, Label: "resolved-color",
	})
	resolvedDepth := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatDepth32F,
		Usage: gpu.TextureDepth | gpu.TextureTransfer, Label: "resolved-depth",
	})
	colorReadback := b.Alloc(msaaSize*msaaSize*4, gpu.MemoryHost, "color-readback")
	depthReadback := b.Alloc(msaaSize*msaaSize*4, gpu.MemoryHost, "depth-readback")

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{1, 0, 0, 1}}},
		Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: 0.25},
	})
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput|gpu.StageDepth, gpu.StageColorOutput|gpu.StageDepth, 0)
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadKeep, Store: gpu.StoreDontCare, ResolveTexture: resolvedColor}},
		Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadKeep, Store: gpu.StoreDontCare, ResolveTexture: resolvedDepth},
	})
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput|gpu.StageDepth, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(colorReadback, resolvedColor, 0, 0)
	cmd.CopyTextureToBuffer(depthReadback, resolvedDepth, 0, 0)
	b.Wait(b.Submit(cmd))

	pixels := unsafe.Slice((*[4]byte)(colorReadback.Ptr), msaaSize*msaaSize)
	depths := unsafe.Slice((*float32)(depthReadback.Ptr), msaaSize*msaaSize)
	for i := range pixels {
		if pixels[i] != [4]byte{255, 0, 0, 255} || depths[i] != 0.25 {
			t.Fatalf("resolved pixel %d = colour %v, depth %v; want the first pass's red and 0.25", i, pixels[i], depths[i])
		}
	}
}
