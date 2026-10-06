package ui

import (
	"errors"
	"fmt"
	stdimage "image"
	"math"
)

// ClickHandler receives a native button activation.
type ClickHandler func()

// TextChangeHandler receives the current text after a text field changes.
type TextChangeHandler func(text string)

// ValueChangeHandler receives the current value after a value control changes.
type ValueChangeHandler func(value float64)

// ResizeHandler receives a drawable's new size in logical pixels.
type ResizeHandler func(size Size)

// MoveHandler receives a floating panel's new position in logical pixels.
type MoveHandler func(position Point)

// ExpansionChangeHandler receives a section's expanded state after it changes.
type ExpansionChangeHandler func(expanded bool)

// ToggleHandler receives the current state after a toggle control changes.
type ToggleHandler func(checked bool)

// SelectionChangeHandler receives the selected item index after it changes.
type SelectionChangeHandler func(index int)

// Widget is a native control owned by a window. Bounds use logical pixels in
// the coordinate system of the widget's parent.
type Widget interface {
	Close()
	Bounds() Rect
	SetBounds(bounds Rect)
	PreferredSize() Size

	IsVisible() bool
	SetVisible(visible bool)
	IsEnabled() bool
	SetEnabled(enabled bool)
}

// Layout is a native container whose LayoutSpec arranges its children. A child
// can belong to only one layout at a time.
type Layout interface {
	Widget
	Spec() LayoutSpec
	SetSpec(spec LayoutSpec) error
	Children() []Widget
}

// Label displays non-editable text.
type Label interface {
	Widget
	Text() string
	SetText(text string)
}

// Button is a native push button.
type Button interface {
	Widget
	Text() string
	SetText(text string)
	OnClick(handler ClickHandler) Unsubscribe
}

// TextField is a native single-line text editor. OnChange handlers receive user
// edits; SetText does not invoke them.
type TextField interface {
	Widget
	Text() string
	SetText(text string)
	Placeholder() string
	SetPlaceholder(placeholder string)
	OnChange(handler TextChangeHandler) Unsubscribe
}

// TextAreaOptions describes a native multiline text editor.
type TextAreaOptions struct {
	Text        string
	Placeholder string
	ReadOnly    bool
	Wrap        bool
}

// TextArea is a native multiline text editor. OnChange handlers receive user
// edits; SetText does not invoke them.
type TextArea interface {
	Widget
	Text() string
	SetText(text string)
	Placeholder() string
	SetPlaceholder(placeholder string)
	IsReadOnly() bool
	SetReadOnly(readOnly bool)
	OnChange(handler TextChangeHandler) Unsubscribe
}

// CheckBox is a native independently checked control.
type CheckBox interface {
	Widget
	Text() string
	SetText(text string)
	IsChecked() bool
	SetChecked(checked bool)
	OnChange(handler ToggleHandler) Unsubscribe
}

// RadioGroupOptions describes a native group of mutually exclusive choices.
type RadioGroupOptions struct {
	Items         []string
	SelectedIndex int
}

// Validate checks that the group has choices and a valid selection.
func (options RadioGroupOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: radio group requires at least one item")
	}
	if options.SelectedIndex < 0 || options.SelectedIndex >= len(options.Items) {
		return errors.New("ui: radio group selection is out of range")
	}
	return nil
}

// RadioGroup is a native group of mutually exclusive choices.
type RadioGroup interface {
	Widget
	Items() []string
	SelectedIndex() int
	SetSelectedIndex(index int) error
	OnChange(handler SelectionChangeHandler) Unsubscribe
}

// SelectOptions describes a native drop-down selection control.
type SelectOptions struct {
	Items         []string
	SelectedIndex int
}

// Validate checks that the drop-down has choices and a valid selection.
func (options SelectOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: select requires at least one item")
	}
	if options.SelectedIndex < 0 || options.SelectedIndex >= len(options.Items) {
		return errors.New("ui: select selection is out of range")
	}
	return nil
}

// Select is a native drop-down selection control.
type Select interface {
	Widget
	Items() []string
	SelectedIndex() int
	SetSelectedIndex(index int) error
	OnChange(handler SelectionChangeHandler) Unsubscribe
}

// SliderOptions describes a native slider. Minimum must be less than Maximum,
// and Value must fall within that range.
type SliderOptions struct {
	Minimum float64
	Maximum float64
	Value   float64
	Step    float64
}

// Validate checks that the slider range and initial value are usable.
func (options SliderOptions) Validate() error {
	if math.IsNaN(options.Minimum) || math.IsInf(options.Minimum, 0) ||
		math.IsNaN(options.Maximum) || math.IsInf(options.Maximum, 0) {
		return errors.New("ui: slider bounds must be finite")
	}
	if options.Minimum >= options.Maximum {
		return errors.New("ui: slider minimum must be less than maximum")
	}
	if math.IsNaN(options.Value) || math.IsInf(options.Value, 0) ||
		options.Value < options.Minimum || options.Value > options.Maximum {
		return errors.New("ui: slider value must be within its range")
	}
	if math.IsNaN(options.Step) || math.IsInf(options.Step, 0) || options.Step < 0 {
		return errors.New("ui: slider step must be finite and non-negative")
	}
	return nil
}

// Slider is a native scalar-value control. OnChange handlers receive user
// changes; SetValue does not invoke them.
type Slider interface {
	Widget
	Value() float64
	SetValue(value float64)
	OnChange(handler ValueChangeHandler) Unsubscribe
}

// ProgressBar displays completion in the inclusive range zero through one.
type ProgressBar interface {
	Widget
	Value() float64
	SetValue(value float64)
}

