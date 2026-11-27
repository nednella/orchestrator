package main

import (
	"bytes"
	"image/png"
	"os"
	"testing"

	"github.com/nednella/orchestrator/internal/project"
)

func TestDockIconShowsEachThemesIcon(t *testing.T) {
	for _, theme := range append(append([]string{}, project.DarkThemes...), project.LightThemes...) {
		t.Run(theme, func(t *testing.T) {
			var shown [][]byte
			dockIcon(func(icon []byte) { shown = append(shown, icon) })(theme)
			if len(shown) != 1 {
				t.Fatalf("set called %d times", len(shown))
			}
			want, err := os.ReadFile("build/themes/icon-" + theme + ".png")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(shown[0], want) {
				t.Errorf("set got another image than icon-%s.png", theme)
			}
			img, err := png.Decode(bytes.NewReader(shown[0]))
			if err != nil {
				t.Fatal(err)
			}
			if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
				t.Errorf("icon is %dx%d, not 512x512", b.Dx(), b.Dy())
			}
		})
	}
}

func TestDockIconIgnoresUnknownThemes(t *testing.T) {
	for _, theme := range []string{"", "system", "dark", "sepia", "../icon", "black.png"} {
		dockIcon(func([]byte) { t.Errorf("set called for %q", theme) })(theme)
	}
}
