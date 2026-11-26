package app

import (
	"os"
	"testing"

	"github.com/nednella/orchestrator/internal/bus"
)

func TestBundleOf(t *testing.T) {
	for exe, want := range map[string]string{
		"/Applications/orchestrator.app/Contents/MacOS/orchestrator": "/Applications/orchestrator.app",
		"/repo/bin/orchestrator.app/Contents/MacOS/orchestrator":     "/repo/bin/orchestrator.app",
		"/repo/bin/orchestrator-desktop":                             "",
		"/orchestrator":                                              "",
	} {
		if got := bundleOf(exe); got != want {
			t.Errorf("bundleOf(%q) = %q, want %q", exe, got, want)
		}
	}
}

func TestRecordBundle(t *testing.T) {
	dir := t.TempDir()
	if err := recordBundleOf(bundleOf("/repo/bin/orchestrator-desktop"), dir); err != nil || exists(bus.AppPath(dir)) {
		t.Errorf("a binary outside a bundle recorded a path: %v", err)
	}
	if err := recordBundleOf(bundleOf("/Apps/orchestrator.app/Contents/MacOS/orchestrator"), dir); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bus.AppPath(dir)); string(got) != "/Apps/orchestrator.app\n" {
		t.Errorf("recorded %q", got)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
