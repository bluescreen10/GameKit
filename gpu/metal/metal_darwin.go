//go:build darwin && cgo

// Package metal implements gpu.Backend directly on Metal 3 (macOS 13+), using
// GPU addresses and Tier 2 argument buffers. See README.md for the shader ABI.
package metal

/*
#cgo CFLAGS: -mmacosx-version-min=13.0
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

func New() *Backend { return &Backend{} }
func init()         { gpu.RegisterBackend(New(), "metal", 1) }
func (b *Backend) call(op int, a *C.MBArgs) uint64 {
	if b.native == nil {
		panic("metal: backend is not initialized")
	}
	// Args may contain borrowed shader/readback slices. Pin their backing arrays
	// for this synchronous call; native code never retains these pointers.
	var pins runtime.Pinner
	defer pins.Unpin()
	for _, p := range a.p {
		if p != nil {
			pins.Pin(p)
		}
	}
	r := uint64(C.mbCall(b.native, C.int(op), a))
	if a.error[0] != 0 {
		panic("metal: " + C.GoString(&a.error[0]))
	}
	return r
}
func args(v ...uint64) C.MBArgs {
	var a C.MBArgs
	for i, x := range v {
		a.u[i] = C.uint64_t(x)
	}
	return a
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
	a := args()
	C.mbCall(b.native, C.MBInit, &a)
	if a.error[0] != 0 {
		msg := C.GoString(&a.error[0])
		C.mbCall(b.native, C.MBDestroy, &C.MBArgs{})
		b.native = nil
		return fmt.Errorf("metal: %s", msg)
	}
	return nil
}
func (b *Backend) Destroy() {
	if b.native != nil {
		b.WaitIdle()
		b.call(C.MBDestroy, &C.MBArgs{})
		b.native = nil
	}
}
func (b *Backend) Alloc(size uint64, mem gpu.MemoryType, label string) gpu.Buffer {
	a := args(size, uint64(mem))
	p, done := cstr(label)
	defer done()
	a.p[0] = unsafe.Pointer(p)
	h := b.call(C.MBAlloc, &a)
	q := args(h)
	addr := b.call(C.MBAddress, &q)
	q = args(h)
	return gpu.Buffer{H: gpu.Handle(h), Size: size, Addr: addr, Ptr: C.mbPointer(C.uint64_t(b.call(C.MBContents, &q)))}
}
func (b *Backend) Free(v gpu.Buffer) { a := args(uint64(v.H)); b.call(C.MBFree, &a) }
func (b *Backend) CreateTexture(d gpu.TextureDescriptor) gpu.Texture {
	a := args(uint64(d.Kind), uint64(d.Width), uint64(d.Height), uint64(d.Depth), uint64(d.Layers), uint64(d.Mips), uint64(d.Format), uint64(d.Usage), uint64(d.Samples))
	p, done := cstr(d.Label)
	defer done()
	a.p[0] = unsafe.Pointer(p)
	h := b.call(C.MBTexture, &a)
	return gpu.Texture{H: gpu.Handle(h), Index: uint32(a.u[31])}
}
func (b *Backend) TextureView(t gpu.Texture, k gpu.TextureKind, m, n, l, c uint32) gpu.Texture {
	a := args(uint64(t.H), uint64(k), uint64(m), uint64(n), uint64(l), uint64(c))
	h := b.call(C.MBView, &a)
	return gpu.Texture{H: gpu.Handle(h), Index: uint32(a.u[31])}
}
func (b *Backend) release(h gpu.Handle)         { a := args(uint64(h)); b.call(C.MBRelease, &a) }
func (b *Backend) DestroyTexture(t gpu.Texture) { b.release(t.H) }
func (b *Backend) CreateSampler(d gpu.SamplerDescriptor) gpu.Sampler {
	a := args(flag(d.MinLinear), flag(d.MagLinear), flag(d.MipLinear), uint64(d.AddressU), uint64(d.AddressV), uint64(d.AddressW), uint64(d.Compare), uint64(d.MaxAnisotropy))
	p, done := cstr(d.Label)
	defer done()
	a.p[0] = unsafe.Pointer(p)
	h := b.call(C.MBSampler, &a)
	return gpu.Sampler{H: gpu.Handle(h), Index: uint32(a.u[31])}
}
func (b *Backend) DestroySampler(s gpu.Sampler) { b.release(s.H) }
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
	a := args(uint64(len(d.Shader)), uint64(group[0]), uint64(group[1]), uint64(group[2]))
	a.p[0] = shader(d.Shader)
	p, done := cstr(entry(d.Entry))
	defer done()
	a.p[1] = unsafe.Pointer(p)
	l, done2 := cstr(d.Label)
	defer done2()
	a.p[2] = unsafe.Pointer(l)
	return gpu.Pipeline{H: gpu.Handle(b.call(C.MBCompute, &a))}
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
	a := args(uint64(len(d.VertexShader)), uint64(len(d.FragmentShader)), uint64(d.Topology), uint64(d.DepthFormat), uint64(d.Samples), uint64(d.CullMode), flag(d.FrontFaceCW), flag(d.DepthTest), flag(d.DepthWrite), uint64(d.DepthCompare), uint64(len(d.ColorFormats)))
	a.p[0] = shader(d.VertexShader)
	a.p[1] = shader(d.FragmentShader)
	v, dv := cstr(entry(d.VertexEntry))
	defer dv()
	f, df := cstr(entry(d.FragmentEntry))
	defer df()
	a.p[2] = unsafe.Pointer(v)
	a.p[3] = unsafe.Pointer(f)
	for i, x := range d.ColorFormats {
		a.u[11+i] = C.uint64_t(x)
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
		a.u[19+i] = C.uint64_t(flag(x.Enable) | uint64(mask)<<1 | uint64(x.ColorOp.Src)<<5 | uint64(x.ColorOp.Dst)<<9 | uint64(x.ColorOp.Op)<<13 | uint64(x.AlphaOp.Src)<<17 | uint64(x.AlphaOp.Dst)<<21 | uint64(x.AlphaOp.Op)<<25)
	}
	return gpu.Pipeline{H: gpu.Handle(b.call(C.MBPipeline, &a))}
}
func (b *Backend) DestroyPipeline(p gpu.Pipeline) { b.release(p.H) }
func (b *Backend) Begin() gpu.CommandBuffer       { return &command{b: b, h: b.call(C.MBBegin, &C.MBArgs{})} }
func (b *Backend) cmd(c gpu.CommandBuffer) *command {
	v, ok := c.(*command)
	if !ok || v.b != b || v.submitted {
		panic("metal: invalid or submitted command buffer")
	}
	return v
}
func (b *Backend) Submit(c gpu.CommandBuffer) gpu.Fence {
	v := b.cmd(c)
	a := args(v.h)
	h := b.call(C.MBSubmit, &a)
	v.submitted = true
	return gpu.Fence{H: gpu.Handle(h)}
}
func (b *Backend) Wait(f gpu.Fence) { a := args(uint64(f.H)); b.call(C.MBWait, &a) }
func (b *Backend) WaitIdle()        { b.call(C.MBIdle, &C.MBArgs{}) }

// CocoaMetalLayer installs and returns a CAMetalLayer on a Cocoa NSWindow. It is
// shared with the Vulkan macOS surface path so only one cgo package needs to
// compile Objective-C and link the Objective-C runtime.
func CocoaMetalLayer(window unsafe.Pointer) uintptr {
	a := args()
	a.p[0] = window
	r := uintptr(C.mbCall(nil, C.MBSurface, &a))
	if a.error[0] != 0 {
		panic("metal: " + C.GoString(&a.error[0]))
	}
	return r
}

func (b *Backend) CreateMetalSurface(window unsafe.Pointer) uintptr { return CocoaMetalLayer(window) }

// CreateSwapchain takes a CAMetalLayer pointer, e.g. from CreateMetalSurface.
func (b *Backend) CreateSwapchain(surface uintptr, w, h uint32) gpu.Swapchain {
	a := args(uint64(surface), uint64(w), uint64(h))
	return gpu.Swapchain{H: gpu.Handle(b.call(C.MBSwapchain, &a))}
}
func (b *Backend) ResizeSwapchain(s gpu.Swapchain, w, h uint32) {
	b.WaitIdle()
	a := args(uint64(s.H), uint64(w), uint64(h))
	b.call(C.MBResize, &a)
}
func (b *Backend) SwapchainSize(s gpu.Swapchain) (uint32, uint32) {
	a := args(uint64(s.H), 0, 0, 1)
	b.call(C.MBResize, &a)
	return uint32(a.u[1]), uint32(a.u[2])
}
func (b *Backend) SwapchainFormat(gpu.Swapchain) gpu.Format { return gpu.FormatBGRA8Unorm }
func (b *Backend) AcquireNext(s gpu.Swapchain) (gpu.Texture, gpu.Fence) {
	a := args(uint64(s.H))
	h := b.call(C.MBAcquire, &a)
	return gpu.Texture{H: gpu.Handle(h)}, gpu.Fence{}
}
func (b *Backend) Present(s gpu.Swapchain, c gpu.CommandBuffer) {
	v := b.cmd(c)
	a := args(uint64(s.H), v.h)
	b.call(C.MBPresent, &a)
	v.submitted = true
}
func (b *Backend) CreateTimestampPool(n uint32) gpu.QueryPool {
	a := args(uint64(n))
	return gpu.QueryPool{H: gpu.Handle(b.call(C.MBPool, &a))}
}
func (b *Backend) DestroyTimestampPool(p gpu.QueryPool) { b.release(p.H) }
func (b *Backend) ReadTimestamps(p gpu.QueryPool, n uint32) []uint64 {
	if !p.Valid() || n == 0 {
		return nil
	}
	out := make([]uint64, n)
	a := args(uint64(p.H), uint64(n))
	a.p[0] = unsafe.Pointer(&out[0])
	b.call(C.MBReadTimes, &a)
	return out
}
func (b *Backend) TimestampPeriod() float64 { return 1 }

type command struct {
	b         *Backend
	h         uint64
	submitted bool
}

func (c *command) call(op int, a *C.MBArgs) {
	if c.submitted {
		panic("metal: command buffer already submitted")
	}
	a.u[30] = C.uint64_t(c.h)
	c.b.call(op, a)
}
func (c *command) BeginRenderPass(r gpu.RenderTargets) {
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
	a := args()
	a.p[0] = unsafe.Pointer(&d)
	c.call(C.MBRenderBegin, &a)
}
func (c *command) EndRenderPass()             { c.call(C.MBRenderEnd, &C.MBArgs{}) }
func (c *command) SetPipeline(p gpu.Pipeline) { a := args(uint64(p.H)); c.call(C.MBSetPipeline, &a) }
func (c *command) Root(addr uint64)           { a := args(addr); c.call(C.MBRoot, &a) }
func (c *command) Viewport(x, y, w, h, min, max float32) {
	a := args()
	for i, v := range []float32{x, y, w, h, min, max} {
		a.f[i] = C.double(v)
	}
	c.call(C.MBViewport, &a)
}
func (c *command) Scissor(x, y, w, h int32) {
	if x < 0 || y < 0 || w < 0 || h < 0 {
		panic("metal: negative scissor")
	}
	a := args(uint64(x), uint64(y), uint64(w), uint64(h))
	c.call(C.MBScissor, &a)
}
func (c *command) Draw(n, i, v, base uint32) {
	a := args(uint64(n), uint64(i), uint64(v), uint64(base))
	c.call(C.MBDraw, &a)
}
func (c *command) DrawIndexed(b gpu.Buffer, n, i, first uint32, off int32, base uint32) {
	a := args(uint64(b.H), uint64(n), uint64(i), uint64(first), uint64(int64(off)), uint64(base))
	c.call(C.MBIndexed, &a)
}
func (c *command) DrawIndexedIndirect(b, a gpu.Buffer, off uint64, n, stride uint32) {
	v := args(uint64(b.H), uint64(a.H), off, uint64(n), uint64(stride))
	c.call(C.MBIndirect, &v)
}
func (c *command) Dispatch(x, y, z uint32) {
	a := args(uint64(x), uint64(y), uint64(z))
	c.call(C.MBDispatch, &a)
}
func (c *command) DispatchIndirect(b gpu.Buffer, off uint64) {
	a := args(uint64(b.H), off)
	c.call(C.MBDispatchIndirect, &a)
}
func (c *command) Barrier(src, dst gpu.Stage, f gpu.BarrierFlags) { c.call(C.MBBarrier, &C.MBArgs{}) }
func (c *command) PrepareSampled(t gpu.Texture, s gpu.Stage)      { c.Barrier(gpu.StageAll, s, 0) }
func (c *command) CopyBuffer(dst, src gpu.Buffer, do, so, size uint64) {
	a := args(uint64(dst.H), uint64(src.H), do, so, size)
	c.call(C.MBCopyBuffer, &a)
}
func (c *command) CopyBufferToTexture(dst gpu.Texture, m, l uint32, src gpu.Buffer, off uint64) {
	a := args(uint64(dst.H), uint64(m), uint64(l), uint64(src.H), off)
	c.call(C.MBUpload, &a)
}
func (c *command) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, m, l uint32) {
	a := args(uint64(src.H), uint64(m), uint64(l), uint64(dst.H), 0)
	c.call(C.MBReadback, &a)
}
func (c *command) ResetTimestamps(p gpu.QueryPool, n uint32) {
	a := args(uint64(p.H), uint64(n))
	c.call(C.MBResetTimes, &a)
}
func (c *command) WriteTimestamp(p gpu.QueryPool, i uint32, s gpu.Stage) {
	a := args(uint64(p.H), uint64(i), uint64(s))
	c.call(C.MBTimestamp, &a)
}
