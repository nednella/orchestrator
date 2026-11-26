package sessions

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/internal/activity"
	"github.com/nednella/agentos/internal/project"
	"github.com/nednella/agentos/internal/session"
	"github.com/nednella/agentos/internal/term"
)

func prJSON(state string, draft bool, rollup string, comments, reviews int) string {
	repeat := func(n int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(`{"id":"x"},`, n), ",") + "]"
	}
	return fmt.Sprintf(`[{"number":12,"url":"https://github.com/acme/widgets/pull/12","state":%q,"isDraft":%v,"statusCheckRollup":%s,"comments":%s,"reviews":%s,"updatedAt":"2026-10-01T12:00:00Z"}]`,
		state, draft, rollup, repeat(comments), repeat(reviews))
}

func TestParsePR(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		state, check string
		comments     int
	}{
		{"draft", prJSON("OPEN", true, "[]", 0, 0), "draft", "none", 0},
		{"open", prJSON("OPEN", false, "[]", 1, 2), "open", "none", 3},
		{"approval is not a review to address", strings.Replace(prJSON("OPEN", false, "[]", 1, 2), `[{"id":"x"},{"id":"x"}]`, `[{"state":"APPROVED"},{"state":"CHANGES_REQUESTED"}]`, 1), "open", "none", 2},
		{"the gh user's own comments and replies are not news", strings.Replace(strings.Replace(prJSON("OPEN", false, "[]", 1, 2), `[{"id":"x"},{"id":"x"}]`, `[{"author":{"login":"bot"},"state":"CHANGES_REQUESTED"},{"author":{"login":"me"},"state":"COMMENTED"}]`, 1), `[{"id":"x"}]`, `[{"author":{"login":"me"}}]`, 1), "open", "none", 1},
		{"merged", prJSON("MERGED", false, "[]", 0, 0), "merged", "none", 0},
		{"closed", prJSON("CLOSED", false, "[]", 0, 0), "closed", "none", 0},
		{"passing", prJSON("OPEN", false, `[{"status":"COMPLETED","conclusion":"SUCCESS"},{"status":"COMPLETED","conclusion":"SKIPPED"},{"state":"SUCCESS"}]`, 0, 0), "open", "passing", 0},
		{"pending", prJSON("OPEN", false, `[{"status":"COMPLETED","conclusion":"SUCCESS"},{"status":"IN_PROGRESS","conclusion":""}]`, 0, 0), "open", "pending", 0},
		{"failing wins", prJSON("OPEN", false, `[{"status":"IN_PROGRESS"},{"status":"COMPLETED","conclusion":"FAILURE"},{"status":"COMPLETED","conclusion":"SUCCESS"}]`, 0, 0), "open", "failing", 0},
		{"status context error", prJSON("OPEN", false, `[{"state":"ERROR"}]`, 0, 0), "open", "failing", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr, err := parsePR([]byte(tt.data), "me")
			if err != nil || pr == nil {
				t.Fatalf("parsePR = %v, %v", pr, err)
			}
			if pr.State != tt.state || pr.Checks != tt.check || pr.Comments != tt.comments || pr.Number != 12 || pr.UpdatedAt == 0 {
				t.Errorf("pr = %+v", pr)
			}
		})
	}
	if pr, err := parsePR([]byte("[]"), "me"); pr != nil || err != nil {
		t.Errorf("no PR gave %v, %v", pr, err)
	}
	if _, err := parsePR([]byte("nope"), "me"); err == nil {
		t.Error("bad json accepted")
	}
}

func TestParsePRChecksAndTitle(t *testing.T) {
	data := `[{"number":12,"title":"Show the status","url":"u","state":"OPEN","isDraft":false,"statusCheckRollup":[` +
		`{"name":"lint","status":"COMPLETED","conclusion":"FAILURE"},{"context":"ci/test","state":"ERROR"},` +
		`{"name":"build","status":"IN_PROGRESS"},{"name":"vet","status":"COMPLETED","conclusion":"SUCCESS"}],"comments":[],"reviews":[],"updatedAt":"2026-10-01T12:00:00Z"}]`
	pr, err := parsePR([]byte(data), "me")
	if err != nil {
		t.Fatal(err)
	}
	if pr.Title != "Show the status" || pr.Checks != "failing" || !slices.Equal(pr.FailedChecks, []string{"lint", "ci/test"}) || pr.RunningChecks != 1 || pr.PassedChecks != 1 {
		t.Errorf("pr = %+v", pr)
	}
	if pr, _ := parsePR([]byte(prJSON("OPEN", false, "[]", 0, 0)), "me"); pr.FailedChecks == nil {
		t.Error("no failing checks must be an empty list, not null")
	}
}

