//go:build darwin && cgo

// Package metal implements gpu.Backend directly on Metal 4 (macOS 26+), using
// GPU addresses and Tier 2 argument buffers. See README.md for the shader ABI.
package metal

/*
#cgo CFLAGS: -mmacosx-version-min=26.0
#cgo LDFLAGS: -framework Metal -framework Foundation -framework QuartzCore -framework Cocoa
#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// Backend owns one device and serial queue. Like command recording, resource
// creation/destruction must be externally serialized. Wait before freeing resources.
type Backend struct{ native unsafe.Pointer }

var _ gpu.Backend = (*Backend)(nil)
var _ gpu.CommandBuffer = (*command)(nil)

// ShaderFormat identifies the native shader format to renderer integrations.
func (*Backend) ShaderFormat() string { return "metal" }

// supportedFormats is the static set this backend accepts. Every entry here
// must have a real case in bridge_darwin.m's format(); ETC2/EAC are excluded
// because Metal never exposes them, on any platform.
var supportedFormats = []gpu.Format{
	gpu.FormatR8Unorm, gpu.FormatRG8Unorm, gpu.FormatRGBA8Unorm, gpu.FormatBGRA8Unorm,
	gpu.FormatRGBA8Srgb, gpu.FormatBGRA8Srgb,
	gpu.FormatR8Snorm, gpu.FormatRG8Snorm, gpu.FormatRGBA8Snorm,
	gpu.FormatR8Uint, gpu.FormatRG8Uint, gpu.FormatRGBA8Uint,
	gpu.FormatR8Sint, gpu.FormatRG8Sint, gpu.FormatRGBA8Sint,
	gpu.FormatR16Unorm, gpu.FormatRG16Unorm, gpu.FormatRGBA16Unorm,
	gpu.FormatR16Snorm, gpu.FormatRG16Snorm, gpu.FormatRGBA16Snorm,
	gpu.FormatR16Uint, gpu.FormatRG16Uint, gpu.FormatRGBA16Uint,
	gpu.FormatR16Sint, gpu.FormatRG16Sint, gpu.FormatRGBA16Sint,
	gpu.FormatR16F, gpu.FormatRG16F, gpu.FormatRGBA16F,
	gpu.FormatR32Uint, gpu.FormatRG32Uint, gpu.FormatRGBA32Uint,
	gpu.FormatR32Sint, gpu.FormatRG32Sint, gpu.FormatRGBA32Sint,
	gpu.FormatR32F, gpu.FormatRG32F, gpu.FormatRGBA32F,
	gpu.FormatRGB10A2Unorm, gpu.FormatRGB10A2Uint, gpu.FormatRG11B10F, gpu.FormatRGB9E5F,
	gpu.FormatDepth16Unorm, gpu.FormatDepth32F, gpu.FormatDepth24Stencil8, gpu.FormatDepth32FStencil8, gpu.FormatStencil8,
	gpu.FormatBC1Unorm, gpu.FormatBC1Srgb, gpu.FormatBC3Unorm, gpu.FormatBC3Srgb,
	gpu.FormatBC4Unorm, gpu.FormatBC4Snorm, gpu.FormatBC5Unorm, gpu.FormatBC5Snorm,
	gpu.FormatBC6HFloat, gpu.FormatBC6HUFloat, gpu.FormatBC7Unorm, gpu.FormatBC7Srgb,
	gpu.FormatASTC4x4Unorm, gpu.FormatASTC4x4Srgb,
	gpu.FormatASTC5x5Unorm, gpu.FormatASTC5x5Srgb,
	gpu.FormatASTC6x6Unorm, gpu.FormatASTC6x6Srgb,
	gpu.FormatASTC8x8Unorm, gpu.FormatASTC8x8Srgb,
	gpu.FormatASTC10x10Unorm, gpu.FormatASTC10x10Srgb,
	gpu.FormatASTC12x12Unorm, gpu.FormatASTC12x12Srgb,
}

// SupportedFormats reports the Format values this backend can create textures with.
func (b *Backend) SupportedFormats() []gpu.Format { return supportedFormats }

func New() *Backend {
	return &Backend{}
}

func init() {
	gpu.RegisterBackend(New(), "metal", 1)
}

func result(r C.MBResult) uint64 {
	if r.error != 0 {
		panic("metal: " + C.GoString(C.mbError()))
	}
	return uint64(r.value)
}

func pointerResult(r C.MBPointerResult) unsafe.Pointer {
	if r.error != 0 {
		panic("metal: " + C.GoString(C.mbError()))
	}
	return r.value
}

func flag(v bool) uint64 {
	if v {
		return 1
	}
	return 0
}

func cstr(s string) (*C.char, func()) {
	p := C.CString(s)
	return p, func() { C.free(unsafe.Pointer(p)) }
}

func (b *Backend) Init() error {
	if b.native != nil {
		return nil
	}
	b.native = C.mbCreate()
	r := C.mbInit(b.native)
	if r.error != 0 {
		msg := C.GoString(C.mbError())
		C.mbDestroy(b.native)
		b.native = nil
		return fmt.Errorf("metal: %s", msg)
	}
	return nil
}

func (b *Backend) Destroy() {
	if b.native != nil {
		b.WaitIdle()
		result(C.mbDestroy(b.native))
		b.native = nil
	}
}

func (b *Backend) Alloc(size uint64, mem gpu.MemoryType, label string) gpu.Buffer {
	p, done := cstr(label)
	defer done()
	h := result(C.mbAlloc(b.native, C.uint64_t(size), C.uint32_t(mem), p))
	addr := result(C.mbBufferAddress(b.native, C.uint64_t(h)))
	contents := pointerResult(C.mbBufferContents(b.native, C.uint64_t(h)))
	return gpu.Buffer{H: gpu.Handle(h), Size: size, Addr: addr, Ptr: contents}
}

func (b *Backend) Free(v gpu.Buffer) {
	result(C.mbReleaseResource(b.native, C.uint64_t(v.H)))
}

func (b *Backend) CreateTexture(d gpu.TextureDescriptor) gpu.Texture {
	p, done := cstr(d.Label)
	defer done()
	r := C.mbCreateTexture(b.native, C.uint32_t(d.Kind), C.uint32_t(d.Width), C.uint32_t(d.Height), C.uint32_t(d.Depth), C.uint32_t(d.Layers), C.uint32_t(d.Mips), C.uint32_t(d.Format), C.uint32_t(d.Usage), C.uint32_t(d.Samples), p)
	h := result(r)
	return gpu.Texture{H: gpu.Handle(h), Index: uint32(r.auxiliary)}
}

func (b *Backend) TextureView(t gpu.Texture, k gpu.TextureKind, m, n, l, c uint32) gpu.Texture {
	r := C.mbCreateTextureView(b.native, C.uint64_t(t.H), C.uint32_t(k), C.uint32_t(m), C.uint32_t(n), C.uint32_t(l), C.uint32_t(c))
	h := result(r)
	return gpu.Texture{H: gpu.Handle(h), Index: uint32(r.auxiliary)}
}

func (b *Backend) release(h gpu.Handle) {
	result(C.mbReleaseResource(b.native, C.uint64_t(h)))
}

func (b *Backend) DestroyTexture(t gpu.Texture) {
	b.release(t.H)
}

func (b *Backend) CreateSampler(d gpu.SamplerDescriptor) gpu.Sampler {
	p, done := cstr(d.Label)
	defer done()
	r := C.mbCreateSampler(b.native, C.uint32_t(flag(d.MinLinear)), C.uint32_t(flag(d.MagLinear)), C.uint32_t(flag(d.MipLinear)), C.uint32_t(d.AddressU), C.uint32_t(d.AddressV), C.uint32_t(d.AddressW), C.uint32_t(d.Compare), C.uint32_t(d.MaxAnisotropy), p)
	h := result(r)
	return gpu.Sampler{H: gpu.Handle(h), Index: uint32(r.auxiliary)}
}

func (b *Backend) DestroySampler(s gpu.Sampler) {
	b.release(s.H)
}

func shader(p []byte) unsafe.Pointer {
	if len(p) == 0 {
		return nil
	}
	return unsafe.Pointer(&p[0])
}

func entry(s string) string {
	if s == "" || s == "main" {
		return "main0"
	}
	return s
}

// shaderCode unwraps Pix's precompiled Metal artifact format. Translation and
// metallib compilation happen in the application at build time; the backend only
// consumes the resulting bytes and the local threadgroup dimensions stored beside
// them. Raw MSL and metallib bytes remain valid and default to a 1x1x1 group.
func shaderCode(data []byte) ([]byte, [3]uint32, error) {
	group := [3]uint32{1, 1, 1}
	if !bytes.HasPrefix(data, []byte("PIXMTL01")) {
		return data, group, nil
	}
	if len(data) < 20 {
		return nil, group, fmt.Errorf("truncated Metal shader header")
	}
	for i := range group {
		group[i] = binary.LittleEndian.Uint32(data[8+i*4:])
		if group[i] == 0 {
			return nil, group, fmt.Errorf("zero workgroup dimension")
		}
	}
	return data[20:], group, nil
}

func (b *Backend) CreateComputePipeline(d gpu.ComputePipelineDescriptor) gpu.Pipeline {
	code, group, err := shaderCode(d.Shader)
	if err != nil {
		panic("metal: " + err.Error())
	}
	d.Shader = code
	p, done := cstr(entry(d.Entry))
	defer done()
	l, done2 := cstr(d.Label)
	defer done2()
	desc := C.MBComputePipelineDesc{
		shader:     shader(d.Shader),
		shaderSize: C.uint64_t(len(d.Shader)),
		entry:      p,
		label:      l,
		groupX:     C.uint32_t(group[0]),
		groupY:     C.uint32_t(group[1]),
		groupZ:     C.uint32_t(group[2]),
	}
	var pins runtime.Pinner
	if desc.shader != nil {
		pins.Pin(desc.shader)
		defer pins.Unpin()
	}
	return gpu.Pipeline{H: gpu.Handle(result(C.mbCreateComputePipeline(b.native, &desc)))}
}

func (b *Backend) CreateGraphicsPipeline(d gpu.PipelineDescriptor) gpu.Pipeline {
	var err error
	d.VertexShader, _, err = shaderCode(d.VertexShader)
	if err != nil {
		panic(err)
	}
	d.FragmentShader, _, err = shaderCode(d.FragmentShader)
	if err != nil {
		panic(err)
	}
	if len(d.ColorFormats) > 8 {
		panic("metal: at most 8 color targets")
	}
	v, dv := cstr(entry(d.VertexEntry))
	defer dv()
	f, df := cstr(entry(d.FragmentEntry))
	defer df()
	desc := C.MBGraphicsPipelineDesc{
		vertexShader:       shader(d.VertexShader),
		vertexShaderSize:   C.uint64_t(len(d.VertexShader)),
		fragmentShader:     shader(d.FragmentShader),
		fragmentShaderSize: C.uint64_t(len(d.FragmentShader)),
		vertexEntry:        v,
		fragmentEntry:      f,
		topology:           C.uint32_t(d.Topology),
		depthFormat:        C.uint32_t(d.DepthFormat),
		samples:            C.uint32_t(d.Samples),
		cullMode:           C.uint32_t(d.CullMode),
		frontFaceCW:        C.uint32_t(flag(d.FrontFaceCW)),
		depthTest:          C.uint32_t(flag(d.DepthTest)),
		depthWrite:         C.uint32_t(flag(d.DepthWrite)),
		depthCompare:       C.uint32_t(d.DepthCompare),
		colorCount:         C.uint32_t(len(d.ColorFormats)),
	}
	for i, x := range d.ColorFormats {
		desc.colorFormats[i] = C.uint32_t(x)
	}
	// Each target's blend state is packed into a bridge word.
	for i, x := range d.Blend {
		if i >= len(d.ColorFormats) {
			break
		}
		mask := x.WriteMask
		if mask == 0 {
			mask = 15
		}
		desc.blend[i] = C.uint64_t(flag(x.Enable) | uint64(mask)<<1 | uint64(x.ColorOp.Src)<<5 | uint64(x.ColorOp.Dst)<<9 | uint64(x.ColorOp.Op)<<13 | uint64(x.AlphaOp.Src)<<17 | uint64(x.AlphaOp.Dst)<<21 | uint64(x.AlphaOp.Op)<<25)
	}
	var pins runtime.Pinner
	if desc.vertexShader != nil {
		pins.Pin(desc.vertexShader)
	}
	if desc.fragmentShader != nil {
		pins.Pin(desc.fragmentShader)
	}
	defer pins.Unpin()
	return gpu.Pipeline{H: gpu.Handle(result(C.mbCreateGraphicsPipeline(b.native, &desc)))}
}
func (b *Backend) DestroyPipeline(p gpu.Pipeline) {
	b.release(p.H)
}

func (b *Backend) Begin() gpu.CommandBuffer {
	return &command{b: b, h: result(C.mbBegin(b.native))}
}

func (b *Backend) cmd(c gpu.CommandBuffer) *command {
	v, ok := c.(*command)
	if !ok || v.b != b || v.submitted {
		panic("metal: invalid or submitted command buffer")
	}
	return v
}
func (b *Backend) Submit(c gpu.CommandBuffer) gpu.Fence {
	v := b.cmd(c)
	h := result(C.mbSubmit(b.native, C.uint64_t(v.h)))
	v.submitted = true
	return gpu.Fence{H: gpu.Handle(h)}
}

func (b *Backend) Wait(f gpu.Fence) {
	result(C.mbWait(b.native, C.uint64_t(f.H)))
}

func (b *Backend) WaitIdle() {
	result(C.mbWaitIdle(b.native))
}

// CocoaMetalLayer installs and returns a CAMetalLayer on a Cocoa NSWindow. It is
// shared with the Vulkan macOS surface path so only one cgo package needs to
// compile Objective-C and link the Objective-C runtime.
func CocoaMetalLayer(window unsafe.Pointer) uintptr {
	return uintptr(result(C.mbCreateMetalSurface(window)))
}

func (b *Backend) CreateMetalSurface(window unsafe.Pointer) uintptr {
	return CocoaMetalLayer(window)
}

// CreateSwapchain takes a CAMetalLayer pointer, e.g. from CreateMetalSurface.
func (b *Backend) CreateSwapchain(surface uintptr, w, h uint32) gpu.Swapchain {
	return gpu.Swapchain{H: gpu.Handle(result(C.mbCreateSwapchain(b.native, C.uintptr_t(surface), C.uint32_t(w), C.uint32_t(h))))}
}

func (b *Backend) ResizeSwapchain(s gpu.Swapchain, w, h uint32) {
	b.WaitIdle()
	result(C.mbResizeSwapchain(b.native, C.uint64_t(s.H), C.uint32_t(w), C.uint32_t(h)))
}

func (b *Backend) SwapchainSize(s gpu.Swapchain) (uint32, uint32) {
	r := C.mbSwapchainSize(b.native, C.uint64_t(s.H))
	result(r)
	return uint32(r.value), uint32(r.auxiliary)
}

func (b *Backend) SwapchainFormat(gpu.Swapchain) gpu.Format {
	return gpu.FormatBGRA8Unorm
}

func (b *Backend) AcquireNext(s gpu.Swapchain) (gpu.Texture, gpu.Fence) {
	h := result(C.mbAcquireNext(b.native, C.uint64_t(s.H)))
	return gpu.Texture{H: gpu.Handle(h)}, gpu.Fence{}
}

func (b *Backend) Present(s gpu.Swapchain, c gpu.CommandBuffer) {
	v := b.cmd(c)
	result(C.mbPresent(b.native, C.uint64_t(s.H), C.uint64_t(v.h)))
	v.submitted = true
}
func (b *Backend) CreateTimestampPool(n uint32) gpu.QueryPool {
	return gpu.QueryPool{H: gpu.Handle(result(C.mbCreateTimestampPool(b.native, C.uint32_t(n))))}
}
func (b *Backend) DestroyTimestampPool(p gpu.QueryPool) { b.release(p.H) }
func (b *Backend) ReadTimestamps(p gpu.QueryPool, n uint32) []uint64 {
	if !p.IsValid() || n == 0 {
		return nil
	}
	out := make([]uint64, n)
	result(C.mbReadTimestamps(b.native, C.uint64_t(p.H), C.uint32_t(n), (*C.uint64_t)(unsafe.Pointer(&out[0]))))
	return out
}
func (b *Backend) TimestampPeriod() float64 { return 1 }

type command struct {
	b         *Backend
	h         uint64
	submitted bool
}

func (c *command) ready() {
	if c.submitted {
		panic("metal: command buffer already submitted")
	}
}
func (c *command) BeginRenderPass(r gpu.RenderTargets) {
	c.ready()
	if len(r.Color) > 8 {
		panic("metal: at most 8 color targets")
	}
	var d C.MBRenderDesc
	d.colorCount = C.uint32_t(len(r.Color))
	for i, x := range r.Color {
		d.color[i] = C.uint64_t(x.Texture.H)
		d.colorLoad[i] = C.uint32_t(x.Load)
		d.colorStore[i] = C.uint32_t(x.Store)
		for j, v := range x.Clear {
			d.colorClear[i][j] = C.float(v)
		}
	}
	if r.Depth != nil {
		x := r.Depth
		d.depth = C.uint64_t(x.Texture.H)
		d.depthLoad = C.uint32_t(x.Load)
		d.depthStore = C.uint32_t(x.Store)
		d.depthReadOnly = C.uint32_t(flag(x.ReadOnly))
		d.depthClear = C.float(x.Clear)
	}
	result(C.mbBeginRenderPass(c.b.native, C.uint64_t(c.h), &d))
}
func (c *command) EndRenderPass() {
	c.ready()
	result(C.mbEndRenderPass(c.b.native, C.uint64_t(c.h)))
}
func (c *command) SetPipeline(p gpu.Pipeline) {
	c.ready()
	result(C.mbSetPipeline(c.b.native, C.uint64_t(c.h), C.uint64_t(p.H)))
}

// bridgeData returns data for a synchronous bridge call. Metal copies it into the
// command buffer before the function returns.
func bridgeData(data []byte) (unsafe.Pointer, C.uint32_t) {
	if len(data) == 0 {
		return nil, 0
	}
	return unsafe.Pointer(&data[0]), C.uint32_t(len(data))
}

func (c *command) SetViewport(x, y, w, h, min, max float32) {
	c.ready()
	result(C.mbSetViewport(c.b.native, C.uint64_t(c.h), C.double(x), C.double(y), C.double(w), C.double(h), C.double(min), C.double(max)))
}
func (c *command) SetScissor(x, y, w, h int32) {
	c.ready()
	if x < 0 || y < 0 || w < 0 || h < 0 {
		panic("metal: negative scissor")
	}
	result(C.mbSetScissor(c.b.native, C.uint64_t(c.h), C.uint32_t(x), C.uint32_t(y), C.uint32_t(w), C.uint32_t(h)))
}
func (c *command) SetDepthBias(bias, slope, clamp float32) {
	c.ready()
	result(C.mbSetDepthBias(c.b.native, C.uint64_t(c.h), C.double(bias), C.double(slope), C.double(clamp)))
}
func (c *command) Draw(data []byte, n, i, v, base uint32) {
	c.ready()
	p, size := bridgeData(data)
	result(C.mbDraw(c.b.native, C.uint64_t(c.h), p, size, C.uint32_t(n), C.uint32_t(i), C.uint32_t(v), C.uint32_t(base)))
}
func (c *command) DrawIndexed(data []byte, b gpu.Buffer, it gpu.IndexType, n, i, first uint32, off int32, base uint32) {
	c.ready()
	p, size := bridgeData(data)
	result(C.mbDrawIndexed(c.b.native, C.uint64_t(c.h), p, size, C.uint64_t(b.H), C.uint32_t(n), C.uint32_t(i), C.uint32_t(first), C.int32_t(off), C.uint32_t(base), C.uint32_t(it.Size())))
}
func (c *command) DrawIndexedIndirect(data []byte, b gpu.Buffer, it gpu.IndexType, a gpu.Buffer, off uint64, n, stride uint32) {
	c.ready()
	p, size := bridgeData(data)
	result(C.mbDrawIndexedIndirect(c.b.native, C.uint64_t(c.h), p, size, C.uint64_t(b.H), C.uint64_t(a.H), C.uint64_t(off), C.uint32_t(n), C.uint32_t(stride), C.uint32_t(it.Size())))
}
func (c *command) Dispatch(data []byte, x, y, z uint32) {
	c.ready()
	p, size := bridgeData(data)
	result(C.mbDispatch(c.b.native, C.uint64_t(c.h), p, size, C.uint32_t(x), C.uint32_t(y), C.uint32_t(z)))
}
func (c *command) DispatchIndirect(data []byte, b gpu.Buffer, off uint64) {
	c.ready()
	p, size := bridgeData(data)
	result(C.mbDispatchIndirect(c.b.native, C.uint64_t(c.h), p, size, C.uint64_t(b.H), C.uint64_t(off)))
}
func (c *command) Barrier(src, dst gpu.Stage, f gpu.BarrierFlags) {
	c.ready()
	result(C.mbBarrier(c.b.native, C.uint64_t(c.h)))
}
func (c *command) PrepareSampled(t gpu.Texture, s gpu.Stage) {
	c.Barrier(gpu.StageAll, s, 0)
}

func (c *command) CopyBuffer(dst, src gpu.Buffer, do, so, size uint64) {
	c.ready()
	result(C.mbCopyBuffer(c.b.native, C.uint64_t(c.h), C.uint64_t(dst.H), C.uint64_t(src.H), C.uint64_t(do), C.uint64_t(so), C.uint64_t(size)))
}

func (c *command) CopyBufferToTexture(dst gpu.Texture, m, l uint32, src gpu.Buffer, off uint64) {
	c.ready()
	result(C.mbCopyBufferToTexture(c.b.native, C.uint64_t(c.h), C.uint64_t(dst.H), C.uint32_t(m), C.uint32_t(l), C.uint64_t(src.H), C.uint64_t(off)))
}
func (c *command) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, m, l uint32) {
	c.ready()
	result(C.mbCopyTextureToBuffer(c.b.native, C.uint64_t(c.h), C.uint64_t(dst.H), C.uint64_t(src.H), C.uint32_t(m), C.uint32_t(l)))
}
func (c *command) ResetTimestamps(p gpu.QueryPool, n uint32) {
	c.ready()
	result(C.mbResetTimestamps(c.b.native, C.uint64_t(c.h), C.uint64_t(p.H), C.uint32_t(n)))
}
func (c *command) WriteTimestamp(p gpu.QueryPool, i uint32, s gpu.Stage) {
	c.ready()
	result(C.mbWriteTimestamp(c.b.native, C.uint64_t(c.h), C.uint64_t(p.H), C.uint32_t(i), C.uint32_t(s)))
}

// GetMaxDataSize is the largest draw/dispatch data this device accepts, in bytes.
//
// Metal's setBytes: is documented for data under 4 KB; past that Apple directs callers
// to an MTLBuffer instead. There is no queryable device property for it, so the
// documented threshold is the limit.
func (b *Backend) GetMaxDataSize() int32 {
	return 4096
}
