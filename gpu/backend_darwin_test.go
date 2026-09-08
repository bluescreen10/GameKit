//go:build darwin && cgo

package gpu_test

// Register both implementations; GAMEKIT_GPU_BACKEND selects the conformance backend.
import (
	_ "github.com/bluescreen10/gamekit/gpu/metal"
	_ "github.com/bluescreen10/gamekit/gpu/vulkan"
)

// Regenerate Metal fixtures from the same SPIR-V as Vulkan (requires spirv-cross).
//go:generate go run ./metal/cmd/metalshader -in testdata/triangle.vert.spv -out testdata/triangle.vert.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/triangle.frag.spv -out testdata/triangle.frag.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/mesh.vert.spv -out testdata/mesh.vert.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/mesh.frag.spv -out testdata/mesh.frag.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/instanced.vert.spv -out testdata/instanced.vert.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/instanced.comp.spv -out testdata/instanced.comp.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/fill_indirect.comp.spv -out testdata/fill_indirect.comp.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/textured.vert.spv -out testdata/textured.vert.metalbin
//go:generate go run ./metal/cmd/metalshader -in testdata/textured.frag.spv -out testdata/textured.frag.metalbin
