//go:build darwin && cgo

package window_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/bluescreen10/gamekit/gpu"
	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/window"
)

var cocoaWidgets struct {
	err error

	rootBounds    ui.Rect
	canvasBounds  ui.Rect
	sidebarBounds ui.Rect
	labelText     string
	buttonText    string
	placeholder   string
	sliderValue   float64
	framebuffer   ui.Size
	windowTarget  gpu.NativeSurface
	canvasTarget  gpu.NativeSurface
	panelTitle    string
	panelMovable  bool
	panelBounds   ui.Rect
	sectionTitle  string
	sectionOpen   bool
	expandedSize  ui.Size
	collapsedSize ui.Size
	darkTheme     ui.Theme
}

var cocoaWindow ui.Window

func TestMain(m *testing.M) {
	backend := &window.Backend{}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:  "ui widget test",
		Size:   ui.Size{Width: 600, Height: 300},
		Hidden: true,
	})
	if err == nil {
		cocoaWindow = win
		err = buildCocoaWidgetTest(win)
	}
	cocoaWidgets.err = err

	code := m.Run()
	if win != nil {
		win.Close()
	}
	os.Exit(code)
}

func TestCocoaDialogHonorsCancelledContext(t *testing.T) {
	if cocoaWidgets.err != nil {
		t.Fatal(cocoaWidgets.err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := cocoaWindow.OpenFile(ctx, ui.OpenFileOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("OpenFile() error = %v, want context.Canceled", err)
	}
}

func buildCocoaWidgetTest(win ui.Window) error {
	label, err := win.CreateLabel("Material")
	if err != nil {
		return err
	}
	button, err := win.CreateButton("Reset")
	if err != nil {
		return err
	}
	field, err := win.CreateTextField()
	if err != nil {
		return err
	}
	field.SetPlaceholder("Name")
	slider, err := win.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 4, Value: 1, Step: 0.25})
	if err != nil {
		return err
	}
	canvas, err := win.CreateCanvas()
	if err != nil {
		return err
	}

	sidebar, err := win.CreateLayout(
		ui.Column(
			ui.Item(label),
			ui.Item(button),
			ui.Item(field),
			ui.Item(slider),
			ui.Space(),
		).WithPadding(ui.UniformInsets(16)).WithGap(12),
	)
	if err != nil {
		return err
	}
	root, err := win.CreateLayout(ui.Row(
		ui.Item(canvas).Grow(2),
		ui.Item(sidebar).Grow(1),
	))
	if err != nil {
		return err
	}
	if err := win.SetContent(root); err != nil {
		return err
	}

	cocoaWidgets.rootBounds = root.Bounds()
	cocoaWidgets.canvasBounds = canvas.Bounds()
	cocoaWidgets.sidebarBounds = sidebar.Bounds()
	cocoaWidgets.labelText = label.Text()
	cocoaWidgets.buttonText = button.Text()
	cocoaWidgets.placeholder = field.Placeholder()
	cocoaWidgets.sliderValue = slider.Value()
	cocoaWidgets.framebuffer = canvas.FramebufferSize()
	cocoaWidgets.windowTarget = win.NativeSurface()
	cocoaWidgets.canvasTarget = canvas.NativeSurface()

	panelLabel, err := win.CreateLabel("Exposure")
	if err != nil {
		return err
	}
	sectionContent, err := win.CreateLayout(ui.Column(ui.Item(panelLabel)).WithPadding(ui.UniformInsets(8)))
	if err != nil {
		return err
	}
	section, err := win.CreateSection(ui.SectionOptions{
		Title:    "Rendering",
		Content:  sectionContent,
		Expanded: true,
	})
	if err != nil {
		return err
	}
	panelContent, err := win.CreateLayout(ui.Column(ui.Item(section), ui.Space()))
	if err != nil {
		return err
	}
	panel, err := win.CreatePanel(ui.PanelOptions{
		Title:    "Inspector",
		Content:  panelContent,
		Position: ui.Point{X: 24, Y: 20},
		Size:     ui.Size{Width: 240, Height: 180},
	})
	if err != nil {
		return err
	}
	cocoaWidgets.panelTitle = panel.Title()
	cocoaWidgets.panelMovable = panel.IsMovable()
	cocoaWidgets.panelBounds = panel.Bounds()
	panel.BringToFront()
	cocoaWidgets.sectionTitle = section.Title()
	cocoaWidgets.sectionOpen = section.IsExpanded()
	cocoaWidgets.expandedSize = section.PreferredSize()
	section.SetExpanded(false)
	cocoaWidgets.collapsedSize = section.PreferredSize()
	section.SetExpanded(true)
	if err := win.SetTheme(ui.ThemeDark); err != nil {
		return err
	}
	cocoaWidgets.darkTheme = win.Theme()
	if err := win.SetTheme(ui.ThemeSystem); err != nil {
		return err
	}
	return testExpandedCocoaWidgets(win)
}

