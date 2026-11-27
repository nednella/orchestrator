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
	"black":          {R: 0, G: 0, B: 0, A: 255},
	"midnight":       {R: 10, G: 17, B: 31, A: 255},
	"umber":          {R: 25, G: 20, B: 16, A: 255},
	"linen-dark":     {R: 25, G: 23, B: 18, A: 255},
	"upscope-dark":   {R: 3, G: 3, B: 4, A: 255},
	"light":          {R: 252, G: 252, B: 252, A: 255},
	"linen":          {R: 246, G: 243, B: 237, A: 255},
	"midnight-light": {R: 239, G: 244, B: 252, A: 255},
	"umber-light":    {R: 247, G: 238, B: 231, A: 255},
	"upscope-light":  {R: 248, G: 247, B: 251, A: 255},
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
