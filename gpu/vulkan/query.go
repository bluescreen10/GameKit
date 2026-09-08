package vulkan

/*
#include "bridge.h"
*/
import "C"

import "github.com/bluescreen10/gamekit/gpu"

// CreateTimestampPool allocates a pool of count timestamp query slots.
func (b *Backend) CreateTimestampPool(count uint32) gpu.QueryPool {
	pool := C.vkbCreateTimestampPool(b.device, C.uint32_t(count))
	h := b.nextID.Add(1)
	if b.queryPools == nil {
		b.queryPools = map[uint64]C.VkQueryPool{}
	}
	b.queryPools[h] = pool
	return gpu.QueryPool{H: gpu.Handle(h)}
}

// DestroyTimestampPool releases a timestamp pool.
func (b *Backend) DestroyTimestampPool(p gpu.QueryPool) {
	if pool, ok := b.queryPools[uint64(p.H)]; ok {
		C.vkDestroyQueryPool(b.device, pool, nil)
		delete(b.queryPools, uint64(p.H))
	}
}

// ReadTimestamps blocks until the pool's count results are available and returns
// them as raw ticks (convert deltas with TimestampPeriod). Returns nil if the pool
// has no valid results yet.
func (b *Backend) ReadTimestamps(p gpu.QueryPool, count uint32) []uint64 {
	pool, ok := b.queryPools[uint64(p.H)]
	if !ok {
		return nil
	}
	out := make([]uint64, count)
	if r := C.vkbGetTimestamps(b.device, pool, C.uint32_t(count), (*C.uint64_t)(&out[0])); r != C.VK_SUCCESS {
		return nil
	}
	return out
}

// TimestampPeriod returns nanoseconds per timestamp tick for this device.
func (b *Backend) TimestampPeriod() float64 {
	return float64(C.vkbTimestampPeriod(b.physicalDevice))
}

// TimestampValidBits returns how many bits of the graphics/compute queue's
// timestamps are meaningful (0 = timestamps unsupported on that queue).
func (b *Backend) TimestampValidBits() uint32 {
	return uint32(C.vkbTimestampValidBits(b.physicalDevice, C.uint32_t(b.queueFamily)))
}

func (c *cmdBuffer) ResetTimestamps(p gpu.QueryPool, count uint32) {
	if pool, ok := c.b.queryPools[uint64(p.H)]; ok {
		C.vkbCmdResetQueryPool(c.cb, pool, C.uint32_t(count))
	}
}

func (c *cmdBuffer) WriteTimestamp(p gpu.QueryPool, index uint32, at gpu.Stage) {
	pool, ok := c.b.queryPools[uint64(p.H)]
	if !ok {
		return
	}
	stage, _ := stageAccess(at)
	if stage == 0 {
		// StageNone → the earliest point, for a frame-start timestamp.
		stage = vkStageTop
	}
	C.vkbCmdWriteTimestamp(c.cb, stage, pool, C.uint32_t(index))
}
