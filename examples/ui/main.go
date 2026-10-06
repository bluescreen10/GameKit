package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"runtime"
	"time"

	"github.com/bluescreen10/gamekit/ui"
	"github.com/bluescreen10/gamekit/ui/gtk"
	"github.com/bluescreen10/gamekit/ui/window"
)

func main() {
	runtime.LockOSThread()

	var backend ui.Backend = &window.Backend{}
	if runtime.GOOS == "linux" {
		backend = &gtk.Backend{}
	}
	win, err := backend.CreateWindow(ui.WindowOptions{
		Title:     "GameKit widget tester",
		Size:      ui.Size{Width: 980, Height: 700},
		Resizable: true,
		Theme:     ui.ThemeSystem,
	})
	if err != nil {
		panic(err)
	}
	defer win.Close()

	if err := buildWidgetTester(win); err != nil {
		panic(err)
	}
	for !win.ShouldClose() {
		win.PollEvents()
		time.Sleep(time.Second / 120)
	}
}

func buildWidgetTester(win ui.Window) error {
	status, err := win.CreateLabel("Ready")
	if err != nil {
		return err
	}
	controlsPage, err := createControlsPage(win, status)
	if err != nil {
		return err
	}
	selectionPage, err := createSelectionPage(win, status)
	if err != nil {
		return err
	}
	visualPage, err := createVisualPage(win, status)
	if err != nil {
		return err
	}
	scrollingPage, err := createScrollingPage(win, status)
	if err != nil {
		return err
	}
	canvasPage, err := createCanvasPage(win, status)
	if err != nil {
		return err
	}

	tabs, err := win.CreateTabs(ui.TabsOptions{
		Items: []ui.TabItem{
			{Title: "Controls", Content: controlsPage},
			{Title: "Selection", Content: selectionPage},
			{Title: "Visual", Content: visualPage},
			{Title: "Scrolling", Content: scrollingPage},
			{Title: "Canvas", Content: canvasPage},
		},
		SelectedIndex: 0,
	})
	if err != nil {
		return err
	}
	tabs.OnChange(func(index int) {
		status.SetText(fmt.Sprintf("Selected tab: %s", tabs.Items()[index].Title))
	})

	theme, err := win.CreateButton("Theme: system")
	if err != nil {
		return err
	}
	open, err := win.CreateButton("Open file…")
	if err != nil {
		return err
	}
	quit, err := win.CreateButton("Quit")
	if err != nil {
		return err
	}
	footerButtons, err := win.CreateLayout(ui.Row(
		ui.Item(theme), ui.Item(open), ui.Space(), ui.Item(quit),
	).WithGap(8))
	if err != nil {
		return err
	}
	root, err := win.CreateLayout(ui.Column(
		ui.Item(tabs).Grow(1),
		ui.Item(status),
		ui.Item(footerButtons),
	).WithPadding(ui.UniformInsets(12)).WithGap(10))
	if err != nil {
		return err
	}
	if err := win.SetContent(root); err != nil {
		return err
	}

	panelLabel, err := win.CreateLabel("Drag this native panel by its title bar.")
	if err != nil {
		return err
	}
	panelBody, err := win.CreateLayout(
		ui.Column(ui.Item(panelLabel), ui.Space()).WithPadding(ui.UniformInsets(8)),
	)
	if err != nil {
		return err
	}
	panel, err := win.CreatePanel(ui.PanelOptions{
		Title:    "Floating panel",
		Content:  panelBody,
		Position: ui.Point{X: 28, Y: 56},
		Size:     ui.Size{Width: 280, Height: 130},
	})
	if err == nil {
		panel.OnMove(func(position ui.Point) {
			status.SetText(fmt.Sprintf("Panel position: %d, %d", position.X, position.Y))
		})
	} else if errors.Is(err, ui.ErrNotSupported) {
		panelBody.Close()
	} else {
		return err
	}

	themes := []struct {
		value ui.Theme
		name  string
	}{
		{ui.ThemeSystem, "system"},
		{ui.ThemeLight, "light"},
		{ui.ThemeDark, "dark"},
	}
	themeIndex := 0
	theme.OnClick(func() {
		themeIndex = (themeIndex + 1) % len(themes)
		selected := themes[themeIndex]
		if themeErr := win.SetTheme(selected.value); themeErr != nil {
			status.SetText("Theme: " + themeErr.Error())
			return
		}
		theme.SetText("Theme: " + selected.name)
	})
	open.OnClick(func() {
		path, dialogErr := win.OpenFile(context.Background(), ui.OpenFileOptions{
			Title: "Choose a file",
			Filters: []ui.FileFilter{
				{Name: "Images", Patterns: []string{"*.png", "*.jpg", "*.jpeg"}},
			},
		})
		if errors.Is(dialogErr, ui.ErrCancelled) {
			status.SetText("File dialog cancelled")
		} else if dialogErr != nil {
			status.SetText("File dialog: " + dialogErr.Error())
		} else {
			status.SetText(path)
		}
	})
	quit.OnClick(func() { win.SetShouldClose(true) })
	return nil
}