// ActivityIndicator displays indeterminate ongoing activity.
type ActivityIndicator interface {
	Widget
	IsRunning() bool
	SetRunning(running bool)
}

// Separator is a native horizontal or vertical visual divider.
type Separator interface {
	Widget
	Direction() LayoutDirection
}

// ImageScaling controls how an image is fitted into its widget bounds.
type ImageScaling uint8

const (
	// ImageScaleFit preserves aspect ratio and shows the entire image.
	ImageScaleFit ImageScaling = iota
	// ImageScaleFill preserves aspect ratio and fills the bounds, cropping if needed.
	ImageScaleFill
	// ImageScaleStretch fills the bounds without preserving aspect ratio.
	ImageScaleStretch
)

// ImageOptions describes a native image view.
type ImageOptions struct {
	Source  stdimage.Image
	Scaling ImageScaling
}

// Validate checks that the image and scaling mode are usable.
func (options ImageOptions) Validate() error {
	if options.Source == nil {
		return errors.New("ui: image source is required")
	}
	if options.Source.Bounds().Empty() {
		return errors.New("ui: image source is empty")
	}
	if options.Scaling > ImageScaleStretch {
		return fmt.Errorf("ui: invalid image scaling %d", options.Scaling)
	}
	return nil
}

// ImageView displays an image using native drawing and scaling.
type ImageView interface {
	Widget
	SetImage(source stdimage.Image) error
	Scaling() ImageScaling
	SetScaling(scaling ImageScaling) error
}

// Link is a native hyperlink control. OnClick handlers receive activations;
// opening the URL is left to the application.
type Link interface {
	Widget
	Text() string
	SetText(text string)
	URL() string
	SetURL(url string)
	OnClick(handler ClickHandler) Unsubscribe
}

// TabItem describes one page in a native tab view.
type TabItem struct {
	Title   string
	Content Widget
}

// TabsOptions describes a native tab view.
type TabsOptions struct {
	Items         []TabItem
	SelectedIndex int
}

// Validate checks that every tab has content and the selection is valid.
func (options TabsOptions) Validate() error {
	if len(options.Items) == 0 {
		return errors.New("ui: tabs require at least one item")
	}
	if options.SelectedIndex < 0 || options.SelectedIndex >= len(options.Items) {
		return errors.New("ui: tab selection is out of range")
	}
	for i, item := range options.Items {
		if item.Content == nil {
			return fmt.Errorf("ui: tab %d has no content", i)
		}
	}
	return nil
}

// Tabs is a native tab view containing one visible page at a time.
type Tabs interface {
	Widget
	Items() []TabItem
	SelectedIndex() int
	SetSelectedIndex(index int) error
	OnChange(handler SelectionChangeHandler) Unsubscribe
}

// ScrollAxes controls which directions a scroll view can move through its
// content. The zero value enables vertical scrolling.
type ScrollAxes uint8

const (
	ScrollVertical ScrollAxes = iota
	ScrollHorizontal
	ScrollBoth
)

// ScrollViewOptions describes a native scrolling viewport.
type ScrollViewOptions struct {
	Content Widget
	Axes    ScrollAxes
}

// Validate checks that the scroll view has content and recognized axes.
func (options ScrollViewOptions) Validate() error {
	if options.Content == nil {
		return errors.New("ui: scroll view content is required")
	}
	if options.Axes > ScrollBoth {
		return fmt.Errorf("ui: invalid scroll axes %d", options.Axes)
	}
	return nil
}

// ScrollView displays content through a native scrolling viewport. ScrollOffset
// is measured in logical pixels from the content's top-left corner.
type ScrollView interface {
	Widget
	Content() Widget
	ScrollOffset() Point
	SetScrollOffset(offset Point)
}

// Canvas is a native control backed by a GPU-presentable surface. It can be
// placed beside ordinary controls in the same window. RequestRedraw marks it
// dirty; TakeRedrawRequest reports and clears that state.
type Canvas interface {
	Widget
	Drawable
	OnResize(handler ResizeHandler) Unsubscribe
	RequestRedraw()
	TakeRedrawRequest() bool
}

// PanelOptions describes a floating panel layered above a window's content.
// Size must be valid. A panel is movable unless Fixed is true.
type PanelOptions struct {
	Title    string
	Content  Widget
	Position Point
	Size     Size
	Fixed    bool
}

// Validate checks that the panel has content and a usable size.
func (options PanelOptions) Validate() error {
	if options.Content == nil {
		return errors.New("ui: panel content is required")
	}
	if !options.Size.IsValid() {
		return errors.New("ui: panel size must be positive")
	}
	return nil
}

// Panel is a native container that floats above a window's content. It is not
// arranged by the window's root Layout and can be dragged when it is movable.
type Panel interface {
	Widget
	Title() string
	SetTitle(title string)
	Content() Widget
	SetContent(content Widget) error
	IsMovable() bool
	SetMovable(movable bool)
	BringToFront()
	OnMove(handler MoveHandler) Unsubscribe
}

// SectionOptions describes a titled, collapsible native container.
type SectionOptions struct {
	Title    string
	Content  Widget
	Expanded bool
}

// Validate checks that the section has content.
func (options SectionOptions) Validate() error {
	if options.Content == nil {
		return errors.New("ui: section content is required")
	}
	return nil
}

// Section is a titled container whose content can be expanded or collapsed.
// Multiple sections in a column form an accordion.
type Section interface {
	Widget
	Title() string
	SetTitle(title string)
	Content() Widget
	IsExpanded() bool
	SetExpanded(expanded bool)
	OnChange(handler ExpansionChangeHandler) Unsubscribe
}
