package ui

import (
	"errors"
	"fmt"
	"math"
)

// LayoutDirection is the primary direction in which a layout places children.
type LayoutDirection uint8

const (
	// Horizontal places children from left to right.
	Horizontal LayoutDirection = iota
	// Vertical places children from top to bottom.
	Vertical
)

// Alignment controls placement across a layout's secondary axis.
type Alignment uint8

const (
	AlignStretch Alignment = iota
	AlignStart
	AlignCenter
	AlignEnd
)

// Insets are logical-pixel distances from the four edges of a layout.
type Insets struct {
	Top    int
	Right  int
	Bottom int
	Left   int
}

// UniformInsets returns equal insets on all four edges.
func UniformInsets(size int) Insets {
	return Insets{
		Top:    size,
		Right:  size,
		Bottom: size,
		Left:   size,
	}
}

// LayoutItem describes one child or flexible spacer in a LayoutSpec.
type LayoutItem struct {
	Widget     Widget
	GrowWeight float32
	Alignment  Alignment
	Minimum    Size
	IsSpacer   bool
}

// Item creates a layout item for widget.
func Item(widget Widget) LayoutItem {
	return LayoutItem{Widget: widget}
}

// Space creates an empty item that consumes remaining space.
func Space() LayoutItem {
	return LayoutItem{
		GrowWeight: 1,
		IsSpacer:   true,
	}
}

// Grow returns a copy that receives weight shares of the space left after fixed
// items and growing items' minimum sizes.
func (item LayoutItem) Grow(weight float32) LayoutItem {
	item.GrowWeight = weight
	return item
}

// Align returns a copy with alignment on the layout's secondary axis.
func (item LayoutItem) Align(alignment Alignment) LayoutItem {
	item.Alignment = alignment
	return item
}

// MinimumSize returns a copy constrained to at least size.
func (item LayoutItem) MinimumSize(size Size) LayoutItem {
	item.Minimum = size
	return item
}

// LayoutSpec declaratively describes a row or column of native controls.
type LayoutSpec struct {
	Direction LayoutDirection
	Items     []LayoutItem
	Padding   Insets
	Gap       int
}

// Row describes a horizontal layout containing items.
func Row(items ...LayoutItem) LayoutSpec {
	return LayoutSpec{
		Direction: Horizontal,
		Items:     append([]LayoutItem(nil), items...),
	}
}

// Column describes a vertical layout containing items.
func Column(items ...LayoutItem) LayoutSpec {
	return LayoutSpec{
		Direction: Vertical,
		Items:     append([]LayoutItem(nil), items...),
	}
}

// WithPadding returns a copy with padding around its children.
func (spec LayoutSpec) WithPadding(padding Insets) LayoutSpec {
	spec.Padding = padding
	return spec
}

// WithGap returns a copy with gap logical pixels between adjacent items.
func (spec LayoutSpec) WithGap(gap int) LayoutSpec {
	spec.Gap = gap
	return spec
}

// Validate checks that a layout can be resolved. Widget ownership is checked by
// the Window that creates the native layout.
func (spec LayoutSpec) Validate() error {
	if spec.Direction != Horizontal && spec.Direction != Vertical {
		return fmt.Errorf("ui: invalid layout direction %d", spec.Direction)
	}
	if spec.Gap < 0 {
		return errors.New("ui: layout gap must not be negative")
	}
	if spec.Padding.Top < 0 || spec.Padding.Right < 0 || spec.Padding.Bottom < 0 || spec.Padding.Left < 0 {
		return errors.New("ui: layout padding must not be negative")
	}
	for i, item := range spec.Items {
		if err := item.validate(); err != nil {
			return fmt.Errorf("ui: layout item %d: %w", i, err)
		}
	}
	return nil
}

func (item LayoutItem) validate() error {
	if item.IsSpacer && item.Widget != nil {
		return errors.New("spacer must not contain a widget")
	}
	if !item.IsSpacer && item.Widget == nil {
		return errors.New("item has no widget")
	}
	if item.GrowWeight < 0 || math.IsNaN(float64(item.GrowWeight)) || math.IsInf(float64(item.GrowWeight), 0) {
		return errors.New("grow weight must be finite and non-negative")
	}
	if item.Alignment > AlignEnd {
		return errors.New("invalid alignment")
	}
	if item.Minimum.Width < 0 || item.Minimum.Height < 0 {
		return errors.New("minimum size must not be negative")
	}
	return nil
}

// PreferredSize returns the smallest logical size that fits the preferred
// sizes of the spec's children, its gaps, and its padding.
func (spec LayoutSpec) PreferredSize() (Size, error) {
	if err := spec.Validate(); err != nil {
		return Size{}, err
	}

	primary := 0
	cross := 0
	for _, item := range spec.Items {
		size := item.preferredSize()
		itemPrimary, itemCross := spec.axes(size)
		primary += itemPrimary
		cross = max(cross, itemCross)
	}
	primary += spec.gapsSize()

	if spec.Direction == Horizontal {
		return Size{
			Width:  primary + spec.Padding.Left + spec.Padding.Right,
			Height: cross + spec.Padding.Top + spec.Padding.Bottom,
		}, nil
	}
	return Size{
		Width:  cross + spec.Padding.Left + spec.Padding.Right,
		Height: primary + spec.Padding.Top + spec.Padding.Bottom,
	}, nil
}

