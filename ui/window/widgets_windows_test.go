//go:build windows && cgo

package window_test

import (
	"image"
	"runtime"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/window"
)

func TestWin32NativeWidgets(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	backend := &window.Backend{}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "Win32 widget test",
		Size:   ui.Size{Width: 600, Height: 300},
		Hidden: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer win.Close()

	label, err := win.CreateLabel("Material")
	if err != nil {
		t.Fatal(err)
	}
	field, err := win.CreateTextField()
	if err != nil {
		t.Fatal(err)
	}
	field.SetPlaceholder("Name")
	slider, err := win.CreateSlider(ui.SliderOptions{
		Minimum: 0,
		Maximum: 4,
		Value:   1,
		Step:    0.25,
	})
	if err != nil {
		t.Fatal(err)
	}
	canvas, err := win.CreateCanvas()
	if err != nil {
		t.Fatal(err)
	}
	sidebar, err := win.CreateLayout(ui.Column(
		ui.Item(label),
		ui.Item(field),
		ui.Item(slider),
		ui.Space(),
	).WithPadding(ui.UniformInsets(16)).WithGap(12))
	if err != nil {
		t.Fatal(err)
	}
	root, err := win.CreateLayout(ui.Row(
		ui.Item(canvas).Grow(2),
		ui.Item(sidebar).Grow(1),
	))
	if err != nil {
		t.Fatal(err)
	}
	area, err := win.CreateTextArea(ui.TextAreaOptions{Text: "Notes", Wrap: true})
	if err != nil {
		t.Fatal(err)
	}
	check, err := win.CreateCheckBox("Enabled", true)
	if err != nil {
		t.Fatal(err)
	}
	radio, err := win.CreateRadioGroup(ui.RadioGroupOptions{Items: []string{"A", "B"}, SelectedIndex: 1})
	if err != nil {
		t.Fatal(err)
	}
	selectControl, err := win.CreateSelect(ui.SelectOptions{Items: []string{"One", "Two"}, SelectedIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	progress, err := win.CreateProgressBar(0.5)
	if err != nil {
		t.Fatal(err)
	}
	activity, err := win.CreateActivityIndicator(true)
	if err != nil {
		t.Fatal(err)
	}
	separator, err := win.CreateSeparator(ui.Horizontal)
	if err != nil {
		t.Fatal(err)
	}
	imageView, err := win.CreateImage(ui.ImageOptions{Source: image.NewRGBA(image.Rect(0, 0, 8, 8))})
	if err != nil {
		t.Fatal(err)
	}
	link, err := win.CreateLink("GameKit", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	newControls, err := win.CreateLayout(ui.Column(
		ui.Item(area), ui.Item(check), ui.Item(radio), ui.Item(selectControl),
		ui.Item(progress), ui.Item(activity), ui.Item(separator), ui.Item(imageView), ui.Item(link),
	))
	if err != nil {
		t.Fatal(err)
	}
	tabs, err := win.CreateTabs(ui.TabsOptions{Items: []ui.TabItem{
		{Title: "Original", Content: root},
		{Title: "New", Content: newControls},
	}, SelectedIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	scrollLabel, err := win.CreateLabel("Scrollable")
	if err != nil {
		t.Fatal(err)
	}
	scrollContent, err := win.CreateLayout(ui.Column(
		ui.Item(scrollLabel).MinimumSize(ui.Size{Height: 500}),
	))
	if err != nil {
		t.Fatal(err)
	}
	scroll, err := win.CreateScrollView(ui.ScrollViewOptions{Content: scrollContent})
	if err != nil {
		t.Fatal(err)
	}
	defer scroll.Close()
	scroll.SetBounds(ui.Rect{Size: ui.Size{Width: 200, Height: 100}})
	scroll.SetScrollOffset(ui.Point{Y: 64})
	if scroll.Content() != scrollContent || scroll.ScrollOffset().Y != 64 {
		t.Fatalf("scroll view = (%T, %v), want its content at offset 64", scroll.Content(), scroll.ScrollOffset())
	}
	if err := win.SetContent(tabs); err != nil {
		t.Fatal(err)
	}

	sectionContent, err := win.CreateLayout(ui.Column(ui.Item(label)))
	if err == nil {
		t.Fatal("CreateLayout accepted a child that already has a parent")
	}
	_ = sectionContent
	panelLabel, err := win.CreateLabel("Exposure")
	if err != nil {
		t.Fatal(err)
	}
	sectionBody, err := win.CreateLayout(ui.Column(ui.Item(panelLabel)))
	if err != nil {
		t.Fatal(err)
	}
	section, err := win.CreateSection(ui.SectionOptions{
		Title:    "Rendering",
		Content:  sectionBody,
		Expanded: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	panelBody, err := win.CreateLayout(ui.Column(ui.Item(section), ui.Space()))
	if err != nil {
		t.Fatal(err)
	}
	panel, err := win.CreatePanel(ui.PanelOptions{
		Title:    "Inspector",
		Content:  panelBody,
		Position: ui.Point{X: 24, Y: 20},
		Size:     ui.Size{Width: 240, Height: 180},
	})
	if err != nil {
		t.Fatal(err)
	}

	if label.Text() != "Material" || field.Placeholder() != "Name" || slider.Value() != 1 {
		t.Fatalf("native values = (%q, %q, %v)", label.Text(), field.Placeholder(), slider.Value())
	}
	if area.Text() != "Notes" || !check.IsChecked() || radio.SelectedIndex() != 1 ||
		selectControl.SelectedIndex() != 0 || progress.Value() != 0.5 || !activity.IsRunning() {
		t.Fatal("expanded native controls did not preserve their initial values")
	}
	if link.URL() != "https://example.com" || tabs.SelectedIndex() != 0 {
		t.Fatal("native link or tabs did not preserve their initial value")
	}
	if err := tabs.SetSelectedIndex(1); err != nil || tabs.SelectedIndex() != 1 {
		t.Fatalf("SetSelectedIndex(1) = %v, selected %d", err, tabs.SelectedIndex())
	}
	if target := canvas.NativeSurface(); target.Kind != gpu.NativeSurfaceWin32Window || target.Handle == 0 {
		t.Fatalf("canvas target = %#v, want a valid child HWND", target)
	}
	if panel.Title() != "Inspector" || !panel.IsMovable() {
		t.Fatalf("panel = (%q, movable %v)", panel.Title(), panel.IsMovable())
	}
	if !section.IsExpanded() {
		t.Fatal("section is collapsed, want expanded")
	}
	section.SetExpanded(false)
	if section.IsExpanded() {
		t.Fatal("section remained expanded")
	}
	if err := win.SetTheme(ui.ThemeDark); err != nil {
		t.Fatal(err)
	}
	win.PollEvents()
}
