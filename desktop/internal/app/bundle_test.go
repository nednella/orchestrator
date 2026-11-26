package app

import "testing"

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
