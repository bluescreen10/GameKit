package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
)

// bufferEntry is the backend-side record for a Buffer handle.
type bufferEntry struct {
	buf C.VkBuffer
	mem C.VkDeviceMemory
}

// Alloc creates a dedicated buffer + memory. MemoryHost is CPU-mapped and
// coherent (Buffer.Ptr set); MemoryDevice is GPU-only (Ptr nil). Both expose a
// device address (Buffer.Addr). The engine suballocates within an Alloc via
// offsets on Addr — the gpu itself does no suballocation.
func (b *Backend) Alloc(size uint64, mem gpu.MemoryType, label string) gpu.Buffer {
	var (
		buf  C.VkBuffer
		dmem C.VkDeviceMemory
		ptr  unsafe.Pointer
		addr C.VkDeviceAddress
	)
	host := C.int(0)
	if mem == gpu.MemoryHost {
		host = 1
	}
	r := C.vkbAllocBuffer(b.device, b.physicalDevice, C.VkDeviceSize(size), host, &buf, &dmem, &ptr, &addr)
	if r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: Alloc(%d bytes, %q) failed (%d)", size, label, int(r)))
	}

	h := b.nextID.Add(1)
	b.buffers[h] = bufferEntry{buf: buf, mem: dmem}
	return gpu.Buffer{Addr: uint64(addr), Ptr: ptr, Size: size, H: gpu.Handle(h)}
}

// Free destroys the buffer and releases its memory. No-op on a stale handle.
func (b *Backend) Free(buf gpu.Buffer) {
	e, ok := b.buffers[uint64(buf.H)]
	if !ok {
		return
	}
	C.vkbFreeBuffer(b.device, e.buf, e.mem)
	delete(b.buffers, uint64(buf.H))
}

// destroyAllBuffers releases every live buffer (shutdown).
func (b *Backend) destroyAllBuffers() {
	for k, e := range b.buffers {
		C.vkbFreeBuffer(b.device, e.buf, e.mem)
		delete(b.buffers, k)
	}
}
