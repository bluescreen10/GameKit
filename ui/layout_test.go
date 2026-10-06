package ui_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

func TestLayoutSpecBuilders(t *testing.T) {
	items := []ui.LayoutItem{ui.Space()}
	base := ui.Column(items...)
	spec := base.WithPadding(ui.UniformInsets(16)).WithGap(12)

	items[0] = ui.LayoutItem{}

	if spec.Direction != ui.Vertical {
		t.Errorf("Direction = %v, want Vertical", spec.Direction)
	}
	if len(spec.Items) != 1 || !spec.Items[0].IsSpacer {
		t.Errorf("Items = %#v, want one spacer copied from input", spec.Items)
	}
	if spec.Padding != (ui.Insets{Top: 16, Right: 16, Bottom: 16, Left: 16}) {
		t.Errorf("Padding = %#v, want uniform 16", spec.Padding)
	}
	if spec.Gap != 12 {
		t.Errorf("Gap = %d, want 12", spec.Gap)
	}
	if base.Gap != 0 || base.Padding != (ui.Insets{}) {
		t.Errorf("builder changed base spec: %#v", base)
	}
}

func TestLayoutItemBuilders(t *testing.T) {
	item := ui.Space().Grow(2).Align(ui.AlignCenter).MinimumSize(ui.Size{Width: 80, Height: 24})

	if item.GrowWeight != 2 {
		t.Errorf("GrowWeight = %v, want 2", item.GrowWeight)
	}
	if item.Alignment != ui.AlignCenter {
		t.Errorf("Alignment = %v, want AlignCenter", item.Alignment)
	}
	if item.Minimum != (ui.Size{Width: 80, Height: 24}) {
		t.Errorf("Minimum = %#v, want 80 by 24", item.Minimum)
	}
}
