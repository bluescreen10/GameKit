package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"unsafe"

	"fmt"

	"github.com/bluescreen10/gamekit/gpu"
)

var (
	vkStageNone       = C.vkbStageNone()
	vkStageTop        = C.vkbStageTop()
	vkStageIndirect   = C.vkbStageIndirect()
	vkStageVertex     = C.vkbStageVertex()
	vkStageFragment   = C.vkbStageFragment()
	vkStageColor      = C.vkbStageColor()
	vkStageEarlyDepth = C.vkbStageEarlyDepth()
	vkStageLateDepth  = C.vkbStageLateDepth()
	vkStageCompute    = C.vkbStageCompute()
	vkStageTransfer   = C.vkbStageTransfer()
	vkStageAll        = C.vkbStageAll()

	vkAccessNone          = C.vkbAccessNone()
	vkAccessIndirectRead  = C.vkbAccessIndirectRead()
	vkAccessShaderRead    = C.vkbAccessShaderRead()
	vkAccessShaderWrite   = C.vkbAccessShaderWrite()
	vkAccessColorRead     = C.vkbAccessColorRead()
	vkAccessColorWrite    = C.vkbAccessColorWrite()
	vkAccessDepthRead     = C.vkbAccessDepthRead()
	vkAccessDepthWrite    = C.vkbAccessDepthWrite()
	vkAccessTransferRead  = C.vkbAccessTransferRead()
	vkAccessTransferWrite = C.vkbAccessTransferWrite()
	vkAccessMemoryRead    = C.vkbAccessMemoryRead()
	vkAccessMemoryWrite   = C.vkbAccessMemoryWrite()
)

func (b *Backend) initCommands() error {
	if r := C.vkbCreateCommandPool(b.device, C.uint32_t(b.queueFamily), &b.cmdPool); r != C.VK_SUCCESS {
		return fmt.Errorf("vulkan: command pool creation failed (%d)", int(r))
	}
	b.fences = map[uint64]fenceEntry{}
	return nil
}

func (b *Backend) destroyCommands() {
	for _, f := range b.fences {
		C.vkDestroyFence(b.device, f.fence, nil)
		C.vkFreeCommandBuffers(b.device, b.cmdPool, 1, &f.cb)
	}
	if b.cmdPool != nil {
		C.vkDestroyCommandPool(b.device, b.cmdPool, nil)
	}
}

type fenceEntry struct {
	cb    C.VkCommandBuffer
	fence C.VkFence
}

// cmdBuffer is the Vulkan CommandBuffer: a primary command buffer that has the
// bindless heap bound and the shared pipeline layout for push constants.
type cmdBuffer struct {
	b  *Backend
	cb C.VkCommandBuffer
	// inRenderPass is set between BeginRenderPass and EndRenderPass, where no image
	// can be initialized: a barrier inside dynamic rendering may not change layouts.
	inRenderPass bool
}

// Begin allocates a transient command buffer, begins recording, and binds the
// global bindless descriptor set.
func (b *Backend) Begin() gpu.CommandBuffer {
	var cb C.VkCommandBuffer
	if r := C.vkbAllocCmd(b.device, b.cmdPool, &cb); r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: allocate command buffer failed (%d)", int(r)))
	}
	if r := C.vkbBeginCmd(cb); r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: begin command buffer failed (%d)", int(r)))
	}
	C.vkbBindHeap(cb, b.pipelineLayout, b.descSet)
	c := &cmdBuffer{b: b, cb: cb}
	c.initializeImages()
	return c
}

// Submit ends recording and submits, returning a fence for the completion.
func (b *Backend) Submit(cmd gpu.CommandBuffer) gpu.Fence {
	c := cmd.(*cmdBuffer)
	C.vkEndCommandBuffer(c.cb)
	var fence C.VkFence
	if r := C.vkbSubmit(b.device, b.queue, c.cb, &fence); r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: submit failed (%d)", int(r)))
	}
	h := b.nextID.Add(1)
	b.fences[h] = fenceEntry{cb: c.cb, fence: fence}
	return gpu.Fence{H: gpu.Handle(h)}
}

