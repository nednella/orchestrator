package orchestration_test

import (
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/threads"
)

// sideChatOf starts a side chat of the thread, which withReply gave a reply.
func sideChatOf(t *testing.T, h *apptest.Harness, parent string) threads.Thread {
	t.Helper()
	var side threads.Thread
	// The Stop hook reaches the app a moment after it runs; until then the thread has no conversation to fork.
	apptest.Eventually(t, "the side chat", func() bool { s, err := h.SideChatStart(parent, ""); side = s; return err == nil })
	return side
}

// A side chat is the owner's alone, as in the thread list: no tool lists it, reads it, messages it or waits on it,
// not even for the thread it was started from.
func TestSideChatsAreNoThreadsToTheTools(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "work")
	withReply(t, h, parent.ID)
	other := newThread(t, h, "other")
	side := sideChatOf(t, h, parent.ID)

	for _, caller := range []string{parent.ID, other.ID} {
		if _, listed := listThreads(t, h, caller, `{"project":"all"}`)[side.ID]; listed {
			t.Errorf("the threads tool lists the side chat to %s", caller)
		}
		want := "No thread " + side.ID
		for tool, input := range map[string]string{"read": `{"thread":"` + side.ID + `"}`, "send": `{"thread":"` + side.ID + `","text":"hello"}`} {
			if err := useTool(t, h, caller, tool, input, nil); err != want {
				t.Errorf("%s of the side chat by %s: %q, want %q", tool, caller, err, want)
			}
		}
		if _, refused, err := waitCall(h, tokenOf(t, h, caller), side.ID, time.Second); err != nil || refused != want {
			t.Errorf("wait on the side chat by %s: %q (%v), want %q", caller, refused, err, want)
		}
		if err := useTool(t, h, caller, "stop", `{"thread":"`+side.ID+`"}`, nil); err == "" {
			t.Errorf("%s stopped the side chat", caller)
		}
	}
}

// A side chat is a question on the side: it hands no work to a child, which would outlive it.
func TestSideChatCannotDelegate(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "work")
	withReply(t, h, parent.ID)
	side := sideChatOf(t, h, parent.ID)
	if err := useTool(t, h, side.ID, "delegate", `{"task":"count the files"}`, nil); err != "A side chat cannot delegate" {
		t.Errorf("delegate from a side chat: %q", err)
	}
	if children := h.Threads().ChildrenOf(side.ID); len(children) != 0 {
		t.Errorf("the side chat has children %q", children)
	}
}
