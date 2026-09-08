//go:build linux && cgo

package gpu_test

// Register Vulkan for the backend-agnostic conformance suite. This is headless:
// render targets are offscreen textures, so Mesa's lavapipe needs no X server.
import _ "github.com/bluescreen10/gamekit/gpu/vulkan"
