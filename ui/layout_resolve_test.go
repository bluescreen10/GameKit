package ui_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

type widget struct {
	preferred ui.Size
}

func (w *widget) Close() {
}

func (w *widget) Bounds() ui.Rect {
	return ui.Rect{}
}

func (w *widget) SetBounds(ui.Rect) {
}

func (w *widget) PreferredSize() ui.Size {
	return w.preferred
}

func (w *widget) IsVisible() bool {
	return true
}

func (w *widget) SetVisible(bool) {
}

func (w *widget) IsEnabled() bool {
	return true
}

func (w *widget) SetEnabled(bool) {
}

func TestResolveRow(t *testing.T) {
	canvas := &widget{preferred: ui.Size{Width: 100, Height: 50}}
	panel := &widget{preferred: ui.Size{Width: 80, Height: 40}}
	spec := ui.Row(
		ui.Item(canvas).Grow(2),
		ui.Item(panel).Grow(1).Align(ui.AlignCenter),
	).WithPadding(ui.Insets{Top: 10, Right: 10, Bottom: 10, Left: 10}).WithGap(10)

	positions, err := spec.Resolve(ui.Size{Width: 390, Height: 120})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	want := []ui.Rect{
		{Position: ui.Point{X: 10, Y: 10}, Size: ui.Size{Width: 240, Height: 100}},
		{Position: ui.Point{X: 260, Y: 40}, Size: ui.Size{Width: 120, Height: 40}},
	}
	assertRects(t, positions, want)
}

func TestResolveColumnWithSpacer(t *testing.T) {
	label := &widget{preferred: ui.Size{Width: 80, Height: 20}}
	slider := &widget{preferred: ui.Size{Width: 120, Height: 24}}
	spec := ui.Column(
		ui.Item(label),
		ui.Item(slider),
		ui.Space(),
	).WithPadding(ui.UniformInsets(16)).WithGap(12)

	positions, err := spec.Resolve(ui.Size{Width: 200, Height: 200})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	want := []ui.Rect{
		{Position: ui.Point{X: 16, Y: 16}, Size: ui.Size{Width: 168, Height: 20}},
		{Position: ui.Point{X: 16, Y: 48}, Size: ui.Size{Width: 168, Height: 24}},
		{Position: ui.Point{X: 16, Y: 84}, Size: ui.Size{Width: 168, Height: 100}},
	}
	assertRects(t, positions, want)
}

func TestResolveShrinksTowardMinimumSize(t *testing.T) {
	first := &widget{preferred: ui.Size{Width: 100, Height: 40}}
	second := &widget{preferred: ui.Size{Width: 100, Height: 40}}
	spec := ui.Row(
		ui.Item(first).MinimumSize(ui.Size{Width: 60, Height: 20}),
		ui.Item(second).MinimumSize(ui.Size{Width: 60, Height: 20}),
	).WithGap(10)

	positions, err := spec.Resolve(ui.Size{Width: 150, Height: 30})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	want := []ui.Rect{
		{Size: ui.Size{Width: 70, Height: 30}},
		{Position: ui.Point{X: 80}, Size: ui.Size{Width: 70, Height: 30}},
	}
	assertRects(t, positions, want)
}

func TestLayoutPreferredSize(t *testing.T) {
	first := &widget{preferred: ui.Size{Width: 40, Height: 20}}
	second := &widget{preferred: ui.Size{Width: 60, Height: 30}}
	spec := ui.Row(ui.Item(first), ui.Item(second)).
		WithPadding(ui.Insets{Top: 2, Right: 3, Bottom: 4, Left: 5}).
		WithGap(7)

	size, err := spec.PreferredSize()
	if err != nil {
		t.Fatalf("PreferredSize() error: %v", err)
	}
	want := ui.Size{Width: 115, Height: 36}
	if size != want {
		t.Errorf("PreferredSize() = %#v, want %#v", size, want)
	}
}

func TestLayoutValidation(t *testing.T) {
	tests := []struct {
		name string
		spec ui.LayoutSpec
	}{
		{name: "negative gap", spec: ui.Row().WithGap(-1)},
		{name: "negative padding", spec: ui.Row().WithPadding(ui.Insets{Left: -1})},
		{name: "missing widget", spec: ui.Row(ui.LayoutItem{})},
		{name: "spacer with widget", spec: ui.Row(ui.LayoutItem{Widget: &widget{}, IsSpacer: true})},
		{name: "negative growth", spec: ui.Row(ui.Space().Grow(-1))},
		{name: "invalid alignment", spec: ui.Row(ui.Space().Align(ui.Alignment(20)))},
		{name: "negative minimum", spec: ui.Row(ui.Space().MinimumSize(ui.Size{Width: -1}))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.spec.Validate(); err == nil {
				t.Error("Validate() succeeded, want an error")
			}
		})
	}
}

func assertRects(t *testing.T, got, want []ui.Rect) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Resolve() returned %d rectangles, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rectangle %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}