func TestVerdict(t *testing.T) {
	by := func(login, state string) review {
		r := review{State: state}
		r.Author.Login = login
		return r
	}
	tests := []struct {
		name    string
		reviews []review
		want    string
	}{
		{"none", nil, ""},
		{"a comment", []review{by("a", "COMMENTED")}, "commented"},
		{"an approval", []review{by("a", "APPROVED")}, "approved"},
		{"a comment after an approval keeps it", []review{by("a", "APPROVED"), by("a", "COMMENTED")}, "approved"},
		{"changes by one reviewer win over another's approval", []review{by("a", "CHANGES_REQUESTED"), by("b", "APPROVED")}, "changes"},
		{"a reviewer's later approval replaces their changes", []review{by("a", "CHANGES_REQUESTED"), by("a", "APPROVED")}, "approved"},
		{"a dismissal undoes the approval", []review{by("a", "APPROVED"), by("a", "DISMISSED")}, ""},
		{"a pending review is not one", []review{by("a", "PENDING")}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := verdict(tt.reviews); got != tt.want {
				t.Errorf("verdict = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseWorktrees(t *testing.T) {
	w := parseWorktrees("worktree /p\nHEAD abc\nbranch refs/heads/main\n\nworktree /p/trees/issue-7\nHEAD def\nbranch refs/heads/issue-7\n\nworktree /p/detached\nHEAD 123\ndetached\n")
	if w.main != "/p" || w.byBranch["issue-7"] != "/p/trees/issue-7" || w.byBranch["main"] != "/p" || len(w.byBranch) != 2 || !slices.Equal(w.detached, []string{"/p/detached"}) {
		t.Errorf("worktrees = %+v", w)
	}
}

type noActivity struct{}

func (noActivity) Record(string, activity.Event)              {}
func (noActivity) Since(string, string, func(activity.Event)) {}

func TestPollResultForAForgottenSessionIsDropped(t *testing.T) {
	l := newLifecycle(nil, nil, t.TempDir(), t.TempDir(), nil, noActivity{}, func(string, any) {})
	proj := project.Project{Name: "p"}
	for _, tt := range []struct {
		name   string
		forget bool
		want   bool
	}{{"forgotten", true, false}, {"kept", false, true}} {
		t.Run(tt.name, func(t *testing.T) {
			id := "p/" + tt.name
			if tt.forget {
				l.forget(id)
			}
			o := l.update(target{id: id, branch: "issue-7", proj: proj}, "", &PR{Number: 7, State: "open"})
			_, tracked := l.tracks[id]
			if tracked != tt.want || (!tt.want && o != (outcome{})) {
				t.Errorf("tracked %v, outcome %+v, want tracked %v", tracked, o, tt.want)
			}
		})
	}
}

func TestThreadGetsItsPRsCommandsAndOpenedAlert(t *testing.T) {
	l := newLifecycle(nil, nil, t.TempDir(), t.TempDir(), nil, noActivity{}, func(string, any) {})
	proj := project.Project{Name: "p", OnReview: "/address-review {n}"}
	tg := target{id: "p/a", proj: proj, branch: "x"}
	l.update(tg, "", &PR{Number: 7, State: "open"})
	if !l.opened(tg.id) {
		t.Error("the thread did not hear of its new PR")
	}
	if o := l.update(tg, "", &PR{Number: 7, State: "open", Comments: 1}); o.wake != "/address-review 7" {
		t.Errorf("the thread got %q, want the review command", o.wake)
	}
}

func TestReviewAfterACommentIsDeletedWakesTheSession(t *testing.T) {
	l := newLifecycle(nil, nil, t.TempDir(), t.TempDir(), nil, noActivity{}, func(string, any) {})
	proj := project.Project{Name: "p", OnReview: "/address-review {n}"}
	tg := target{id: "p/a", proj: proj, branch: "x"}
	for i, tt := range []struct {
		comments int
		want     string
	}{{3, ""}, {2, ""}, {3, "/address-review 7"}, {3, ""}} {
		if o := l.update(tg, "", &PR{Number: 7, State: "open", Comments: tt.comments}); o.wake != tt.want {
			t.Errorf("fetch %d with %d comments woke with %q, want %q", i, tt.comments, o.wake, tt.want)
		}
	}
}

func TestRemoveWorktreeRemovesOnlyTheThreadsOwn(t *testing.T) {
	recordProjectFolder := func(t *testing.T, l *Lifecycle, dir string) {
		t.Helper()
		if err := l.sessions.made.Add("p", Made{Path: dir, Branch: "issue-7", Thread: "p/t1"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".git", markerFile), []byte("p/t1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name   string
		place  func(t *testing.T, l *Lifecycle, dir string) (projDir, path string)
		reason string
	}{
		{"the thread's own", func(t *testing.T, l *Lifecycle, dir string) (string, string) {
			return dir, madeTree(t, l, dir, "issue-7", "p/t1")
		}, ""},
		{"the project folder", func(t *testing.T, l *Lifecycle, dir string) (string, string) {
			recordProjectFolder(t, l, dir)
			return dir, dir
		}, "holds the project's own checkout, which orchestrator never removes"},
		{"the repo's checkout around a project in a folder of it", func(t *testing.T, l *Lifecycle, dir string) (string, string) {
			recordProjectFolder(t, l, dir)
			sub := filepath.Join(dir, "sub")
			if err := os.Mkdir(sub, 0o700); err != nil {
				t.Fatal(err)
			}
			return sub, dir
		}, "holds the project's own checkout, which orchestrator never removes"},
		{"a human's worktree at a path orchestrator removed", func(t *testing.T, l *Lifecycle, dir string) (string, string) {
			wt := madeTree(t, l, dir, "issue-7", "p/t1")
			gitT(t, dir, "worktree", "remove", wt)
			if err := l.sessions.made.Removed("p", wt, time.Now()); err != nil {
				t.Fatal(err)
			}
			gitT(t, dir, "worktree", "add", wt, "issue-7")
			return dir, wt
		}, "is not the worktree orchestrator made for this thread"},
		{"another thread's", func(t *testing.T, l *Lifecycle, dir string) (string, string) {
			return dir, madeTree(t, l, dir, "issue-7", "p/t2")
		}, "is not the worktree orchestrator made for this thread"},
		{"the thread's own, once the thread woke", func(t *testing.T, l *Lifecycle, dir string) (string, string) {
			l.sessions.info = append(l.sessions.info, term.Info{Name: session.Name{Project: "p", Token: "t1"}})
			return dir, madeTree(t, l, dir, "issue-7", "p/t1")
		}, "woke during the clean-up"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, dir, _ := madeRepo(t)
			l.sessions.tmux, l.sessions.closeTerm = testTmux(t), func(string) {}
			projDir, path := tt.place(t, l, dir)
			shell := session.Name{Project: "p", Shell: 1}
			if err := l.sessions.tmux.NewSession(t.Context(), shell, "shell", path, nil, []string{"sleep", "60"}, 80, 24); err != nil {
				t.Fatal(err)
			}
			if err := l.sessions.tmux.SetThread(t.Context(), shell, "p/t1"); err != nil {
				t.Fatal(err)
			}
			ran := filepath.Join(t.TempDir(), "ran")
			proj := project.Project{Name: "p", Dir: projDir, CleanupCommand: fmt.Sprintf("echo {worktree} >> %q && git worktree remove --force {worktree}", ran)}
			plan := cleanupPlan{branchPlan: branchPlan{branch: "issue-7", worktree: path, made: true}, force: true}
			removed, reason := l.removeWorktree(t.Context(), target{id: "p/t1", branch: "issue-7", proj: proj}, &plan)
			if tt.reason == "" {
				if reason != "" || !slices.Equal(removed, []string{"worktree " + path}) || !gone(path) {
					t.Errorf("removed %q, reason %q, folder gone %v", removed, reason, gone(path))
				}
				if l.sessions.tmux.Has(t.Context(), shell) {
					t.Error("the thread's shell still runs")
				}
				return
			}
			if !strings.Contains(reason, tt.reason) || len(removed) != 0 {
				t.Errorf("removed %q, reason %q, want %q", removed, reason, tt.reason)
			}
			if !l.sessions.tmux.Has(t.Context(), shell) {
				t.Error("a refused removal closed the thread's shell")
			}
			if _, err := os.Stat(ran); err == nil {
				t.Error("the clean-up command ran")
			}
			if _, err := os.Stat(filepath.Join(path, "README")); err != nil {
				t.Errorf("the folder is not there: %v", err)
			}
		})
	}
}

// testTmux is a tmux server on a private socket, stopped when the test ends.
func testTmux(t *testing.T) *term.Tmux {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("aostest-life-%d-%d", os.Getpid(), rand.Uint32())
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", socket, "kill-server").Run() })
	tmux, err := term.NewTmux(socket, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return tmux
}

// openWindows is a browser with a window open for every thread; closed lists the windows it closed.
type openWindows struct {
	fakeBrowsers
	closed *[]string
}

func (w openWindows) Has(string) bool                    { return true }
func (w openWindows) Close(_ context.Context, id string) { *w.closed = append(*w.closed, id) }

func TestThreadThatWakesDuringTheCleanupKeepsWhatIsLeft(t *testing.T) {
	for _, tt := range []struct {
		name    string
		removed bool
	}{{"no worktree orchestrator made", false}, {"a worktree orchestrator removed", true}} {
		t.Run(tt.name, func(t *testing.T) {
			l, dir, _ := madeRepo(t)
			name := session.Name{Project: "p", Token: "abcdefgh"}
			id := name.String()
			if tt.removed {
				wt := madeTree(t, l, dir, "issue-7", id)
				gitT(t, dir, "worktree", "remove", wt)
				if err := l.sessions.made.Removed("p", wt, time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			var closed []string
			l.sessions.browsers = openWindows{closed: &closed}
			tmp := l.sessions.tmpDir(name)
			if err := os.MkdirAll(tmp, 0o700); err != nil {
				t.Fatal(err)
			}
			l.sessions.nums[id] = 3
			// The clean-up read the thread as settled; it has woken since.
			l.sessions.info = append(l.sessions.info, term.Info{Name: name, Branch: "issue-7", Path: dir})

			removed, reason := l.removeAll(t.Context(), target{id: id, branch: "issue-7", proj: project.Project{Name: "p", Dir: dir}, ended: true}, nil, false)
			if reason != "Thread 3 woke during the clean-up" || len(removed) != 0 {
				t.Errorf("removed %q, reason %q", removed, reason)
			}
			if _, err := os.Stat(tmp); err != nil || len(closed) != 0 {
				t.Errorf("temp folder: %v; windows closed %q", err, closed)
			}
		})
	}
}

type noEvidence struct{}

func (noEvidence) Count(string) int { return 0 }
func (noEvidence) Purge(string) int { return 0 }

// asleepAndKept is an app with one asleep thread whose clean-up the checks kept after its PR merged. The thread has no
// worktree orchestrator made, so a clean-up that goes on runs no git, and every command fails.
func asleepAndKept(t *testing.T) (*Sessions, session.Name) {
	t.Helper()
	dir := t.TempDir()
	proj := project.Project{Name: "p", Dir: dir}
	noCommands := func(context.Context, string, string, ...string) ([]byte, error) {
		return nil, errors.New("this test runs no commands")
	}
	s := New(Options{LocalDir: t.TempDir(), StateDir: t.TempDir(), Run: noCommands, Projects: fixedProjects{proj}, Current: proj, Activity: noActivity{}, Browsers: fakeBrowsers{}, Evidence: noEvidence{}, Emit: func(string, any) {}})
	name := session.Name{Project: "p", Token: "abcdefgh"}
	id := name.String()
	s.nums[id] = 3
	s.ended[id] = &endedSession{info: term.Info{Name: name, Branch: "issue-7", Path: dir}}
	s.life.tracks[id] = &track{branch: "issue-7", cleanup: "blocked", reason: "a shell is open in this worktree", pr: &PR{Number: 12, State: "merged"}}
	return s, name
}

func TestSweptCleanupStopsForADeleteOrAWake(t *testing.T) {
	for _, tt := range []struct {
		name string
		mark func(s *Sessions, id string)
		err  string
	}{
		{"nothing else runs", func(*Sessions, string) {}, ""},
		{"a delete runs", func(s *Sessions, id string) { s.deleting[id] = true }, "Thread 3 is being deleted"},
		{"a wake runs", func(s *Sessions, id string) { s.waking[id] = true }, "Thread 3 is waking; clean it up once it runs"},
		{"the thread runs again", func(s *Sessions, id string) {
			s.info = append(s.info, s.ended[id].info) // a wake that ended after the sweep read the thread as asleep
		}, "Thread 3 runs again"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, name := asleepAndKept(t)
			id := name.String()
			tt.mark(s, id)
			_, _, err := s.life.cleanupNow(t.Context(), id, false, bySweep)
			if tt.err == "" {
				if err != nil || s.life.fields(id).cleanup != "" || len(s.life.Cleanups("p")) != 1 {
					t.Errorf("err %v, cleanup %q, log %+v; want the clean-up done", err, s.life.fields(id).cleanup, s.life.Cleanups("p"))
				}
				return
			}
			f := s.life.fields(id)
			if err == nil || err.Error() != tt.err || f.cleanup != "blocked" || f.reason != "a shell is open in this worktree" || len(s.life.Cleanups("p")) != 0 {
				t.Errorf("err %v, cleanup %q (%q), log %+v; want %q and the kept clean-up as it was", err, f.cleanup, f.reason, s.life.Cleanups("p"), tt.err)
			}
		})
	}
}

func TestAChildsSleepCleanupAndADeleteNeverBothGoOn(t *testing.T) {
	t.Run("the clean-up decides first", func(t *testing.T) {
		s, name := asleepAndKept(t)
		id := name.String()
		// The clean-up holds createMu from its look for a delete until its mark is on.
		s.createMu.Lock()
		done := make(chan error, 1)
		go func() { _, err := s.startDelete(id); done <- err }()
		time.Sleep(100 * time.Millisecond)
		s.mu.Lock()
		marked := s.deleting[id]
		s.mu.Unlock()
		if marked {
			t.Error("the delete marked itself while the clean-up decided")
		}
		s.life.mu.Lock()
		s.life.tracks[id].cleanup = "pending"
		s.life.mu.Unlock()
		s.createMu.Unlock()
		if err := <-done; err == nil || err.Error() != "Thread 3 is being cleaned up; delete it when that ends" {
			t.Errorf("delete = %v, want it refused for the clean-up", err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.deleting[id] {
			t.Error("the refused delete left its mark")
		}
	})

	t.Run("the delete decides first", func(t *testing.T) {
		s, name := asleepAndKept(t)
		id := name.String()
		family, err := s.startDelete(id)
		if err != nil {
			t.Fatal(err)
		}
		defer s.endDelete(family)
		if _, _, ok := s.life.startChildCleanup(id); ok {
			t.Error("the clean-up went on beside the delete")
		}
	})
}

func TestNewShellIsRefusedWhileItsThreadIsDeleted(t *testing.T) {
	s, name := asleepAndKept(t)
	s.deleting[name.String()] = true
	if _, err := s.NewShell(name.String()); err == nil || err.Error() != "Thread 3 is being deleted; open a shell when that ends" {
		t.Errorf("shell = %v, want it refused while the delete runs, which closes the thread's shells", err)
	}
}

func TestWakeIsRefusedWhileACleanupRuns(t *testing.T) {
	s, name := asleepAndKept(t)
	id := name.String()
	s.life.tracks[id].cleanup = "pending"
	if err := s.startAgain(name, ""); err == nil || err.Error() != "Thread 3 is being cleaned up; wake it when that ends" {
		t.Errorf("wake = %v, want it refused while the clean-up runs", err)
	}
	if s.waking[id] {
		t.Error("the refused wake left its mark, so no clean-up could start")
	}
}

func TestSweepLeavesAThreadBeingDeleted(t *testing.T) {
	s, name := asleepAndKept(t)
	id := name.String()
	s.deleting[id] = true
	s.life.sweep(t.Context())
	s.life.cleanups.Wait()
	if f := s.life.fields(id); f.cleanup != "blocked" || len(s.life.Cleanups("p")) != 0 {
		t.Errorf("cleanup %q, log %+v; want the kept clean-up left as it was", f.cleanup, s.life.Cleanups("p"))
	}
}

func TestNoSweepRunsWhenItIsOff(t *testing.T) {
	s, name := asleepAndKept(t)
	s.SetCleanupSweep(0)
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	s.life.Run(ctx)
	s.life.cleanups.Wait()
	if f := s.life.fields(name.String()); f.cleanup != "blocked" || len(s.life.Cleanups("p")) != 0 {
		t.Errorf("cleanup %q, log %+v; want the kept clean-up left as it was", f.cleanup, s.life.Cleanups("p"))
	}
}
