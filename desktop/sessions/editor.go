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

	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/session"
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
		log.Printf("orchestrator: opening %s in %s: %v", target, app, err)
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
// name given or by the name its symlinks lead to; of names whose folder dir is, for the error. Whether the path leaves
// dir is judged before whether it exists, so a refusal never says whether a file outside dir is there. A hard link to a
// file outside dir is read, since no path shows one.
func OpenMarkdown(dir, path, of string) (*os.File, string, error) {
	gone := fmt.Errorf("The folder of %s is gone", of)
	outside := fmt.Errorf("This file is outside %s's folder", of)
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil || !filepath.IsAbs(dir) {
		return nil, "", gone
	}
	rel, inside := filepath.Clean(path), filepath.IsLocal(path)
	if filepath.IsAbs(path) {
		if rel, inside = within(dir, path); !inside {
			rel, inside = within(realDir, path)
		}
	}
	if !inside {
		return nil, "", outside
	}
	// The root refuses a path whose symlinks lead out of the folder, whether or not their target exists.
	root, err := os.OpenRoot(realDir)
	if err != nil {
		return nil, "", gone
	}
	defer root.Close()
	checked, err := root.Stat(rel)
	switch {
	case errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR):
		return nil, "", errors.New("This file does not exist")
	case errors.Is(err, fs.ErrPermission):
		return nil, "", errCannotRead
	case err != nil:
		return nil, "", outside
	}
	real, err := filepath.EvalSymlinks(filepath.Join(realDir, rel))
	if err != nil {
		log.Printf("orchestrator: reading %s in the folder of %s: %v", path, of, err)
		return nil, "", errCannotRead
	}
	if _, inside := within(realDir, real); !inside {
		return nil, "", outside
	}
	if !checked.Mode().IsRegular() || !IsMarkdown(path) || !IsMarkdown(real) {
		return nil, "", errNotMarkdown
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	// Without O_NONBLOCK a named pipe put in the file's place would hold the open up.
	f, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		log.Printf("orchestrator: opening %s in the folder of %s: %v", path, of, err)
		return nil, "", errCannotRead
	}
	// The file can be swapped between the check and the open, as for a symlink to another file in the folder.
	if opened, err := f.Stat(); err != nil || !os.SameFile(checked, opened) {
		f.Close()
		return nil, "", errors.New("This file changed while it was opened")
	}
	return f, real, nil
}

// beforeOpen, when set, runs between OpenMarkdown's check of a file and its open, for a test to change the file there.
var beforeOpen func()

var errCannotRead = errors.New("Could not read this file")

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