// Resolve calculates one child rectangle per item within size. Rectangles use
// coordinates relative to the layout's top-left corner. Spacer rectangles have
// no widget but remain in the result so indices match LayoutSpec.Items.
func (spec LayoutSpec) Resolve(size Size) ([]Rect, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if size.Width < 0 || size.Height < 0 {
		return nil, errors.New("ui: layout size must not be negative")
	}

	primarySize, crossSize := spec.contentAxes(size)
	primarySizes, minimumPrimarySizes := spec.primarySizes()
	spec.shrinkToFit(primarySize, primarySizes, minimumPrimarySizes)
	spec.distributeGrowth(primarySize, primarySizes)

	positions := make([]Rect, len(spec.Items))
	primaryPosition := spec.primaryStart()
	for i, item := range spec.Items {
		itemSize := item.preferredSize()
		minimumSize := item.minimumSize()
		_, itemCross := spec.axes(itemSize)
		_, minimumCross := spec.axes(minimumSize)
		crossPosition, resolvedCross := spec.alignCross(item.Alignment, crossSize, itemCross, minimumCross)
		positions[i] = spec.rect(primaryPosition, crossPosition, primarySizes[i], resolvedCross)
		primaryPosition += primarySizes[i] + spec.Gap
	}
	return positions, nil
}

func (item LayoutItem) preferredSize() Size {
	size := Size{}
	if item.Widget != nil {
		size = item.Widget.PreferredSize()
	}
	size.Width = max(size.Width, item.Minimum.Width)
	size.Height = max(size.Height, item.Minimum.Height)
	return size
}

func (item LayoutItem) minimumSize() Size {
	return item.Minimum
}

func (spec LayoutSpec) axes(size Size) (primary, cross int) {
	if spec.Direction == Horizontal {
		return size.Width, size.Height
	}
	return size.Height, size.Width
}

func (spec LayoutSpec) contentAxes(size Size) (primary, cross int) {
	if spec.Direction == Horizontal {
		primary = size.Width - spec.Padding.Left - spec.Padding.Right - spec.gapsSize()
		cross = size.Height - spec.Padding.Top - spec.Padding.Bottom
	} else {
		primary = size.Height - spec.Padding.Top - spec.Padding.Bottom - spec.gapsSize()
		cross = size.Width - spec.Padding.Left - spec.Padding.Right
	}
	return max(0, primary), max(0, cross)
}

func (spec LayoutSpec) gapsSize() int {
	if len(spec.Items) < 2 {
		return 0
	}
	return (len(spec.Items) - 1) * spec.Gap
}

func (spec LayoutSpec) primarySizes() (preferred, minimum []int) {
	sizes := make([]int, len(spec.Items))
	minimums := make([]int, len(spec.Items))
	for i, item := range spec.Items {
		primary, _ := spec.axes(item.preferredSize())
		minimumPrimary, _ := spec.axes(item.minimumSize())
		if item.GrowWeight > 0 {
			primary = minimumPrimary
		}
		sizes[i] = primary
		minimums[i] = minimumPrimary
	}
	return sizes, minimums
}

func (spec LayoutSpec) shrinkToFit(available int, sizes, minimums []int) {
	used := 0
	totalCapacity := 0
	lastShrinkable := -1
	for i, size := range sizes {
		used += size
		capacity := size - minimums[i]
		if capacity > 0 {
			totalCapacity += capacity
			lastShrinkable = i
		}
	}

	deficit := used - available
	if deficit <= 0 || totalCapacity == 0 {
		return
	}
	deficit = min(deficit, totalCapacity)

	removed := 0
	for i, size := range sizes {
		capacity := size - minimums[i]
		if capacity <= 0 {
			continue
		}
		shrink := deficit - removed
		if i != lastShrinkable {
			shrink = deficit * capacity / totalCapacity
		}
		sizes[i] -= shrink
		removed += shrink
	}
}

func (spec LayoutSpec) distributeGrowth(available int, sizes []int) {
	used := 0
	totalWeight := float64(0)
	lastGrowing := -1
	for i, size := range sizes {
		used += size
		if spec.Items[i].GrowWeight > 0 {
			totalWeight += float64(spec.Items[i].GrowWeight)
			lastGrowing = i
		}
	}

	remaining := available - used
	if remaining <= 0 || totalWeight == 0 {
		return
	}

	distributed := 0
	for i, item := range spec.Items {
		if item.GrowWeight <= 0 {
			continue
		}
		growth := remaining - distributed
		if i != lastGrowing {
			growth = int(float64(remaining) * float64(item.GrowWeight) / totalWeight)
		}
		sizes[i] += growth
		distributed += growth
	}
}

func (spec LayoutSpec) primaryStart() int {
	if spec.Direction == Horizontal {
		return spec.Padding.Left
	}
	return spec.Padding.Top
}

func (spec LayoutSpec) alignCross(alignment Alignment, available, preferred, minimum int) (position, size int) {
	start := spec.Padding.Top
	if spec.Direction == Vertical {
		start = spec.Padding.Left
	}

	switch alignment {
	case AlignStretch:
		return start, max(available, minimum)
	case AlignStart:
		return start, max(minimum, min(preferred, available))
	case AlignCenter:
		size = max(minimum, min(preferred, available))
		return start + (available-size)/2, size
	case AlignEnd:
		size = max(minimum, min(preferred, available))
		return start + available - size, size
	default:
		return start, max(minimum, min(preferred, available))
	}
}

func (spec LayoutSpec) rect(primaryPosition, crossPosition, primarySize, crossSize int) Rect {
	if spec.Direction == Horizontal {
		return Rect{
			Position: Point{X: primaryPosition, Y: crossPosition},
			Size:     Size{Width: primarySize, Height: crossSize},
		}
	}
	return Rect{
		Position: Point{X: crossPosition, Y: primaryPosition},
		Size:     Size{Width: crossSize, Height: primarySize},
	}
}
