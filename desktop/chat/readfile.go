package chat

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/nednella/orchestrator/desktop/threads"
)

const (
	maxShown = 1 << 20 // the most of a text file the Files view reads
	maxImage = 8 << 20 // the largest image the Files view shows
)

var errTooBig = errors.New("This file is too big to show")

// FileContent is a file as the Files view shows it. Kind is "text", "image" or "binary". Text is the file's text, at
// most its first maxShown bytes, and Truncated says it was cut; Size is the file's own size in bytes. An image has its
// Mime type and its bytes as base64 in Data. A binary file has neither.
type FileContent struct {
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
	Mime      string `json:"mime"`
	Data      string `json:"data"`
}

// imageMimes are the image types the Files view shows, by file extension.
var imageMimes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".bmp": "image/bmp", ".ico": "image/x-icon", ".avif": "image/avif", ".svg": "image/svg+xml",
}

// ChatReadFile is the file at path, relative to the thread's folder or absolute: its worktree, else the project
// folder. Only a regular file inside that folder, symlinks resolved, is read.
func (v *Service) ChatReadFile(id, path string) (FileContent, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return FileContent{}, err
	}
	return readFile(v.threadDir(ch), path, fmt.Sprintf("thread %d", ch.N))
}

// ProjectReadFile is ChatReadFile for the project's folder.
func (v *Service) ProjectReadFile(project, path string) (FileContent, error) {
	dir := v.projectDir(project)
	if dir == "" {
		return FileContent{}, fmt.Errorf("No project %s", project)
	}
	return readFile(dir, path, "project "+project)
}

// threadDir is the folder the thread works in: its worktree, else the project folder.
func (v *Service) threadDir(ch threads.Chat) string { return cmp.Or(ch.Dir, v.projectDir(ch.Project)) }

// readFile is the file at path in dir, as threads.OpenFile opens it; of names whose folder dir is, for the error.
func readFile(dir, path, of string) (FileContent, error) {
	f, real, err := threads.OpenFile(dir, path, of)
	if err != nil {
		return FileContent{}, err
	}
	return contentOf(f, real, path, of)
}

// ChatReadOutside is the file at the absolute path, read only, wherever it is on disk: the file the owner clicked in the
// chat, which may lie outside the thread's folder. Only a regular file is read, symlinks resolved, under the limits of
// ChatReadFile. It never writes and never lists a folder.
func (v *Service) ChatReadOutside(id, path string) (FileContent, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return FileContent{}, err
	}
	of := fmt.Sprintf("thread %d", ch.N)
	f, real, err := threads.OpenAbsolute(path, of)
	if err != nil {
		return FileContent{}, err
	}
	return contentOf(f, real, path, of)
}

// contentOf is the open file f, at real, as the Files view shows it, and closes it.
func contentOf(f *os.File, real, path, of string) (FileContent, error) {
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		log.Printf("orchestrator: reading %s in the folder of %s: %v", path, of, err)
		return FileContent{}, errors.New("Could not read this file")
	}
	if mime, ok := imageMimes[strings.ToLower(filepath.Ext(real))]; ok {
		data, err := readAtMost(f, maxImage)
		if err != nil {
			log.Printf("orchestrator: reading %s in the folder of %s: %v", path, of, err)
			return FileContent{}, errors.New("Could not read this file")
		}
		if len(data) > maxImage {
			return FileContent{}, errTooBig
		}
		return FileContent{Kind: "image", Size: int64(len(data)), Mime: mime, Data: base64.StdEncoding.EncodeToString(data)}, nil
	}
	data, err := readAtMost(f, maxShown)
	if err != nil {
		log.Printf("orchestrator: reading %s in the folder of %s: %v", path, of, err)
		return FileContent{}, errors.New("Could not read this file")
	}
	content := FileContent{Kind: "text", Size: max(info.Size(), int64(len(data)))}
	if len(data) > maxShown {
		data, content.Truncated = data[:maxShown], true
		// The cut may fall inside a character.
		for cut := 0; cut < utf8.UTFMax && !utf8.Valid(data) && len(data) > 0; cut++ {
			data = data[:len(data)-1]
		}
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return FileContent{Kind: "binary", Size: content.Size}, nil
	}
	content.Text = string(data)
	return content, nil
}

// readAtMost is the first limit+1 bytes of f at most, so that a longer file shows as longer than limit.
func readAtMost(f io.Reader, limit int) ([]byte, error) {
	return io.ReadAll(io.LimitReader(f, int64(limit)+1))
}
