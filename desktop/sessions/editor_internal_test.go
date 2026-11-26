package sessions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/session"
	"github.com/nednella/orchestrator/internal/term"
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
	s.info = []term.Info{{Name: name, Path: wt, Branch: "issue-7"}}
	if err := s.OpenInEditor(name.String(), ""); err != nil {
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
	s.info = []term.Info{{Name: name, Path: started}}
	s.life.tracks[name.String()] = &track{worktree: followed}
	if err := s.OpenInEditor(name.String(), ""); err != nil {
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
	s.info = []term.Info{{Name: name}}
	if err := s.OpenInEditor(name.String(), ""); err != nil {
		t.Fatal(err)
	}
	if got := o.calls[0][3]; got != projectDir {
		t.Errorf("opened %q, want %q", got, projectDir)
	}
}

func TestOpenInEditorOpensTheFolderOfAThreadOfAnotherProject(t *testing.T) {
	currentDir, otherDir := t.TempDir(), t.TempDir()
	o := &opened{}
	current, other := project.Project{Name: "shop", Dir: currentDir}, project.Project{Name: "blog", Dir: otherDir}
	s := New(Options{LocalDir: t.TempDir(), StateDir: t.TempDir(), Run: o.run, Projects: fixedProjects{current, other}, Current: current, Emit: func(string, any) {}})
	name := session.Name{Project: other.Key(), Token: "bbbbbbbb"}
	s.info = []term.Info{{Name: name}}
	if err := s.OpenInEditor(name.String(), ""); err != nil {
		t.Fatal(err)
	}
	if got := o.calls[0][3]; got != otherDir {
		t.Errorf("opened %q, want the project of the thread, %q", got, otherDir)
	}
}

func TestOpenInEditorNeverFallsBackToTheProjectFolderForARemovedWorktree(t *testing.T) {
	projectDir, wt := t.TempDir(), filepath.Join(t.TempDir(), "gone")
	o := &opened{}
	s, name := editorSessions(t, o, nil, projectDir)
	s.info = []term.Info{{Name: name, Path: wt, Branch: "issue-7"}}
	err := s.OpenInEditor(name.String(), "")
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
	s.ended[name.String()] = &endedSession{info: term.Info{Name: name, Path: wt}}
	if err := s.OpenInEditor(name.String(), ""); err != nil {
		t.Fatal(err)
	}
	if got := o.calls[0][3]; got != wt {
		t.Errorf("opened %q, want %q", got, wt)
	}
}

func TestOpenInEditorUsesTheConfiguredApp(t *testing.T) {
	o := &opened{}
	s, name := editorSessions(t, o, func() string { return "Cursor" }, t.TempDir())
	s.info = []term.Info{{Name: name}}
	if err := s.OpenInEditor(name.String(), ""); err != nil {
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
			s.info = []term.Info{{Name: name}}
			if err := s.OpenInEditor(name.String(), ""); err == nil || err.Error() != tt.want {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestOpenInEditorRefusesWhatIsNotAThread(t *testing.T) {
	o := &opened{}
	s, name := editorSessions(t, o, nil, t.TempDir())
	for _, id := range []string{name.String(), "../../etc", session.Name{Project: "shop", Shell: 1}.String(), ""} {
		if err := s.OpenInEditor(id, ""); err == nil {
			t.Errorf("OpenInEditor(%q) = nil, want an error", id)
		}
	}
	if len(o.calls) != 0 {
		t.Errorf("ran %q", o.calls)
	}
}

func TestOpenInEditorOpensAFileInTheFolder(t *testing.T) {
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "docs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "docs", "plan.md"), []byte("# Plan"), 0o600); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(wt)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/plan.md", filepath.Join(wt, "docs", "plan.md")} {
		o := &opened{}
		s, name := editorSessions(t, o, nil, t.TempDir())
		s.info = []term.Info{{Name: name, Path: wt}}
		if err := s.OpenInEditor(name.String(), path); err != nil {
			t.Fatalf("OpenInEditor(%q): %v", path, err)
		}
		want := [][]string{{"open", "-a", "Visual Studio Code", filepath.Join(real, "docs", "plan.md")}}
		if !reflect.DeepEqual(o.calls, want) {
			t.Errorf("OpenInEditor(%q) ran %q, want %q", path, o.calls, want)
		}
	}
}

func TestOpenInEditorRefusesAFileOutside(t *testing.T) {
	parent := t.TempDir()
	wt := filepath.Join(parent, "issue-7")
	if err := os.MkdirAll(wt, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{filepath.Join(parent, "secret.md"): "# Secret", filepath.Join(wt, "notes.txt"): "notes"} {
		if err := os.WriteFile(name, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(parent, "secret.md"), filepath.Join(wt, "out.md")); err != nil {
		t.Fatal(err)
	}
	o := &opened{}
	s, name := editorSessions(t, o, nil, t.TempDir())
	s.info = []term.Info{{Name: name, Path: wt}}
	s.nums[name.String()] = 3
	for path, want := range map[string]string{
		"../secret.md":                     "This file is outside thread 3's folder",
		filepath.Join(parent, "secret.md"): "This file is outside thread 3's folder",
		"out.md":                           "This file is outside thread 3's folder",
		"notes.txt":                        "This is not a markdown file",
	} {
		if err := s.OpenInEditor(name.String(), path); err == nil || err.Error() != want {
			t.Errorf("OpenInEditor(%q) = %v, want %q", path, err, want)
		}
	}
	if len(o.calls) != 0 {
		t.Errorf("ran %q for a file it refused", o.calls)
	}
}

func TestOpenMarkdownRefusesAFileSwappedAfterTheCheck(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret.md")
	for name, swap := range map[string]struct{ target, want string }{
		"a file in the folder": {"other.md", "This file changed while it was opened"},
		"a file outside":       {outside, "Could not read this file"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for file, text := range map[string]string{"plan.md": "# Plan", "other.md": "# Other", outside: "# Secret"} {
				if !filepath.IsAbs(file) {
					file = filepath.Join(dir, file)
				}
				if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			beforeOpen = func() {
				plan := filepath.Join(dir, "plan.md")
				if err := os.Remove(plan); err != nil {
					t.Error(err)
				}
				if err := os.Symlink(swap.target, plan); err != nil {
					t.Error(err)
				}
			}
			t.Cleanup(func() { beforeOpen = nil })
			f, _, err := OpenMarkdown(dir, "plan.md", "thread 3")
			if err == nil {
				f.Close()
			}
			if err == nil || err.Error() != swap.want {
				t.Errorf("OpenMarkdown of a file swapped after the check = %v, want %q", err, swap.want)
			}
		})
	}
}
