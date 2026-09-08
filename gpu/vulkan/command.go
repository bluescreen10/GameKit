package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
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
	vkStageCopy       = C.vkbStageCopy()
	vkStageAll        = C.vkbStageAll()

	vkAccessNone          = C.vkbAccessNone()
	vkAccessIndirectRead  = C.vkbAccessIndirectRead()
	vkAccessShaderRead    = C.vkbAccessShaderRead()
	vkAccessShaderWrite   = C.vkbAccessShaderWrite()
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
	return &cmdBuffer{b: b, cb: cb}
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
	if e.depth {
		return C.VK_IMAGE_ASPECT_DEPTH_BIT
	}
	return C.VK_IMAGE_ASPECT_COLOR_BIT
}

// transition moves an image to newLayout and orders the new access against however
// the image was last used (e.lastStage/lastAccess), then records this access as the
// new "last use". The source scope comes from the tracked state rather than the
// caller so that cross-pass hazards — sampling a G-buffer the previous pass rendered,
// or blending onto a color target a previous pass wrote — are actually synchronized;
// a caller-supplied TOP_OF_PIPE would silently establish no dependency at all.
//
// The barrier is emitted even when the layout is unchanged: a write-after-write on the
// same layout (two render passes onto one color target) still needs ordering.
func (c *cmdBuffer) transition(e *textureEntry, newLayout C.VkImageLayout,
	dstStage C.VkPipelineStageFlags2, dstAccess C.VkAccessFlags2) {
	srcStage, srcAccess := e.lastStage, e.lastAccess
	if srcStage == 0 {
		srcStage = vkStageTop
	}
	C.vkbImageBarrier(c.cb, e.img, aspectOf(e), e.layout, newLayout, srcStage, srcAccess, dstStage, dstAccess)
	e.layout = newLayout
	e.lastStage, e.lastAccess = dstStage, dstAccess
}

// maxColorAttachments bounds a single render pass's color targets (today's largest
// user is the G-buffer fill: albedo/normal/metal-rough + emissive-to-color).
const maxColorAttachments = 4

func (c *cmdBuffer) BeginRenderPass(rt gpu.RenderTargets) {
	if len(rt.Color) > maxColorAttachments {
		panic("vulkan: BeginRenderPass: too many color attachments")
	}
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
		c.transition(e, C.VK_IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
			vkStageColor, vkAccessColorWrite)
	}
	var depthView C.VkImageView
	hasDepth := C.int(0)
	depthClear := C.int(0)
	var dclear float32
	depthLayout := C.VkImageLayout(C.VK_IMAGE_LAYOUT_DEPTH_ATTACHMENT_OPTIMAL)
	if rt.Depth != nil {
		e := c.b.tex(rt.Depth.Texture)
		w, h = e.width, e.height
		depthView = e.view
		hasDepth = 1
		dclear = rt.Depth.Clear
		if rt.Depth.Load == gpu.LoadClear {
			depthClear = 1
		}
		// A read-only depth attachment rests in DEPTH_STENCIL_READ_ONLY_OPTIMAL — the
		// same layout its bindless descriptor declares — so the pass may depth-test
		// against the image while also sampling it (deferred lighting does both).
		if rt.Depth.ReadOnly {
			depthLayout = sampledLayout(true)
			c.transition(e, depthLayout,
				vkStageEarlyDepth|vkStageLateDepth|vkStageFragment,
				vkAccessDepthRead|vkAccessShaderRead)
		} else {
			c.transition(e, depthLayout,
				vkStageEarlyDepth|vkStageLateDepth,
				vkAccessDepthWrite)
		}
	}
	var viewsPtr *C.VkImageView
	var loadsPtr *C.int
	var clearsPtr *C.float
	if n := len(rt.Color); n > 0 {
		viewsPtr, loadsPtr, clearsPtr = &views[0], &loads[0], &clears[0]
	}
	C.vkbBeginRendering(c.cb, C.uint32_t(w), C.uint32_t(h),
		viewsPtr, loadsPtr, clearsPtr, C.uint32_t(len(rt.Color)),
		hasDepth, depthView, depthClear, C.float(dclear), depthLayout)
}

func (c *cmdBuffer) EndRenderPass() { C.vkCmdEndRendering(c.cb) }

// SetPipeline binds a pipeline to its own bind point (graphics or compute),
// recorded when the pipeline was created.
func (c *cmdBuffer) SetPipeline(p gpu.Pipeline) {
	e := c.b.pipelines[uint64(p.H)]
	C.vkCmdBindPipeline(c.cb, e.bindPoint, e.pipe)
}

func (c *cmdBuffer) Root(addr uint64) { C.vkbPush(c.cb, c.b.pipelineLayout, C.uint64_t(addr)) }

func (c *cmdBuffer) Viewport(x, y, width, height, minDepth, maxDepth float32) {
	vp := C.VkViewport{x: C.float(x), y: C.float(y), width: C.float(width), height: C.float(height),
		minDepth: C.float(minDepth), maxDepth: C.float(maxDepth)}
	C.vkCmdSetViewport(c.cb, 0, 1, &vp)
}

