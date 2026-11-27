package threads

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/folder"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/term"
	"github.com/nednella/orchestrator/internal/thread"
)

// startedAt is a threads whose host for the thread listens, answering nobody, and a start whose agent runs and whose
// worktree is placed: the steps a retry skips. The chat log is the host's.
func startedAt(t *testing.T, listen ...bool) (*Threads, *starting) {
	t.Helper()
	dir, err := os.MkdirTemp("", "orc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	s := newThreads(Options{StateDir: dir, LocalDir: dir, Projects: noProjects{}, Emit: func(string, any) {}})
	name := thread.Name{Project: "p", Token: "aaaaaaaa"}
	socket := s.hostSocket(name)
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		t.Fatal(err)
	}
	if len(listen) == 0 || listen[0] {
		ln, err := net.Listen("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}()
	}
	if err := os.MkdirAll(filepath.Dir(s.chatLogPath(name)), 0o700); err != nil {
		t.Fatal(err)
	}
	return s, &starting{name: name, app: t.Context(), o: StartOptions{Later: true}, placed: true, launched: true, restore: func() {}}
}

func logChat(t *testing.T, s *Threads, name thread.Name, events ...chat.Event) {
	t.Helper()
	log, err := chat.OpenLog(s.chatLogPath(name))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	for _, e := range events {
		if _, err := log.Append(e); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRetryDoesNotSendTheFirstMessageTheHostTook(t *testing.T) {
	t.Parallel()
	s, st := startedAt(t)
	st.text, st.send, st.sending, st.sentBefore = "fix the login", true, sendingFirst, 0
	// The host logged the message and its reply was lost.
	logChat(t, s, st.name, chat.Event{Kind: chat.KindUser, ID: "u1", Text: "fix the login"})
	if err := s.execute(t.Context(), st); err != nil {
		t.Fatalf("the retry = %v, want it to find the message with the host and send nothing", err)
	}
	if !st.delivered || st.sending != "" {
		t.Errorf("delivered = %v, sending = %q, want the message marked delivered", st.delivered, st.sending)
	}
}

func TestRetryDoesNotSendAHeldMessageTheHostTook(t *testing.T) {
	t.Parallel()
	s, st := startedAt(t)
	st.delivered = true
	st.queue = []host.Message{{Text: "and the signup form"}}
	st.sending, st.sentBefore = sendingHeld, 0
	logChat(t, s, st.name, chat.Event{Kind: chat.KindQueued, ID: "q1", Text: "and the signup form"})
	if err := s.deliverQueued(t.Context(), st); err != nil {
		t.Fatalf("the retry = %v, want the held message found with the host", err)
	}
	if len(st.queue) != 0 {
		t.Errorf("the queue = %+v, want it empty", st.queue)
	}
}

func TestHostTookNeedsANewMessageSinceTheTry(t *testing.T) {
	t.Parallel()
	s, st := startedAt(t)
	m := host.Message{Text: "ok"}
	// An earlier message with the same words, and its echo as the user's, are not this try's.
	logChat(t, s, st.name, chat.Event{Kind: chat.KindQueued, ID: "q1", Text: "ok"}, chat.Event{Kind: chat.KindUser, ID: "q1", Text: "ok"})
	st.sending, st.sentBefore = sendingHeld, s.hostHolds(st.name, m)
	if st.sentBefore != 1 {
		t.Fatalf("the log holds %d such messages, want 1: a message and its echo are one", st.sentBefore)
	}
	if s.hostTook(st, sendingHeld, m) {
		t.Error("an earlier message with the same words was taken for this try's")
	}
	logChat(t, s, st.name, chat.Event{Kind: chat.KindQueued, ID: "q2", Text: "ok"})
	if !s.hostTook(st, sendingHeld, m) {
		t.Error("the message the try sent was not found")
	}
	if st.sending = ""; s.hostTook(st, sendingHeld, m) {
		t.Error("a message was found for a try that sent nothing")
	}
}

func TestMakeWorktreeFindsTheOneAnEarlierTryOfTheSameThreadMade(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	wt := madeTree(t, l, dir, "feature", "p/aaaaaaaa")
	proj := project.Project{Name: "p", Dir: dir}
	got, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "aaaaaaaa"}, "feature", "", true, worktreeTry{})
	if err != nil {
		t.Fatal(err)
	}
	if got.dir != folder.Canonical(wt) || got.branch != "feature" || !got.made {
		t.Errorf("worktree = %+v, want %s as the thread's own, made by orchestrator", got, wt)
	}
	if n := len(strings.Split(strings.TrimSpace(gitT(t, dir, "worktree", "list")), "\n")); n != 2 {
		t.Errorf("git lists %d worktrees, want 2", n)
	}
	// Another thread's start of an issue's branch is not told the worktree is its own to remove.
	other, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "bbbbbbbb"}, "feature", "", false, worktreeTry{})
	if err != nil {
		t.Fatal(err)
	}
	if other.made {
		t.Errorf("worktree for another thread = %+v, want it unrecorded", other)
	}
}

