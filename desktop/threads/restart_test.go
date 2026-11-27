package threads_test

import (
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/internal/thread"
)

func TestUntaggedTmuxSessionIsIgnored(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	id := "main/aaaaaaaa"
	tmuxRun(t, h, "-f", "/dev/null", "new-session", "-d", "-s", id, "-c", h.Dir, "sleep", "600")
	tmuxRun(t, h, "set-option", "-t", id, "@orchestrator-issue", "12", ";", "set-option", "-t", id, "@orchestrator-name", "#12 work")
	h.Threads().Refresh()
	if row, ok := h.Thread(id); ok {
		t.Errorf("a tmux session with no access tag has the row %+v", row)
	}
	if list := h.Threads().List(); len(list) != 0 {
		t.Errorf("list = %+v, want nothing", list)
	}
	if h.Threads().Has(id) {
		t.Error("Has says the untagged session is a thread")
	}
	s, err := h.StartThread("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if s.N != 1 {
		t.Errorf("the first thread is number %d, want 1: the untagged session took none", s.N)
	}
	if !tmuxHas(h, id) {
		t.Error("the untagged tmux session was stopped")
	}
}

func TestRestartKeepsAThreadOfThisBuild(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Threads().Create(h.Threads().Current(), "work", "", false, 0, thread.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	h.App.Stop()

	second := h.Restart(t)
	eventually(t, "the thread after the restart", func() bool {
		row, ok := second.Thread(src.ID)
		return ok && row.State == "idle" && row.Title == "work" && row.Access == thread.Supervised && row.N == src.N
	})
	if !tmuxHas(second, src.ID) {
		t.Fatal("the restart stopped the thread")
	}

	if err := second.SettleThread(src.ID); err != nil {
		t.Fatalf("sleep after the restart: %v", err)
	}
	eventually(t, "the thread asleep", func() bool { row, _ := second.Thread(src.ID); return row.State == "settled" })
	if _, err := second.WakeThread(src.ID); err != nil {
		t.Fatalf("wake after the restart: %v", err)
	}
	eventually(t, "the thread awake", func() bool { row, _ := second.Thread(src.ID); return row.State != "settled" })

	fork, err := second.ForkThread(src.ID, "r1b")
	if err != nil {
		t.Fatalf("fork after the restart: %v", err)
	}
	if fork.ForkOf != src.ID {
		t.Errorf("fork = %+v, want it forked from %s", fork, src.ID)
	}

	for _, id := range []string{fork.ID, src.ID} {
		if err := second.DeleteThread(id, false); err != nil {
			t.Fatalf("delete of %s after the restart: %v", id, err)
		}
		if _, ok := second.Thread(id); ok || tmuxHas(second, id) {
			t.Errorf("thread %s is still there after its delete", id)
		}
	}
}
