package threads_test

import (
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/thread"
)

func TestWorkingSinceIsTheStartOfTheStretchAndSurvivesEventsAndARestart(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("work", "")
	if err != nil {
		t.Fatal(err)
	}
	since := func() int64 { got, _ := h.Thread(s.ID); return got.WorkingSince }

	h.Observe(t, s.ID, "SessionStart", `{}`)
	if got := since(); got != 0 {
		t.Fatalf("workingSince = %d before any work, want 0", got)
	}
	before := time.Now().UnixMilli()
	h.Observe(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
	eventually(t, "working", func() bool { return stateOf(h, s.ID) == "working" })
	began := since()
	if began < before || began > time.Now().UnixMilli() {
		t.Fatalf("workingSince = %d, want the time the stretch began, not before %d", began, before)
	}

	time.Sleep(20 * time.Millisecond)
	h.Observe(t, s.ID, "PostToolUse", `{"tool_name":"Read","tool_input":{}}`)
	if got := since(); got != began {
		t.Errorf("workingSince = %d after an event in the stretch, want %d", got, began)
	}

	second := h.Restart(t)
	eventually(t, "working after a restart", func() bool { return stateOf(second, s.ID) == "working" })
	if got, _ := second.Thread(s.ID); got.WorkingSince != began {
		t.Errorf("workingSince = %d after a restart, want %d", got.WorkingSince, began)
	}

	second.Observe(t, s.ID, "Stop", `{}`)
	eventually(t, "idle", func() bool { return stateOf(second, s.ID) == "idle" })
	if got, _ := second.Thread(s.ID); got.WorkingSince != 0 {
		t.Errorf("workingSince = %d after the stretch, want 0", got.WorkingSince)
	}
}

func TestWorkingSinceOfBackgroundWorkIsTheStartOfTheTurnThatStartedIt(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("beside", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.ChatWatch(s.ID); err != nil {
		t.Fatal(err)
	}

	h.Observe(t, s.ID, "UserPromptSubmit", `{"prompt":"go"}`)
	eventually(t, "working", func() bool { return stateOf(h, s.ID) == "working" })
	began, _ := h.Thread(s.ID)
	time.Sleep(20 * time.Millisecond)
	h.Threads().BackgroundChanged(s.ID, threads.Background{Subagents: 1})
	h.Observe(t, s.ID, "Stop", `{}`)
	eventually(t, "working on in the background", func() bool {
		got, _ := h.Thread(s.ID)
		return got.Subagents == 1 && got.State == thread.Working
	})
	if got, _ := h.Thread(s.ID); got.WorkingSince != began.WorkingSince || began.WorkingSince == 0 {
		t.Errorf("workingSince = %d while a subagent runs on, want the turn's %d", got.WorkingSince, began.WorkingSince)
	}
}
