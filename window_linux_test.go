//go:build linux && cgo

package gamekit

import (
	"math"
	"os"
	"testing"

	"github.com/bluescreen10/gamekit/gpu/vulkan"
	"github.com/bluescreen10/gamekit/keyboard"
)

func TestNativeWindowAndVulkanSurface(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("DISPLAY is not set")
	}

	window, err := CreateWindow("gamekit test", 320, 200, &WindowOptions{Hidden: true, Resizable: true})
	if err != nil {
		t.Fatal(err)
	}
	defer window.Destroy()
	if window.NativeDisplay() == nil || window.NativeHandle() == 0 {
		t.Fatal("native X11 handles are missing")
	}
	if width, height := window.Size(); width != 320 || height != 200 {
		t.Fatalf("window size = %dx%d, want 320x200", width, height)
	}
	x, y := window.GetPointerPos()
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		t.Fatalf("cursor position = (%v, %v), want finite coordinates", x, y)
	}
	if action := window.GetKey(keyboard.KeyEsc); action != keyboard.KeyReleased {
		t.Fatalf("initial escape action = %v, want KeyReleased", action)
	}
	window.SetTitle("renamed")
	if window.Title() != "renamed" {
		t.Fatalf("title = %q, want renamed", window.Title())
	}
	window.SetShouldClose(true)
	if !window.ShouldClose() {
		t.Fatal("SetShouldClose(true) was ignored")
	}
	window.SetShouldClose(false)

	backend := vulkan.New("VK_KHR_surface", "VK_KHR_xlib_surface")
	if err := backend.Init(); err != nil {
		t.Fatalf("initialize Vulkan: %v", err)
	}
	defer backend.Destroy()
	surface, err := window.CreateSurface(backend)
	if err != nil {
		t.Fatal(err)
	}
	if surface == 0 {
		t.Fatal("Vulkan surface is nil")
	}
	swapchain := backend.CreateSwapchain(surface, 320, 200)
	if swapchain.H == 0 {
		t.Fatal("Vulkan swapchain is invalid")
	}
}
