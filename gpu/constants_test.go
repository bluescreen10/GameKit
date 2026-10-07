package gpu_test

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/utils"
)

// shaderConstants matches Constants in testdata/constants.comp.
type shaderConstants struct {
	enabled uint32
	offset  int32
	count   uint32
	scale   float32
}

// readComputeConstants runs constants.comp specialized by constants and returns what
// it saw.
func readComputeConstants(t *testing.T, constants map[string]float64) shaderConstants {
	t.Helper()
	b := testBackend(t)

	out := b.Alloc(uint64(unsafe.Sizeof(shaderConstants{})), gpu.MemoryHost, "constants")
	defer b.Free(out)
	pipe := b.CreateComputePipeline(gpu.ComputePipelineDescriptor{
		Shader: constantsComp, Constants: constants, Label: "constants",
	})
	defer b.DestroyPipeline(pipe)

	cmd := b.Begin()
	cmd.SetPipeline(pipe)
	addr := out.Addr
	cmd.Dispatch(utils.ToBytes(&addr), 1, 1, 1)
	b.Wait(b.Submit(cmd))
	return *(*shaderConstants)(out.Ptr)
}

// TestComputeConstantsDefault: a pipeline that sets no constants runs its shader with
// the defaults the shader declares.
func TestComputeConstantsDefault(t *testing.T) {
	got := readComputeConstants(t, nil)
	want := shaderConstants{enabled: 0, offset: -3, count: 7, scale: 0.5}
	if got != want {
		t.Errorf("constants = %+v, want the declared defaults %+v", got, want)
	}
}

// TestComputeConstantsByName: each constant type reaches the shader, set by name and
// converted to its declared type — the bool through Metal's one-byte bool, the int's
// sign intact, the uint above the int32 range.
func TestComputeConstantsByName(t *testing.T) {
	got := readComputeConstants(t, map[string]float64{
		"ENABLED": 1,
		"OFFSET":  -42,
		"COUNT":   1<<31 + 5,
		"SCALE":   2.25,
	})
	want := shaderConstants{enabled: 1, offset: -42, count: 1<<31 + 5, scale: 2.25}
	if got != want {
		t.Errorf("constants = %+v, want %+v", got, want)
	}
}

// TestComputeConstantsByID: a key may be a constant's ID in decimal instead of its
// name.
func TestComputeConstantsByID(t *testing.T) {
	got := readComputeConstants(t, map[string]float64{"0": 1, "3": -1})
	want := shaderConstants{enabled: 1, offset: -3, count: 7, scale: -1}
	if got != want {
		t.Errorf("constants = %+v, want %+v", got, want)
	}
}

// TestComputeConstantsPartial: setting some constants leaves the rest at their
// defaults.
func TestComputeConstantsPartial(t *testing.T) {
	got := readComputeConstants(t, map[string]float64{"COUNT": 9})
	want := shaderConstants{enabled: 0, offset: -3, count: 9, scale: 0.5}
	if got != want {
		t.Errorf("constants = %+v, want %+v", got, want)
	}
}

// TestComputeConstantsIgnoresUndeclared: a constant the shader does not declare, by
// name or by ID, is ignored, and the ones it does declare still take effect.
func TestComputeConstantsIgnoresUndeclared(t *testing.T) {
	got := readComputeConstants(t, map[string]float64{"MISSING": 1, "9": 1, "COUNT": 9})
	want := shaderConstants{enabled: 0, offset: -3, count: 9, scale: 0.5}
	if got != want {
		t.Errorf("constants = %+v, want %+v", got, want)
	}
}

