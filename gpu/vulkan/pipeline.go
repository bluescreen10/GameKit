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

func topology(t gpu.Topology) C.VkPrimitiveTopology {
	switch t {
	case gpu.TopologyTriangleStrip:
		return C.VK_PRIMITIVE_TOPOLOGY_TRIANGLE_STRIP
	case gpu.TopologyLines:
		return C.VK_PRIMITIVE_TOPOLOGY_LINE_LIST
	case gpu.TopologyPoints:
		return C.VK_PRIMITIVE_TOPOLOGY_POINT_LIST
	default:
		return C.VK_PRIMITIVE_TOPOLOGY_TRIANGLE_LIST
	}
}

func cullMode(m gpu.CullMode) C.VkCullModeFlags {
	switch m {
	case gpu.CullBack:
		return C.VK_CULL_MODE_BACK_BIT
	case gpu.CullFront:
		return C.VK_CULL_MODE_FRONT_BIT
	default:
		return C.VK_CULL_MODE_NONE
	}
}

// ComputePipeline compiles a compute pipeline from SPIR-V, sharing the global
// bindless pipeline layout.
func (b *Backend) CreateComputePipeline(desc gpu.ComputePipelineDescriptor) gpu.Pipeline {
	if desc.Entry == "" {
		desc.Entry = "main"
	}
	cEntry := C.CString(desc.Entry)
	defer C.free(unsafe.Pointer(cEntry))
	var p C.VkPipeline
	r := C.vkbCreateComputePipeline(b.device, b.pipelineLayout,
		unsafe.Pointer(&desc.Shader[0]), C.size_t(len(desc.Shader)), cEntry, &p)
	if r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q) failed (%d)", desc.Label, int(r)))
	}
	return b.registerPipeline(p, C.VK_PIPELINE_BIND_POINT_COMPUTE)
}

// GraphicsPipeline compiles a graphics pipeline (dynamic rendering), sharing the
// global bindless pipeline layout.
func (b *Backend) CreateGraphicsPipeline(d gpu.PipelineDescriptor) gpu.Pipeline {
	entry := d.VertexEntry
	if entry == "" {
		entry = "main"
	}
	cEntry := C.CString(entry)
	defer C.free(unsafe.Pointer(cEntry))

	colorFmts := make([]C.VkFormat, len(d.ColorFormats))
	for i, f := range d.ColorFormats {
		colorFmts[i] = vkFormat(f)
	}
	var colorPtr *C.VkFormat
	if len(colorFmts) > 0 {
		colorPtr = &colorFmts[0]
	}
	depthTest := C.int(0)
	if d.DepthTest {
		depthTest = 1
	}
	depthWrite := C.int(0)
	if d.DepthWrite {
		depthWrite = 1
	}
	samples := d.Samples
	if samples == 0 {
		samples = 1
	}
	// blendMode: 0 opaque, 1 src-alpha over, 2 additive (from the first target's
	// color op destination factor).
	blendMode := C.int(0)
	if len(d.Blend) > 0 && d.Blend[0].Enable {
		if d.Blend[0].ColorOp.Dst == gpu.BlendOne {
			blendMode = 2
		} else {
			blendMode = 1
		}
	}

	frontFaceCW := C.int(0)
	if d.FrontFaceCW {
		frontFaceCW = 1
	}

	var p C.VkPipeline
	r := C.vkbCreateCreateGraphicsPipeline(b.device, b.pipelineLayout,
		unsafe.Pointer(&d.VertexShader[0]), C.size_t(len(d.VertexShader)),
		unsafe.Pointer(&d.FragmentShader[0]), C.size_t(len(d.FragmentShader)), cEntry,
		topology(d.Topology), colorPtr, C.uint32_t(len(d.ColorFormats)),
		vkFormat(d.DepthFormat), cullMode(d.CullMode), frontFaceCW, blendMode,
		depthTest, depthWrite, compareOp(d.DepthCompare), C.uint32_t(samples), &p)
	if r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: CreateGraphicsPipeline(%q) failed (%d)", d.Label, int(r)))
	}
	return b.registerPipeline(p, C.VK_PIPELINE_BIND_POINT_GRAPHICS)
}

// pipelineEntry records a pipeline and the bind point it was created for, so
// SetPipeline can bind it without the caller distinguishing compute vs graphics.
type pipelineEntry struct {
	pipe      C.VkPipeline
	bindPoint C.VkPipelineBindPoint
}

func (b *Backend) registerPipeline(p C.VkPipeline, bindPoint C.VkPipelineBindPoint) gpu.Pipeline {
	h := b.nextID.Add(1)
	b.pipelines[h] = pipelineEntry{pipe: p, bindPoint: bindPoint}
	return gpu.Pipeline{H: gpu.Handle(h)}
}

// DestroyPipeline releases a pipeline.
func (b *Backend) DestroyPipeline(p gpu.Pipeline) {
	e, ok := b.pipelines[uint64(p.H)]
	if !ok {
		return
	}
	C.vkDestroyPipeline(b.device, e.pipe, nil)
	delete(b.pipelines, uint64(p.H))
}
