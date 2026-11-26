package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/internal/control"
)

func TestDefaultPathsShareNothingWithAgentos(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, key := range []string{
		"AGENTOS_CONFIG", "AGENTOS_STATE_DIR", "AGENTOS_DEV_DATA_DIR", "AGENTOS_TMUX_SOCKET", "AGENTOS_DIR",
		"ORCHESTRATOR_CONFIG", "ORCHESTRATOR_STATE_DIR", "ORCHESTRATOR_DEV_DATA_DIR", "ORCHESTRATOR_TMUX_SOCKET", "ORCHESTRATOR_DIR",
	} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{
		"state":          cfg.StateDir,
		"data":           cfg.DataDir,
		"local":          cfg.LocalDir,
		"config":         ConfigPath(home),
		"control socket": control.SocketPath(cfg.StateDir),
		"tmux socket":    defaultTmuxSocket,
	}
	for name, path := range paths {
		rel := strings.TrimPrefix(path, home)
		if !strings.Contains(rel, "orchestrator") || strings.Contains(strings.ToLower(rel), "agentos") {
			t.Errorf("%s = %q, want a name with orchestrator and not agentos", name, path)
		}
	}
	if want := filepath.Join(home, ".config", "orchestrator", "config.yaml"); ConfigPath(home) != want {
		t.Errorf("config = %q, want %q", ConfigPath(home), want)
	}
}
