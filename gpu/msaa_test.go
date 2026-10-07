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

// readSamplesData matches PC in read_samples.comp.
type readSamplesData struct {
	samples           uint64
	color, depth      uint32
	size, sampleCount uint32
}

// TestMultisampledTexturesReadPerSample draws a white triangle at depth 0 into a 4x
// colour target and depth buffer cleared to black and 1, then reads every sample back
// through the sampled heap from a compute shader. Each sample must hold what was drawn
// at it: white with depth 0 under the triangle, black with depth 1 outside, and the
// pixels its edges cross must hold some of each — which only individual samples can
// show, since a resolve would average them.
func TestMultisampledTexturesReadPerSample(t *testing.T) {
	b := testBackend(t)

	color := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget | gpu.TextureSampled, Samples: msaaSamples, Label: "msaa-color",
	})
	depth := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatDepth32F,
		Usage: gpu.TextureDepth | gpu.TextureSampled, Samples: msaaSamples, Label: "msaa-depth",
	})
	pipeline := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: triangleFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		DepthFormat: gpu.FormatDepth32F, DepthTest: true, DepthWrite: true, DepthCompare: gpu.CompareAlways,
		Samples: msaaSamples, CullMode: gpu.CullNone, Label: "msaa-depth-triangle",
	})
	reader := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Shader: readSamples, Label: "read-samples"})
	rootAddr := whiteTriangleRoot(b)
	const sampleBytes = 8
	samples := b.Alloc(msaaSize*msaaSize*msaaSamples*sampleBytes, gpu.MemoryHost, "samples")

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: color, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
		Depth: &gpu.DepthAttachment{Texture: depth, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: 1},
	})
	cmd.SetPipeline(pipeline)
	cmd.SetViewport(0, 0, msaaSize, msaaSize, 0, 1)
	cmd.SetScissor(0, 0, msaaSize, msaaSize)
	cmd.Draw(utils.ToBytes(&rootAddr), 3, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput|gpu.StageDepth, gpu.StageCompute, 0)
	cmd.SetPipeline(reader)
	data := readSamplesData{samples: samples.Addr, color: color.Index, depth: depth.Index, size: msaaSize, sampleCount: msaaSamples}
	cmd.Dispatch(utils.ToBytes(&data), msaaSize/8, msaaSize/8, 1)
	cmd.Barrier(gpu.StageCompute, gpu.StageAll, 0)
	b.Wait(b.Submit(cmd))

	values := unsafe.Slice((*[2]float32)(samples.Ptr), msaaSize*msaaSize*msaaSamples)
	var partial int
	for pixel := 0; pixel < msaaSize*msaaSize; pixel++ {
		var covered int
		for s := 0; s < msaaSamples; s++ {
			red, z := values[pixel*msaaSamples+s][0], values[pixel*msaaSamples+s][1]
			switch {
			case red == 1 && z == 0:
				covered++
			case red == 0 && z == 1:
			default:
				t.Fatalf("pixel %d sample %d = red %v, depth %v; want white at depth 0 or black at depth 1", pixel, s, red, z)
			}
		}
		if covered > 0 && covered < msaaSamples {
			partial++
		}
	}
	corner := values[(1*msaaSize+msaaSize-2)*msaaSamples]
	middle := values[(msaaSize/2*msaaSize+msaaSize/2-4)*msaaSamples]
	if corner != [2]float32{0, 1} || middle != [2]float32{1, 0} {
		t.Fatalf("empty corner = %v, triangle middle = %v; want [0 1] and [1 0]", corner, middle)
	}
	if partial == 0 {
		t.Fatal("no pixel holds both covered and uncovered samples, want the triangle's edges to cross some")
	}
	t.Logf("%d pixels hold both covered and uncovered samples", partial)
}

// TestTransientMultisampleResolve draws a white triangle at depth 0 into 4x colour and
// depth that are transient — they live only within the pass, in tile memory on GPUs
// that have it — and resolves both as the pass ends. The resolves must be what they are
// for stored images: the triangle's edges partly covered, black around it, and each
// pixel's sample zero depth.
func TestTransientMultisampleResolve(t *testing.T) {
	b := testBackend(t)

	color := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget | gpu.TextureTransient, Samples: msaaSamples, Label: "transient-color",
	})
	depth := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatDepth32F,
		Usage: gpu.TextureDepth | gpu.TextureTransient, Samples: msaaSamples, Label: "transient-depth",
	})
	resolvedColor := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatRGBA8Unorm,
		Usage: gpu.TextureRenderTarget | gpu.TextureTransfer, Label: "resolved-color",
	})
	resolvedDepth := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: msaaSize, Height: msaaSize, Format: gpu.FormatDepth32F,
		Usage: gpu.TextureDepth | gpu.TextureTransfer, Label: "resolved-depth",
	})
	pipeline := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: triangleFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		DepthFormat: gpu.FormatDepth32F, DepthTest: true, DepthWrite: true, DepthCompare: gpu.CompareAlways,
		Samples: msaaSamples, CullMode: gpu.CullNone, Label: "transient-triangle",
	})
	rootAddr := whiteTriangleRoot(b)
	colorReadback := b.Alloc(msaaSize*msaaSize*4, gpu.MemoryHost, "color-readback")
	depthReadback := b.Alloc(msaaSize*msaaSize*4, gpu.MemoryHost, "depth-readback")

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{
			Texture: color, Load: gpu.LoadClear, Store: gpu.StoreDontCare,
			Clear: [4]float32{0, 0, 0, 1}, ResolveTexture: resolvedColor,
		}},
		Depth: &gpu.DepthAttachment{
			Texture: depth, Load: gpu.LoadClear, Store: gpu.StoreDontCare, Clear: 1,
			ResolveTexture: resolvedDepth,
		},
	})
	cmd.SetPipeline(pipeline)
	cmd.SetViewport(0, 0, msaaSize, msaaSize, 0, 1)
	cmd.SetScissor(0, 0, msaaSize, msaaSize)
	cmd.Draw(utils.ToBytes(&rootAddr), 3, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput|gpu.StageDepth, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(colorReadback, resolvedColor, 0, 0)
	cmd.CopyTextureToBuffer(depthReadback, resolvedDepth, 0, 0)
	b.Wait(b.Submit(cmd))

	pixels := unsafe.Slice((*byte)(colorReadback.Ptr), msaaSize*msaaSize*4)
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
	if black == 0 || white == 0 || partial == 0 {
		t.Errorf("resolved colour has %d black, %d white, %d partly covered pixels, want some of each", black, white, partial)
	}
	depths := unsafe.Slice((*float32)(depthReadback.Ptr), msaaSize*msaaSize)
	if got := depths[1*msaaSize+msaaSize-2]; got != 1 {
		t.Errorf("resolved depth at the empty corner = %v, want the clear depth 1", got)
	}
	if got := depths[(msaaSize/2)*msaaSize+msaaSize/2-4]; got != 0 {
		t.Errorf("resolved depth inside the triangle = %v, want the triangle's depth 0", got)
	}
}
