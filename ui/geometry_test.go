package ui_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

func TestSizeValidity(t *testing.T) {
	tests := []struct {
		name  string
		size  ui.Size
		valid bool
	}{
		{name: "positive", size: ui.Size{Width: 640, Height: 480}, valid: true},
		{name: "zero width", size: ui.Size{Height: 480}},
		{name: "zero height", size: ui.Size{Width: 640}},
		{name: "negative width", size: ui.Size{Width: -1, Height: 480}},
		{name: "negative height", size: ui.Size{Width: 640, Height: -1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.size.IsValid(); got != test.valid {
				t.Errorf("Size.IsValid() = %t, want %t", got, test.valid)
			}
		})
	}
}
