package main

import (
	"testing"

	"github.com/nednella/orchestrator/internal/project"
	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestWindowColourFollowsTheAppearanceAndItsTheme(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		appearance, dark, lit string
		systemDark            bool
		want                  options.RGBA
	}{
		{"dark default", "dark", "black", "light", false, options.RGBA{A: 255}},
		{"dark midnight", "dark", "midnight", "light", false, options.RGBA{R: 10, G: 17, B: 31, A: 255}},
		{"dark umber", "dark", "umber", "linen", false, options.RGBA{R: 25, G: 20, B: 16, A: 255}},
		{"light default", "light", "black", "light", true, options.RGBA{R: 244, G: 245, B: 248, A: 255}},
		{"light linen", "light", "black", "linen", true, options.RGBA{R: 246, G: 243, B: 237, A: 255}},
		{"system dark", "", "midnight", "linen", true, options.RGBA{R: 10, G: 17, B: 31, A: 255}},
		{"system light", "", "midnight", "linen", false, options.RGBA{R: 246, G: 243, B: 237, A: 255}},
		{"dark linen", "dark", "linen-dark", "linen", false, options.RGBA{R: 25, G: 23, B: 18, A: 255}},
		{"dark upscope", "dark", "upscope-dark", "light", false, options.RGBA{R: 3, G: 3, B: 4, A: 255}},
		{"light midnight", "light", "black", "midnight-light", true, options.RGBA{R: 239, G: 244, B: 252, A: 255}},
		{"light umber", "light", "black", "umber-light", true, options.RGBA{R: 247, G: 238, B: 231, A: 255}},
		{"light upscope", "light", "black", "upscope-light", true, options.RGBA{R: 248, G: 247, B: 251, A: 255}},
		{"unknown dark theme", "dark", "violet", "light", false, options.RGBA{A: 255}},
		{"unknown light theme", "light", "black", "sepia", false, options.RGBA{R: 244, G: 245, B: 248, A: 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := *windowColour(tc.appearance, tc.dark, tc.lit, tc.systemDark); got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestEveryThemeHasAWindowColour(t *testing.T) {
	for _, name := range append(append([]string{}, project.DarkThemes...), project.LightThemes...) {
		if _, ok := themeBackground[name]; !ok {
			t.Errorf("no window colour for %s", name)
		}
	}
}
