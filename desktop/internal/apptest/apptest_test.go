package apptest_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/internal/environ"
	"github.com/nednella/orchestrator/internal/session"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

// defaultSockets are the tmux sockets that hold the owner's live sessions, under the names of this app and of its
// predecessor.
var defaultSockets = []string{"orchestrator", "agentos"}

func TestTheHarnessNeverUsesADefaultSocket(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  []string
	}{
		{"no environment given", nil},
		{"a default socket given", []string{"ORCHESTRATOR_TMUX_SOCKET=orchestrator"}},
		{"an empty socket given", []string{"ORCHESTRATOR_TMUX_SOCKET="}},
		{"the predecessor's socket given", []string{"ORCHESTRATOR_TMUX_SOCKET=agentos", "AGENTOS_TMUX_SOCKET=agentos"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := apptest.NewWith(t, apptest.Options{Env: tt.env})
			got := environ.Get(h.Env, "ORCHESTRATOR_TMUX_SOCKET")
			if got != h.Socket || got == "" || strings.Contains(strings.Join(defaultSockets, " "), got) {
				t.Errorf("the app's socket = %q, the harness's = %q; want one private name", got, h.Socket)
			}
			s, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
			if err != nil {
				t.Fatal(err)
			}
			if err := exec.Command("tmux", "-L", h.Socket, "has-session", "-t", "="+s.ID).Run(); err != nil {
				t.Errorf("the session is not on the harness's socket: %v", err)
			}
		})
	}
}

func TestHarnessesInOneProcessShareNothing(t *testing.T) {
	t.Parallel()
	before := os.Environ()
	a, b := apptest.New(t), apptest.New(t)
	for _, tt := range []struct {
		name string
		a, b string
	}{
		{"the tmux socket", a.Socket, b.Socket},
		{"the state folder", a.State, b.State},
		{"the config file", a.Conf, b.Conf},
		{"the project folder", a.Dir, b.Dir},
		{"the stand-in claude's folder", a.ClaudeDir(), b.ClaudeDir()},
		{"the Claude settings folder", a.ClaudeConfigDir(), b.ClaudeConfigDir()},
		{"the data folder", environ.Get(a.Env, "ORCHESTRATOR_DEV_DATA_DIR"), environ.Get(b.Env, "ORCHESTRATOR_DEV_DATA_DIR")},
	} {
		if tt.a == "" || tt.a == tt.b {
			t.Errorf("%s: %q and %q; want two names of their own", tt.name, tt.a, tt.b)
		}
	}
	sa, err := a.NewSession("a", "")
	if err != nil {
		t.Fatal(err)
	}
	sb, err := b.NewSession("b", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		h       *apptest.Harness
		in, out string
	}{{a, sa.ID, sb.ID}, {b, sb.ID, sa.ID}} {
		if err := exec.Command("tmux", "-L", tt.h.Socket, "has-session", "-t", "="+tt.in).Run(); err != nil {
			t.Errorf("%s is not on its own socket: %v", tt.in, err)
		}
		if err := exec.Command("tmux", "-L", tt.h.Socket, "has-session", "-t", "="+tt.out).Run(); err == nil {
			t.Errorf("%s is on the socket of another harness", tt.out)
		}
	}
	if after := os.Environ(); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Error("the harness changed the process's environment")
	}
}

func TestRestartKeepsTheHarnessIsolation(t *testing.T) {
	t.Parallel()
	for _, env := range [][]string{
		{"ORCHESTRATOR_TMUX_SOCKET="},
		{"ORCHESTRATOR_TMUX_SOCKET=orchestrator"},
		{"ORCHESTRATOR_TMUX_SOCKET=agentos", "ORCHESTRATOR_STATE_DIR=/nonexistent"},
	} {
		h := apptest.New(t)
		second := h.Restart(t, env...)
		for _, k := range []string{"ORCHESTRATOR_TMUX_SOCKET", "ORCHESTRATOR_STATE_DIR", "ORCHESTRATOR_CONFIG", "ORCHESTRATOR_DEV_DATA_DIR"} {
			if got, want := environ.Get(second.Env, k), environ.Get(h.Env, k); got != want || got == "" {
				t.Errorf("after Restart(%v) %s = %q, want the harness's %q", env, k, got, want)
			}
		}
	}
}