func TestMakeWorktreeTakesForItsOwnOnlyTheOneItBeganToMake(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	proj := project.Project{Name: "p", Dir: dir}
	name := thread.Name{Project: "p", Token: "aaaaaaaa"}
	wt := folder.Canonical(filepath.Join(dir, "trees", "feature"))
	gitT(t, dir, "worktree", "add", "-b", "feature", filepath.Join(dir, "trees", "feature"))

	// With no word of a start that began to make it, a worktree on the branch is someone else's: the owner's, say.
	got, err := l.makeWorktree(t.Context(), proj, name, "feature", "", true, worktreeTry{})
	if err != nil || got.made {
		t.Fatalf("worktree = %+v, %v; want it left unrecorded: no start began to make it", got, err)
	}
	// A start that began to make it at another folder did not make this one.
	got, err = l.makeWorktree(t.Context(), proj, name, "feature", "", true, worktreeTry{prior: filepath.Join(dir, "trees", "other")})
	if err != nil || got.made {
		t.Fatalf("worktree = %+v, %v; want it left unrecorded: the start began elsewhere", got, err)
	}
	// Nor one that carries another thread's mark.
	if err := l.mark(t.Context(), wt, "p/bbbbbbbb"); err != nil {
		t.Fatal(err)
	}
	got, err = l.makeWorktree(t.Context(), proj, name, "feature", "", true, worktreeTry{prior: wt})
	if err != nil || got.made {
		t.Fatalf("worktree = %+v, %v; want another thread's worktree left alone", got, err)
	}
	if _, ok := l.threads.made.Of("p", name.String()); ok {
		t.Error("a record was made for a worktree of another thread")
	}
}

func TestMakeWorktreeTakesTheOneAQuitLeftUnrecordedThatTheStartBeganToMake(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	proj := project.Project{Name: "p", Dir: dir}
	name := thread.Name{Project: "p", Token: "aaaaaaaa"}
	gitT(t, dir, "worktree", "add", "-b", "feature", filepath.Join(dir, "trees", "feature"))
	wt := folder.Canonical(filepath.Join(dir, "trees", "feature"))
	got, err := l.makeWorktree(t.Context(), proj, name, "feature", "", true, worktreeTry{prior: wt})
	if err != nil {
		t.Fatal(err)
	}
	if got.dir != wt || !got.made {
		t.Fatalf("worktree = %+v, want %s taken as the thread's own", got, wt)
	}
	if w, ok := l.threads.made.Of("p", name.String()); !ok || w.Path != wt || w.Branch != "feature" || w.BaseOid == "" || !l.ours(t.Context(), "p", got.dir, name.String()) {
		t.Errorf("record = %+v, %v; want it recorded and marked for the thread", w, ok)
	}
}