func createControlsPage(win ui.Window, status ui.Label) (ui.Widget, error) {
	name, err := win.CreateTextField()
	if err != nil {
		return nil, err
	}
	name.SetPlaceholder("Single-line text field")
	notes, err := win.CreateTextArea(ui.TextAreaOptions{
		Text:        "This is a native multiline text area.\nResize the window to test its layout.",
		Placeholder: "Multiline text",
		Wrap:        true,
	})
	if err != nil {
		return nil, err
	}
	check, err := win.CreateCheckBox("Enable editing", true)
	if err != nil {
		return nil, err
	}
	link, err := win.CreateLink("GameKit link control", "https://github.com/bluescreen10/gamekit")
	if err != nil {
		return nil, err
	}
	separator, err := win.CreateSeparator(ui.Horizontal)
	if err != nil {
		return nil, err
	}
	button, err := win.CreateButton("Native button")
	if err != nil {
		return nil, err
	}
	sectionBody, err := win.CreateLayout(ui.Column(
		ui.Item(name), ui.Item(check), ui.Item(link),
	).WithGap(8))
	if err != nil {
		return nil, err
	}
	section, err := win.CreateSection(ui.SectionOptions{
		Title: "Basic controls", Content: sectionBody, Expanded: true,
	})
	if err != nil {
		return nil, err
	}
	page, err := win.CreateLayout(ui.Column(
		ui.Item(section),
		ui.Item(separator).MinimumSize(ui.Size{Height: 2}),
		ui.Item(notes).Grow(1).MinimumSize(ui.Size{Height: 180}),
		ui.Item(button),
	).WithPadding(ui.UniformInsets(16)).WithGap(12))
	if err != nil {
		return nil, err
	}
	name.OnChange(func(text string) { status.SetText("Text field: " + text) })
	notes.OnChange(func(text string) { status.SetText(fmt.Sprintf("Text area: %d characters", len(text))) })
	check.OnChange(func(checked bool) {
		notes.SetReadOnly(!checked)
		status.SetText(fmt.Sprintf("Text area editable: %t", checked))
	})
	link.OnClick(func() { status.SetText("Link activated: " + link.URL()) })
	button.OnClick(func() { status.SetText("Native button clicked") })
	section.OnChange(func(expanded bool) {
		status.SetText(fmt.Sprintf("Basic controls expanded: %t", expanded))
	})
	return page, nil
}

func createSelectionPage(win ui.Window, status ui.Label) (ui.Widget, error) {
	radio, err := win.CreateRadioGroup(ui.RadioGroupOptions{
		Items: []string{"Wireframe", "Shaded", "Rendered"}, SelectedIndex: 1,
	})
	if err != nil {
		return nil, err
	}
	selectControl, err := win.CreateSelect(ui.SelectOptions{
		Items: []string{"Nearest", "Linear", "Anisotropic"}, SelectedIndex: 1,
	})
	if err != nil {
		return nil, err
	}
	sliderLabel, err := win.CreateLabel("Value: 50%")
	if err != nil {
		return nil, err
	}
	slider, err := win.CreateSlider(ui.SliderOptions{Minimum: 0, Maximum: 1, Value: 0.5, Step: 0.01})
	if err != nil {
		return nil, err
	}
	progress, err := win.CreateProgressBar(0.5)
	if err != nil {
		return nil, err
	}
	activity, err := win.CreateActivityIndicator(true)
	if err != nil {
		return nil, err
	}
	activityCheck, err := win.CreateCheckBox("Activity indicator running", true)
	if err != nil {
		return nil, err
	}
	page, err := win.CreateLayout(ui.Column(
		ui.Item(radio), ui.Item(selectControl), ui.Item(sliderLabel), ui.Item(slider),
		ui.Item(progress), ui.Item(activity), ui.Item(activityCheck), ui.Space(),
	).WithPadding(ui.UniformInsets(16)).WithGap(12))
	if err != nil {
		return nil, err
	}
	radio.OnChange(func(index int) { status.SetText("Radio: " + radio.Items()[index]) })
	selectControl.OnChange(func(index int) { status.SetText("Select: " + selectControl.Items()[index]) })
	slider.OnChange(func(value float64) {
		progress.SetValue(value)
		sliderLabel.SetText(fmt.Sprintf("Value: %.0f%%", value*100))
	})
	activityCheck.OnChange(func(checked bool) {
		activity.SetRunning(checked)
		status.SetText(fmt.Sprintf("Activity running: %t", checked))
	})
	return page, nil
}

