package projects_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/app"
	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/environ"
	"github.com/nednella/orchestrator/internal/project"
)

func TestProjects(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	root := t.TempDir()
	other, clash := filepath.Join(root, "a", "other"), filepath.Join(root, "b", "other")
	for _, d := range []string{other, clash} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	snap, err := h.AddProjectDir(other)
	if err != nil || snap.Project.Name != "other" || snap.Project.Dir != other {
		t.Fatalf("AddProjectDir = %+v, %v", snap.Project, err)
	}
	if again, err := h.AddProjectDir(other); err != nil || len(again.Projects) != 2 {
		t.Errorf("adding the same folder twice: %d projects, %v", len(again.Projects), err)
	}
	if snap, err = h.AddProjectDir(clash); err != nil || snap.Project.Name != "other-2" {
		t.Errorf("name clash gave %+v, %v", snap.Project, err)
	}
	if _, err := h.AddProjectDir(filepath.Join(root, "missing")); err == nil {
		t.Error("a folder that does not exist was accepted")
	}

	cfg, err := project.Load(h.Conf)
	if err != nil || len(cfg.Projects) != 3 || len(cfg.Projects[0].QueueSections) != 3 {
		t.Fatalf("saved config = %+v, %v", cfg, err)
	}
	if got := projects.ReadLast(h.State); got != "other-2" {
		t.Errorf("last project = %q", got)
	}

	t.Run("picker", func(t *testing.T) {
		if dir, err := h.PickFolder(); err != nil || dir != "" {
			t.Errorf("a cancelled picker gave %q, %v", dir, err)
		}
		picked := filepath.Join(root, "c", "picked")
		if err := os.MkdirAll(picked, 0o700); err != nil {
			t.Fatal(err)
		}
		h.Rec.SetPick(picked)
		if dir, err := h.PickFolder(); err != nil || dir != picked {
			t.Errorf("picked folder = %q, %v", dir, err)
		}
	})

	t.Run("a folder that is not there is made on request", func(t *testing.T) {
		made := filepath.Join(root, "d", "made")
		if _, err := h.AddProjectDir(made); err == nil {
			t.Error("a missing folder was added without being made")
		}
		if snap, err := h.AddProjectDirCreating(made); err != nil || snap.Project.Dir != made || !exists(made) {
			t.Errorf("AddProjectDirCreating = %+v, %v", snap.Project, err)
		}
	})

	t.Run("projects event", func(t *testing.T) {
		added := filepath.Join(root, "e", "added")
		if err := os.MkdirAll(added, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := h.AddProjectDir(added); err != nil {
			t.Fatal(err)
		}
		eventually(t, "a projects event with the added project", func() bool {
			list, _ := h.Rec.Last("projects").([]threads.Project)
			return slices.ContainsFunc(list, func(p threads.Project) bool { return p.Dir == added })
		})

		a, err := h.StartThread("a", "")
		if err != nil {
			t.Fatal(err)
		}
		h.Threads().Refresh()
		sent := h.Rec.Count("projects")
		h.Observe(t, a.ID, "PreToolUse", `{"tool_name":"Edit"}`)
		h.Threads().Refresh()
		if n := h.Rec.Count("projects"); n != sent {
			t.Errorf("a change of a thread's state sent the projects %d more times", n-sent)
		}
	})

	t.Run("last project at start", func(t *testing.T) {
		if _, err := h.SwitchProject("other-2"); err != nil {
			t.Fatal(err)
		}
		cfg, err := app.LoadEnv(environ.With(h.Env, "ORCHESTRATOR_DIR=/"))
		if err != nil || cfg.Project.Name != "other-2" {
			t.Errorf("from Finder: %+v, %v", cfg.Project, err)
		}
		if cfg, _ = app.LoadEnv(environ.With(h.Env, "ORCHESTRATOR_DIR="+h.Dir)); cfg.Project.Name != "main" {
			t.Errorf("inside a known folder: %q", cfg.Project.Name)
		}
		if err := projects.WriteLast(h.State, "gone"); err != nil {
			t.Fatal(err)
		}
		if cfg, _ = app.LoadEnv(environ.With(h.Env, "ORCHESTRATOR_DIR=/")); cfg.Project.Name != "main" {
			t.Errorf("unknown last project gave %q, want the first one", cfg.Project.Name)
		}
	})
}

func TestRegistryKeepAwake(t *testing.T) {
	t.Parallel()
	off := false
	r := projects.NewRegistry("", project.Config{
		KeepAwake: &off,
		Projects:  []project.Project{{Name: "Quiet Work", Dir: "/q", KeepAwake: new(true)}, {Name: "plain", Dir: "/p"}},
	})
	for key, want := range map[string]bool{"quiet-work": true, "plain": false, "unlisted": false} {
		if got := r.KeepAwake(key); got != want {
			t.Errorf("KeepAwake(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestRegistryDir(t *testing.T) {
	t.Parallel()
	r := projects.NewRegistry("", project.Config{Projects: []project.Project{{Name: "Quiet Work", Dir: "/q"}}})
	for key, want := range map[string]string{"quiet-work": "/q", "unlisted": ""} {
		if got := r.Dir(key); got != want {
			t.Errorf("Dir(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestRegistryListSkipsMissingFolders(t *testing.T) {
	t.Parallel()
	here := t.TempDir()
	path := filepath.Join(t.TempDir(), "config.yaml")
	r := projects.NewRegistry(path, project.Config{
		Projects: []project.Project{{Name: "here", Dir: here}, {Name: "elsewhere", Dir: filepath.Join(here, "missing")}},
	})
	if got := r.List(); len(got) != 1 || got[0].Name != "here" {
		t.Fatalf("List = %+v, want only here", got)
	}
	if _, err := r.Add(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	cfg, err := project.Load(path)
	if err != nil || len(cfg.Projects) != 3 || cfg.Projects[1].Name != "elsewhere" {
		t.Errorf("saved config = %+v, %v; want the missing project kept", cfg.Projects, err)
	}
}
