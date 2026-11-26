package sessions

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
)

const editorTimeout = 10 * time.Second

var errFolderGone = errors.New("This thread's folder is gone")

// OpenInEditor opens the folder the thread works in with the editor app of the config: its worktree when it has one,
// else the project folder. The folder comes from the thread's own record, never from the caller. A path opens that
// markdown file instead, only when it is inside the folder, as OpenMarkdown has it.
func (s *Sessions) OpenInEditor(id, path string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if name.IsShell() {
		return errNotSession
	}
	target, err := s.threadFolder(id)
	if err != nil {
		return err
	}
	if path != "" {
		f, real, err := OpenMarkdown(target, path, fmt.Sprintf("thread %d", s.Number(id)))
		if err != nil {
			return err
		}
		f.Close()
		target = real
	}
	app := project.DefaultEditorApp
	if s.editorApp != nil {
		app = s.editorApp()
	}
	label := app
	if app == project.DefaultEditorApp {
		label = "VS Code"
	}
	ctx, cancel := context.WithTimeout(s.context(), editorTimeout)
	defer cancel()
	if _, err := s.run(ctx, "", "open", "-a", app, target); err != nil {
		log.Printf("agentos: opening %s in %s: %v", target, app, err)
		if strings.Contains(err.Error(), "Unable to find application") {
			return fmt.Errorf("%s is not installed", label)
		}
		return fmt.Errorf("%s did not open", label)
	}
	return nil
}

// threadFolder is the folder the thread works in, which must exist: the worktree the lifecycle follows, else the
// folder it started in, else the project folder. A thread whose worktree was removed has its own folder still named,
// so it never falls back to the project folder.
func (s *Sessions) threadFolder(id string) (string, error) {
	s.mu.Lock()
	in, ok := s.infoOf(id)
	projs := s.projects()
	s.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("Thread %s is gone", id)
	}
	dir := cmp.Or(s.life.fields(id).worktree, in.Path)
	if dir == "" {
		proj, found := projectIn(projs, in)
		if !found {
			return "", errFolderGone
		}
		dir = proj.Dir
	}
	if !filepath.IsAbs(dir) {
		return "", errFolderGone
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", errFolderGone
	}
	return dir, nil
}

// OpenMarkdown opens the markdown file at path, relative to dir or absolute, and gives its real path. It refuses a file
// whose real path, symlinks resolved, is not inside dir, and one that is not a regular .md or .markdown file by the
// name given or by the name its symlinks lead to; of names whose folder dir is, for the error.
func OpenMarkdown(dir, path, of string) (*os.File, string, error) {
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil || !filepath.IsAbs(dir) {
		return nil, "", fmt.Errorf("The folder of %s is gone", of)
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(dir, full)
	}
	real, err := filepath.EvalSymlinks(full)
	missing := err != nil
	if missing {
		real = filepath.Clean(full)
	}
	rel, inside := within(realDir, real)
	if !inside && missing {
		_, inside = within(dir, real)
	}
	switch {
	case !inside:
		return nil, "", fmt.Errorf("This file is outside %s's folder", of)
	case errors.Is(err, fs.ErrNotExist):
		return nil, "", errors.New("This file does not exist")
	case missing:
		log.Printf("agentos: reading %s in the folder of %s: %v", path, of, err)
		return nil, "", errors.New("Could not read this file")
	case !IsMarkdown(path) || !IsMarkdown(real):
		return nil, "", errNotMarkdown
	}
	// The folder's files can change between the check above and the open, so the open goes through a root, which
	// refuses a path that has left the folder by then. Without O_NONBLOCK a named pipe would hold the open up.
	root, err := os.OpenRoot(realDir)
	if err != nil {
		return nil, "", fmt.Errorf("The folder of %s is gone", of)
	}
	defer root.Close()
	f, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		log.Printf("agentos: opening %s in the folder of %s: %v", path, of, err)
		return nil, "", errors.New("Could not read this file")
	}
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, "", errNotMarkdown
	}
	return f, real, nil
}

var errNotMarkdown = errors.New("This is not a markdown file")

// IsMarkdown says whether path names a .md or .markdown file.
func IsMarkdown(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

// within is path relative to dir, and whether it is inside it.
func within(dir, path string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	return rel, err == nil && rel != "." && filepath.IsLocal(rel)
}
