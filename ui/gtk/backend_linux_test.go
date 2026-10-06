//go:build linux && cgo && gtk

package gtk_test

import (
	"image"
	"runtime"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/gtk"
)

func TestNativeWidgets(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	backend := &gtk.Backend{}
	window, err := backend.CreateWindow(ui.WindowOptions{
		Title:     "GTK smoke test",
		Size:      ui.Size{Width: 640, Height: 480},
		Resizable: true,
		Hidden:    true,
		Theme:     ui.ThemeSystem,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()

	label, err := window.CreateLabel("Exposure")
	if err != nil {
		t.Fatal(err)
	}
	slider, err := window.CreateSlider(ui.SliderOptions{
		Minimum: 0,
		Maximum: 2,
		Value:   1,
		Step:    0.1,
	})
	if err != nil {
		t.Fatal(err)
	}
	controls, err := window.CreateLayout(ui.Column(
		ui.Item(label),
		ui.Item(slider),
	).WithGap(8).WithPadding(ui.UniformInsets(12)))
	if err != nil {
		t.Fatal(err)
	}
	section, err := window.CreateSection(ui.SectionOptions{
		Title:    "Rendering",
		Content:  controls,
		Expanded: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	canvas, err := window.CreateCanvas()
	if err != nil {
		t.Fatal(err)
	}
	content, err := window.CreateLayout(ui.Row(
		ui.Item(canvas).Grow(2),
		ui.Item(section).Grow(1).MinimumSize(ui.Size{Width: 180}),
	).WithGap(8))
	if err != nil {
		t.Fatal(err)
	}
	area, err := window.CreateTextArea(ui.TextAreaOptions{Text: "Notes", Wrap: true})
	if err != nil {
		t.Fatal(err)
	}
	check, err := window.CreateCheckBox("Enabled", true)
	if err != nil {
		t.Fatal(err)
	}
	radio, err := window.CreateRadioGroup(ui.RadioGroupOptions{Items: []string{"A", "B"}, SelectedIndex: 1})
	if err != nil {
		t.Fatal(err)
	}
	selectControl, err := window.CreateSelect(ui.SelectOptions{Items: []string{"One", "Two"}, SelectedIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	progress, err := window.CreateProgressBar(0.5)
	if err != nil {
		t.Fatal(err)
	}
	activity, err := window.CreateActivityIndicator(true)
	if err != nil {
		t.Fatal(err)
	}
	separator, err := window.CreateSeparator(ui.Horizontal)
	if err != nil {
		t.Fatal(err)
	}
	imageView, err := window.CreateImage(ui.ImageOptions{Source: image.NewRGBA(image.Rect(0, 0, 8, 8))})
	if err != nil {
		t.Fatal(err)
	}
	link, err := window.CreateLink("GameKit", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	newControls, err := window.CreateLayout(ui.Column(
		ui.Item(area), ui.Item(check), ui.Item(radio), ui.Item(selectControl),
		ui.Item(progress), ui.Item(activity), ui.Item(separator), ui.Item(imageView), ui.Item(link),
	))
	if err != nil {
		t.Fatal(err)
	}
	tabs, err := window.CreateTabs(ui.TabsOptions{Items: []ui.TabItem{
		{Title: "Original", Content: content},
		{Title: "New", Content: newControls},
	}, SelectedIndex: 0})
	if err != nil {
		t.Fatal(err)
	}
	scrollLabel, err := window.CreateLabel("Scrollable")
	if err != nil {
		t.Fatal(err)
	}
	scrollContent, err := window.CreateLayout(ui.Column(
		ui.Item(scrollLabel).MinimumSize(ui.Size{Height: 500}),
	))
	if err != nil {
		t.Fatal(err)
	}
	scroll, err := window.CreateScrollView(ui.ScrollViewOptions{Content: scrollContent})
	if err != nil {
		t.Fatal(err)
	}
	defer scroll.Close()
	if scroll.Content() != scrollContent {
		t.Fatal("scroll view did not preserve its content")
	}
	if err := window.SetContent(tabs); err != nil {
		t.Fatal(err)
	}
	panelLabel, err := window.CreateLabel("Drag me")
	if err != nil {
		t.Fatal(err)
	}
	panelContent, err := window.CreateLayout(ui.Column(ui.Item(panelLabel)))
	if err != nil {
		t.Fatal(err)
	}
	panel, err := window.CreatePanel(ui.PanelOptions{
		Title:    "Inspector",
		Content:  panelContent,
		Position: ui.Point{X: 24, Y: 32},
		Size:     ui.Size{Width: 240, Height: 160},
	})
	if err != nil {
		t.Fatal(err)
	}
	window.PollEvents()

	if got := window.NativeSurface(); got.Kind != gpu.NativeSurfaceXlibWindow || got.Handle == 0 || got.Display == 0 {
		t.Fatalf("window surface = %#v, want a complete Xlib surface", got)
	}
	if got := canvas.NativeSurface(); got.Kind != gpu.NativeSurfaceXlibWindow || got.Handle == 0 || got.Display == 0 {
		t.Fatalf("canvas surface = %#v, want a complete Xlib surface", got)
	}
	if label.Text() != "Exposure" {
		t.Fatalf("label text = %q, want %q", label.Text(), "Exposure")
	}
	if slider.Value() != 1 {
		t.Fatalf("slider value = %v, want 1", slider.Value())
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
	if !section.IsExpanded() {
		t.Fatal("section should initially be expanded")
	}
	if panel.Title() != "Inspector" || panel.Content() != panelContent || !panel.IsMovable() {
		t.Fatal("panel did not preserve its initial values")
	}
	if panel.Bounds() != (ui.Rect{
		Position: ui.Point{X: 24, Y: 32},
		Size:     ui.Size{Width: 240, Height: 160},
	}) {
		t.Fatalf("panel bounds = %#v, want position 24,32 and size 240x160", panel.Bounds())
	}
	panel.SetTitle("Tools")
	panel.SetMovable(false)
	panel.BringToFront()
	if panel.Title() != "Tools" || panel.IsMovable() {
		t.Fatal("panel setters did not update its state")
	}
	section.SetExpanded(false)
	if section.IsExpanded() {
		t.Fatal("section should be collapsed")
	}
	canvas.RequestRedraw()
	if !canvas.TakeRedrawRequest() || canvas.TakeRedrawRequest() {
		t.Fatal("canvas redraw request should be consumed once")
	}
	if err := window.SetTheme(ui.ThemeDark); err != nil {
		t.Fatal(err)
	}
}
