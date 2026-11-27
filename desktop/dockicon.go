package main

import (
	"embed"
	"slices"

	"github.com/nednella/orchestrator/internal/project"
)

//go:embed build/themes/*.png
var themeIcons embed.FS

// dockIcon is what shows the icon of the theme the window shows: it hands set that theme's PNG and ignores a name
// with no icon.
func dockIcon(set func(png []byte)) func(theme string) {
	return func(theme string) {
		if !slices.Contains(project.DarkThemes, theme) && !slices.Contains(project.LightThemes, theme) {
			return
		}
		png, err := themeIcons.ReadFile("build/themes/icon-" + theme + ".png")
		if err != nil {
			return
		}
		set(png)
	}
}
