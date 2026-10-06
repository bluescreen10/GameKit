package test_test

import (
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	gputest "github.com/bluescreen10/gamekit/gpu/test"
)

// gpuBufferBytes views a host-mapped buffer's contents for assertions.
func gpuBufferBytes(b gpu.Buffer) []byte {
	return unsafe.Slice((*byte)(b.Ptr), b.Size)
}

func newBackend(t *testing.T) *gputest.Backend {
	t.Helper()
	b := gputest.New()
	if err := b.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(b.Destroy)
	return b
}

// TestRegistersAtLowPriority: importing the package for its side effect must never
// silently outrank a real backend — a caller has to ask for "test" by name.
func TestRegistersAtLowPriority(t *testing.T) {
	b, ok := gpu.Lookup("test")
	if !ok {
		t.Fatal(`gpu.Lookup("test") found nothing — the package did not register itself`)
	}
	if _, ok := b.(*gputest.Backend); !ok {
		t.Fatalf("registered backend is a %T, not *test.Backend", b)
	}
}

// TestHostAllocHasMappedPointer: MemoryHost is the whole point — a buffer callers can
// Write into and read back through Ptr, exactly like a real persistently-mapped one.
func TestHostAllocHasMappedPointer(t *testing.T) {
	b := newBackend(t)

	buf := b.Alloc(16, gpu.MemoryHost, "test")
	defer b.Free(buf)
	if buf.Ptr == nil {
		t.Fatal("MemoryHost buffer has a nil Ptr")
	}
	if buf.Size != 16 {
		t.Fatalf("Size = %d, want 16", buf.Size)
	}

	buf.Write([]byte{1, 2, 3, 4}, 4)
	got := gpuBufferBytes(buf)
	want := []byte{0, 0, 0, 0, 1, 2, 3, 4, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buffer = %v, want %v", got, want)
		}
	}
}

// TestDeviceAllocHasNoMappedPointer pins the documented Buffer contract: a
// MemoryDevice allocation has no Ptr, so code that only ever reaches memory through
// Write (never Ptr directly) still needs a real backing store, which is what
// CopyBuffer below exercises.
func TestDeviceAllocHasNoMappedPointer(t *testing.T) {
	b := newBackend(t)

	buf := b.Alloc(16, gpu.MemoryDevice, "test")
	defer b.Free(buf)
	if buf.Ptr != nil {
		t.Fatal("MemoryDevice buffer unexpectedly has a mapped Ptr")
	}
}

// TestCopyBufferMovesBytesIntoADeviceBuffer: a MemoryDevice buffer has no Ptr, but it
// still has to receive real bytes through CopyBuffer — the path Store.Sync etc. rely
// on to populate device-only resources.
func TestCopyBufferMovesBytesIntoADeviceBuffer(t *testing.T) {
	b := newBackend(t)

	src := b.Alloc(4, gpu.MemoryHost, "src")
	defer b.Free(src)
	src.Write([]byte{9, 8, 7, 6}, 0)

	dst := b.Alloc(4, gpu.MemoryDevice, "dst")
	defer b.Free(dst)

	cmd := b.Begin()
	cmd.CopyBuffer(dst, src, 0, 0, 4)
	b.Wait(b.Submit(cmd))

	// Read the device buffer back the only way available: copy it into a fresh host
	// buffer and inspect that.
	readback := b.Alloc(4, gpu.MemoryHost, "readback")
	defer b.Free(readback)
	cmd = b.Begin()
	cmd.CopyBuffer(readback, dst, 0, 0, 4)
	b.Wait(b.Submit(cmd))

	got := gpuBufferBytes(readback)
	want := []byte{9, 8, 7, 6}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("readback = %v, want %v", got, want)
		}
	}
}

// TestTextureRoundTripsThroughBufferCopies checks CopyBufferToTexture and
// CopyTextureToBuffer against each other: upload pixels, read them back, expect the
// same bytes — the RGBA8 upload/readback path textures.Store and Renderer.Capture
// both depend on.
func TestTextureRoundTripsThroughBufferCopies(t *testing.T) {
	b := newBackend(t)

	tex := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: 2, Height: 2,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureSampled | gpu.TextureTransfer,
	})
	defer b.DestroyTexture(tex)
	if tex.Index == 0 {
		t.Fatal("a TextureSampled texture got heap index 0, want a nonzero bindless slot")
	}

	pixels := []byte{
		255, 0, 0, 255, 0, 255, 0, 255,
		0, 0, 255, 255, 255, 255, 0, 255,
	}
	upload := b.Alloc(uint64(len(pixels)), gpu.MemoryHost, "upload")
	defer b.Free(upload)
	upload.Write(pixels, 0)

	cmd := b.Begin()
	cmd.CopyBufferToTexture(tex, 0, 0, upload, 0)
	b.Wait(b.Submit(cmd))

	readback := b.Alloc(uint64(len(pixels)), gpu.MemoryHost, "readback")
	defer b.Free(readback)
	cmd = b.Begin()
	cmd.CopyTextureToBuffer(readback, tex, 0, 0)
	b.Wait(b.Submit(cmd))

	got := gpuBufferBytes(readback)
	for i := range pixels {
		if got[i] != pixels[i] {
			t.Fatalf("readback pixel bytes = %v, want %v", got, pixels)
		}
	}
}

// TestCreateTextureRejectsBlockCompressedFormats: this backend only stores raw texel
// bytes, so a format it cannot represent must fail loudly, not silently corrupt.
func TestCreateTextureRejectsBlockCompressedFormats(t *testing.T) {
	b := newBackend(t)
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic creating a BC1 texture")
		}
	}()
	b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: 4, Height: 4,
		Format: gpu.FormatBC1Unorm, Usage: gpu.TextureSampled,
	})
}

