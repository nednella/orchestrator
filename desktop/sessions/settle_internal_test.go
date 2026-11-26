package sessions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/session"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/tmuxtest"
)

func TestSettleClosesNoTerminal(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("aostest-settle-%d", os.Getpid())
	t.Cleanup(func() { tmuxtest.Stop(socket) })
	tmux, err := term.NewTmux(socket, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	proj := project.Project{Name: "p", Dir: dir}
	noCommands := func(context.Context, string, string, ...string) ([]byte, error) {
		return nil, errors.New("this test runs no commands")
	}
	var closed []string
	s := New(Options{
		Tmux: tmux, LocalDir: t.TempDir(), StateDir: t.TempDir(), Run: noCommands, Projects: fixedProjects{proj}, Current: proj,
		Activity: noActivity{}, Browsers: fakeBrowsers{}, Evidence: noEvidence{}, Emit: func(string, any) {},
		CloseTerminal: func(id string) { closed = append(closed, id) },
	})
	name := session.Name{Project: "p", Token: "abcdefgh"}
	if err := tmux.NewSession(t.Context(), name, "a thread", dir, nil, []string{"sleep", "60"}, 80, 24); err != nil {
		t.Fatal(err)
	}
	s.info = append(s.info, term.Info{Name: name, Path: dir})

	if err := s.settle(name.String(), chat.StopSettle, ""); err != nil {
		t.Fatal(err)
	}
	if tmux.Has(t.Context(), name) {
		t.Error("the thread still runs")
	}
	if len(closed) != 0 {
		t.Errorf("settle closed the terminals %q, want none", closed)
	}
}
