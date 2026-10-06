package ui_test

import (
	"testing"

	"github.com/bluescreen10/gamekit/ui"
)

func TestThemeValidation(t *testing.T) {
	for _, theme := range []ui.Theme{ui.ThemeSystem, ui.ThemeLight, ui.ThemeDark} {
		if err := theme.Validate(); err != nil {
			t.Errorf("Theme(%d).Validate() = %v, want nil", theme, err)
		}
	}
	if err := ui.Theme(255).Validate(); err == nil {
		t.Error("Theme(255).Validate() = nil, want an error")
	}
}