func testExpandedCocoaWidgets(win ui.Window) error {
	area, err := win.CreateTextArea(ui.TextAreaOptions{Text: "Notes", Wrap: true})
	if err != nil {
		return err
	}
	check, err := win.CreateCheckBox("Enabled", true)
	if err != nil {
		return err
	}
	radio, err := win.CreateRadioGroup(ui.RadioGroupOptions{Items: []string{"A", "B"}, SelectedIndex: 1})
	if err != nil {
		return err
	}
	selectControl, err := win.CreateSelect(ui.SelectOptions{Items: []string{"One", "Two"}, SelectedIndex: 0})
	if err != nil {
		return err
	}
	progress, err := win.CreateProgressBar(0.5)
	if err != nil {
		return err
	}
	activity, err := win.CreateActivityIndicator(true)
	if err != nil {
		return err
	}
	separator, err := win.CreateSeparator(ui.Horizontal)
	if err != nil {
		return err
	}
	imageView, err := win.CreateImage(ui.ImageOptions{Source: image.NewRGBA(image.Rect(0, 0, 8, 8))})
	if err != nil {
		return err
	}
	link, err := win.CreateLink("GameKit", "https://example.com")
	if err != nil {
		return err
	}
	page, err := win.CreateLayout(ui.Column(
		ui.Item(area), ui.Item(check), ui.Item(radio), ui.Item(selectControl),
		ui.Item(progress), ui.Item(activity), ui.Item(separator), ui.Item(imageView), ui.Item(link),
	))
	if err != nil {
		return err
	}
	secondLabel, err := win.CreateLabel("Second")
	if err != nil {
		return err
	}
	second, err := win.CreateLayout(ui.Column(ui.Item(secondLabel)))
	if err != nil {
		return err
	}
	tabs, err := win.CreateTabs(ui.TabsOptions{Items: []ui.TabItem{
		{Title: "New", Content: page},
		{Title: "Second", Content: second},
	}, SelectedIndex: 0})
	if err != nil {
		return err
	}
	defer tabs.Close()
	tabs.SetBounds(ui.Rect{Size: ui.Size{Width: 500, Height: 280}})
	scrollLabel, err := win.CreateLabel("Scrollable")
	if err != nil {
		return err
	}
	scrollContent, err := win.CreateLayout(ui.Column(
		ui.Item(scrollLabel).MinimumSize(ui.Size{Height: 500}),
	))
	if err != nil {
		return err
	}
	scroll, err := win.CreateScrollView(ui.ScrollViewOptions{Content: scrollContent})
	if err != nil {
		return err
	}
	defer scroll.Close()
	scroll.SetBounds(ui.Rect{Size: ui.Size{Width: 200, Height: 100}})
	scroll.SetScrollOffset(ui.Point{Y: 64})
	if scroll.Content() != scrollContent || scroll.ScrollOffset().Y != 64 {
		return fmt.Errorf("Cocoa scroll view offset = %v, want 64", scroll.ScrollOffset())
	}
	if area.Text() != "Notes" || !check.IsChecked() || radio.SelectedIndex() != 1 ||
		selectControl.SelectedIndex() != 0 || progress.Value() != 0.5 || !activity.IsRunning() {
		return errors.New("expanded Cocoa controls did not preserve their initial values")
	}
	if link.URL() != "https://example.com" || tabs.SelectedIndex() != 0 {
		return errors.New("Cocoa link or tabs did not preserve their initial value")
	}
	activity.SetRunning(false)
	if activity.IsRunning() {
		return errors.New("Cocoa activity indicator remained running")
	}
	if err := tabs.SetSelectedIndex(1); err != nil || tabs.SelectedIndex() != 1 {
		return fmt.Errorf("Cocoa tabs selection = %d: %w", tabs.SelectedIndex(), err)
	}
	return nil
}

