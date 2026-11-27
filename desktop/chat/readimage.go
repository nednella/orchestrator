package chat

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// previewImageTypes are the image types a question's preview shows. An SVG is left out: it may carry script.
var previewImageTypes = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}

// ChatReadImage is the image at path for a question's preview: a file in the folder the thread works in, relative to it
// or absolute, or an absolute path in the thread's own temp folder or its chat images. Anything else is refused.
func (v *Service) ChatReadImage(id, path string) (FileContent, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return FileContent{}, err
	}
	if !slices.Contains(previewImageTypes, strings.ToLower(filepath.Ext(path))) {
		return FileContent{}, errors.New("This file is not an image")
	}
	of := fmt.Sprintf("thread %d", ch.N)
	first, err := readFile(v.threadDir(ch), path, of)
	if err == nil && first.Kind == "image" {
		return first, nil
	}
	if filepath.IsAbs(path) {
		for _, dir := range []string{ch.Tmp, ch.Media} {
			if dir == "" {
				continue
			}
			if c, e := readFile(dir, path, of); e == nil && c.Kind == "image" {
				return c, nil
			}
		}
	}
	if err == nil {
		return FileContent{}, errors.New("This file is not an image")
	}
	return FileContent{}, err
}