// Wait blocks until the fence signals, then frees the command buffer + fence.
func (b *Backend) Wait(f gpu.Fence) {
	e, ok := b.fences[uint64(f.H)]
	if !ok {
		return
	}
	C.vkWaitForFences(b.device, 1, &e.fence, C.VK_TRUE, C.UINT64_MAX)
	C.vkDestroyFence(b.device, e.fence, nil)
	C.vkFreeCommandBuffers(b.device, b.cmdPool, 1, &e.cb)
	delete(b.fences, uint64(f.H))
}

// WaitIdle blocks until the device is idle.
func (b *Backend) WaitIdle() { C.vkDeviceWaitIdle(b.device) }

// --- CommandBuffer ---

func (b *Backend) tex(t gpu.Texture) *textureEntry { return b.textures[uint64(t.H)] }

func (b *Backend) bufRaw(buf gpu.Buffer) C.VkBuffer { return b.buffers[uint64(buf.H)].buf }

func aspectOf(e *textureEntry) C.VkImageAspectFlags {
	if e.isDepthFormat {
		return C.VK_IMAGE_ASPECT_DEPTH_BIT
	}
	return C.VK_IMAGE_ASPECT_COLOR_BIT
}

// initializeImages moves every texture still in UNDEFINED to GENERAL, where it then
// stays for its whole life. It runs before anything that could be a texture's first use
// outside a render pass — a pass, a dispatch, a copy, a barrier — so a texture created
// while this command buffer records is ready before it is used.
func (c *cmdBuffer) initializeImages() {
	if c.inRenderPass {
		return
	}
	for _, h := range c.b.uninitialized {
		if e, ok := c.b.textures[h]; ok {
			C.vkbInitializeImage(c.cb, e.img, aspectOf(e))
		}
	}
	c.b.uninitialized = c.b.uninitialized[:0]
}

// maxColorAttachments bounds a single render pass's color targets (today's largest
// user is the G-buffer fill: albedo/normal/metal-rough + emissive-to-color).
const maxColorAttachments = 4

// BeginRenderPass starts rendering into rt. It orders nothing: a pass using an image
// an earlier command wrote needs a Barrier before it.
func (c *cmdBuffer) BeginRenderPass(rt gpu.RenderTargets) {
	if len(rt.Color) > maxColorAttachments {
		panic("vulkan: BeginRenderPass: too many color attachments")
	}
	c.initializeImages()
	var w, h uint32
	var views [maxColorAttachments]C.VkImageView
	var loads [maxColorAttachments]C.int
	var clears [maxColorAttachments * 4]C.float
	for i, ca := range rt.Color {
		e := c.b.tex(ca.Texture)
		w, h = e.width, e.height
		views[i] = e.view
		if ca.Load == gpu.LoadClear {
			loads[i] = 1
		}
		clears[i*4+0] = C.float(ca.Clear[0])
		clears[i*4+1] = C.float(ca.Clear[1])
		clears[i*4+2] = C.float(ca.Clear[2])
		clears[i*4+3] = C.float(ca.Clear[3])
	}
	var depthView C.VkImageView
	hasDepth := C.int(0)
	depthClear := C.int(0)
	var dclear float32
	if rt.Depth != nil {
		e := c.b.tex(rt.Depth.Texture)
		w, h = e.width, e.height
		depthView = e.view
		hasDepth = 1
		dclear = rt.Depth.Clear
		if rt.Depth.Load == gpu.LoadClear {
			depthClear = 1
		}
		// A read-only depth attachment needs nothing of its own: in GENERAL the pass may
		// depth-test against the image while also sampling it.
	}
	var viewsPtr *C.VkImageView
	var loadsPtr *C.int
	var clearsPtr *C.float
	if n := len(rt.Color); n > 0 {
		viewsPtr, loadsPtr, clearsPtr = &views[0], &loads[0], &clears[0]
	}
	C.vkbBeginRendering(c.cb, C.uint32_t(w), C.uint32_t(h),
		viewsPtr, loadsPtr, clearsPtr, C.uint32_t(len(rt.Color)),
		hasDepth, depthView, depthClear, C.float(dclear))
	c.inRenderPass = true
}

func (c *cmdBuffer) EndRenderPass() {
	C.vkCmdEndRendering(c.cb)
	c.inRenderPass = false
}

