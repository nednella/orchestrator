package chat

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nednella/orchestrator/desktop/threads"
)

// maxChips is the most paths one ChatFileChips call checks.
const maxChips = 500

// FileChip says what is at a path the chat names. Path is the path as asked. Exists is false when nothing is there or
// it is neither a regular file nor a folder. Abs is the path made absolute: ~ and a relative path resolved from the
// thread's folder; for a path outside that folder it is the real path, symlinks resolved, which ChatReadOutside reads. Inside says the path is in the thread's folder, symlinks resolved, and Rel is then its path from
// that folder with / between names, "" for the folder itself. Size is the size of a file.
type FileChip struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Dir    bool   `json:"dir"`
	Size   int64  `json:"size"`
	Abs    string `json:"abs"`
	Inside bool   `json:"inside"`
	Rel    string `json:"rel"`
}

// ChatFileChips is what is at each of paths, in order, for the chips the chat shows. It only looks at the paths, and
// reads no file.
func (v *Service) ChatFileChips(id string, paths []string) ([]FileChip, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return nil, err
	}
	if len(paths) > maxChips {
		return nil, fmt.Errorf("Cannot check more than %d files at once", maxChips)
	}
	dir := v.threadDir(ch)
	realDir, _ := filepath.EvalSymlinks(dir)
	chips := make([]FileChip, len(paths))
	for i, path := range paths {
		chips[i] = chipAt(dir, realDir, path)
	}
	return chips, nil
}

func chipAt(dir, realDir, path string) FileChip {
	chip := FileChip{Path: path, Abs: threads.ResolveLink(dir, path)}
	if chip.Abs == "" {
		return chip
	}
	real, err := filepath.EvalSymlinks(chip.Abs)
	if err != nil {
		return chip
	}
	info, err := os.Stat(real)
	if err != nil || !(info.Mode().IsRegular() || info.IsDir()) {
		return chip
	}
	chip.Exists, chip.Dir = true, info.IsDir()
	if !chip.Dir {
		chip.Size = info.Size()
	}
	if realDir != "" {
		if rel, err := filepath.Rel(realDir, real); err == nil && (rel == "." || filepath.IsLocal(rel)) {
			chip.Inside = true
			if rel != "." {
				chip.Rel = filepath.ToSlash(rel)
			}
		}
	}
	if !chip.Inside {
		chip.Abs = real
	}
	return chip
}
