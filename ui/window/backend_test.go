package window_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/window"
)

func TestCreateWindowRejectsInvalidSize(t *testing.T) {
	backend := &window.Backend{}

	created, err := backend.CreateWindow(ui.WindowOptions{
		Title: "invalid",
		Size:  ui.Size{Width: 0, Height: 480},
	})
	if err == nil || created != nil {
		t.Fatalf("CreateWindow() = (%v, %v), want (nil, error)", created, err)
	}
}