func TestMakeWorktreeTellsGitsBeginningBeforeItBegins(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	proj := project.Project{Name: "p", Dir: dir}
	var began string
	try := worktreeTry{begin: func(d string) {
		began = d
		if exists(d) {
			t.Errorf("git made %s before the start said it would", d)
		}
	}}
	got, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "aaaaaaaa"}, "feature", "", true, try)
	if err != nil || began != got.dir {
		t.Errorf("worktree = %+v, %v; began = %q, want the folder it made", got, err, began)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestDropStartWorktreeRemovesOnlyWhatTheStartMade(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	proj := project.Project{Name: "p", Dir: dir}
	// A worktree no record names for the start, with no word that it began to make it, is not the start's, and stays with its ignored files.
	other := filepath.Join(dir, "trees", "owners")
	gitT(t, dir, "worktree", "add", "-b", "owners", other)
	if err := os.WriteFile(filepath.Join(other, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if reason := l.dropStartWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "bbbbbbbb"}, "work", "owners", ""); reason != "" || !exists(filepath.Join(other, ".env")) {
		t.Errorf("drop = %q; want the owner's worktree untouched", reason)
	}
	// A record that names another thread's worktree, or the project's own checkout, is refused.
	taken := madeTree(t, l, dir, "taken", "p/cccccccc")
	if err := l.threads.made.Add("p", Made{Path: taken, Branch: "taken", Thread: "p/dddddddd", Created: 1}); err != nil {
		t.Fatal(err)
	}
	if reason := l.dropStartWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "dddddddd"}, "work", "taken", ""); reason == "" || !exists(taken) {
		t.Errorf("drop = %q, taken there: %v; want a refusal that keeps another thread's worktree", reason, exists(taken))
	}
	if err := l.threads.made.Add("p", Made{Path: dir, Branch: "main", Thread: "p/eeeeeeee", Created: 1}); err != nil {
		t.Fatal(err)
	}
	if reason := l.dropStartWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "eeeeeeee"}, "work", "main", ""); reason == "" || !exists(filepath.Join(dir, ".git")) {
		t.Errorf("drop = %q; want a refusal that keeps the project's checkout", reason)
	}
}

