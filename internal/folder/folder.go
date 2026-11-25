// Package folder compares folders as the file system sees them, with every link resolved.
package folder

import (
	"path/filepath"
	"strings"
)

// Canonical is path with every link resolved, also when its last parts are gone, as they are once a worktree is
// removed: `/var/x/trees/a` and `/private/var/x/trees/a` are one path.
func Canonical(path string) string {
	path = filepath.Clean(path)
	var gone []string
	for {
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return filepath.Join(append([]string{real}, gone...)...)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Join(append([]string{path}, gone...)...)
		}
		gone = append([]string{filepath.Base(path)}, gone...)
		path = parent
	}
}

// Inside says whether path is dir or a file or folder inside it, also when either is gone.
func Inside(path, dir string) bool {
	rel, err := filepath.Rel(Canonical(dir), Canonical(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
