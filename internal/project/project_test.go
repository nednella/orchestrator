package project

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestKey(t *testing.T) {
	tests := []struct{ name, want string }{
		{"api", "api"},
		{"My App", "my-app"},
		{"a.b:c/d", "a-b-c-d"},
		{"--x--", "x"},
		{"日本", "project"},
		{"", "project"},
	}
	for _, tt := range tests {
		if got := (Project{Name: tt.name}).Key(); got != tt.want {
			t.Errorf("Key(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	files := 0
	write := func(body string) string {
		files++
		path := filepath.Join(dir, fmt.Sprintf("config%d.yaml", files))
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	tests := []struct {
		name    string
		path    string
		want    Config
		wantErr bool
	}{
		{name: "missing file", path: filepath.Join(dir, "nope.yaml")},
		{
			name: "an old agent_command is ignored",
			path: write("agent_command: bash\nprojects:\n  - {name: api, directory: /srv/api}\n"),
			want: Config{Projects: []Project{{Name: "api", Dir: "/srv/api"}}},
		},
		{name: "project without dir", path: write("projects:\n  - {name: api}\n"), wantErr: true},
		{
			name: "pull request watch",
			path: write("projects:\n  - {name: api, directory: /srv/api, pr_watch_method: poll, pr_poll_interval: 1m}\n"),
			want: Config{Projects: []Project{{Name: "api", Dir: "/srv/api", PRWatch: "poll", PRPoll: "1m"}}},
		},
		{name: "unknown pr_watch", path: write("projects:\n  - {name: api, directory: /srv/api, pr_watch_method: push}\n"), wantErr: true},
		{name: "bad pr_poll", path: write("projects:\n  - {name: api, directory: /srv/api, pr_poll_interval: often}\n"), wantErr: true},
		{
			name: "retired model keys are ignored",
			path: write("session_model: haiku\nsession_effort: low\nprojects:\n  - {name: api, directory: /srv/api, session_model: opus}\n"),
			want: Config{Projects: []Project{{Name: "api", Dir: "/srv/api"}}},
		},
		{
			name: "queue sections",
			path: write("projects:\n  - name: api\n    directory: /srv/api\n    session_prompt_send: manual\n    queue_sections:\n      - {name: Ready, labels: [ready], actions: [{name: Work, command: \"/work {n}\", model: opus, effort: high}, {name: Plan, kind: chat}]}\n"),
			want: Config{Projects: []Project{{Name: "api", Dir: "/srv/api", SessionPromptSend: "manual", QueueSections: []Section{{Name: "Ready", Labels: []string{"ready"}, Actions: []Action{{Name: "Work", Command: "/work {n}", Model: "opus", Effort: "high"}, {Name: "Plan", Kind: "chat"}}}}}}},
		},
		{name: "section without a name", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{labels: [x]}]}\n"), wantErr: true},
		{name: "two sections of a name", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A}, {name: A}]}\n"), wantErr: true},
		{name: "action without a name", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A, actions: [{command: x}]}]}\n"), wantErr: true},
		{name: "two actions of a name", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A, actions: [{name: X}, {name: X}]}]}\n"), wantErr: true},
		{name: "section after a * section", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: Rest, labels: [\"*\"]}, {name: Ready, labels: [ready]}]}\n"), wantErr: true},
		{name: "draggable_from names itself", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A, draggable_from: [A]}]}\n"), wantErr: true},
		{name: "draggable_from names no section", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A, draggable_from: [B]}]}\n"), wantErr: true},
		{name: "draggable_from on a * section", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A}, {name: Rest, labels: [\"*\"], draggable_from: [A]}]}\n"), wantErr: true},
		{name: "empty draggable_from on a * section", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: Rest, labels: [\"*\"], draggable_from: []}]}\n"), wantErr: true},
		{name: "bad action effort", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A, actions: [{name: X, effort: huge}]}]}\n"), wantErr: true},
		{name: "unknown action kind", path: write("projects:\n  - {name: api, directory: /srv/api, queue_sections: [{name: A, actions: [{name: X, kind: shell}]}]}\n"), wantErr: true},
		{name: "unknown prompt send", path: write("projects:\n  - {name: api, directory: /srv/api, session_prompt_send: later}\n"), wantErr: true},
		{name: "theme", path: write("app_theme: dark\n"), want: Config{Theme: "dark"}},
		{name: "unknown theme", path: write("app_theme: sepia\n"), wantErr: true},
		{name: "bad yaml", path: write("agent: [\n"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got.Projects) != len(tt.want.Projects) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			for i := range got.Projects {
				if !reflect.DeepEqual(got.Projects[i], tt.want.Projects[i]) {
					t.Errorf("project %d = %+v, want %+v", i, got.Projects[i], tt.want.Projects[i])
				}
			}
		})
	}
}

func TestLoadExpandsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("projects:\n  - {name: a, directory: ~/code/a}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "code/a"); cfg.Projects[0].Dir != want {
		t.Errorf("dir = %q, want %q", cfg.Projects[0].Dir, want)
	}
}

