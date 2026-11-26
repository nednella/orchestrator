package chat_test

import (
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
)

func stateOf(h *apptest.Harness, id string) string {
	s, _ := h.Session(id)
	return string(s.State)
}

func TestAThreadsStateFollowsItsHost(t *testing.T) {
	h, id := newThread(t)
	if got := stateOf(h, id); got != "idle" {
		t.Fatalf("a new thread is %s, want idle from its host's start", got)
	}

	send(t, h, id, "tool wait")
	eventually(t, "working at the tool", func() bool { return stateOf(h, id) == "working" })
	h.Proceed(t, id)
	h.Finish(t, id)
	eventually(t, "idle at the turn's end", func() bool { return turnEnds(t, h, id) == 1 && stateOf(h, id) == "idle" })

	send(t, h, id, "ask")
	eventually(t, "waiting at the approval", func() bool { return stateOf(h, id) == "waiting" })
	pending := h.ChatPending()
	if len(pending) != 1 {
		t.Fatalf("pending = %+v, want one", pending)
	}
	if err := h.ChatAnswer(id, pending[0].Request, true, ""); err != nil {
		t.Fatal(err)
	}
	eventually(t, "idle once the answered turn ends", func() bool { return turnEnds(t, h, id) == 2 && stateOf(h, id) == "idle" })

	send(t, h, id, "question")
	eventually(t, "waiting at a reply that ends in a question", func() bool { return turnEnds(t, h, id) == 3 && stateOf(h, id) == "waiting" })
	send(t, h, id, "yes")
	eventually(t, "idle again at the next turn's end", func() bool { return turnEnds(t, h, id) == 4 && stateOf(h, id) == "idle" })

	send(t, h, id, "work")
	eventually(t, "working in a turn that stays open", func() bool { return stateOf(h, id) == "working" })
	if err := h.ChatInterrupt(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "idle after the interrupt, which runs no Stop", func() bool { return turnEnds(t, h, id) == 5 && stateOf(h, id) == "idle" })
	time.Sleep(200 * time.Millisecond)
	if got := stateOf(h, id); got != "idle" {
		t.Errorf("state = %s, want idle", got)
	}
}
