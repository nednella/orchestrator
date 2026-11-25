package sessions_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/evidence"
	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/stats"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
)

func evidenceDir(h *apptest.Harness, id string) string {
	name, _ := session.ParseName(id)
	return filepath.Join(h.State, "data", name.Project, "evidence", name.Token)
}

func TestNumbersAreNeverReused(t *testing.T) {
	h := newHarness(t)
	first, err := h.NewSession("first", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.NewSession("second", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.ArchiveThread(first.ID); err != nil {
		t.Fatal(err)
	}
	third, err := h.NewSession("third", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.ID == third.ID || second.ID == third.ID {
		t.Errorf("ids are not distinct: %s %s %s", first.ID, second.ID, third.ID)
	}
	if first.N != 1 || second.N != 2 || third.N != 3 {
		t.Errorf("numbers = %d %d %d, want 1 2 3", first.N, second.N, third.N)
	}
}

func TestNumbersAfterARestartFollowCreationOrder(t *testing.T) {
	h := newHarness(t)
	a, err := h.NewSession("a", "")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // tmux dates a session to the second
	if _, err := h.NewSession("b", ""); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	rec := session.Record{Session: "main/old1", State: session.Settled, Event: "SessionEnd", At: old, Identity: session.Identity{Title: "old", Created: old.UnixMilli()}, SettledAt: old.Add(time.Hour).UnixMilli()}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}

	second := h.Restart(t)
	got := map[string]int{}
	for _, s := range second.Sessions().List() {
		got[s.Title] = s.N
	}
	if got["old"] != 1 || got["a"] != 2 || got["b"] != 3 {
		t.Errorf("numbers = %v, want old 1, a 2, b 3 (a is %s)", got, a.ID)
	}
	c, err := second.NewSession("c", "")
	if err != nil || c.N != 4 {
		t.Errorf("the next session = %+v, %v, want number 4", c, err)
	}
}

func TestNewSessionSeesNothingOfAnArchivedOne(t *testing.T) {
	h := newHarness(t)
	old, err := h.NewSession("old", "")
	if err != nil {
		t.Fatal(err)
	}
	store := evidence.New(filepath.Join(h.State, "data"), stats.NewLog(filepath.Join(h.State, "data"), "testtest"))
	if _, err := store.AddText(old.ID, "proof", "", "agent"); err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(old.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.ArchiveThread(old.ID); err != nil {
		t.Fatal(err)
	}
	if !exists(evidenceDir(h, old.ID)) {
		t.Error("the archived thread's evidence went")
	}

	fresh, err := h.NewSession("fresh", "")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Evidence != 0 || exists(evidenceDir(h, fresh.ID)) {
		t.Errorf("the new session has evidence: %+v", fresh)
	}
}

func TestSessionHasItsOwnTempFolder(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	name, err := session.ParseName(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(h.State, "data", name.Project, "tmp", name.Token)
	if !exists(tmp) {
		t.Fatalf("no temp folder at %s", tmp)
	}
	if got := claudeEnv(t, h, s.ID)["TMPDIR"]; got != tmp {
		t.Errorf("claude was started with TMPDIR=%s, want %s", got, tmp)
	}

	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.ArchiveThread(s.ID); err != nil {
		t.Fatal(err)
	}
	if exists(tmp) {
		t.Error("the temp folder is still there after the archive")
	}
}

func TestSettledThreadStays(t *testing.T) {
	h := newHarness(t)
	s, err := h.NewSession("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the row to settle", func() bool {
		got, ok := h.Session(s.ID)
		return ok && got.State == session.Settled && got.SettledAt > 0
	})
	if err, want := h.SettleThread(s.ID), "Thread "+s.ID+" has settled"; err == nil || err.Error() != want {
		t.Errorf("a second Settle = %v, want %q", err, want)
	}

	const id = "main/old1"
	long := time.Now().Add(-8 * 24 * time.Hour)
	rec := session.Record{Session: id, State: session.Settled, Event: "SessionEnd", At: long, Identity: session.Identity{Title: "old"}, SettledAt: long.UnixMilli()}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := evidence.New(filepath.Join(h.State, "data"), stats.NewLog(filepath.Join(h.State, "data"), "testtest")).AddText(id, "proof", "", "agent"); err != nil {
		t.Fatal(err)
	}
	h.Sessions().Refresh()
	if got, ok := h.Session(id); !ok || got.State != session.Settled || got.Title != "old" || got.SettledAt != long.UnixMilli() {
		t.Errorf("a thread settled 8 days ago = %+v, %v; want its settled row", got, ok)
	}
	if !exists(filepath.Join(h.State, "sessions", "main", "old1.json")) || !exists(evidenceDir(h, id)) {
		t.Error("the state file or the evidence of a thread settled 8 days ago went")
	}
}
