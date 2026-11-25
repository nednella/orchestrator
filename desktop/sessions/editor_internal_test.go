package sessions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/term"
)

type fixedProjects []project.Project

func (f fixedProjects) List() []project.Project   { return f }
func (fixedProjects) Remove(string) (bool, error) { return false, nil }

type opened struct {
	mu    sync.Mutex
	calls [][]string
	err   error
}

func (o *opened) run(_ context.Context, _, name string, args ...string) ([]byte, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls = append(o.calls, append([]string{name}, args...))
	return nil, o.err
}

func editorSessions(t *testing.T, o *opened, app func() string, projectDir string) (*Sessions, session.Name) {
	t.Helper()
	proj := project.Project{Name: "shop", Dir: projectDir}
	s := New(Options{LocalDir: t.TempDir(), StateDir: t.TempDir(), Run: o.run, Projects: fixedProjects{proj}, Current: proj, EditorApp: app, Emit: func(string, any) {}})
	return s, session.Name{Project: proj.Key(), Token: "aaaaaaaa"}
}

func TestOpenInEditorOpensTheWorktreeOfAThread(t *testing.T) {
	projectDir, wt := t.TempDir(), filepath.Join(t.TempDir(), "my trees", "issue 7")
	if err := os.MkdirAll(wt, 0o700); err != nil {
		t.Fatal(err)
	}
	o := &opened{}
	s, name := editorSessions(t, o, nil, projectDir)
	s.info = []term.Info{{Name: name, Path: wt, Branch: "issue-7", Hosted: true}}
	if err := s.OpenInEditor(name.String()); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"open", "-a", "Visual Studio Code", wt}}
	if !reflect.DeepEqual(o.calls, want) {
		t.Errorf("runs = %q, want %q", o.calls, want)
	}
}

func TestOpenInEditorPrefersTheWorktreeTheBranchIsReadIn(t *testing.T) {
	projectDir, started, followed := t.TempDir(), t.TempDir(), t.TempDir()
	o := &opened{}
	s, name := editorSessions(t, o, nil, projectDir)
	s.info = []term.Info{{Name: name, Path: started, Hosted: true}}
	s.life.tracks[name.String()] = &track{worktree: followed}
	if err := s.OpenInEditor(name.String()); err != nil {
		t.Fatal(err)
	}
	if got := o.calls[0][3]; got != followed {
		t.Errorf("opened %q, want %q", got, followed)
	}
}

func TestOpenInEditorOpensTheProjectFolderOfAThreadWithoutAWorktree(t *testing.T) {
	projectDir := t.TempDir()
	o := &opened{}
	s, name := editorSessions(t, o, nil, projectDir)
	s.info = []term.Info{{Name: name, Hosted: true}}
	if err := s.OpenInEditor(name.String()); err != nil {
		t.Fatal(err)
	}
	if got := o.calls[0][3]; got != projectDir {
		t.Errorf("opened %q, want %q", got, projectDir)
	}
}

func TestOpenInEditorNeverFallsBackToTheProjectFolderForARemovedWorktree(t *testing.T) {
	projectDir, wt := t.TempDir(), filepath.Join(t.TempDir(), "gone")
	o := &opened{}
	s, name := editorSessions(t, o, nil, projectDir)
	s.info = []term.Info{{Name: name, Path: wt, Branch: "issue-7", Hosted: true}}
	err := s.OpenInEditor(name.String())
	if err == nil || err.Error() != "This thread's folder is gone" {
		t.Errorf("err = %v, want the folder is gone", err)
	}
	if len(o.calls) != 0 {
		t.Errorf("ran %q for a folder that is gone", o.calls)
	}
}

func TestOpenInEditorWorksForASettledThread(t *testing.T) {
	wt := t.TempDir()
	o := &opened{}
	s, name := editorSessions(t, o, nil, t.TempDir())
	s.ended[name.String()] = &endedSession{info: term.Info{Name: name, Path: wt, Hosted: true}}
	if err := s.OpenInEditor(name.String()); err != nil {
		t.Fatal(err)
	}
	if got := o.calls[0][3]; got != wt {
		t.Errorf("opened %q, want %q", got, wt)
	}
}

func TestOpenInEditorUsesTheConfiguredApp(t *testing.T) {
	o := &opened{}
	s, name := editorSessions(t, o, func() string { return "Cursor" }, t.TempDir())
	s.info = []term.Info{{Name: name, Hosted: true}}
	if err := s.OpenInEditor(name.String()); err != nil {
		t.Fatal(err)
	}
	if got := o.calls[0][2]; got != "Cursor" {
		t.Errorf("app = %q, want Cursor", got)
	}
}

func TestOpenInEditorSaysWhyItFailed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"app missing", errors.New("open -a: exit status 1: Unable to find application named 'Visual Studio Code'"), "VS Code is not installed"},
		{"other failure", errors.New("open -a: exit status 1: boom"), "VS Code did not open"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &opened{err: tt.err}
			s, name := editorSessions(t, o, nil, t.TempDir())
			s.info = []term.Info{{Name: name, Hosted: true}}
			if err := s.OpenInEditor(name.String()); err == nil || err.Error() != tt.want {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestOpenInEditorRefusesWhatIsNotAThread(t *testing.T) {
	o := &opened{}
	s, name := editorSessions(t, o, nil, t.TempDir())
	for _, id := range []string{name.String(), "../../etc", session.Name{Project: "shop", Shell: 1}.String(), ""} {
		if err := s.OpenInEditor(id); err == nil {
			t.Errorf("OpenInEditor(%q) = nil, want an error", id)
		}
	}
	if len(o.calls) != 0 {
		t.Errorf("ran %q", o.calls)
	}
}
