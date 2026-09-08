package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"fmt"

	"github.com/bluescreen10/GameKit/gpu"
)

// Desired heap capacities (clamped to device update-after-bind limits at init).
const (
	wantSampledImages = 16384
	wantStorageImages = 4096
	wantSamplers      = 512
)

// initBindless creates the global descriptor heap + shared pipeline layout.
// TODO: make wantedSampledImages, wantStorageImages, and wantSamplers configurable
func (b *Backend) initBindless() error {
	var cs, cst, csa C.uint32_t
	r := C.vkbCreateBindlessHeap(b.device, b.physicalDevice,
		wantSampledImages, wantStorageImages, wantSamplers,
		&b.setLayout, &b.descPool, &b.descSet, &b.pipelineLayout,
		&cs, &cst, &csa)
	if r != C.VK_SUCCESS {
		return fmt.Errorf("vulkan: bindless heap creation failed (%d)", int(r))
	}
	b.capSampled, b.capStorage, b.capSampler = uint32(cs), uint32(cst), uint32(csa)
	b.samplers = map[uint64]C.VkSampler{}
	return nil
}

func (b *Backend) destroyBindless() {
	for _, s := range b.samplers {
		C.vkDestroySampler(b.device, s, nil)
	}
	if b.pipelineLayout != nil {
		C.vkDestroyPipelineLayout(b.device, b.pipelineLayout, nil)
	}
	if b.descPool != nil {
		C.vkDestroyDescriptorPool(b.device, b.descPool, nil) // frees the set too
	}
	if b.setLayout != nil {
		C.vkDestroyDescriptorSetLayout(b.device, b.setLayout, nil)
	}
}

func filter(linear bool) C.VkFilter {
	if linear {
		return C.VK_FILTER_LINEAR
	}
	return C.VK_FILTER_NEAREST
}

func mipmapMode(linear bool) C.VkSamplerMipmapMode {
	if linear {
		return C.VK_SAMPLER_MIPMAP_MODE_LINEAR
	}
	return C.VK_SAMPLER_MIPMAP_MODE_NEAREST
}

func addressMode(m gpu.AddressMode) C.VkSamplerAddressMode {
	switch m {
	case gpu.AddressRepeat:
		return C.VK_SAMPLER_ADDRESS_MODE_REPEAT
	case gpu.AddressMirror:
		return C.VK_SAMPLER_ADDRESS_MODE_MIRRORED_REPEAT
	default:
		return C.VK_SAMPLER_ADDRESS_MODE_CLAMP_TO_EDGE
	}
}

// CreateSampler registers a sampler into the bindless heap and returns its index.
func (b *Backend) CreateSampler(d gpu.SamplerDescriptor) gpu.Sampler {
	compareEnable := C.int(0)
	if d.Compare != gpu.CompareNever {
		compareEnable = 1
	}
	// Clamp to what the device supports: a sampler asking for more anisotropy than
	// maxSamplerAnisotropy is invalid, and callers shouldn't have to query first.
	aniso := float32(d.MaxAnisotropy)
	if aniso > b.maxAnisotropy {
		aniso = b.maxAnisotropy
	}
	var s C.VkSampler
	r := C.vkbCreateSampler(b.device,
		filter(d.MagLinear), filter(d.MinLinear), mipmapMode(d.MipLinear),
		addressMode(d.AddressU), addressMode(d.AddressV), addressMode(d.AddressW),
		compareEnable, compareOp(d.Compare), C.float(aniso), &s)
	if r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: CreateSampler failed (%d)", int(r)))
	}
	index := b.samplerNext
	b.samplerNext++
	C.vkbWriteSampler(b.device, b.descSet, C.uint32_t(index), s)

	h := b.nextID.Add(1)
	b.samplers[h] = s
	return gpu.Sampler{Index: index, H: gpu.Handle(h)}
}

// DestroySampler releases a sampler (its heap slot is not reclaimed yet).
func (b *Backend) DestroySampler(s gpu.Sampler) {
	vs, ok := b.samplers[uint64(s.H)]
	if !ok {
		return
	}
	C.vkDestroySampler(b.device, vs, nil)
	delete(b.samplers, uint64(s.H))
}

// compareOp maps an gpu compare op to Vulkan.
func compareOp(op gpu.CompareOp) C.VkCompareOp {
	switch op {
	case gpu.CompareLess:
		return C.VK_COMPARE_OP_LESS
	case gpu.CompareEqual:
		return C.VK_COMPARE_OP_EQUAL
	case gpu.CompareLessEqual:
		return C.VK_COMPARE_OP_LESS_OR_EQUAL
	case gpu.CompareGreater:
		return C.VK_COMPARE_OP_GREATER
	case gpu.CompareNotEqual:
		return C.VK_COMPARE_OP_NOT_EQUAL
	case gpu.CompareGreaterEqual:
		return C.VK_COMPARE_OP_GREATER_OR_EQUAL
	case gpu.CompareAlways:
		return C.VK_COMPARE_OP_ALWAYS
	default:
		return C.VK_COMPARE_OP_NEVER
	}
}

// HeapCapacities returns the clamped bindless array sizes (diagnostics).
func (b *Backend) HeapCapacities() (sampled, storage, sampler uint32) {
	return b.capSampled, b.capStorage, b.capSampler
}