// TestSamplerIndicesAreUnique: two samplers must not collide on the same bindless
// slot, or the second would silently sample with the first's settings.
func TestSamplerIndicesAreUnique(t *testing.T) {
	b := newBackend(t)

	a := b.CreateSampler(gpu.SamplerDescriptor{Label: "a"})
	defer b.DestroySampler(a)
	c := b.CreateSampler(gpu.SamplerDescriptor{Label: "c"})
	defer b.DestroySampler(c)

	if a.Index == c.Index {
		t.Fatalf("two samplers share heap index %d", a.Index)
	}
}

// TestPipelinesGetDistinctValidHandles: nothing renders through them, but a caller
// that dedups pipelines by handle (as the renderer does) needs them distinguishable.
func TestPipelinesGetDistinctValidHandles(t *testing.T) {
	b := newBackend(t)

	g := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{Label: "g"})
	defer b.DestroyPipeline(g)
	c := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{Label: "c"})
	defer b.DestroyPipeline(c)

	if !g.IsValid() || !c.IsValid() {
		t.Fatal("a created pipeline reports invalid")
	}
	if g.H == c.H {
		t.Fatal("graphics and compute pipelines share a handle")
	}
}

// TestSwapchainAcquireReturnsAUsableBackbuffer: headless renderers never touch this,
// but anything that does needs a texture it can attach as a render target.
func TestSwapchainAcquireReturnsAUsableBackbuffer(t *testing.T) {
	b := newBackend(t)

	sc, err := b.CreateSwapchain(0, gpu.SwapchainDescriptor{Width: 64, Height: 32})
	if err != nil {
		t.Fatal(err)
	}
	tex, _ := b.AcquireNext(sc)
	if tex.H == 0 {
		t.Fatal("AcquireNext returned an invalid texture")
	}

	b.ResizeSwapchain(sc, 128, 64)
	resized, _ := b.AcquireNext(sc)
	if resized.H == tex.H {
		t.Fatal("resizing the swapchain did not replace the backbuffer")
	}
}

// TestTimestampsAdvanceAcrossWrites: readGPU-style code (pix's Renderer.readGPU) reads
// two slots and expects the second to read later than the first once real work
// happened in between.
func TestTimestampsAdvanceAcrossWrites(t *testing.T) {
	b := newBackend(t)

	pool := b.CreateTimestampPool(2)
	defer b.DestroyTimestampPool(pool)

	cmd := b.Begin()
	cmd.ResetTimestamps(pool, 2)
	cmd.WriteTimestamp(pool, 0, gpu.StageNone)
	cmd.WriteTimestamp(pool, 1, gpu.StageColorOutput)
	b.Wait(b.Submit(cmd))

	ts := b.ReadTimestamps(pool, 2)
	if len(ts) != 2 {
		t.Fatalf("ReadTimestamps returned %d values, want 2", len(ts))
	}
	if ts[1] <= ts[0] {
		t.Fatalf("timestamps = %v, want the second strictly after the first", ts)
	}
}

// TestSwapchainFormatIsTheOneAskedFor: a swapchain presents in the format its descriptor
// names — any the surface lists — and the backend's default when it names none. A
// format the surface does not list is an error, never a silent substitute.
func TestSwapchainFormatIsTheOneAskedFor(t *testing.T) {
	b := newBackend(t)

	for _, format := range b.SwapchainFormats(0) {
		sc, err := b.CreateSwapchain(0, gpu.SwapchainDescriptor{Width: 8, Height: 8, Format: format})
		if err != nil {
			t.Fatalf("CreateSwapchain(%v) = %v, want a swapchain: the surface lists that format", format, err)
		}
		if got := b.SwapchainFormat(sc); got != format {
			t.Errorf("SwapchainFormat() = %v, want %v, the format asked for", got, format)
		}
	}

	sc, err := b.CreateSwapchain(0, gpu.SwapchainDescriptor{Width: 8, Height: 8})
	if err != nil {
		t.Fatalf("CreateSwapchain(default) = %v, want a swapchain", err)
	}
	if got := b.SwapchainFormat(sc); got != gpu.FormatBGRA8Unorm {
		t.Errorf("SwapchainFormat() with no format = %v, want the default %v", got, gpu.FormatBGRA8Unorm)
	}

	if _, err := b.CreateSwapchain(0, gpu.SwapchainDescriptor{Width: 8, Height: 8, Format: gpu.FormatR8Unorm}); err == nil {
		t.Error("CreateSwapchain(FormatR8Unorm) succeeded, want an error: the surface does not list it")
	}
}

func TestSwapchainKeepsItsPresentMode(t *testing.T) {
	b := newBackend(t)

	sc, err := b.CreateSwapchain(0, gpu.SwapchainDescriptor{Width: 8, Height: 8, PresentMode: gpu.PresentImmediate})
	if err != nil {
		t.Fatal(err)
	}
	if got := b.PresentMode(sc); got != gpu.PresentImmediate {
		t.Errorf("PresentMode() = %v after creating with PresentImmediate, want PresentImmediate", got)
	}
	b.ResizeSwapchain(sc, 16, 16)
	if got := b.PresentMode(sc); got != gpu.PresentImmediate {
		t.Errorf("PresentMode() = %v after a resize, want the PresentImmediate it was created with", got)
	}
	for _, mode := range []gpu.PresentMode{gpu.PresentMailbox, gpu.PresentVSync} {
		b.SetPresentMode(sc, mode)
		if got := b.PresentMode(sc); got != mode {
			t.Errorf("PresentMode() = %v after SetPresentMode(%v), want %v", got, mode, mode)
		}
	}
}
