package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"fmt"

	"github.com/bluescreen10/gamekit/gpu"
)

type swapchainState struct {
	surface  C.VkSurfaceKHR
	swap     C.VkSwapchainKHR
	format   C.VkFormat
	gpuFmt   gpu.Format
	w, h     uint32
	images   []gpu.Texture // one textureEntry per image (owned=false)
	acquire  []C.VkSemaphore
	rendered []C.VkSemaphore
	frame    uint32 // rotating sync index
	curImage uint32 // last acquired image index
}

func gpuFormatOf(f C.VkFormat) gpu.Format {
	if f == C.VK_FORMAT_R8G8B8A8_UNORM {
		return gpu.FormatRGBA8Unorm
	}
	return gpu.FormatBGRA8Unorm
}

// CreateSwapchain wraps a platform VkSurfaceKHR in a swapchain, surfacing
// backbuffers as render-target Textures.
func (b *Backend) CreateSwapchain(surface uintptr, width, height uint32) gpu.Swapchain {
	s := &swapchainState{surface: C.vkbSurfaceFromHandle(C.uint64_t(surface))}
	b.buildSwapchain(s, width, height, nil)

	h := b.nextID.Add(1)
	b.swapchains[h] = s
	return gpu.Swapchain{H: gpu.Handle(h)}
}

func (b *Backend) buildSwapchain(s *swapchainState, width, height uint32, old C.VkSwapchainKHR) {
	var swap C.VkSwapchainKHR
	var cfmt C.VkFormat
	var w, h C.uint32_t
	if r := C.vkbCreateSwapchain(b.physicalDevice, b.device, s.surface, C.uint32_t(width), C.uint32_t(height), old,
		&swap, &cfmt, &w, &h); r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: swapchain creation failed (%d)", int(r)))
	}
	s.swap = swap
	s.format = cfmt
	s.gpuFmt = gpuFormatOf(cfmt)
	s.w, s.h = uint32(w), uint32(h)

	n := uint32(C.vkbSwapchainImageCount(b.device, swap))
	imgs := make([]C.VkImage, n)
	C.vkbSwapchainImages(b.device, swap, C.uint32_t(n), &imgs[0])

	s.images = make([]gpu.Texture, n)
	s.acquire = make([]C.VkSemaphore, n)
	s.rendered = make([]C.VkSemaphore, n)
	for i := uint32(0); i < n; i++ {
		var view C.VkImageView
		C.vkbSwapImageView(b.device, imgs[i], cfmt, &view)
		hh := b.nextID.Add(1)
		b.textures[hh] = &textureEntry{
			img: imgs[i], view: view, format: cfmt,
			width: s.w, height: s.h, layout: C.VK_IMAGE_LAYOUT_UNDEFINED, owned: false,
		}
		s.images[i] = gpu.Texture{H: gpu.Handle(hh)}
		C.vkbCreateSemaphore(b.device, &s.acquire[i])
		C.vkbCreateSemaphore(b.device, &s.rendered[i])
	}
}

// AcquireNext acquires the next backbuffer and returns it as a render-target
// Texture. The returned Fence is unused (present sync is via internal semaphores;
// CPU throttling is via WaitIdle for now).
func (b *Backend) AcquireNext(sc gpu.Swapchain) (gpu.Texture, gpu.Fence) {
	s := b.swapchains[uint64(sc.H)]
	s.frame = (s.frame + 1) % uint32(len(s.acquire))
	var idx C.uint32_t
	C.vkbAcquire(b.device, s.swap, s.acquire[s.frame], &idx)
	s.curImage = uint32(idx)
	b.activeSwap = s
	// Fresh acquire: the image's prior contents are undefined for our purposes, and
	// last frame's producer is no longer the hazard to order against (the acquire
	// semaphore covers that), so clear the tracked access too.
	e := b.tex(s.images[s.curImage])
	e.layout = C.VK_IMAGE_LAYOUT_UNDEFINED
	e.lastStage, e.lastAccess = 0, 0
	return s.images[s.curImage], gpu.Fence{}
}

// Present ends and submits the recorded command list (waiting on the acquire
// semaphore, signalling render-done) and presents the backbuffer. The caller
// must have recorded rendering into the Texture returned by AcquireNext.
func (b *Backend) Present(sc gpu.Swapchain, cmd gpu.CommandBuffer) {
	s := b.swapchains[uint64(sc.H)]
	c := cmd.(*cmdBuffer)

	// Transition the backbuffer to PRESENT_SRC before ending the buffer.
	e := b.tex(s.images[s.curImage])
	C.vkbPresentBarrier(c.cb, e.img, e.layout)
	e.layout = C.VK_IMAGE_LAYOUT_PRESENT_SRC_KHR
	C.vkEndCommandBuffer(c.cb)

	var noFence C.VkFence
	C.vkbSubmitPresent(b.queue, c.cb, s.swap, C.uint32_t(s.curImage),
		s.acquire[s.frame], s.rendered[s.frame], noFence)

	// Simple throttle (per-frame fences / frames-in-flight come later).
	C.vkQueueWaitIdle(b.queue)
	C.vkFreeCommandBuffers(b.device, b.cmdPool, 1, &c.cb)
	b.activeSwap = nil
}

// SwapchainFormat returns the swapchain's color format.
func (b *Backend) SwapchainFormat(sc gpu.Swapchain) gpu.Format {
	return b.swapchains[uint64(sc.H)].gpuFmt
}

// SwapchainSize returns the swapchain's actual backbuffer extent (which may
// differ from a window's framebuffer size on HiDPI). Use it for the viewport.
func (b *Backend) SwapchainSize(sc gpu.Swapchain) (width, height uint32) {
	s := b.swapchains[uint64(sc.H)]
	return s.w, s.h
}

// ResizeSwapchain recreates the swapchain at a new size.
func (b *Backend) ResizeSwapchain(sc gpu.Swapchain, width, height uint32) {
	s := b.swapchains[uint64(sc.H)]
	C.vkDeviceWaitIdle(b.device)
	b.destroySwapchainResources(s)
	b.buildSwapchain(s, width, height, s.swap)
}

func (b *Backend) destroySwapchainResources(s *swapchainState) {
	for i := range s.images {
		e := b.tex(s.images[i])
		if e != nil {
			C.vkDestroyImageView(b.device, e.view, nil)
			delete(b.textures, uint64(s.images[i].H))
		}
		C.vkDestroySemaphore(b.device, s.acquire[i], nil)
		C.vkDestroySemaphore(b.device, s.rendered[i], nil)
	}
	C.vkDestroySwapchainKHR(b.device, s.swap, nil)
}

func (b *Backend) destroyAllSwapchains() {
	for _, s := range b.swapchains {
		b.destroySwapchainResources(s)
		C.vkDestroySurfaceKHR(b.instance, s.surface, nil)
	}
	b.swapchains = map[uint64]*swapchainState{}
}
