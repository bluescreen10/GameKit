package gpu_test

import (
	"math"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

// TestBlendStates draws a fullscreen triangle of one colour over a target cleared to
// another, under each blend state, and checks the blended texel. Every factor and
// operation is the one the state names, for colour and alpha separately, and a write
// mask keeps the channels it leaves out. All values are exact in half floats.
func TestBlendStates(t *testing.T) {
	clear := [4]float32{1, 0.5, 0.25, 0.25}
	source := [4]float32{0.5, 0.5, 0.5, 0.75}

	tests := []struct {
		name  string
		blend gpu.BlendState
		want  [4]float32
	}{
		{
			name: "colour scaled by one minus destination alpha",
			blend: gpu.BlendState{
				Enable:  true,
				ColorOp: gpu.BlendFactorOp{Src: gpu.BlendZero, Dst: gpu.BlendOneMinusDstAlpha, Op: gpu.BlendAdd},
				AlphaOp: gpu.BlendFactorOp{Src: gpu.BlendZero, Dst: gpu.BlendOne, Op: gpu.BlendAdd},
			},
			want: [4]float32{0.75, 0.375, 0.1875, 0.25},
		},
		{
			name: "colour scaled by destination alpha",
			blend: gpu.BlendState{
				Enable:  true,
				ColorOp: gpu.BlendFactorOp{Src: gpu.BlendZero, Dst: gpu.BlendDstAlpha, Op: gpu.BlendAdd},
				AlphaOp: gpu.BlendFactorOp{Src: gpu.BlendZero, Dst: gpu.BlendOne, Op: gpu.BlendAdd},
			},
			want: [4]float32{0.25, 0.125, 0.0625, 0.25},
		},
		{
			name: "alpha alone, by subtraction",
			blend: gpu.BlendState{
				Enable:    true,
				ColorOp:   gpu.BlendFactorOp{Src: gpu.BlendOne, Dst: gpu.BlendZero, Op: gpu.BlendAdd},
				AlphaOp:   gpu.BlendFactorOp{Src: gpu.BlendOne, Dst: gpu.BlendSrcAlpha, Op: gpu.BlendSubtract},
				WriteMask: 1 << 3,
			},
			// 0.75 - 0.25*0.75; the colour is masked off, so the source never replaces it.
			want: [4]float32{1, 0.5, 0.25, 0.5625},
		},
		{
			name: "over, with alpha accumulated",
			blend: gpu.BlendState{
				Enable:  true,
				ColorOp: gpu.BlendFactorOp{Src: gpu.BlendSrcAlpha, Dst: gpu.BlendOneMinusSrcAlpha, Op: gpu.BlendAdd},
				AlphaOp: gpu.BlendFactorOp{Src: gpu.BlendOne, Dst: gpu.BlendOneMinusSrcAlpha, Op: gpu.BlendAdd},
			},
			want: [4]float32{0.625, 0.5, 0.4375, 0.8125},
		},
		{
			name: "reverse subtraction",
			blend: gpu.BlendState{
				Enable:  true,
				ColorOp: gpu.BlendFactorOp{Src: gpu.BlendOne, Dst: gpu.BlendOne, Op: gpu.BlendReverseSubtract},
				AlphaOp: gpu.BlendFactorOp{Src: gpu.BlendZero, Dst: gpu.BlendOne, Op: gpu.BlendAdd},
			},
			want: [4]float32{0.5, 0, -0.25, 0.25},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blendedTexel(t, clear, source, tt.blend)
			if got != tt.want {
				t.Errorf("%v blended over %v = %v, want %v", source, clear, got, tt.want)
			}
		})
	}
}

// blendedTexel clears a one-texel half-float target to clear, draws source over it with
// blend, and returns the texel.
func blendedTexel(t *testing.T, clear, source [4]float32, blend gpu.BlendState) [4]float32 {
	b := testBackend(t)

	target := b.CreateTexture(gpu.TextureDescriptor{
		Kind: gpu.Texture2D, Width: 1, Height: 1, Format: gpu.FormatRGBA16F,
		Usage: gpu.TextureRenderTarget | gpu.TextureTransfer, Label: "blend-target",
	})
	defer b.DestroyTexture(target)
	pipeline := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: triangleVert, FragmentShader: tintFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA16F},
		CullMode: gpu.CullNone, Blend: []gpu.BlendState{blend}, Label: "blend",
	})
	defer b.DestroyPipeline(pipeline)

	vb := b.Alloc(256, gpu.MemoryHost, "verts")
	defer b.Free(vb)
	verts := unsafe.Slice((*vertex)(vb.Ptr), 3)
	verts[0] = vertex{-1, -1, 1, 1, 1}
	verts[1] = vertex{3, -1, 1, 1, 1}
	verts[2] = vertex{-1, 3, 1, 1, 1}
	rb := b.Alloc(64, gpu.MemoryHost, "root")
	defer b.Free(rb)
	*(*rootData)(rb.Ptr) = rootData{tint: source, verts: vb.Addr}
	readback := b.Alloc(8, gpu.MemoryHost, "readback")
	defer b.Free(readback)

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: clear}},
	})
	cmd.SetPipeline(pipeline)
	cmd.SetViewport(0, 0, 1, 1, 0, 1)
	cmd.SetScissor(0, 0, 1, 1)
	rootAddr := rb.Addr
	cmd.Draw(utils.ToBytes(&rootAddr), 3, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, target, 0, 0)
	b.Wait(b.Submit(cmd))

	halves := unsafe.Slice((*uint16)(readback.Ptr), 4)
	var texel [4]float32
	for i, h := range halves {
		texel[i] = halfToFloat32(h)
	}
	return texel
}

// halfToFloat32 decodes an IEEE 754 half-precision float.
func halfToFloat32(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exponent := uint32(h>>10) & 0x1f
	mantissa := uint32(h) & 0x3ff
	switch {
	case exponent == 0:
		value := float32(mantissa) / (1 << 24)
		if sign != 0 {
			value = -value
		}
		return value
	case exponent == 0x1f:
		return math.Float32frombits(sign | 0xff<<23 | mantissa<<13)
	default:
		return math.Float32frombits(sign | (exponent+112)<<23 | mantissa<<13)
	}
}