// TestComputeConstantsRejected: a value its constant's type cannot hold makes creating
// the pipeline panic, naming the constant.
func TestComputeConstantsRejected(t *testing.T) {
	for _, tc := range []struct {
		name      string
		constants map[string]float64
		key       string
	}{
		{"negative uint", map[string]float64{"COUNT": -1}, "COUNT"},
		{"fractional int", map[string]float64{"OFFSET": 1.5}, "OFFSET"},
		{"int out of range", map[string]float64{"OFFSET": 1 << 31}, "OFFSET"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := testBackend(t)
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("CreateComputePipeline(%v) did not panic", tc.constants)
				}
				if message, _ := r.(string); !strings.Contains(message, `"`+tc.key+`"`) {
					t.Errorf("CreateComputePipeline(%v) panicked with %v, want it to name %q", tc.constants, r, tc.key)
				}
			}()
			b.CreateComputePipeline(gpu.ComputePipelineDescriptor{
				Shader: constantsComp, Constants: tc.constants, Label: "constants",
			})
		})
	}
}

// drawConstantsQuad draws constants.vert/.frag specialized by constants into a cleared
// black target and returns the centre pixels of the target's left and right halves.
func drawConstantsQuad(t *testing.T, constants map[string]float64) (left, right [4]byte) {
	t.Helper()
	b := testBackend(t)

	const size = 64
	target := b.CreateTexture(gpu.TextureDescriptor{Kind: gpu.Texture2D, Width: size, Height: size,
		Format: gpu.FormatRGBA8Unorm, Usage: gpu.TextureRenderTarget | gpu.TextureTransfer, Label: "target"})
	defer b.DestroyTexture(target)
	pipe := b.CreateGraphicsPipeline(gpu.PipelineDescriptor{
		VertexShader: constantsVert, FragmentShader: constantsFrag,
		Topology: gpu.TopologyTriangles, ColorFormats: []gpu.Format{gpu.FormatRGBA8Unorm},
		CullMode: gpu.CullNone, Constants: constants, Label: "constants",
	})
	defer b.DestroyPipeline(pipe)
	readback := b.Alloc(size*size*4, gpu.MemoryHost, "readback")
	defer b.Free(readback)

	cmd := b.Begin()
	cmd.BeginRenderPass(gpu.RenderTargets{
		Color: []gpu.ColorAttachment{{Texture: target, Load: gpu.LoadClear, Store: gpu.StoreKeep, Clear: [4]float32{0, 0, 0, 1}}},
	})
	cmd.SetPipeline(pipe)
	cmd.SetViewport(0, 0, size, size, 0, 1)
	cmd.SetScissor(0, 0, size, size)
	cmd.Draw(nil, 6, 1, 0, 0)
	cmd.EndRenderPass()
	cmd.Barrier(gpu.StageColorOutput, gpu.StageTransfer, 0)
	cmd.CopyTextureToBuffer(readback, target, 0, 0)
	b.Wait(b.Submit(cmd))

	pixels := unsafe.Slice((*byte)(readback.Ptr), size*size*4)
	pixel := func(x, y int) [4]byte {
		i := (y*size + x) * 4
		return [4]byte(pixels[i : i+4])
	}
	return pixel(size/4, size/2), pixel(3*size/4, size/2)
}

// TestGraphicsConstantsDefault: with no constants set, the quad covers the target in
// the fragment shader's default blue.
func TestGraphicsConstantsDefault(t *testing.T) {
	left, right := drawConstantsQuad(t, nil)
	blue := [4]byte{0, 0, 255, 255}
	if left != blue || right != blue {
		t.Errorf("left, right = %v, %v, want both %v", left, right, blue)
	}
}

// TestGraphicsConstantsReachEachStage: one set of constants specializes both stages —
// the vertex stage's narrows the quad to the left half, the fragment stage's turn it
// red — though each stage declares only its own.
func TestGraphicsConstantsReachEachStage(t *testing.T) {
	left, right := drawConstantsQuad(t, map[string]float64{
		"LEFT_HALF": 1,
		"RED":       1,
		"BLUE":      0,
	})
	red := [4]byte{255, 0, 0, 255}
	black := [4]byte{0, 0, 0, 255}
	if left != red {
		t.Errorf("left = %v, want %v", left, red)
	}
	if right != black {
		t.Errorf("right = %v, want the clear colour %v", right, black)
	}
}
