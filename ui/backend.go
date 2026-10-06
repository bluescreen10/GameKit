package ui

import (
	"context"
	"errors"

	"github.com/bluescreen10/gamekit/gpu"
)

// Theme controls the native appearance used by a window and its controls.
type Theme uint8

const (
	ThemeSystem Theme = iota
	ThemeLight
	ThemeDark
)

// Validate checks that the theme is recognized.
func (theme Theme) Validate() error {
	if theme > ThemeDark {
		return errors.New("ui: invalid theme")
	}
	return nil
}

// WindowOptions describes a top-level native window. Size must be valid.
type WindowOptions struct {
	Title string
	Size  Size

	Resizable              bool
	Hidden                 bool
	Borderless             bool
	Maximized              bool
	AlwaysOnTop            bool
	TransparentFramebuffer bool
	Theme                  Theme
}

// Backend creates native windows.
type Backend interface {
	CreateWindow(options WindowOptions) (Window, error)
}

// Drawable is a native UI object that can be presented by a GPU backend.
// FramebufferSize is measured in physical pixels, unlike widget geometry.
type Drawable interface {
	gpu.SurfaceTarget
	FramebufferSize() Size
}

// Window is a top-level native window and the owner of its controls and dialogs.
// Close must be safe to call repeatedly. PollEvents and methods that create or
// display native UI must be called from the platform UI thread. On macOS that
// is the program's main thread.
type Window interface {
	Drawable
	Input

	Close()
	Title() string
	SetTitle(title string)
	Theme() Theme
	SetTheme(theme Theme) error
	Size() Size

	ShouldClose() bool
	SetShouldClose(close bool)
	PollEvents()

	SetContent(content Widget) error
	CreateLayout(spec LayoutSpec) (Layout, error)
	CreateLabel(text string) (Label, error)
	CreateButton(text string) (Button, error)
	CreateTextField() (TextField, error)
	CreateTextArea(options TextAreaOptions) (TextArea, error)
	CreateCheckBox(text string, checked bool) (CheckBox, error)
	CreateRadioGroup(options RadioGroupOptions) (RadioGroup, error)
	CreateSelect(options SelectOptions) (Select, error)
	CreateSlider(options SliderOptions) (Slider, error)
	CreateProgressBar(value float64) (ProgressBar, error)
	CreateActivityIndicator(running bool) (ActivityIndicator, error)
	CreateSeparator(direction LayoutDirection) (Separator, error)
	CreateImage(options ImageOptions) (ImageView, error)
	CreateLink(text, url string) (Link, error)
	CreateTabs(options TabsOptions) (Tabs, error)
	CreateScrollView(options ScrollViewOptions) (ScrollView, error)
	CreateCanvas() (Canvas, error)
	CreatePanel(options PanelOptions) (Panel, error)
	CreateSection(options SectionOptions) (Section, error)

	OpenFile(ctx context.Context, options OpenFileOptions) (string, error)
	OpenFiles(ctx context.Context, options OpenFileOptions) ([]string, error)
	SaveFile(ctx context.Context, options SaveFileOptions) (string, error)
	SelectDirectory(ctx context.Context, options DirectoryDialogOptions) (string, error)
}
