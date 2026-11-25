package sessions

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
)

const editorTimeout = 10 * time.Second

var errFolderGone = errors.New("This thread's folder is gone")

// OpenInEditor opens the folder the thread works in with the editor app of the config: its worktree when it has one,
// else the project folder. The folder comes from the thread's own record, never from the caller.
func (s *Sessions) OpenInEditor(id string) error {
	name, err := session.ParseName(id)
	if err != nil {
		return err
	}
	if name.IsShell() {
		return errNotSession
	}
	dir, err := s.threadFolder(id)
	if err != nil {
		return err
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
	if _, err := s.run(ctx, "", "open", "-a", app, dir); err != nil {
		log.Printf("agentos: opening %s in %s: %v", dir, app, err)
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
