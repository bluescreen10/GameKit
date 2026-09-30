package test

import "github.com/bluescreen10/gamekit/gpu"

// commandBuffer implements gpu.CommandBuffer (asserted below). Draws, dispatches and
// pipeline/render-pass state are no-ops — see the package doc for why. Only the copy
// methods and GPU-timestamp bookkeeping do real work.
var _ gpu.CommandBuffer = (*commandBuffer)(nil)

type commandBuffer struct {
	backend *Backend
}

func (c *commandBuffer) BeginRenderPass(gpu.RenderTargets) {}
func (c *commandBuffer) EndRenderPass()                    {}

func (c *commandBuffer) SetPipeline(gpu.Pipeline) {}

func (c *commandBuffer) SetViewport(x, y, width, height, minDepth, maxDepth float32) {}
func (c *commandBuffer) SetScissor(x, y, width, height int32)                        {}
func (c *commandBuffer) SetDepthBias(bias, slope, clamp float32)                     {}

func (c *commandBuffer) Draw(data []byte, vertexCount, instanceCount, firstVertex, firstInstance uint32) {
}

func (c *commandBuffer) DrawIndexed(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, indexCount, instanceCount, firstIndex uint32, vertexOffset int32, firstInstance uint32) {
}

func (c *commandBuffer) DrawIndexedIndirect(data []byte, indexBuf gpu.Buffer, indexType gpu.IndexType, args gpu.Buffer, argsOffset uint64, drawCount, stride uint32) {
}

func (c *commandBuffer) Dispatch(data []byte, x, y, z uint32) {}

func (c *commandBuffer) DispatchIndirect(data []byte, args gpu.Buffer, offset uint64) {}

func (c *commandBuffer) Barrier(src, dst gpu.Stage, flags gpu.BarrierFlags) {}

func (c *commandBuffer) ResetTimestamps(pool gpu.QueryPool, count uint32) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	slots, ok := c.backend.timestamps[pool.H]
	if !ok {
		return
	}
	for i := range slots {
		if uint32(i) < count {
			slots[i] = 0
		}
	}
}

// WriteTimestamp advances a fake, backend-wide tick counter so consecutive writes
// (e.g. one at the start and one at the end of a frame) always read back increasing —
// enough for any real code's "did time pass between these two writes" check.
func (c *commandBuffer) WriteTimestamp(pool gpu.QueryPool, index uint32, at gpu.Stage) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	c.backend.nextTick++
	if slots, ok := c.backend.timestamps[pool.H]; ok && index < uint32(len(slots)) {
		slots[index] = c.backend.nextTick
	}
}

func (c *commandBuffer) CopyBuffer(dst, src gpu.Buffer, dstOffset, srcOffset, size uint64) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	d, ok := c.backend.buffers[dst.H]
	if !ok {
		panic("gpu/test: CopyBuffer: dst is not a live buffer")
	}
	s, ok := c.backend.buffers[src.H]
	if !ok {
		panic("gpu/test: CopyBuffer: src is not a live buffer")
	}
	copy(d.data[dstOffset:dstOffset+size], s.data[srcOffset:srcOffset+size])
}

func (c *commandBuffer) CopyBufferToTexture(dst gpu.Texture, mip, layer uint32, src gpu.Buffer, srcOffset uint64) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	t, ok := c.backend.textures[dst.H]
	if !ok {
		panic("gpu/test: CopyBufferToTexture: dst is not a live texture")
	}
	s, ok := c.backend.buffers[src.H]
	if !ok {
		panic("gpu/test: CopyBufferToTexture: src is not a live buffer")
	}
	start, end := t.mipRange(mip)
	copy(t.data[start:end], s.data[srcOffset:srcOffset+(end-start)])
}

func (c *commandBuffer) CopyTextureToBuffer(dst gpu.Buffer, src gpu.Texture, mip, layer uint32) {
	c.backend.mu.Lock()
	defer c.backend.mu.Unlock()
	d, ok := c.backend.buffers[dst.H]
	if !ok {
		panic("gpu/test: CopyTextureToBuffer: dst is not a live buffer")
	}
	t, ok := c.backend.textures[src.H]
	if !ok {
		panic("gpu/test: CopyTextureToBuffer: src is not a live texture")
	}
	start, end := t.mipRange(mip)
	copy(d.data[:end-start], t.data[start:end])
}

// mipRange is the byte range mip m occupies within t.data. Array layers and cube
// faces are folded into it rather than addressed individually — see texture's doc
// comment — so layer is not part of the range.
func (t *texture) mipRange(mip uint32) (start, end uint64) {
	return t.offset[mip], t.offset[mip+1]
}