// SetPipeline binds a pipeline to its own bind point (graphics or compute),
// recorded when the pipeline was created.
func (c *cmdBuffer) SetPipeline(p gpu.Pipeline) {
	e := c.b.pipelines[uint64(p.H)]
	C.vkCmdBindPipeline(c.cb, e.bindPoint, e.pipe)
	if e.bindPoint == C.VK_PIPELINE_BIND_POINT_GRAPHICS {
		// Depth bias is always a dynamic pipeline state (see bridge.c); Vulkan
		// requires it be set at least once before a draw uses it, so bind a
		// no-bias default here and let SetDepthBias override it.
		C.vkCmdSetDepthBias(c.cb, 0, 0, 0)
	}
}

// push records the draw/dispatch data as push constants. Vulkan requires the size to
// be a multiple of 4, and the copy happens here so the caller's slice need not outlive
// the call.
func (c *cmdBuffer) push(data []byte) {
	if len(data) == 0 {
		return
	}
	size := len(data) &^ 3 // round down to a multiple of 4; Vulkan rejects the rest
	if size == 0 {
		return
	}
	C.vkbPush(c.cb, c.b.pipelineLayout, unsafe.Pointer(&data[0]), C.uint32_t(size))
}

func (c *cmdBuffer) SetViewport(x, y, width, height, minDepth, maxDepth float32) {
	vp := C.VkViewport{x: C.float(x), y: C.float(y), width: C.float(width), height: C.float(height),
		minDepth: C.float(minDepth), maxDepth: C.float(maxDepth)}
	C.vkCmdSetViewport(c.cb, 0, 1, &vp)
}

func (c *cmdBuffer) SetScissor(x, y, width, height int32) {
	sc := C.VkRect2D{offset: C.VkOffset2D{x: C.int32_t(x), y: C.int32_t(y)},
		extent: C.VkExtent2D{width: C.uint32_t(width), height: C.uint32_t(height)}}
	C.vkCmdSetScissor(c.cb, 0, 1, &sc)
}

func (c *cmdBuffer) SetDepthBias(bias, slope, clamp float32) {
	C.vkCmdSetDepthBias(c.cb, C.float(bias), C.float(clamp), C.float(slope))
}

func (c *cmdBuffer) Draw(data []byte, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	c.push(data)
	C.vkCmdDraw(c.cb, C.uint32_t(vertexCount), C.uint32_t(instanceCount), C.uint32_t(firstVertex), C.uint32_t(firstInstance))
}

// vkIndexType maps the RHI's index width to Vulkan's enum.
func vkIndexType(t gpu.IndexType) C.VkIndexType {
	if t == gpu.IndexUint16 {
		return C.VK_INDEX_TYPE_UINT16
	}
	return C.VK_INDEX_TYPE_UINT32
}

func (c *cmdBuffer) DrawIndexed(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, indexCount, instanceCount, firstIndex uint32, vertexOffset int32, firstInstance uint32) {
	c.push(data)
	C.vkCmdBindIndexBuffer(c.cb, c.b.bufRaw(indexBuf), 0, vkIndexType(indexType))
	C.vkCmdDrawIndexed(c.cb, C.uint32_t(indexCount), C.uint32_t(instanceCount), C.uint32_t(firstIndex), C.int32_t(vertexOffset), C.uint32_t(firstInstance))
}

func (c *cmdBuffer) DrawIndexedIndirect(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, args gpu.Buffer, argsOffset uint64, drawCount, stride uint32) {
	c.push(data)
	C.vkCmdBindIndexBuffer(c.cb, c.b.bufRaw(indexBuf), 0, vkIndexType(indexType))
	C.vkCmdDrawIndexedIndirect(c.cb, c.b.bufRaw(args), C.VkDeviceSize(argsOffset), C.uint32_t(drawCount), C.uint32_t(stride))
}

func (c *cmdBuffer) Dispatch(data []byte, x, y, z uint32) {
	c.initializeImages()
	c.push(data)
	C.vkCmdDispatch(c.cb, C.uint32_t(x), C.uint32_t(y), C.uint32_t(z))
}

func (c *cmdBuffer) DispatchIndirect(data []byte, args gpu.Buffer, offset uint64) {
	c.initializeImages()
	c.push(data)
	C.vkCmdDispatchIndirect(c.cb, c.b.bufRaw(args), C.VkDeviceSize(offset))
}