func TestResolve(t *testing.T) {
	root := t.TempDir()
	api, web, sub := filepath.Join(root, "api"), filepath.Join(root, "web"), filepath.Join(root, "api", "pkg")
	other := filepath.Join(root, "other")
	for _, d := range []string{api, web, sub, other} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	cfg := Config{Projects: []Project{{Name: "api", Dir: api}, {Name: "apipkg", Dir: sub}, {Name: "web", Dir: web}}}
	tests := []struct {
		name string
		cwd  string
		want Project
	}{
		{"inside a project", api, Project{Name: "api", Dir: api}},
		{"deepest project wins", sub, Project{Name: "apipkg", Dir: sub}},
		{"outside every project", other, Project{Name: "other", Dir: other}},
		{"sibling sharing a prefix", filepath.Join(root, "api2"), Project{Name: "api2", Dir: filepath.Join(root, "api2")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.Resolve(tt.cwd); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resolve(%q) = %+v, want %+v", tt.cwd, got, tt.want)
			}
		})
	}
}

func TestLoadDataDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("data_dir: ~/Notes/agentos\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.DataDir != filepath.Join(home, "Notes/agentos") {
		t.Errorf("data_dir = %q, %v", cfg.DataDir, err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "config.yaml")
	want := Config{Projects: []Project{
		{Name: "a", Dir: "/srv/a", NoteSessionCommand: "/plan {text}", QueueSections: []Section{{Name: "Go", Labels: []string{"go"}, Actions: []Action{{Name: "Work", Command: "/x {n}"}}}}},
		{Name: "b", Dir: "/srv/b"},
	}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveKeepsCommentsAndTildePaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := `# my agentos setup
data_dir: ~/Notes/agentos # synced
projects:
  # the main one
  - name: api
    directory: ~/code/api   # work
    note_session_command: "/plan {text}"
  - {name: web, directory: ~/code/web}
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("adding a project", func(t *testing.T) {
		cfg.Projects = append(cfg.Projects, Project{Name: "new", Dir: "/srv/new"})
		if err := Save(path, cfg); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		for _, want := range []string{"# my agentos setup", "~/Notes/agentos # synced", "# the main one", "~/code/api", "# work", "{name: web, directory: ~/code/web}", "/srv/new"} {
			if !strings.Contains(string(got), want) {
				t.Errorf("the saved file lost %q:\n%s", want, got)
			}
		}
		if strings.Contains(string(got), home) {
			t.Errorf("a ~ path was written out in full:\n%s", got)
		}
		reloaded, err := Load(path)
		if err != nil || !reflect.DeepEqual(reloaded, Config{DataDir: cfg.DataDir, Projects: cfg.Projects}) {
			t.Errorf("reload = %+v, %v", reloaded, err)
		}
	})

	t.Run("removing a project", func(t *testing.T) {
		cfg.Projects = cfg.Projects[1:]
		if err := Save(path, cfg); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if strings.Contains(string(got), "name: api") || !strings.Contains(string(got), "# my agentos setup") || !strings.Contains(string(got), "~/code/web") {
			t.Errorf("saved file:\n%s", got)
		}
		reloaded, err := Load(path)
		if err != nil || len(reloaded.Projects) != 2 || reloaded.Projects[0].Name != "web" || reloaded.Projects[1].Name != "new" {
			t.Errorf("reload = %+v, %v", reloaded, err)
		}
	})

	t.Run("removing every project", func(t *testing.T) {
		cfg.Projects = nil
		if err := Save(path, cfg); err != nil {
			t.Fatal(err)
		}
		reloaded, err := Load(path)
		if err != nil || len(reloaded.Projects) != 0 {
			t.Errorf("reload = %+v, %v", reloaded, err)
		}
	})
}

func TestSaveDropsRetiredModelKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := "session_model: haiku\nsession_effort: low\n# keep\nprojects:\n  - name: api\n    directory: /srv/api\n    session_model: opus\n    session_effort: high\n    note_session_command: x\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "session_model") || strings.Contains(string(got), "session_effort") {
		t.Errorf("the saved file kept a retired key:\n%s", got)
	}
	if !strings.Contains(string(got), "# keep") || !strings.Contains(string(got), "note_session_command: x") {
		t.Errorf("the saved file lost other settings:\n%s", got)
	}
}

func TestLoadRejectsAnUnknownCleanupMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("projects:\n  - name: a\n    directory: /a\n    session_cleanup_mode: {merge: now}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "auto or manual") {
		t.Errorf("Load = %v, want an error naming auto or manual", err)
	}
}

func TestLoadIgnoresAnOldActionAgent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "projects:\n  - name: a\n    directory: /a\n    queue_sections:\n      - name: Inbox\n        actions:\n          - {name: Work, command: \"/work {n}\", agent: codex}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Projects[0].QueueSections[0].Actions; !reflect.DeepEqual(got, []Action{{Name: "Work", Command: "/work {n}"}}) {
		t.Errorf("actions = %+v", got)
	}
}

func TestLoadTextScale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	for scale, ok := range map[string]bool{"1.2": true, "0.4823": true, "2.0736": true, "0.4": false, "3": false, "-1": false} {
		if err := os.WriteFile(path, []byte("app_text_scale: "+scale+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); (err == nil) != ok {
			t.Errorf("app_text_scale %s: err = %v, want ok = %v", scale, err, ok)
		}
	}
}

func TestLoadDraggable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	yaml := "projects:\n  - name: api\n    directory: /srv/api\n    queue_sections:\n" +
		"      - {name: A}\n" +
		"      - {name: B, labels: [b], draggable: false, draggable_from: []}\n" +
		"      - {name: C, labels: [c], draggable_from: [A, B]}\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := cfg.Projects[0].QueueSections[0], cfg.Projects[0].QueueSections[1], cfg.Projects[0].QueueSections[2]
	if a.Draggable != nil || a.DraggableFrom != nil {
		t.Errorf("omitted keys = %v %v, want unset", a.Draggable, a.DraggableFrom)
	}
	if b.Draggable == nil || *b.Draggable {
		t.Errorf("draggable: false = %v", b.Draggable)
	}
	if b.DraggableFrom == nil || len(*b.DraggableFrom) != 0 {
		t.Errorf("draggable_from: [] = %v, want set and empty", b.DraggableFrom)
	}
	if c.DraggableFrom == nil || !reflect.DeepEqual(*c.DraggableFrom, []string{"A", "B"}) {
		t.Errorf("draggable_from = %v", c.DraggableFrom)
	}
}

func TestLoadChecksAnActionsModelWithClaude(t *testing.T) {
	tests := []struct {
		name, config, action, err string
	}{
		{"a claude model on claude", "", "{name: Work, model: opus, effort: max}", ""},
		{"another provider's model on claude", "", "{name: Work, model: other-model}", `claude has no model "other-model"`},
		{"an old agent_command changes nothing", "agent_command: codex\n", "{name: Work, model: gpt-9}", `claude has no model "gpt-9"`},
	}
	for _, tt := range tests {
		path := filepath.Join(t.TempDir(), "config.yaml")
		body := tt.config + "projects:\n  - name: a\n    directory: /a\n    queue_sections:\n      - name: Inbox\n        actions: [" + tt.action + "]\n"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if tt.err == "" && err != nil || tt.err != "" && (err == nil || !strings.Contains(err.Error(), tt.err)) {
			t.Errorf("%s: Load = %v, want %q", tt.name, err, tt.err)
		}
	}
}

func TestThemeKeysValidate(t *testing.T) {
	load := func(yaml string) (Config, error) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		return Load(path)
	}

	cfg, err := load("app_theme_dark: violet\n")
	if err != nil || cfg.DarkTheme() != "violet" || cfg.LightTheme() != "light" {
		t.Errorf("violet = %q, %q, %v", cfg.DarkTheme(), cfg.LightTheme(), err)
	}
	if _, err := load("app_theme_dark: pink\n"); err == nil || !strings.Contains(err.Error(), "app_theme_dark") {
		t.Errorf("pink: err = %v, want one naming app_theme_dark", err)
	}
	if _, err := load("app_theme_light: black\n"); err == nil || !strings.Contains(err.Error(), "app_theme_light") {
		t.Errorf("black as light: err = %v, want one naming app_theme_light", err)
	}
	cfg, err = load("projects: []\n")
	if err != nil || cfg.DarkTheme() != "black" || cfg.LightTheme() != "light" {
		t.Errorf("defaults = %q, %q, %v", cfg.DarkTheme(), cfg.LightTheme(), err)
	}
}

func TestWorktreeCommandDefault(t *testing.T) {
	for _, tt := range []struct{ name, set, want string }{
		{"unset", "", DefaultWorktreeCommand},
		{"set", "git worktree add {worktree} {branch}", "git worktree add {worktree} {branch}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Project{WorktreeCommand: tt.set}).MakeWorktree(); got != tt.want {
				t.Errorf("MakeWorktree() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanupCommandDefault(t *testing.T) {
	for _, tt := range []struct{ name, set, want string }{
		{"unset", "", DefaultCleanupCommand},
		{"set", "git worktree remove {worktree}", "git worktree remove {worktree}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Project{CleanupCommand: tt.set}).RemoveWorktree(); got != tt.want {
				t.Errorf("RemoveWorktree() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorktreeCommandIsKeptOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := "projects:\n  - name: api\n    directory: /srv/api\n    session_worktree_command: \"git worktree add {worktree} {branch}\" # mine\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Projects[0].Branch = "feat/{n}"
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `session_worktree_command: "git worktree add {worktree} {branch}" # mine`) {
		t.Errorf("the saved file lost the worktree command:\n%s", got)
	}
}