func createVisualPage(win ui.Window, status ui.Label) (ui.Widget, error) {
	imageView, err := win.CreateImage(ui.ImageOptions{Source: testImage(), Scaling: ui.ImageScaleFit})
	if err != nil {
		return nil, err
	}
	fit, err := win.CreateButton("Fit")
	if err != nil {
		return nil, err
	}
	fill, err := win.CreateButton("Fill")
	if err != nil {
		return nil, err
	}
	stretch, err := win.CreateButton("Stretch")
	if err != nil {
		return nil, err
	}
	buttons, err := win.CreateLayout(ui.Row(
		ui.Item(fit), ui.Item(fill), ui.Item(stretch), ui.Space(),
	).WithGap(8))
	if err != nil {
		return nil, err
	}
	page, err := win.CreateLayout(ui.Column(
		ui.Item(imageView).Grow(1).MinimumSize(ui.Size{Width: 240, Height: 240}),
		ui.Item(buttons),
	).WithPadding(ui.UniformInsets(16)).WithGap(12))
	if err != nil {
		return nil, err
	}
	setScaling := func(scaling ui.ImageScaling, name string) {
		if scalingErr := imageView.SetScaling(scaling); scalingErr != nil {
			status.SetText(scalingErr.Error())
			return
		}
		status.SetText("Image scaling: " + name)
	}
	fit.OnClick(func() { setScaling(ui.ImageScaleFit, "fit") })
	fill.OnClick(func() { setScaling(ui.ImageScaleFill, "fill") })
	stretch.OnClick(func() { setScaling(ui.ImageScaleStretch, "stretch") })
	return page, nil
}

func createCanvasPage(win ui.Window, status ui.Label) (ui.Widget, error) {
	canvas, err := win.CreateCanvas()
	if err != nil {
		return nil, err
	}
	label, err := win.CreateLabel("The area below is a GPU-presentable native canvas.")
	if err != nil {
		return nil, err
	}
	page, err := win.CreateLayout(ui.Column(
		ui.Item(label),
		ui.Item(canvas).Grow(1).MinimumSize(ui.Size{Width: 320, Height: 240}),
	).WithPadding(ui.UniformInsets(16)).WithGap(10))
	if err != nil {
		return nil, err
	}
	canvas.OnResize(func(size ui.Size) {
		status.SetText(fmt.Sprintf("Canvas: %d × %d", size.Width, size.Height))
	})
	return page, nil
}

func createScrollingPage(win ui.Window, status ui.Label) (ui.Widget, error) {
	items := make([]ui.LayoutItem, 0, 24)
	for index := 1; index <= 24; index++ {
		label, err := win.CreateLabel(fmt.Sprintf("Scrollable row %02d", index))
		if err != nil {
			return nil, err
		}
		items = append(items, ui.Item(label))
	}
	content, err := win.CreateLayout(
		ui.Column(items...).WithPadding(ui.UniformInsets(12)).WithGap(8),
	)
	if err != nil {
		return nil, err
	}
	scroll, err := win.CreateScrollView(ui.ScrollViewOptions{
		Content: content,
		Axes:    ui.ScrollVertical,
	})
	if err != nil {
		return nil, err
	}
	top, err := win.CreateButton("Top")
	if err != nil {
		return nil, err
	}
	middle, err := win.CreateButton("Middle")
	if err != nil {
		return nil, err
	}
	bottom, err := win.CreateButton("Bottom")
	if err != nil {
		return nil, err
	}
	buttons, err := win.CreateLayout(ui.Row(
		ui.Item(top), ui.Item(middle), ui.Item(bottom), ui.Space(),
	).WithGap(8))
	if err != nil {
		return nil, err
	}
	page, err := win.CreateLayout(ui.Column(
		ui.Item(scroll).Grow(1).MinimumSize(ui.Size{Width: 320, Height: 200}),
		ui.Item(buttons),
	).WithPadding(ui.UniformInsets(16)).WithGap(10))
	if err != nil {
		return nil, err
	}
	setOffset := func(y int) {
		scroll.SetScrollOffset(ui.Point{Y: y})
		status.SetText(fmt.Sprintf("Scroll offset: %d", scroll.ScrollOffset().Y))
	}
	top.OnClick(func() { setOffset(0) })
	middle.OnClick(func() { setOffset(240) })
	bottom.OnClick(func() { setOffset(10000) })
	return page, nil
}

func testImage() image.Image {
	const size = 192
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			base := uint8(45)
			if ((x/24)+(y/24))%2 == 1 {
				base = 75
			}
			result.SetRGBA(x, y, color.RGBA{
				R: uint8(40 + x*180/size),
				G: uint8(80 + y*140/size),
				B: base + 100,
				A: 255,
			})
		}
	}
	return result
}
