package projects_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/projects"
	"github.com/nednella/orchestrator/internal/gittest"
)

func TestFolderName(t *testing.T) {
	data, err := os.ReadFile("testdata/folder-names.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Name, Folder string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if got := projects.FolderName(c.Name); got != c.Folder {
			t.Errorf("FolderName(%q) = %q, want %q", c.Name, got, c.Folder)
		}
	}
}

// gitEnv gives git a home of its own with a user, and a default branch when branch is not "".
func gitEnv(t *testing.T, branch string, user bool) {
	t.Helper()
	home := t.TempDir()
	conf := ""
	if user {
		conf += "[user]\n\tname = t\n\temail = t@example.com\n"
	} else {
		conf += "[user]\n\tuseConfigOnly = true\n"
	}
	if branch != "" {
		conf += "[init]\n\tdefaultBranch = " + branch + "\n"
	}
	file := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(file, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", file)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "EMAIL"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func newCreateHarness(t *testing.T) (*apptest.Harness, string) {
	root := filepath.Join(t.TempDir(), "code")
	return apptest.NewWith(t, apptest.Options{TopExtra: "projects_dir: " + root + "\n"}), root
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gittest.Command(dir, args...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func TestCreateProject(t *testing.T) {
	gitEnv(t, "", true)
	h, root := newCreateHarness(t)

	res, err := h.CreateProject("  My App  ")
	dir := filepath.Join(root, "my-app")
	if err != nil || res.Dir != dir || res.CommitError != "" || res.Snapshot.Project.Dir != dir || res.Snapshot.Project.Name != "my-app" {
		t.Fatalf("CreateProject = %+v, %v", res, err)
	}
	if readme, _ := os.ReadFile(filepath.Join(dir, "README.md")); string(readme) != "# My App\n" {
		t.Errorf("README = %q", readme)
	}
	if got := gitOut(t, dir, "log", "--format=%s"); got != "Initial commit" {
		t.Errorf("log = %q", got)
	}
	if got := gitOut(t, dir, "symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Errorf("branch = %q", got)
	}

	again, err := h.CreateProject("My App")
	if err != nil || again.Dir != dir+"-2" || again.Snapshot.Project.Name != "my-app-2" {
		t.Errorf("a second My App = %+v, %v", again, err)
	}

	for _, name := range []string{"", "   "} {
		if _, err := h.CreateProject(name); err == nil {
			t.Errorf("%q was accepted", name)
		}
	}
	if _, err := h.CreateProject(strings.Repeat("x", projects.MaxProjectNameLength+1)); err == nil {
		t.Error("a name over the limit was accepted")
	}
}

func TestCreateProjectUsesGitsDefaultBranch(t *testing.T) {
	gitEnv(t, "trunk", true)
	h, _ := newCreateHarness(t)
	res, err := h.CreateProject("branchy")
	if err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, res.Dir, "symbolic-ref", "--short", "HEAD"); got != "trunk" {
		t.Errorf("branch = %q", got)
	}
}

func TestCreateProjectWithoutAGitUserStillOpens(t *testing.T) {
	gitEnv(t, "", false)
	h, _ := newCreateHarness(t)
	res, err := h.CreateProject("nouser")
	if err != nil || res.CommitError == "" || res.Snapshot.Project.Name != "nouser" || !exists(filepath.Join(res.Dir, "README.md")) {
		t.Fatalf("CreateProject = %+v, %v", res, err)
	}
}

func TestCreateProjectFailureLeavesNoFolder(t *testing.T) {
	gitEnv(t, "", true)
	h, root := newCreateHarness(t)
	t.Setenv("PATH", t.TempDir())
	_, err := h.CreateProject("nogit")
	if err == nil || !strings.Contains(err.Error(), "Git isn't installed") {
		t.Fatalf("err = %v", err)
	}
	if exists(filepath.Join(root, "nogit")) {
		t.Error("the folder was left behind")
	}
	if len(h.Snapshot().Projects) != 1 {
		t.Errorf("projects = %d", len(h.Snapshot().Projects))
	}
}

func TestNewProjectOptions(t *testing.T) {
	h, root := newCreateHarness(t)
	got := h.NewProjectOptions()
	if got.Root != root || got.NameMax != projects.MaxProjectNameLength || !strings.HasSuffix(got.StartFolder, "/") {
		t.Errorf("options = %+v", got)
	}
	if got := h.GitHubOwner(); got != apptest.GHUser {
		t.Errorf("owner = %q", got)
	}
	h.GH.UserErr = errors.New("gh: not logged in")
	if got := h.GitHubOwner(); got != "" {
		t.Errorf("owner of a gh that is not signed in = %q", got)
	}
}

func TestPublishProject(t *testing.T) {
	gitEnv(t, "", true)
	h, _ := newCreateHarness(t)
	res, err := h.CreateProject("shiny")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := h.PublishProject(res.Snapshot.Project.Key)
	if err != nil || repo == "" || h.GH.Calls("repo create shiny --private --source . --push") != 1 {
		t.Errorf("PublishProject = %q, %v; gh calls = %q", repo, err, h.GH.CallLog())
	}
	h.GH.CreateRepoErr = errors.New("gh repo: exit status 1: name already exists on this account")
	if _, err := h.PublishProject(res.Snapshot.Project.Key); err == nil || err.Error() != "name already exists on this account" {
		t.Errorf("err = %v", err)
	}
	if _, err := h.PublishProject("nope"); err == nil {
		t.Error("an unknown project was published")
	}
}

func TestListFolders(t *testing.T) {
	h := newHarness(t)
	root := t.TempDir()
	for _, d := range []string{"b", "a/inner", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := h.ListFolders(root + "/")
	var names []string
	for _, e := range got.Entries {
		names = append(names, e.Name)
	}
	if err != nil || !got.Exists || got.Dir != root+"/" || got.Parent != filepath.Dir(root)+"/" || strings.Join(names, ",") != ".hidden,a,b" {
		t.Errorf("ListFolders = %+v, %v", got, err)
	}
	if got.Entries[1].Path != filepath.Join(root, "a") {
		t.Errorf("entry = %+v", got.Entries[1])
	}
	if missing, err := h.ListFolders(filepath.Join(root, "none") + "/"); err != nil || missing.Exists || len(missing.Entries) != 0 {
		t.Errorf("a missing folder = %+v, %v", missing, err)
	}
	if file, err := h.ListFolders(filepath.Join(root, "file") + "/"); err != nil || file.Exists {
		t.Errorf("a file = %+v, %v", file, err)
	}
	if rel, err := h.ListFolders("./"); err != nil || rel.Dir != h.Dir+"/" {
		t.Errorf("a path in the project = %+v, %v (project %s)", rel, err, h.Dir)
	}
	if home, err := h.ListFolders("~/"); err != nil || !strings.HasPrefix(home.Dir, "/") {
		t.Errorf("home = %+v, %v", home, err)
	}
}