// Barrier is a global memory barrier. With every image in GENERAL it orders images as
// well as buffers, so it is the only ordering the gpu API needs.
func (c *cmdBuffer) Barrier(src, dst gpu.Stage, flags gpu.BarrierFlags) {
	c.initializeImages()
	ss, sa := stageAccess(src)
	ds, da := stageAccess(dst)
	C.vkbGlobalBarrier(c.cb, ss, sa, ds, da)
}

func (c *cmdBuffer) CopyBuffer(dst, src gpu.Buffer, dstOffset, srcOffset, size uint64) {
	region := C.VkBufferCopy{srcOffset: C.VkDeviceSize(srcOffset), dstOffset: C.VkDeviceSize(dstOffset), size: C.VkDeviceSize(size)}
	C.vkCmdCopyBuffer(c.cb, c.b.bufRaw(src), c.b.bufRaw(dst), 1, &region)
}

func (c *cmdBuffer) CopyBufferToTexture(dst gpu.Texture, mip, layer uint32, src gpu.Buffer, srcOffset uint64) {
	c.initializeImages()
	e := c.b.tex(dst)
	w, h, d := mipExtent(e, mip)
	C.vkbCopyBufferToImage(c.cb, c.b.bufRaw(src), C.uint64_t(srcOffset), e.img,
		C.uint32_t(w), C.uint32_t(h), C.uint32_t(d), C.uint32_t(mip), C.uint32_t(layer), aspectOf(e))
}

func (c *cmdBuffer) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, mip, layer uint32) {
	c.initializeImages()
	e := c.b.tex(src)
	w, h, d := mipExtent(e, mip)
	C.vkbCopyImageToBuffer(c.cb, e.img, c.b.bufRaw(dst),
		C.uint32_t(w), C.uint32_t(h), C.uint32_t(d), C.uint32_t(mip), C.uint32_t(layer), aspectOf(e))
}

// mipExtent returns a texture's extent at level: each level halves every dimension,
// floored at 1 (so a 4x1 image's level 2 is 1x1, not 1x0). Copying the base extent into
// a smaller level would overrun it. A 3D texture's copy covers its whole depth, as a
// 2D texture's covers one layer.
func mipExtent(e *textureEntry, level uint32) (width, height, depth uint32) {
	return max(e.width>>level, 1), max(e.height>>level, 1), max(e.depth>>level, 1)
}

// stageAccess maps an gpu.Stage bitmask to a coarse (stage, access) pair. Each stage
// carries both the reads and the writes it can make, so the same pair serves as a
// barrier's source and its destination: blending reads the color attachment, a depth
// test reads depth, and a vertex or fragment shader can write storage.
func stageAccess(s gpu.Stage) (C.VkPipelineStageFlags2, C.VkAccessFlags2) {
	var stage C.VkPipelineStageFlags2
	var access C.VkAccessFlags2
	if s == gpu.StageNone {
		return vkStageNone, vkAccessNone
	}
	if s == gpu.StageAll {
		return vkStageAll, vkAccessMemoryRead | vkAccessMemoryWrite
	}
	if s&gpu.StageIndirect != 0 {
		stage |= vkStageIndirect
		access |= vkAccessIndirectRead
	}
	if s&gpu.StageVertex != 0 {
		stage |= vkStageVertex
		access |= vkAccessShaderRead | vkAccessShaderWrite
	}
	if s&gpu.StageFragment != 0 {
		stage |= vkStageFragment
		access |= vkAccessShaderRead | vkAccessShaderWrite
	}
	if s&gpu.StageColorOutput != 0 {
		stage |= vkStageColor
		access |= vkAccessColorRead | vkAccessColorWrite
	}
	if s&gpu.StageDepth != 0 {
		stage |= vkStageEarlyDepth | vkStageLateDepth
		access |= vkAccessDepthRead | vkAccessDepthWrite
	}
	if s&gpu.StageCompute != 0 {
		stage |= vkStageCompute
		access |= vkAccessShaderRead | vkAccessShaderWrite
	}
	if s&gpu.StageTransfer != 0 {
		stage |= vkStageTransfer
		access |= vkAccessTransferRead | vkAccessTransferWrite
	}
	return stage, access
}