func TestCocoaNativeWidgets(t *testing.T) {
	if cocoaWidgets.err != nil {
		t.Fatal(cocoaWidgets.err)
	}
	if cocoaWidgets.labelText != "Material" {
		t.Errorf("label text = %q, want Material", cocoaWidgets.labelText)
	}
	if cocoaWidgets.buttonText != "Reset" {
		t.Errorf("button text = %q, want Reset", cocoaWidgets.buttonText)
	}
	if cocoaWidgets.placeholder != "Name" {
		t.Errorf("text field placeholder = %q, want Name", cocoaWidgets.placeholder)
	}
	if cocoaWidgets.sliderValue != 1 {
		t.Errorf("slider value = %v, want 1", cocoaWidgets.sliderValue)
	}
}

func TestCocoaSurfaceTargets(t *testing.T) {
	if cocoaWidgets.err != nil {
		t.Fatal(cocoaWidgets.err)
	}
	if cocoaWidgets.windowTarget.Kind != gpu.NativeSurfaceCocoaWindow || cocoaWidgets.windowTarget.Handle == 0 {
		t.Errorf("window target = %#v, want a valid Cocoa window", cocoaWidgets.windowTarget)
	}
	if cocoaWidgets.canvasTarget.Kind != gpu.NativeSurfaceCocoaView || cocoaWidgets.canvasTarget.Handle == 0 {
		t.Errorf("canvas target = %#v, want a valid Cocoa view", cocoaWidgets.canvasTarget)
	}
}

func TestCocoaFloatingPanel(t *testing.T) {
	if cocoaWidgets.err != nil {
		t.Fatal(cocoaWidgets.err)
	}
	if cocoaWidgets.panelTitle != "Inspector" {
		t.Errorf("panel title = %q, want Inspector", cocoaWidgets.panelTitle)
	}
	if !cocoaWidgets.panelMovable {
		t.Error("panel is not movable by default")
	}
	if cocoaWidgets.panelBounds != (ui.Rect{
		Position: ui.Point{X: 24, Y: 20},
		Size:     ui.Size{Width: 240, Height: 180},
	}) {
		t.Errorf("panel bounds = %#v, want position 24,20 and size 240x180", cocoaWidgets.panelBounds)
	}
}

func TestCocoaCollapsibleSection(t *testing.T) {
	if cocoaWidgets.err != nil {
		t.Fatal(cocoaWidgets.err)
	}
	if cocoaWidgets.sectionTitle != "Rendering" || !cocoaWidgets.sectionOpen {
		t.Fatalf("section = (%q, expanded %v), want (Rendering, true)", cocoaWidgets.sectionTitle, cocoaWidgets.sectionOpen)
	}
	if cocoaWidgets.collapsedSize.Height >= cocoaWidgets.expandedSize.Height {
		t.Errorf("collapsed height = %d, want less than expanded height %d", cocoaWidgets.collapsedSize.Height, cocoaWidgets.expandedSize.Height)
	}
}

func TestCocoaWindowTheme(t *testing.T) {
	if cocoaWidgets.err != nil {
		t.Fatal(cocoaWidgets.err)
	}
	if cocoaWidgets.darkTheme != ui.ThemeDark {
		t.Errorf("theme = %v, want ThemeDark", cocoaWidgets.darkTheme)
	}
}

func TestCocoaNativeLayout(t *testing.T) {
	if cocoaWidgets.err != nil {
		t.Fatal(cocoaWidgets.err)
	}
	if cocoaWidgets.rootBounds.Size != (ui.Size{Width: 600, Height: 300}) {
		t.Errorf("root size = %#v, want 600 by 300", cocoaWidgets.rootBounds.Size)
	}
	if cocoaWidgets.canvasBounds.Size.Width != 400 {
		t.Errorf("canvas width = %d, want 400", cocoaWidgets.canvasBounds.Size.Width)
	}
	if cocoaWidgets.sidebarBounds != (ui.Rect{
		Position: ui.Point{X: 400},
		Size:     ui.Size{Width: 200, Height: 300},
	}) {
		t.Errorf("sidebar bounds = %#v, want right third", cocoaWidgets.sidebarBounds)
	}
	if !cocoaWidgets.framebuffer.IsValid() {
		t.Errorf("canvas framebuffer size = %#v, want positive dimensions", cocoaWidgets.framebuffer)
	}
}
