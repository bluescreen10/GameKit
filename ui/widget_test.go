package ui_test

import (
	"image"
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

func TestSliderOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		options ui.SliderOptions
		valid   bool
	}{
		{
			name:    "continuous",
			options: ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 0.5},
			valid:   true,
		},
		{
			name:    "stepped",
			options: ui.SliderOptions{Minimum: -1, Maximum: 1, Step: 0.25},
			valid:   true,
		},
		{name: "empty range", options: ui.SliderOptions{Minimum: 1, Maximum: 1}},
		{name: "reversed range", options: ui.SliderOptions{Minimum: 2, Maximum: 1}},
		{name: "value below range", options: ui.SliderOptions{Minimum: 0, Maximum: 1, Value: -1}},
		{name: "value above range", options: ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 2}},
		{name: "negative step", options: ui.SliderOptions{Minimum: 0, Maximum: 1, Step: -1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.options.Validate()
			if test.valid && err != nil {
				t.Errorf("Validate() error: %v", err)
			}
			if !test.valid && err == nil {
				t.Error("Validate() succeeded, want an error")
			}
		})
	}
}

func TestSelectionOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		options interface{ Validate() error }
		valid   bool
	}{
		{name: "radio", options: ui.RadioGroupOptions{Items: []string{"A", "B"}, SelectedIndex: 1}, valid: true},
		{name: "radio empty", options: ui.RadioGroupOptions{}},
		{name: "radio range", options: ui.RadioGroupOptions{Items: []string{"A"}, SelectedIndex: 1}},
		{name: "select", options: ui.SelectOptions{Items: []string{"A"}}, valid: true},
		{name: "select empty", options: ui.SelectOptions{}},
		{name: "select range", options: ui.SelectOptions{Items: []string{"A"}, SelectedIndex: -1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.options.Validate()
			if test.valid && err != nil {
				t.Errorf("Validate() error: %v", err)
			}
			if !test.valid && err == nil {
				t.Error("Validate() succeeded, want an error")
			}
		})
	}
}

func TestImageOptionsValidation(t *testing.T) {
	if err := (ui.ImageOptions{Source: image.NewRGBA(image.Rect(0, 0, 8, 8))}).Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if err := (ui.ImageOptions{}).Validate(); err == nil {
		t.Fatal("Validate() succeeded without a source")
	}
	if err := (ui.ImageOptions{
		Source:  image.NewRGBA(image.Rect(0, 0, 8, 8)),
		Scaling: ui.ImageScaling(255),
	}).Validate(); err == nil {
		t.Fatal("Validate() succeeded with invalid scaling")
	}
}

func TestScrollViewOptionsValidation(t *testing.T) {
	content := &widget{}
	for _, axes := range []ui.ScrollAxes{ui.ScrollVertical, ui.ScrollHorizontal, ui.ScrollBoth} {
		if err := (ui.ScrollViewOptions{Content: content, Axes: axes}).Validate(); err != nil {
			t.Errorf("Validate() with axes %d: %v", axes, err)
		}
	}
	if err := (ui.ScrollViewOptions{}).Validate(); err == nil {
		t.Fatal("Validate() succeeded without content")
	}
	if err := (ui.ScrollViewOptions{Content: content, Axes: ui.ScrollAxes(255)}).Validate(); err == nil {
		t.Fatal("Validate() succeeded with invalid axes")
	}
}
