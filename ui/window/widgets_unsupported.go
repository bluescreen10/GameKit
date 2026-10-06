//go:build (!darwin && !windows) || !cgo

package window

import "github.com/bluescreen10/gamekit/ui"

// SetContent sets the native control that fills the window's content area.
func (w *Window) SetContent(content ui.Widget) error {
	return ui.ErrNotSupported
}

// CreateLayout creates a native layout owned by the window.
func (w *Window) CreateLayout(spec ui.LayoutSpec) (ui.Layout, error) {
	return nil, ui.ErrNotSupported
}

// CreateLabel creates a native label owned by the window.
func (w *Window) CreateLabel(text string) (ui.Label, error) {
	return nil, ui.ErrNotSupported
}

// CreateButton creates a native push button owned by the window.
func (w *Window) CreateButton(text string) (ui.Button, error) {
	return nil, ui.ErrNotSupported
}

// CreateTextField creates a native single-line text editor owned by the window.
func (w *Window) CreateTextField() (ui.TextField, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateTextArea(ui.TextAreaOptions) (ui.TextArea, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateCheckBox(string, bool) (ui.CheckBox, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateRadioGroup(ui.RadioGroupOptions) (ui.RadioGroup, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateSelect(ui.SelectOptions) (ui.Select, error) {
	return nil, ui.ErrNotSupported
}

// CreateSlider creates a native slider owned by the window.
func (w *Window) CreateSlider(options ui.SliderOptions) (ui.Slider, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateProgressBar(float64) (ui.ProgressBar, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateActivityIndicator(bool) (ui.ActivityIndicator, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateSeparator(ui.LayoutDirection) (ui.Separator, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateImage(ui.ImageOptions) (ui.ImageView, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateLink(string, string) (ui.Link, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateTabs(ui.TabsOptions) (ui.Tabs, error) {
	return nil, ui.ErrNotSupported
}

func (w *Window) CreateScrollView(ui.ScrollViewOptions) (ui.ScrollView, error) {
	return nil, ui.ErrNotSupported
}

// CreateCanvas creates a GPU-presentable native control owned by the window.
func (w *Window) CreateCanvas() (ui.Canvas, error) {
	return nil, ui.ErrNotSupported
}

// CreatePanel creates a floating native panel owned by the window.
func (w *Window) CreatePanel(options ui.PanelOptions) (ui.Panel, error) {
	return nil, ui.ErrNotSupported
}

// CreateSection creates a collapsible native section owned by the window.
func (w *Window) CreateSection(options ui.SectionOptions) (ui.Section, error) {
	return nil, ui.ErrNotSupported
}
