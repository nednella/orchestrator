package sessions_test

import (
	"testing"

	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/session"
)

// Not parallel: it compares two samples of the test process's own memory, which other tests change.
func TestSessionsShowTheMemoryTheirProcessesHold(t *testing.T) {
	h := newHarness(t)
	s, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	h.Sessions().SampleMemory()
	row, _ := h.Session(s.ID)
	if row.Memory <= 0 {
		t.Errorf("row memory = %d, want the agent's", row.Memory)
	}
	m, ok := h.Rec.Last("memory").(sessions.Memory)
	if !ok {
		t.Fatalf("memory event = %v", h.Rec.Last("memory"))
	}
	if m.Sessions < row.Memory || m.App <= 0 || m.Total != m.App+m.Browsers+m.Sessions || m.Machine <= m.Total {
		t.Errorf("memory = %+v with a session holding %d", m, row.Memory)
	}
	if m != h.Sessions().Memory() {
		t.Errorf("Memory() = %+v, want the last event %+v", h.Sessions().Memory(), m)
	}
	// A sample that finds the figures where they were sends nothing; a loaded machine may move a rounded figure between
	// two samples, and the test then has nothing to say.
	sent, before, held := h.Rec.Count("sessions"), h.Sessions().Memory(), row.Memory
	h.Sessions().SampleMemory()
	row, _ = h.Session(s.ID)
	if n := h.Rec.Count("sessions"); n != sent && h.Sessions().Memory() == before && row.Memory == held {
		t.Errorf("a sample with the same figures sent the sessions %d more times", n-sent)
	}

	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	h.Sessions().SampleMemory()
	if row, _ := h.Session(s.ID); row.Memory != 0 {
		t.Errorf("ended row memory = %d, want 0", row.Memory)
	}

	next, err := h.Sessions().Create(h.Sessions().Current(), "more work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	h.Sessions().SampleMemory()
	row, _ = h.Session(next.ID)
	if m := h.Sessions().Memory(); row.Memory <= 0 || m.Sessions < row.Memory {
		t.Errorf("memory = %+v with a session of a new tmux server holding %d", m, row.Memory)
	}
}
