package gpu_test

// Backend-agnostic RHI conformance tests. They exercise the gpu.Backend interface
// (buffers, pipelines, dynamic rendering, compute→indirect, bindless textures) and
// so validate ANY backend, not just Vulkan. The backend is obtained from the
// registry and the suite skips when none is registered/initializable — so
// `go test ./...` passes on every platform, running the tests only where a backend
// exists. Register a backend for this suite with a build-tagged blank import (see
// backend_darwin_test.go).

import (
	"embed"
	"os"
	"sync"
	"testing"

	"github.com/bluescreen10/GameKit/gpu"
)

// Test shaders are fixtures under gpu/testdata, embedded here (a _test.go file) so
// they never enter the production shaders package. Metal fixtures are generated
// from the same SPIR-V and carry reflected compute workgroup sizes.
var (
	//go:embed testdata/*.metalbin
	metalFixtures embed.FS
	//go:embed testdata/triangle.vert.spv
	triangleVert []byte
	//go:embed testdata/triangle.frag.spv
	triangleFrag []byte
	//go:embed testdata/mesh.vert.spv
	meshVert []byte
	//go:embed testdata/mesh.frag.spv
	meshFrag []byte
	//go:embed testdata/instanced.comp.spv
	instancedComp []byte
	//go:embed testdata/instanced.vert.spv
	instancedVert []byte
	//go:embed testdata/fill_indirect.comp.spv
	fillIndirect []byte
	//go:embed testdata/textured.vert.spv
	texturedVert []byte
	//go:embed testdata/textured.frag.spv
	texturedFrag []byte
)

var (
	backendOnce sync.Once
	backendInst gpu.Backend
	backendErr  error
)

// testBackend returns the registered gpu backend, initialized once and shared
// across the conformance tests. Automatic selection skips if no device exists.
// GAMEKIT_GPU_BACKEND selects an implementation explicitly and fails on init errors.
func testBackend(t *testing.T) gpu.Backend {
	t.Helper()
	if !gpu.HasBackend() {
		t.Skip("no gpu backend registered for this platform")
	}
	backendOnce.Do(func() {
		name := os.Getenv("GAMEKIT_GPU_BACKEND")
		backendInst = gpu.Instance(&name)
		if metal, ok := gpu.Lookup("metal"); ok && backendInst == metal {
			variant := func(name string) []byte {
				data, err := metalFixtures.ReadFile("testdata/" + name + ".metalbin")
				if err != nil {
					panic(err)
				}
				return data
			}
			triangleVert = variant("triangle.vert")
			triangleFrag = variant("triangle.frag")
			meshVert = variant("mesh.vert")
			meshFrag = variant("mesh.frag")
			instancedVert = variant("instanced.vert")
			instancedComp = variant("instanced.comp")
			fillIndirect = variant("fill_indirect.comp")
			texturedVert = variant("textured.vert")
			texturedFrag = variant("textured.frag")
		}
		backendErr = backendInst.Init()
	})
	if backendErr != nil {
		if os.Getenv("GAMEKIT_GPU_BACKEND") != "" {
			t.Fatalf("requested backend init failed: %v", backendErr)
		}
		t.Skipf("gpu backend init failed (no device?): %v", backendErr)
	}
	return backendInst
}
