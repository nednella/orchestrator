package projects_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/app"
	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/project"
)

func TestProjects(t *testing.T) {
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
		before := h.Snapshot()
		if snap, err := h.AddProject(); err != nil || snap.Project.Name != before.Project.Name {
			t.Errorf("a cancelled picker changed the project: %+v, %v", snap.Project, err)
		}
		picked := filepath.Join(root, "c", "picked")
		if err := os.MkdirAll(picked, 0o700); err != nil {
			t.Fatal(err)
		}
		h.Rec.SetPick(picked)
		if snap, err := h.AddProject(); err != nil || snap.Project.Name != "picked" {
			t.Errorf("picked project = %+v, %v", snap.Project, err)
		}
	})

	t.Run("counts", func(t *testing.T) {
		if _, err := h.SwitchProject("main"); err != nil {
			t.Fatal(err)
		}
		a, err := h.NewSession("a", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.SwitchProject("other"); err != nil {
			t.Fatal(err)
		}
		b, err := h.NewSession("b", "")
		if err != nil {
			t.Fatal(err)
		}
		h.Observe(t, a.ID, "Notification", `{"message":"needs you"}`)
		h.Observe(t, b.ID, "PreToolUse", `{"tool_name":"Edit"}`)
		counts := func() string {
			var out []string
			list, _ := h.Rec.Last("projects").([]sessions.Project)
			for _, p := range list {
				if p.Sessions > 0 {
					out = append(out, fmt.Sprintf("%s:%d/%d/%d", p.Name, p.NeedsYou, p.Working, p.Sessions))
				}
			}
			return strings.Join(out, " ")
		}
		eventually(t, "counts across projects", func() bool { return counts() == "main:1/0/1 other:0/1/1" })
		var viaSnapshot []string
		for _, p := range h.Snapshot().Projects {
			if p.Sessions > 0 {
				viaSnapshot = append(viaSnapshot, fmt.Sprintf("%s:%d/%d/%d", p.Name, p.NeedsYou, p.Working, p.Sessions))
			}
		}
		if got := strings.Join(viaSnapshot, " "); got != "main:1/0/1 other:0/1/1" {
			t.Errorf("snapshot counts = %s", got)
		}
	})

	t.Run("last project at start", func(t *testing.T) {
		if _, err := h.SwitchProject("other-2"); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ORCHESTRATOR_DIR", "/")
		cfg, err := app.Load()
		if err != nil || cfg.Project.Name != "other-2" {
			t.Errorf("from Finder: %+v, %v", cfg.Project, err)
		}
		t.Setenv("ORCHESTRATOR_DIR", h.Dir)
		if cfg, _ = app.Load(); cfg.Project.Name != "main" {
			t.Errorf("inside a known folder: %q", cfg.Project.Name)
		}
		t.Setenv("ORCHESTRATOR_DIR", "/")
		if err := projects.WriteLast(h.State, "gone"); err != nil {
			t.Fatal(err)
		}
		if cfg, _ = app.Load(); cfg.Project.Name != "main" {
			t.Errorf("unknown last project gave %q, want the first one", cfg.Project.Name)
		}
	})
}

func TestRegistryKeepAwake(t *testing.T) {
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
	r := projects.NewRegistry("", project.Config{Projects: []project.Project{{Name: "Quiet Work", Dir: "/q"}}})
	for key, want := range map[string]string{"quiet-work": "/q", "unlisted": ""} {
		if got := r.Dir(key); got != want {
			t.Errorf("Dir(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestRegistryListSkipsMissingFolders(t *testing.T) {
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
