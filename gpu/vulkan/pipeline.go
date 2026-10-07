package vulkan

/*
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/gpu/internal/specialization"
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
	constants, err := resolveConstants(desc.Constants, desc.Shader)
	if err != nil {
		panic(fmt.Sprintf("vulkan: CreateComputePipeline(%q): %v", desc.Label, err))
	}
	var p C.VkPipeline
	r := C.vkbCreateComputePipeline(b.device, b.pipelineLayout,
		unsafe.Pointer(&desc.Shader[0]), C.size_t(len(desc.Shader)), cEntry,
		constants.idsPtr(), constants.bitsPtr(), C.uint32_t(len(constants.ids)), &p)
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
	// One blend state per colour attachment; a target the descriptor gives none is
	// opaque. A nil pointer when there are no targets keeps cgo from indexing an empty
	// slice.
	blends := make([]C.VkPipelineColorBlendAttachmentState, len(d.ColorFormats))
	for i := range blends {
		var blend gpu.BlendState
		if i < len(d.Blend) {
			blend = d.Blend[i]
		}
		blends[i] = blendAttachment(blend)
	}
	var blendsPtr *C.VkPipelineColorBlendAttachmentState
	if len(blends) > 0 {
		blendsPtr = &blends[0]
	}

	frontFaceCW := C.int(0)
	if d.FrontFaceCW {
		frontFaceCW = 1
	}

	// A depth-only pipeline has no fragment stage at all; indexing an empty slice to get
	// its address would panic before the backend ever saw it.
	var fragPtr unsafe.Pointer
	if len(d.FragmentShader) > 0 {
		fragPtr = unsafe.Pointer(&d.FragmentShader[0])
	}

	constants, err := resolveConstants(d.Constants, d.VertexShader, d.FragmentShader)
	if err != nil {
		panic(fmt.Sprintf("vulkan: CreateGraphicsPipeline(%q): %v", d.Label, err))
	}
	var p C.VkPipeline
	r := C.vkbCreateCreateGraphicsPipeline(b.device, b.pipelineLayout,
		unsafe.Pointer(&d.VertexShader[0]), C.size_t(len(d.VertexShader)),
		fragPtr, C.size_t(len(d.FragmentShader)), cEntry,
		topology(d.Topology), colorPtr, C.uint32_t(len(d.ColorFormats)),
		vkFormat(d.DepthFormat), cullMode(d.CullMode), frontFaceCW, blendsPtr,
		depthTest, depthWrite, compareOp(d.DepthCompare), C.uint32_t(samples),
		constants.idsPtr(), constants.bitsPtr(), C.uint32_t(len(constants.ids)), &p)
	if r != C.VK_SUCCESS {
		panic(fmt.Sprintf("vulkan: CreateGraphicsPipeline(%q) failed (%d)", d.Label, int(r)))
	}
	return b.registerPipeline(p, C.VK_PIPELINE_BIND_POINT_GRAPHICS)
}

// specializationConstants is a pipeline's constants as the bridge takes them: ids[i]
// takes the 32 bits bits[i]. Vulkan reads every constant as 4 bytes whatever its type,
// so the type is not passed on.
type specializationConstants struct {
	ids  []C.uint32_t
	bits []C.uint32_t
}

// resolveConstants resolves a descriptor's constants against what the shaders declare.
// A constant only one stage declares is passed to both, and the other ignores it.
func resolveConstants(constants map[string]float64, shaders ...[]byte) (specializationConstants, error) {
	var resolved specializationConstants
	if len(constants) == 0 {
		return resolved, nil
	}
	var stages [][]specialization.Declaration
	for _, code := range shaders {
		if len(code) == 0 {
			continue
		}
		declared, err := specConstants(code)
		if err != nil {
			return resolved, err
		}
		stages = append(stages, declared)
	}
	values, err := specialization.Resolve(constants, stages...)
	if err != nil {
		return resolved, err
	}
	for _, v := range values {
		resolved.ids = append(resolved.ids, C.uint32_t(v.ID))
		resolved.bits = append(resolved.bits, C.uint32_t(v.Bits))
	}
	return resolved, nil
}

// idsPtr and bitsPtr are nil when there are no constants, which keeps cgo from
// indexing an empty slice.
func (s specializationConstants) idsPtr() *C.uint32_t {
	if len(s.ids) == 0 {
		return nil
	}
	return &s.ids[0]
}

func (s specializationConstants) bitsPtr() *C.uint32_t {
	if len(s.bits) == 0 {
		return nil
	}
	return &s.bits[0]
}

// blendAttachment translates a target's blend state. WriteMask 0 means every channel,
// whether or not blending is enabled.
func blendAttachment(b gpu.BlendState) C.VkPipelineColorBlendAttachmentState {
	mask := C.VkColorComponentFlags(b.WriteMask & 0xf)
	if mask == 0 {
		mask = C.VK_COLOR_COMPONENT_R_BIT | C.VK_COLOR_COMPONENT_G_BIT | C.VK_COLOR_COMPONENT_B_BIT | C.VK_COLOR_COMPONENT_A_BIT
	}
	state := C.VkPipelineColorBlendAttachmentState{colorWriteMask: mask}
	if !b.Enable {
		return state
	}
	state.blendEnable = C.VK_TRUE
	state.srcColorBlendFactor = blendFactor(b.ColorOp.Src)
	state.dstColorBlendFactor = blendFactor(b.ColorOp.Dst)
	state.colorBlendOp = blendOp(b.ColorOp.Op)
	state.srcAlphaBlendFactor = blendFactor(b.AlphaOp.Src)
	state.dstAlphaBlendFactor = blendFactor(b.AlphaOp.Dst)
	state.alphaBlendOp = blendOp(b.AlphaOp.Op)
	return state
}

func blendFactor(f gpu.BlendFactor) C.VkBlendFactor {
	switch f {
	case gpu.BlendOne:
		return C.VK_BLEND_FACTOR_ONE
	case gpu.BlendSrcAlpha:
		return C.VK_BLEND_FACTOR_SRC_ALPHA
	case gpu.BlendOneMinusSrcAlpha:
		return C.VK_BLEND_FACTOR_ONE_MINUS_SRC_ALPHA
	case gpu.BlendDstAlpha:
		return C.VK_BLEND_FACTOR_DST_ALPHA
	case gpu.BlendOneMinusDstAlpha:
		return C.VK_BLEND_FACTOR_ONE_MINUS_DST_ALPHA
	default:
		return C.VK_BLEND_FACTOR_ZERO
	}
}

func blendOp(op gpu.BlendOp) C.VkBlendOp {
	switch op {
	case gpu.BlendSubtract:
		return C.VK_BLEND_OP_SUBTRACT
	case gpu.BlendReverseSubtract:
		return C.VK_BLEND_OP_REVERSE_SUBTRACT
	case gpu.BlendMin:
		return C.VK_BLEND_OP_MIN
	case gpu.BlendMax:
		return C.VK_BLEND_OP_MAX
	default:
		return C.VK_BLEND_OP_ADD
	}
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
