package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/term"
)

// settleSetup points the command at a private tmux socket, state dir and config,
// starts three sessions under a host in two projects, and works from the first project's folder.
func settleSetup(t *testing.T) (*term.Tmux, string) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	state, err := os.MkdirTemp("", "aossettle")
	if err != nil {
		t.Fatal(err)
	}
	socket := fmt.Sprintf("aostest-settle-%d", os.Getpid())
	t.Cleanup(func() {
		_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
		os.RemoveAll(state)
	})
	demo, other := t.TempDir(), t.TempDir()
	config := filepath.Join(state, "config.yaml")
	body := fmt.Sprintf("projects:\n  - {name: demo, directory: %s}\n  - {name: other, directory: %s}\n", demo, other)
	if err := os.WriteFile(config, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENTOS_TMUX_SOCKET", socket)
	t.Setenv("AGENTOS_STATE_DIR", state)
	t.Setenv("AGENTOS_CONFIG", config)

	tmux, err := term.NewTmux(socket, state)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		name  session.Name
		title string
		dir   string
	}{{session.Name{Project: "demo", Token: "a1"}, "one", demo}, {session.Name{Project: "demo", Token: "b2"}, "two", demo}, {session.Name{Project: "other", Token: "a1"}, "three", other}} {
		if err := tmux.NewSession(context.Background(), s.name, s.title, s.dir, nil, []string{"sleep", "60"}, 80, 24); err != nil {
			t.Fatal(err)
		}
		if err := tmux.SetHosted(context.Background(), s.name); err != nil {
			t.Fatal(err)
		}
		if err := bus.WriteState(state, session.Record{Session: s.name.String(), State: session.Idle}); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(demo)
	return tmux, state
}

func runSettle(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs(append([]string{"settle"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func names(t *testing.T, tmux *term.Tmux) string {
	t.Helper()
	infos, _, err := tmux.ListAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, in := range infos {
		out = append(out, in.Name.String())
	}
	return strings.Join(out, " ")
}

func TestSettleWithoutNumberKeepsStateFiles(t *testing.T) {
	tmux, state := settleSetup(t)
	out := runSettle(t)
	if !strings.Contains(out, "settling demo/a1 (one)") || !strings.Contains(out, "settling demo/b2 (two)") || strings.Contains(out, "other/a1") {
		t.Errorf("output = %q", out)
	}
	if got := names(t, tmux); got != "other/a1" {
		t.Errorf("sessions left: %q", got)
	}
	if left := bus.ReadAll(state, nil); len(left) != 3 || left["demo/a1"].Session == "" || left["demo/b2"].Session == "" {
		t.Errorf("state files left: %v", left)
	}
	if out := runSettle(t); !strings.Contains(out, "no threads to settle") {
		t.Errorf("a second settle printed %q", out)
	}
}

func TestSettleAllSettlesEveryProject(t *testing.T) {
	tmux, state := settleSetup(t)
	runSettle(t, "--all")
	if got := names(t, tmux); got != "" {
		t.Errorf("sessions left: %q", got)
	}
	if left := bus.ReadAll(state, nil); len(left) != 3 {
		t.Errorf("state files left: %v", left)
	}
}

func TestSettleLeavesShellsUnlessAll(t *testing.T) {
	tmux, _ := settleSetup(t)
	shell := session.Name{Project: "demo", Shell: 1}
	if err := tmux.NewSession(context.Background(), shell, "shell", t.TempDir(), nil, []string{"sleep", "60"}, 80, 24); err != nil {
		t.Fatal(err)
	}
	if out := runSettle(t); strings.Contains(out, "shell") || !tmux.Has(context.Background(), shell) {
		t.Errorf("settle touched the shell: %q", out)
	}
	if out := runSettle(t, "--all"); !strings.Contains(out, "stopping demo/shell") || tmux.Has(context.Background(), shell) {
		t.Errorf("settle --all left the shell: %q", out)
	}
}

func TestSettleLeavesTerminalSessions(t *testing.T) {
	tmux, _ := settleSetup(t)
	// A session from a release before the host: it has no hosted tag.
	old := session.Name{Project: "demo", Token: "c3"}
	if err := tmux.NewSession(context.Background(), old, "old", t.TempDir(), nil, []string{"sleep", "60"}, 80, 24); err != nil {
		t.Fatal(err)
	}
	if out := runSettle(t); !strings.Contains(out, "left 1 session that ran in a terminal") || strings.Contains(out, "demo/c3") {
		t.Errorf("settle printed %q", out)
	}
	if out := runSettle(t, "--all"); !strings.Contains(out, "settling other/a1") || !strings.Contains(out, "left 1 session that ran in a terminal") {
		t.Errorf("settle --all printed %q", out)
	}
	if got := names(t, tmux); got != "demo/c3" {
		t.Errorf("sessions left: %q, want only the terminal session", got)
	}
}
