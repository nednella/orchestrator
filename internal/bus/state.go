package bus

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nednella/orchestrator/internal/atomicfile"
	"github.com/nednella/orchestrator/internal/environ"
	"github.com/nednella/orchestrator/internal/thread"
)

// DefaultDir is where state files and the sockets live for the environment env. ORCHESTRATOR_STATE_DIR overrides it.
func DefaultDir(env []string) (string, error) {
	if dir := environ.Get(env, "ORCHESTRATOR_STATE_DIR"); dir != "" {
		return dir, nil
	}
	home := environ.Get(env, "HOME")
	if home == "" {
		return "", errors.New("finding home dir: $HOME is not defined")
	}
	return filepath.Join(home, ".local", "state", "orchestrator"), nil
}

func stateFile(dir, name string) string {
	return filepath.Join(dir, "sessions", name+".json")
}

// WriteState saves rec.
func WriteState(dir string, rec thread.Record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encoding state: %w", err)
	}
	return atomicfile.Write(stateFile(dir, rec.Thread), data, 0o600)
}

// ReadState returns the saved record, or the zero record when there is none.
func ReadState(dir, name string) (thread.Record, error) {
	data, err := os.ReadFile(stateFile(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return thread.Record{}, nil
	}
	if err != nil {
		return thread.Record{}, fmt.Errorf("reading state of %s: %w", name, err)
	}
	var rec thread.Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return thread.Record{}, fmt.Errorf("decoding state of %s: %w", name, err)
	}
	return rec, nil
}

func RemoveState(dir, name string) error {
	if err := os.Remove(stateFile(dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing state of %s: %w", name, err)
	}
	return nil
}

// ReadAll returns every saved record by thread name, but those skip says to leave unread. skip may be nil.
// Unreadable files are skipped.
func ReadAll(dir string, skip func(name string, info fs.FileInfo) bool) map[string]thread.Record {
	out := map[string]thread.Record{}
	root := filepath.Join(dir, "sessions")
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if skip != nil {
			rel, _ := filepath.Rel(root, path)
			info, err := d.Info()
			if err == nil && skip(strings.TrimSuffix(rel, ".json"), info) {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var rec thread.Record
		if json.Unmarshal(data, &rec) == nil && rec.Thread != "" {
			out[rec.Thread] = rec
		}
		return nil
	})
	return out
}
