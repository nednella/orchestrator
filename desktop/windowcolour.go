package main

import (
	"context"
	"strings"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/wailsapp/wails/v2/pkg/options"
)

// themeBackground is each theme's --bg-app in tokens.css as sRGB.
var themeBackground = map[string]options.RGBA{
	"black":    {R: 0, G: 0, B: 0, A: 255},
	"midnight": {R: 10, G: 17, B: 31, A: 255},
	"umber":    {R: 25, G: 20, B: 16, A: 255},
	"light":    {R: 244, G: 245, B: 248, A: 255},
	"linen":    {R: 246, G: 243, B: 237, A: 255},
}

// windowColour is the colour the window shows before the page loads: the --bg-app of the theme the saved settings
// pick. appearance is the saved app_theme, "" for the system's, which systemDark then gives.
func windowColour(appearance, dark, light string, systemDark bool) *options.RGBA {
	isLight := appearance == "light" || (appearance == "" && !systemDark)
	name, fallback := dark, "black"
	if isLight {
		name, fallback = light, "light"
	}
	colour, ok := themeBackground[name]
	if !ok {
		colour = themeBackground[fallback]
	}
	return &colour
}

// systemIsDark reports whether macOS shows its dark appearance, which it names only then.
func systemIsDark() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := run.Exec(ctx, "", "defaults", "read", "-g", "AppleInterfaceStyle")
	return err == nil && strings.EqualFold(strings.TrimSpace(string(out)), "dark")
}
