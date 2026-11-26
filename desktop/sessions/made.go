package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nednella/agentos/internal/atomicfile"
	"github.com/nednella/agentos/internal/folder"
)

// markerFile, in a worktree's own git folder, holds the id of the thread orchestrator made the worktree
// for. git removes it with the folder, so a worktree a human makes again at the same path has none.
const markerFile = "orchestrator-thread"

// Made is a worktree orchestrator made for a thread. Times are in milliseconds.
type Made struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Base    string `json:"base"`
	BaseOid string `json:"baseOid"`
	Thread  string `json:"thread"`
	Created int64  `json:"created"`
	Removed int64  `json:"removed,omitempty"`
}

// madeTrees is the record of the worktrees orchestrator made, one list per project, oldest first.
// A thread's records go only when the thread is deleted; until then a path orchestrator once used stays known.
type madeTrees struct {
	dir string

	mu    sync.Mutex
	lists map[string][]Made // by project key
}

func newMadeTrees(dir string) *madeTrees {
	return &madeTrees{dir: dir, lists: map[string][]Made{}}
}

func (m *madeTrees) path(key string) string { return filepath.Join(m.dir, key, "worktrees.json") }

// list needs mu. A file it cannot read is an error, so a later write never replaces records it did not see.
func (m *madeTrees) list(key string) ([]Made, error) {
	if list, ok := m.lists[key]; ok {
		return list, nil
	}
	var list []Made
	data, err := os.ReadFile(m.path(key))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("reading the worktree record: %w", err)
	default:
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("reading the worktree record: %w", err)
		}
	}
	m.lists[key] = list
	return list, nil
}

// save needs mu.
func (m *madeTrees) save(key string, list []Made) error {
	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("encoding the worktree record: %w", err)
	}
	if err := atomicfile.Write(m.path(key), data, 0o600); err != nil {
		return fmt.Errorf("saving the worktree record: %w", err)
	}
	m.lists[key] = list
	return nil
}

// Add records a worktree orchestrator made.
func (m *madeTrees) Add(key string, w Made) error {
	if w.Thread == "" {
		return errors.New("recording a worktree: no thread")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.list(key)
	if err != nil {
		return err
	}
	w.Path = folder.Canonical(w.Path)
	return m.save(key, append(list, w))
}

// Of is the newest record of the thread's worktree.
func (m *madeTrees) Of(key, thread string) (Made, bool) {
	return m.newest(key, func(w Made) bool { return w.Thread == thread })
}

// At is the newest record of a worktree at path.
func (m *madeTrees) At(key, path string) (Made, bool) {
	path = folder.Canonical(path)
	return m.newest(key, func(w Made) bool { return w.Path == path })
}

func (m *madeTrees) newest(key string, match func(Made) bool) (Made, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.list(key)
	if err != nil {
		return Made{}, false
	}
	for i := len(list) - 1; i >= 0; i-- {
		if match(list[i]) {
			return list[i], true
		}
	}
	return Made{}, false
}

// SetBranch records the branch the worktree at path is on.
func (m *madeTrees) SetBranch(key, path, branch string) error {
	return m.change(key, path, func(w *Made) { w.Branch = branch })
}

// Removed records that the worktree at path was removed.
func (m *madeTrees) Removed(key, path string, at time.Time) error {
	return m.change(key, path, func(w *Made) { w.Removed = at.UnixMilli() })
}

// Restored records that the worktree at path is back.
func (m *madeTrees) Restored(key, path string) error {
	return m.change(key, path, func(w *Made) { w.Removed = 0 })
}

// Forget drops every record of the thread's worktrees: orchestrator no longer counts them as its own, so no clean-up
// removes them.
func (m *madeTrees) Forget(key, thread string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.list(key)
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(list), func(w Made) bool { return w.Thread == thread })
	if len(kept) == len(list) {
		return nil
	}
	return m.save(key, kept)
}

// change edits the newest record at path.
func (m *madeTrees) change(key, path string, edit func(*Made)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.list(key)
	if err != nil {
		return err
	}
	canonical := folder.Canonical(path)
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Path == canonical {
			list = append([]Made(nil), list...)
			edit(&list[i])
			return m.save(key, list)
		}
	}
	return fmt.Errorf("no worktree orchestrator made at %s", path)
}

// handOver gives the worktree at path, which orchestrator made for the thread from, to the thread to: its record and its
// marker name to from then on. A worktree that is not from's, as one a human made again at its path, stays as it is.
func (l *Lifecycle) handOver(ctx context.Context, key, path, from, to string) error {
	if !l.ours(ctx, key, path, from) {
		return nil
	}
	if err := l.mark(ctx, path, to); err != nil {
		return err
	}
	return l.sessions.made.change(key, path, func(w *Made) { w.Thread = to })
}

// linkedGitDir is the worktree's own git folder, `<repo>/.git/worktrees/<name>`. The project
// folder's own checkout is never a worktree orchestrator made, so it is an error.
func (l *Lifecycle) linkedGitDir(ctx context.Context, worktree string) (string, error) {
	gitDir, common, err := l.gitDirs(ctx, worktree)
	if err != nil {
		return "", err
	}
	if gitDir == "" || samePath(gitDir, common) {
		return "", fmt.Errorf("%s is not a linked worktree", worktree)
	}
	return gitDir, nil
}

// mark writes the thread's id into the worktree's own git folder.
func (l *Lifecycle) mark(ctx context.Context, worktree, thread string) error {
	if thread == "" {
		return errors.New("marking the worktree: no thread")
	}
	gitDir, err := l.linkedGitDir(ctx, worktree)
	if err != nil {
		return fmt.Errorf("marking the worktree: %w", err)
	}
	if err := atomicfile.Write(filepath.Join(gitDir, markerFile), []byte(thread+"\n"), 0o600); err != nil {
		return fmt.Errorf("marking the worktree: %w", err)
	}
	return nil
}

// ours says whether orchestrator made the worktree for the thread and it is still that worktree: the
// newest record at the path is the thread's and not removed, and the worktree's marker holds the
// thread's id. A worktree a human made again at a path orchestrator used has no marker.
func (l *Lifecycle) ours(ctx context.Context, key, worktree, thread string) bool {
	if thread == "" {
		return false
	}
	w, ok := l.sessions.made.At(key, worktree)
	if !ok || w.Thread != thread || w.Removed != 0 {
		return false
	}
	return l.marked(ctx, worktree, thread)
}

// marked says whether the worktree is a linked worktree whose marker holds the thread's id.
func (l *Lifecycle) marked(ctx context.Context, worktree, thread string) bool {
	gitDir, err := l.linkedGitDir(ctx, worktree)
	if err != nil {
		return false
	}
	marker, err := os.ReadFile(filepath.Join(gitDir, markerFile))
	return err == nil && strings.TrimSpace(string(marker)) == thread
}
