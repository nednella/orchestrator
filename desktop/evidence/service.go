package evidence

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/nednella/agentos/desktop/internal/media"
	ctl "github.com/nednella/agentos/internal/control"
)

const maxTextCard = 64 << 10

// Asker checks which session a command comes from.
type Asker interface {
	AskerSession(req ctl.Request) (string, error)
}

// Roots names the folders a session may file evidence from.
type Roots interface {
	ShowRoots(id string) []string
}

// Changes is told when a session's evidence changed.
type Changes struct {
	Emit  func(event string, payload any)
	Touch func() // the session list shows the evidence count
}

// Service is bound to the front end.
type Service struct {
	store   *Store
	changes Changes
}

func NewService(s *Store, c Changes) *Service { return &Service{store: s, changes: c} }

// Evidence is the session's evidence, oldest first.
func (s *Service) Evidence(id string) []Evidence { return s.store.List(id) }

// DeleteEvidence removes one item of a session's evidence.
func (s *Service) DeleteEvidence(id, evidenceID string) error {
	err := s.store.Delete(id, evidenceID)
	if err == nil {
		s.changes.Changed(s.store, id, false)
	}
	return err
}

// Changed tells the front end that a session's evidence changed; an agent's new evidence also raises the session.
func (c Changes) Changed(store *Store, id string, byAgent bool) {
	c.Emit("evidence", map[string]any{"id": id, "items": store.List(id)})
	if byAgent {
		c.Emit("attention", map[string]string{"id": id, "state": "evidence"})
	}
	c.Touch()
}

// Commands answers the show tool.
type Commands struct {
	beforeOpen func(path string) // tests only: runs between the check of a file and its opening
	store      *Store
	asker      Asker
	roots      Roots
	changes    Changes
}

func NewCommands(s *Store, a Asker, r Roots, c Changes) *Commands {
	return &Commands{store: s, asker: a, roots: r, changes: c}
}

// Show files a picture, a text file or some words as the session's evidence.
func (c *Commands) Show(_ context.Context, req ctl.Request) (string, error) {
	id, err := c.asker.AskerSession(req)
	if err != nil {
		return "", err
	}
	caption := req.Opts["caption"]
	if text := req.Opts["text"]; text != "" {
		if _, err := c.store.AddText(id, text, caption, "agent"); err != nil {
			return "", err
		}
		c.changes.Changed(c.store, id, true)
		return "filed as evidence", nil
	}
	if len(req.Args) != 1 {
		return "", errors.New("show needs a file or text")
	}
	path, err := allowedFile(req.Args[0], c.roots.ShowRoots(id))
	if err != nil {
		return "", err
	}
	data, err := c.readRegular(path)
	if err != nil {
		return "", err
	}
	if _, err = c.store.AddImage(id, data, caption, "agent"); err != nil {
		if len(data) > maxTextCard || !utf8.Valid(data) {
			return "", fmt.Errorf("%s is neither a picture nor a text file under 64 KB", path)
		}
		if caption == "" {
			caption = filepath.Base(path)
		}
		if _, err = c.store.AddText(id, string(data), caption, "agent"); err != nil {
			return "", err
		}
	}
	c.changes.Changed(c.store, id, true)
	return "filed as evidence", nil
}

// readRegular reads the regular file at path, which allowedFile resolved. It opens the file once, refusing a symlink
// at its last part, and reads the file it opened only if that is the file checked, so a swap after the check is refused.
func (c *Commands) readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a file", path)
	}
	if info.Size() > media.MaxImageSize {
		return nil, fmt.Errorf("%s is over 10 MB", path)
	}
	if c.beforeOpen != nil {
		c.beforeOpen(path)
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("%s is not a file", path)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("%s changed while it was read", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, media.MaxImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(data) > media.MaxImageSize {
		return nil, fmt.Errorf("%s is over 10 MB", path)
	}
	return data, nil
}

// allowedFile is the real path of file, symlinks resolved, when it lies inside one of the roots, resolved the same way.
// A file that does not exist is judged by its cleaned path, so a refusal never tells whether a file outside is there.
func allowedFile(file string, roots []string) (string, error) {
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		real = filepath.Clean(file)
		if dir, err := filepath.EvalSymlinks(filepath.Dir(real)); err == nil {
			real = filepath.Join(dir, filepath.Base(real))
		}
	}
	for _, root := range roots {
		root, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		if rel, err := filepath.Rel(root, real); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return real, nil
		}
	}
	return "", fmt.Errorf("%s is not in the thread's folder, the project folder or the thread's temp folder, so it cannot be shown", file)
}
