package threads

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/folder"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/thread"
)

// startedAt is a threads whose host for the thread listens, answering nobody, and a start whose agent runs and whose
// worktree is placed: the steps a retry skips. The chat log is the host's.
func startedAt(t *testing.T) (*Threads, *starting) {
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
	got, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "aaaaaaaa"}, "feature", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.dir != folder.Canonical(wt) || got.branch != "feature" || !got.made {
		t.Errorf("worktree = %+v, want %s as the thread's own, made by orchestrator", got, wt)
	}
	if n := len(strings.Split(strings.TrimSpace(gitT(t, dir, "worktree", "list")), "\n")); n != 2 {
		t.Errorf("git lists %d worktrees, want 2", n)
	}
	// Another thread's start is not told the worktree is its own to remove.
	other, err := l.makeWorktree(t.Context(), proj, thread.Name{Project: "p", Token: "bbbbbbbb"}, "feature", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if other.made {
		t.Errorf("worktree for another thread = %+v, want it unrecorded", other)
	}
}
