package app

import (
	"os"
	"path/filepath"
	"strings"
)

// Bundle is the .app the app runs from, or "" when it runs from none.
func Bundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return bundleOf(exe)
}

// bundleOf is the .app folder that holds exe, or "".
func bundleOf(exe string) string {
	for dir := filepath.Dir(exe); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if strings.HasSuffix(dir, ".app") {
			return dir
		}
	}
	return ""
}
