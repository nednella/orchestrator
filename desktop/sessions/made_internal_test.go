package sessions

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/internal/run"
)

var isolatedGit = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null"}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), isolatedGit...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// madeRepo is a throwaway repo with one commit, and a Lifecycle whose record lives in local.
func madeRepo(t *testing.T) (l *Lifecycle, dir, local string) {
	t.Helper()
	dir, local = t.TempDir(), t.TempDir()
	gitT(t, dir, "init")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", "README")
	gitT(t, dir, "commit", "-m", "first")
	runner := func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		return run.ExecEnv(ctx, dir, append(os.Environ(), isolatedGit...), name, args...)
	}
	s := newSessions(Options{LocalDir: local})
	return newLifecycle(runner, nil, local, t.TempDir(), s, noActivity{}, func(string, any) {}), dir, local
}

// madeTree adds a worktree on a new branch at trees/<branch>, records it for thread and marks it.
func madeTree(t *testing.T, l *Lifecycle, dir, branch, thread string) string {
	t.Helper()
	wt := filepath.Join(dir, "trees", branch)
	gitT(t, dir, "worktree", "add", "-b", branch, wt)
	if err := l.sessions.made.Add("p", Made{Path: wt, Branch: branch, Base: "main", Thread: thread, Created: time.Now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if err := l.mark(t.Context(), wt, thread); err != nil {
		t.Fatal(err)
	}
	return wt
}

func TestMadeSurvivesAReload(t *testing.T) {
	local := t.TempDir()
	w := Made{Path: filepath.Join(local, "trees", "issue-7"), Branch: "issue-7", Base: "main", BaseOid: "abc", Thread: "p/aaaaaaaa", Created: 1}
	if err := newMadeTrees(local).Add("p", w); err != nil {
		t.Fatal(err)
	}
	got, ok := newMadeTrees(local).Of("p", "p/aaaaaaaa")
	if !ok || got != w {
		t.Errorf("reloaded record = %+v, %v; want %+v", got, ok, w)
	}
	if _, ok := newMadeTrees(local).Of("q", "p/aaaaaaaa"); ok {
		t.Error("another project's record holds the thread")
	}
}

func TestMadeTreesRecord(t *testing.T) {
	local := t.TempDir()
	trees := filepath.Join(local, "trees")
	if err := os.MkdirAll(filepath.Join(trees, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(trees, link); err != nil {
		t.Fatal(err)
	}
	at := time.UnixMilli(5000)
	tests := []struct {
		name    string
		records []Made
		change  func(m *madeTrees) error
		thread  string // asked of Of
		path    string // asked of At
		want    Made
		wantErr bool
	}{
		{
			name:    "the newest record of a thread",
			records: []Made{{Path: filepath.Join(trees, "a"), Thread: "p/t1"}, {Path: filepath.Join(trees, "b"), Thread: "p/t1"}},
			thread:  "p/t1", path: filepath.Join(trees, "b"),
			want: Made{Path: canonicalPath(filepath.Join(trees, "b")), Thread: "p/t1"},
		},
		{
			name:    "the newest record at a path agentos used twice",
			records: []Made{{Path: filepath.Join(trees, "a"), Thread: "p/t1", Removed: 9}, {Path: filepath.Join(trees, "a"), Thread: "p/t2"}},
			thread:  "p/t2", path: filepath.Join(trees, "a"),
			want: Made{Path: canonicalPath(filepath.Join(trees, "a")), Thread: "p/t2"},
		},
		{
			name:    "a path found through a link",
			records: []Made{{Path: filepath.Join(trees, "a"), Thread: "p/t1"}},
			thread:  "p/t1", path: filepath.Join(link, "a"),
			want: Made{Path: canonicalPath(filepath.Join(trees, "a")), Thread: "p/t1"},
		},
		{
			name:    "branch set",
			records: []Made{{Path: filepath.Join(trees, "a"), Branch: "issue-7", Thread: "p/t1"}},
			change:  func(m *madeTrees) error { return m.SetBranch("p", filepath.Join(trees, "a"), "issue-8") },
			thread:  "p/t1", path: filepath.Join(trees, "a"),
			want: Made{Path: canonicalPath(filepath.Join(trees, "a")), Branch: "issue-8", Thread: "p/t1"},
		},
		{
			name:    "removed",
			records: []Made{{Path: filepath.Join(trees, "a"), Thread: "p/t1"}},
			change:  func(m *madeTrees) error { return m.Removed("p", filepath.Join(trees, "a"), at) },
			thread:  "p/t1", path: filepath.Join(trees, "a"),
			want: Made{Path: canonicalPath(filepath.Join(trees, "a")), Thread: "p/t1", Removed: 5000},
		},
		{
			name:    "restored",
			records: []Made{{Path: filepath.Join(trees, "a"), Thread: "p/t1", Removed: 5000}},
			change:  func(m *madeTrees) error { return m.Restored("p", filepath.Join(trees, "a")) },
			thread:  "p/t1", path: filepath.Join(trees, "a"),
			want: Made{Path: canonicalPath(filepath.Join(trees, "a")), Thread: "p/t1"},
		},
		{
			name:    "a change at a path agentos never used",
			records: []Made{{Path: filepath.Join(trees, "a"), Thread: "p/t1"}},
			change:  func(m *madeTrees) error { return m.Removed("p", filepath.Join(trees, "elsewhere"), at) },
			thread:  "p/t1", path: filepath.Join(trees, "a"),
			want:    Made{Path: canonicalPath(filepath.Join(trees, "a")), Thread: "p/t1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			m := newMadeTrees(dir)
			for _, w := range tt.records {
				if err := m.Add("p", w); err != nil {
					t.Fatal(err)
				}
			}
			if tt.change != nil {
				if err := tt.change(m); (err != nil) != tt.wantErr {
					t.Fatalf("change error = %v, want error %v", err, tt.wantErr)
				}
			}
			reloaded := newMadeTrees(dir)
			if got, ok := reloaded.Of("p", tt.thread); !ok || got != tt.want {
				t.Errorf("Of = %+v, %v; want %+v", got, ok, tt.want)
			}
			if got, ok := reloaded.At("p", tt.path); !ok || got != tt.want {
				t.Errorf("At = %+v, %v; want %+v", got, ok, tt.want)
			}
		})
	}
}

func TestForgetDropsOnlyTheThreadsRecords(t *testing.T) {
	trees := filepath.Join(t.TempDir(), "trees")
	a, b := filepath.Join(trees, "a"), filepath.Join(trees, "b")
	tests := []struct {
		name     string
		records  []Made
		thread   string
		wantKept []string // the threads whose records stay, oldest first
	}{
		{"every record of the thread", []Made{{Path: a, Thread: "p/t1", Removed: 9}, {Path: b, Thread: "p/t2"}, {Path: a, Thread: "p/t1"}}, "p/t1", []string{"p/t2"}},
		{"a thread with no record", []Made{{Path: a, Thread: "p/t1"}}, "p/t3", []string{"p/t1"}},
		{"the only record", []Made{{Path: a, Thread: "p/t1"}}, "p/t1", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			m := newMadeTrees(dir)
			for _, w := range tt.records {
				if err := m.Add("p", w); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.Forget("p", tt.thread); err != nil {
				t.Fatal(err)
			}
			reloaded := newMadeTrees(dir)
			list, err := reloaded.list("p")
			if err != nil {
				t.Fatal(err)
			}
			var kept []string
			for _, w := range list {
				kept = append(kept, w.Thread)
			}
			if !slices.Equal(kept, tt.wantKept) {
				t.Errorf("records of %v kept, want %v", kept, tt.wantKept)
			}
			if _, ok := reloaded.Of("p", tt.thread); ok {
				t.Errorf("%s still has a record", tt.thread)
			}
		})
	}
}

func TestRecordOfAMissingFolderIsFoundByAnySpelling(t *testing.T) {
	trees := filepath.Join(t.TempDir(), "trees")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(trees, link); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		added, asked string
	}{
		{"added by the link, asked by the folder", filepath.Join(link, "a"), filepath.Join(trees, "a")},
		{"added by the folder, asked by the link", filepath.Join(trees, "a"), filepath.Join(link, "a")},
		{"added and asked by the link", filepath.Join(link, "a"), filepath.Join(link, "a")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Join(trees, "a"), 0o700); err != nil {
				t.Fatal(err)
			}
			m := newMadeTrees(t.TempDir())
			if err := m.Add("p", Made{Path: tt.added, Thread: "p/t1"}); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Join(trees, "a")); err != nil {
				t.Fatal(err)
			}
			if err := m.Removed("p", tt.asked, time.UnixMilli(5000)); err != nil {
				t.Fatalf("Removed by another spelling of a missing folder: %v", err)
			}
			if got, ok := m.At("p", tt.asked); !ok || got.Removed != 5000 {
				t.Errorf("At = %+v, %v; want the record, removed at 5000", got, ok)
			}
		})
	}
}