func TestSetupCommandRunsAgainOnARetryUnlessItEndedWithZeroOrStillRuns(t *testing.T) {
	t.Parallel()
	s, st := startedAt(t)
	id := st.name.String()
	exit := filepath.Join(s.tmpDir(st.name), "setup-exit")
	if err := os.MkdirAll(filepath.Dir(exit), 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		exit  string // what the exit file holds, "" for no file
		shell bool   // a terminal tagged with the thread is open
		run   bool   // the app holds a running set-up of the thread
		want  bool
	}{
		{name: "never ran", want: false},
		{name: "ended with 0", exit: "0", want: true},
		{name: "ended with 0, its terminal gone", exit: "0", want: true},
		{name: "failed", exit: "2", shell: true, want: false},
		{name: "cut short: no exit, no terminal", want: false},
		{name: "still runs in its terminal", shell: true, want: true},
		{name: "still runs, terminal not listed yet", run: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Remove(exit)
			if tt.exit != "" {
				if err := os.WriteFile(exit, []byte(tt.exit), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			s.mu.Lock()
			s.shells, s.setups = nil, map[string]*setupRun{}
			if tt.shell {
				s.shells = []term.Shell{{Name: thread.Name{Project: "p", Token: "shell"}, Setup: id}}
			}
			if tt.run {
				s.setups[id] = &setupRun{status: SetupRunning, cancel: func() {}}
			}
			s.mu.Unlock()
			if got := s.setupDone(id, exit); got != tt.want {
				t.Errorf("setupDone = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStopBeforeGitAnswersNeverRunsTheStartInTheProjectFolder(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	gitT(t, dir, "branch", "fix-login")
	proj := project.Project{Name: "p", Dir: dir}
	stopped, stop := context.WithCancel(t.Context())
	stop()
	got, err := l.makeWorktree(stopped, proj, thread.Name{Project: "p", Token: "aaaaaaaa"}, "fix-login-2", "", true, worktreeTry{})
	if !errors.Is(err, context.Canceled) || got.dir != "" {
		t.Errorf("makeWorktree = %+v, %v; want the stop as the error and no folder", got, err)
	}
	if branch, err := l.freeBranch(stopped, dir, "fix-login"); !errors.Is(err, context.Canceled) || branch != "" {
		t.Errorf("freeBranch = %q, %v; want the stop as the error, not the name of a branch that exists", branch, err)
	}
	if branch, err := l.freeBranch(t.Context(), dir, "fix-login"); err != nil || branch != "fix-login-2" {
		t.Errorf("freeBranch = %q, %v; want fix-login-2", branch, err)
	}
}

func TestOnlyAFolderThatIsNoRepositoryRunsTheStartInItself(t *testing.T) {
	t.Parallel()
	l, _, _ := madeRepo(t)
	plain := t.TempDir()
	proj := project.Project{Name: "p", Dir: plain}
	got, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "aaaaaaaa"}, "fix-login", "", true, worktreeTry{})
	if err != nil || got.dir != plain || got.made {
		t.Errorf("makeWorktree = %+v, %v; want the project folder, as no repository", got, err)
	}
	if branch, err := l.freeBranch(t.Context(), plain, "fix-login"); err != nil || branch != "fix-login" {
		t.Errorf("freeBranch = %q, %v; want the name", branch, err)
	}
	// A repository git cannot read is no reason to run in its folder.
	broken := t.TempDir()
	if err := os.Mkdir(filepath.Join(broken, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err = l.makeWorktree(t.Context(), project.Project{Name: "p", Dir: broken}, thread.Name{Project: "p", Token: "aaaaaaaa"}, "fix-login", "", true, worktreeTry{})
	if err == nil || got.dir != "" {
		t.Errorf("makeWorktree in a broken repository = %+v, %v; want an error", got, err)
	}
}

func TestOriginThatDoesNotAnswerInTimeFailsTheLookupNeitherFindingNorMissingTheBranch(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	gitT(t, dir, "remote", "add", "origin", t.TempDir())
	real := l.run
	l.run = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && slices.Contains(args, "ls-remote") {
			<-ctx.Done()
			return nil, fmt.Errorf("git ls-remote: %w", ctx.Err())
		}
		return real(ctx, dir, name, args...)
	}
	l.originWait = 50 * time.Millisecond
	found, unreachable, err := l.onOrigin(t.Context(), dir, "issue-7")
	if err == nil || found || unreachable {
		t.Errorf("onOrigin = %v, %v, %v; want an error: a timeout is no answer, and no unreachable origin", found, unreachable, err)
	}
	stopped, stop := context.WithCancel(t.Context())
	stop()
	if _, _, err := l.onOrigin(stopped, dir, "issue-7"); !errors.Is(err, context.Canceled) {
		t.Errorf("onOrigin after a stop = %v, want the stop", err)
	}
}

// answeringHost makes the host of the thread at socket answer each message with ok, and counts them.
func answeringHost(t *testing.T, socket string) *atomic.Int64 {
	t.Helper()
	var got atomic.Int64
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				if line, err := bufio.NewReader(conn).ReadBytes('\n'); err == nil && strings.Contains(string(line), `"op":"send"`) {
					got.Add(1)
					_, _ = conn.Write([]byte(`{"ok":true,"id":"m"}` + "\n"))
				}
			}()
		}
	}()
	return &got
}

func TestAMessageHeldAsTheStartEndsIsNeverLost(t *testing.T) {
	t.Parallel()
	for i := range 60 {
		s, st := startedAt(t, false)
		got := answeringHost(t, s.hostSocket(st.name))
		st.delivered = true
		s.mu.Lock()
		s.starting[st.name.String()] = st
		st.status, st.running, st.done = Starting, true, make(chan struct{})
		s.mu.Unlock()
		go s.runStart(t.Context(), st)
		// A bounded run of messages, so the start can end; the one that comes as it does is the one that was lost.
		var held int64
		for range 40 {
			if !s.QueueDuringStart(st.name.String(), host.Message{Text: "more"}) {
				break
			}
			held++
			time.Sleep(time.Duration(i%7) * 50 * time.Microsecond)
		}
		<-st.done
		if got.Load() != held {
			t.Fatalf("round %d: %d messages were held and %d reached the host", i, held, got.Load())
		}
	}
}

func TestRetryMakesAgainAWorktreeGitNeverFinishedAndLeavesAnyOtherUncleanOneAlone(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	proj := project.Project{Name: "p", Dir: dir}
	name := thread.Name{Project: "p", Token: "aaaaaaaa"}

	// A git cut off by a quit or a timeout leaves the worktree locked as initializing, with no checkout.
	half := filepath.Join(dir, "trees", "feature")
	gitT(t, dir, "worktree", "add", "--no-checkout", "-b", "feature", half)
	gitT(t, dir, "worktree", "lock", "--reason", "initializing", half)
	got, err := l.makeWorktree(t.Context(), proj, name, "feature", "", true, worktreeTry{prior: folder.Canonical(half)})
	if err != nil || !got.made {
		t.Fatalf("worktree = %+v, %v; want it made again", got, err)
	}
	if !exists(filepath.Join(got.dir, "README")) || gitT(t, got.dir, "status", "--porcelain") != "" {
		t.Errorf("the worktree has no clean checkout: %q", gitT(t, got.dir, "status", "--porcelain"))
	}

	for name, spoil := range map[string]func(wt string){
		"changed":            func(wt string) { _ = os.WriteFile(filepath.Join(wt, "mine.txt"), []byte("x\n"), 0o600) },
		"locked by an owner": func(wt string) { gitT(t, dir, "worktree", "lock", "--reason", "mine", wt) },
	} {
		wt := filepath.Join(dir, "trees", "other-"+strings.ReplaceAll(name, " ", "-"))
		branch := filepath.Base(wt)
		gitT(t, dir, "worktree", "add", "-b", branch, wt)
		spoil(wt)
		got, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "bbbbbbbb"}, branch, "", true, worktreeTry{prior: folder.Canonical(wt)})
		if err == nil || !strings.Contains(err.Error(), "not whole") || got.made {
			t.Errorf("%s: worktree = %+v, %v; want a refusal that says it is not whole", name, got, err)
		}
		if !exists(wt) {
			t.Errorf("%s: the worktree was removed", name)
		}
		if _, ok := l.threads.made.At("p", wt); ok {
			t.Errorf("%s: the worktree was recorded", name)
		}
	}
}

