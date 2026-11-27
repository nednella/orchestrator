package threads_test

import (
	"testing"

	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/thread"
)

// Not parallel: it compares two samples of the test process's own memory, which other tests change.
func TestThreadsShowTheMemoryTheirProcessesHold(t *testing.T) {
	h := newHarness(t)
	s, err := h.Threads().Create(h.Threads().Current(), "work", "", false, 0, thread.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	h.Threads().Refresh()
	h.Threads().SampleMemory()
	row, _ := h.Thread(s.ID)
	if row.Memory <= 0 {
		t.Errorf("row memory = %d, want the agent's", row.Memory)
	}
	m, ok := h.Rec.Last("memory").(threads.Memory)
	if !ok {
		t.Fatalf("memory event = %v", h.Rec.Last("memory"))
	}
	if m.Threads < row.Memory || m.App <= 0 || m.Total != m.App+m.Browsers+m.Threads || m.Machine <= m.Total {
		t.Errorf("memory = %+v with a thread holding %d", m, row.Memory)
	}
	if m != h.Threads().Memory() {
		t.Errorf("Memory() = %+v, want the last event %+v", h.Threads().Memory(), m)
	}
	// A sample that finds the figures where they were sends nothing; a loaded machine may move a rounded figure between
	// two samples, and the test then has nothing to say.
	sent, before, held := h.Rec.Count("sessions"), h.Threads().Memory(), row.Memory
	h.Threads().SampleMemory()
	row, _ = h.Thread(s.ID)
	if n := h.Rec.Count("sessions"); n != sent && h.Threads().Memory() == before && row.Memory == held {
		t.Errorf("a sample with the same figures sent the threads %d more times", n-sent)
	}

	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	h.Threads().Refresh()
	h.Threads().SampleMemory()
	if row, _ := h.Thread(s.ID); row.Memory != 0 {
		t.Errorf("ended row memory = %d, want 0", row.Memory)
	}

	next, err := h.Threads().Create(h.Threads().Current(), "more work", "", false, 0, thread.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	h.Threads().Refresh()
	h.Threads().SampleMemory()
	row, _ = h.Thread(next.ID)
	if m := h.Threads().Memory(); row.Memory <= 0 || m.Threads < row.Memory {
		t.Errorf("memory = %+v with a thread of a new tmux server holding %d", m, row.Memory)
	}
}