func TestEmptyThreadIsRefused(t *testing.T) {
	l, dir, local := madeRepo(t)
	wt := filepath.Join(dir, "trees", "issue-7")
	gitT(t, dir, "worktree", "add", "-b", "issue-7", wt)
	if err := l.sessions.made.Add("p", Made{Path: wt}); err == nil {
		t.Error("recorded a worktree for no thread")
	}
	if err := l.mark(t.Context(), wt, ""); err == nil {
		t.Error("marked a worktree for no thread")
	}
	gitDir := gitT(t, wt, "rev-parse", "--path-format=absolute", "--git-dir")
	if err := os.WriteFile(filepath.Join(gitDir, markerFile), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(local, "p"), 0o700); err != nil {
		t.Fatal(err)
	}
	record := `[{"path":` + strconv.Quote(canonicalPath(wt)) + `,"thread":""}]`
	if err := os.WriteFile(filepath.Join(local, "p", "worktrees.json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := l.sessions.made.At("p", wt); !ok {
		t.Fatal("the hand-written record is not read")
	}
	if l.ours(t.Context(), "p", wt, "") {
		t.Error("a worktree is ours for no thread")
	}
}

func TestUnreadableRecordIsNeverOverwritten(t *testing.T) {
	local := t.TempDir()
	m := newMadeTrees(local)
	if err := os.MkdirAll(filepath.Join(local, "p"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.path("p"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.Add("p", Made{Path: "/x", Thread: "p/t1"}); err == nil {
		t.Error("Add over an unreadable record succeeded")
	}
	if err := m.Removed("p", "/x", time.Now()); err == nil {
		t.Error("Removed over an unreadable record succeeded")
	}
	if data, _ := os.ReadFile(m.path("p")); string(data) != "not json" {
		t.Errorf("record = %q, want it untouched", data)
	}
	if _, ok := m.At("p", "/x"); ok {
		t.Error("an unreadable record claims a worktree")
	}
}

func TestMarkerIsInvisibleToGit(t *testing.T) {
	l, dir, _ := madeRepo(t)
	wt := filepath.Join(dir, "trees", "issue-7")
	gitT(t, dir, "worktree", "add", "-b", "issue-7", wt)
	before := gitT(t, dir, "worktree", "list", "--porcelain")
	gitDir := gitT(t, wt, "rev-parse", "--path-format=absolute", "--git-dir")

	if err := l.mark(t.Context(), wt, "p/t1"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(gitDir, markerFile)); err != nil || strings.TrimSpace(string(data)) != "p/t1" {
		t.Fatalf("marker = %q, %v", data, err)
	}
	if status := gitT(t, wt, "status", "--porcelain", "--ignored"); status != "" {
		t.Errorf("status after mark = %q, want clean", status)
	}
	after := gitT(t, dir, "worktree", "list", "--porcelain")
	if after != before {
		t.Errorf("worktree list after mark = %q, want %q", after, before)
	}
	if w := parseWorktrees(after); !samePath(w.byBranch["issue-7"], wt) || !samePath(w.main, dir) {
		t.Errorf("parsed worktrees = %+v", w)
	}
	gitT(t, dir, "worktree", "remove", wt)
	if _, err := os.Stat(gitDir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the worktree's git folder outlived it: %v", err)
	}
	if _, err := os.Stat(wt); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the worktree outlived its removal: %v", err)
	}
}

func TestHumanWorktreeAtARecordedPathIsNotOurs(t *testing.T) {
	tests := []struct {
		name  string
		again func(t *testing.T, dir, wt string)
	}{
		{"removed with git and added again", func(t *testing.T, dir, wt string) {
			gitT(t, dir, "worktree", "remove", wt)
			gitT(t, dir, "worktree", "add", wt, "issue-7")
		}},
		{"deleted, pruned and added again", func(t *testing.T, dir, wt string) {
			if err := os.RemoveAll(wt); err != nil {
				t.Fatal(err)
			}
			gitT(t, dir, "worktree", "prune")
			gitT(t, dir, "worktree", "add", wt, "issue-7")
		}},
		{"deleted and forced in again", func(t *testing.T, dir, wt string) {
			if err := os.RemoveAll(wt); err != nil {
				t.Fatal(err)
			}
			gitT(t, dir, "worktree", "add", "-f", wt, "issue-7")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, dir, _ := madeRepo(t)
			wt := madeTree(t, l, dir, "issue-7", "p/t1")
			if !l.ours(t.Context(), "p", wt, "p/t1") {
				t.Fatal("the worktree agentos made is not ours")
			}
			tt.again(t, dir, wt)
			if _, ok := l.sessions.made.At("p", wt); !ok {
				t.Fatal("the record of the path is gone")
			}
			if l.ours(t.Context(), "p", wt, "p/t1") {
				t.Error("a worktree a human made at a recorded path is ours")
			}
			if _, err := os.Stat(filepath.Join(wt, "README")); err != nil {
				t.Errorf("the human's worktree is not there: %v", err)
			}
		})
	}
}

func TestRemovedRecordIsNotOurs(t *testing.T) {
	l, dir, _ := madeRepo(t)
	wt := madeTree(t, l, dir, "issue-7", "p/t1")
	if err := l.sessions.made.Removed("p", wt, time.Now()); err != nil {
		t.Fatal(err)
	}
	if l.ours(t.Context(), "p", wt, "p/t1") {
		t.Error("a worktree whose record says removed is ours")
	}
	if err := l.sessions.made.Restored("p", wt); err != nil {
		t.Fatal(err)
	}
	if !l.ours(t.Context(), "p", wt, "p/t1") {
		t.Error("a restored worktree is not ours")
	}
}

func TestOtherThreadsRecordIsNotOurs(t *testing.T) {
	tests := []struct {
		name           string
		record, marker string
		ask            string
		want           bool
	}{
		{"the thread's own", "p/t1", "p/t1", "p/t1", true},
		{"another thread's record and marker", "p/t1", "p/t1", "p/t2", false},
		{"the record names another thread", "p/t2", "p/t1", "p/t1", false},
		{"the marker names another thread", "p/t1", "p/t2", "p/t1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, dir, _ := madeRepo(t)
			wt := madeTree(t, l, dir, "issue-7", tt.record)
			if err := l.mark(t.Context(), wt, tt.marker); err != nil {
				t.Fatal(err)
			}
			if got := l.ours(t.Context(), "p", wt, tt.ask); got != tt.want {
				t.Errorf("ours = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectFolderIsNeverOurs(t *testing.T) {
	l, dir, _ := madeRepo(t)
	if err := l.mark(t.Context(), dir, "p/t1"); err == nil {
		t.Error("marked the project folder")
	}
	if err := l.sessions.made.Add("p", Made{Path: dir, Thread: "p/t1"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", markerFile), []byte("p/t1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if l.ours(t.Context(), "p", dir, "p/t1") {
		t.Error("the project folder is ours")
	}
	if _, err := os.Stat(filepath.Join(dir, "README")); err != nil {
		t.Errorf("the project folder is not there: %v", err)
	}
}