func TestDropStartWorktreeRemovesAnInterruptedOneAndKeepsAnyOtherUncleanOne(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	proj := project.Project{Name: "p", Dir: dir}
	name := thread.Name{Project: "p", Token: "aaaaaaaa"}
	half := filepath.Join(dir, "trees", "feature")
	gitT(t, dir, "worktree", "add", "--no-checkout", "-b", "feature", half)
	gitT(t, dir, "worktree", "lock", "--reason", "initializing", half)
	if reason := l.dropStartWorktree(t.Context(), proj, name, "work", "feature", folder.Canonical(half)); reason != "" || exists(half) {
		t.Errorf("drop = %q, worktree there: %v; want the unfinished worktree removed", reason, exists(half))
	}
	dirty := filepath.Join(dir, "trees", "dirty")
	gitT(t, dir, "worktree", "add", "-b", "dirty", dirty)
	if err := os.WriteFile(filepath.Join(dirty, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reason := l.dropStartWorktree(t.Context(), proj, name, "work", "dirty", folder.Canonical(dirty))
	if !strings.Contains(reason, "not whole") || !exists(filepath.Join(dirty, ".env")) {
		t.Errorf("drop = %q; want a refusal that keeps the owner's file", reason)
	}
}

func TestAWorktreeGitFailedToMakeIsForgottenSoOneMadeByHandIsNeverTaken(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	proj := project.Project{Name: "p", Dir: dir}
	trees := filepath.Join(dir, "trees")
	if err := os.Mkdir(trees, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(trees, 0o700) })
	var began, cleared int
	try := worktreeTry{begin: func(string) { began++ }, clear: func() { cleared++ }}
	if _, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "aaaaaaaa"}, "feature", "", true, try); err == nil {
		t.Skip("git could make a worktree in a folder that cannot be written to")
	}
	if began != 1 || cleared != 1 {
		t.Errorf("began %d times and cleared %d times, want once each", began, cleared)
	}
}

func TestEachQuestionToOriginHasItsOwnTime(t *testing.T) {
	t.Parallel()
	l, dir, _ := madeRepo(t)
	gitT(t, dir, "remote", "add", "origin", t.TempDir())
	real := l.run
	l.run = func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if name == "git" && (slices.Contains(args, "ls-remote") || slices.Contains(args, "fetch")) {
			select {
			case <-time.After(70 * time.Millisecond):
				return nil, nil
			case <-ctx.Done():
				return nil, fmt.Errorf("git: %w", ctx.Err())
			}
		}
		return real(ctx, dir, name, args...)
	}
	l.originWait = 100 * time.Millisecond
	if _, _, err := l.onOrigin(t.Context(), dir, "issue-7"); err != nil {
		t.Errorf("onOrigin = %v; want each of the two questions to have its own 100 ms, so 70 ms each is in time", err)
	}
}