func (c *cmdBuffer) Scissor(x, y, width, height int32) {
	sc := C.VkRect2D{offset: C.VkOffset2D{x: C.int32_t(x), y: C.int32_t(y)},
		extent: C.VkExtent2D{width: C.uint32_t(width), height: C.uint32_t(height)}}
	C.vkCmdSetScissor(c.cb, 0, 1, &sc)
}

func (c *cmdBuffer) Draw(vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	C.vkCmdDraw(c.cb, C.uint32_t(vertexCount), C.uint32_t(instanceCount), C.uint32_t(firstVertex), C.uint32_t(firstInstance))
}

func (c *cmdBuffer) DrawIndexed(indexBuf gpu.Buffer, indexCount, instanceCount, firstIndex uint32, vertexOffset int32, firstInstance uint32) {
	C.vkCmdBindIndexBuffer(c.cb, c.b.bufRaw(indexBuf), 0, C.VK_INDEX_TYPE_UINT32)
	C.vkCmdDrawIndexed(c.cb, C.uint32_t(indexCount), C.uint32_t(instanceCount), C.uint32_t(firstIndex), C.int32_t(vertexOffset), C.uint32_t(firstInstance))
}

func (c *cmdBuffer) DrawIndexedIndirect(indexBuf, args gpu.Buffer, argsOffset uint64, drawCount, stride uint32) {
	C.vkCmdBindIndexBuffer(c.cb, c.b.bufRaw(indexBuf), 0, C.VK_INDEX_TYPE_UINT32)
	C.vkCmdDrawIndexedIndirect(c.cb, c.b.bufRaw(args), C.VkDeviceSize(argsOffset), C.uint32_t(drawCount), C.uint32_t(stride))
}

func (c *cmdBuffer) Dispatch(x, y, z uint32) {
	C.vkCmdDispatch(c.cb, C.uint32_t(x), C.uint32_t(y), C.uint32_t(z))
}

func (c *cmdBuffer) DispatchIndirect(args gpu.Buffer, offset uint64) {
	C.vkCmdDispatchIndirect(c.cb, c.b.bufRaw(args), C.VkDeviceSize(offset))
}

func (c *cmdBuffer) Barrier(src, dst gpu.Stage, flags gpu.BarrierFlags) {
	ss, sa := stageAccess(src)
	ds, da := stageAccess(dst)
	C.vkbGlobalBarrier(c.cb, ss, sa, ds, da)
}

func (c *cmdBuffer) PrepareSampled(t gpu.Texture, at gpu.Stage) {
	e := c.b.tex(t)
	ds, _ := stageAccess(at)
	if ds == 0 {
		ds = vkStageFragment
	}
	c.transition(e, sampledLayout(e.depth), ds, vkAccessShaderRead)
}

func (c *cmdBuffer) CopyBuffer(dst, src gpu.Buffer, dstOffset, srcOffset, size uint64) {
	region := C.VkBufferCopy{srcOffset: C.VkDeviceSize(srcOffset), dstOffset: C.VkDeviceSize(dstOffset), size: C.VkDeviceSize(size)}
	C.vkCmdCopyBuffer(c.cb, c.b.bufRaw(src), c.b.bufRaw(dst), 1, &region)
}

func (c *cmdBuffer) CopyBufferToTexture(dst gpu.Texture, mip, layer uint32, src gpu.Buffer, srcOffset uint64) {
	e := c.b.tex(dst)
	c.transition(e, C.VK_IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL,
		vkStageCopy, vkAccessTransferWrite)
	// A mip level is half the size of the one above it, floored at 1 — copying the
	// base extent into a smaller level would overrun it.
	w, h := mipExtent(e.width, e.height, mip)
	C.vkbCopyBufferToImage(c.cb, c.b.bufRaw(src), C.uint64_t(srcOffset), e.img,
		C.uint32_t(w), C.uint32_t(h), C.uint32_t(mip), C.uint32_t(layer), aspectOf(e))
}

func (c *cmdBuffer) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, mip, layer uint32) {
	e := c.b.tex(src)
	c.transition(e, C.VK_IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL,
		vkStageCopy, vkAccessTransferRead)
	w, h := mipExtent(e.width, e.height, mip)
	C.vkbCopyImageToBuffer(c.cb, e.img, c.b.bufRaw(dst),
		C.uint32_t(w), C.uint32_t(h), C.uint32_t(mip), C.uint32_t(layer), aspectOf(e))
}

// mipExtent returns level's dimensions for a base-size image: each level halves,
// floored at 1 (so a 4x1 image's level 2 is 1x1, not 1x0).
func mipExtent(w, h, level uint32) (uint32, uint32) {
	w >>= level
	h >>= level
	return max(w, 1), max(h, 1)
}

// stageAccess maps an gpu.Stage bitmask to a coarse (stage, access) pair.
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
		access |= vkAccessShaderRead
	}
	if s&gpu.StageFragment != 0 {
		stage |= vkStageFragment
		access |= vkAccessShaderRead
	}
	if s&gpu.StageColorOutput != 0 {
		stage |= vkStageColor
		access |= vkAccessColorWrite
	}
	if s&gpu.StageDepth != 0 {
		stage |= vkStageEarlyDepth | vkStageLateDepth
		access |= vkAccessDepthWrite
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
