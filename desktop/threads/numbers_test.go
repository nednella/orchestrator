package threads_test

import (
	"fmt"
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/evidence"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/stats"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/thread"
)

func evidenceDir(h *apptest.Harness, id string) string {
	name, _ := thread.ParseName(id)
	return filepath.Join(h.State, "data", name.Project, "evidence", name.Token)
}

func TestNumbersAreNeverReused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	first, err := h.StartThread("first", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.StartThread("second", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(first.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.DeleteThread(first.ID, false); err != nil {
		t.Fatal(err)
	}
	third, err := h.StartThread("third", "")
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

func TestNumbersSurviveARestartAndADelete(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	a, err := h.StartThread("a", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.StartThread("b", ""); err != nil {
		t.Fatal(err)
	}
	// A thread from a build before numbers were saved: it is older than both, and takes the next free number once.
	old := time.Now().Add(-2 * time.Hour)
	rec := thread.Record{Thread: "main/old1", State: thread.Settled, Event: "SessionEnd", At: old, Identity: thread.Identity{Title: "old", Created: old.UnixMilli()}, SettledAt: old.Add(time.Hour).UnixMilli()}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}
	h.Threads().Refresh()
	if err := h.SettleThread(a.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.DeleteThread(a.ID, false); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"b": 2, "old": 3}
	if got := numbers(h); !maps.Equal(got, want) {
		t.Fatalf("numbers = %v, want %v", got, want)
	}

	h.App.Stop()
	second := h.Restart(t)
	if got := numbers(second); !maps.Equal(got, want) {
		t.Errorf("numbers after a restart = %v, want %v", got, want)
	}
	c, err := second.StartThread("c", "")
	if err != nil || c.N != 4 {
		t.Errorf("the next thread = %+v, %v, want number 4", c, err)
	}
	second.App.Stop()
	want["c"] = 4
	if got := numbers(second.Restart(t)); !maps.Equal(got, want) {
		t.Errorf("numbers after a second restart = %v, want %v", got, want)
	}
}

func TestADeletedNumberIsNotGivenAgainAfterARestart(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var fifth threads.Thread
	for i := range 5 {
		s, err := h.StartThread(fmt.Sprint(i+1), "")
		if err != nil {
			t.Fatal(err)
		}
		fifth = s
	}
	if fifth.N != 5 {
		t.Fatalf("the fifth thread is #%d", fifth.N)
	}
	if err := h.SettleThread(fifth.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.DeleteThread(fifth.ID, false); err != nil {
		t.Fatal(err)
	}

	h.App.Stop()
	second := h.Restart(t)
	next, err := second.StartThread("next", "")
	if err != nil || next.N != 6 {
		t.Errorf("the thread after a deleted #5 and a restart = %+v, %v, want #6", next, err)
	}
}

// numbers is the number of each thread on the list, by title.
func numbers(h *apptest.Harness) map[string]int {
	got := map[string]int{}
	for _, s := range h.Threads().List() {
		got[s.Title] = s.N
	}
	return got
}

func TestNewThreadSeesNothingOfADeletedOne(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	old, err := h.StartThread("old", "")
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
	if err := h.DeleteThread(old.ID, false); err != nil {
		t.Fatal(err)
	}
	if exists(evidenceDir(h, old.ID)) {
		t.Error("the deleted thread's evidence is still there")
	}

	fresh, err := h.StartThread("fresh", "")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Evidence != 0 || exists(evidenceDir(h, fresh.ID)) {
		t.Errorf("the new thread has evidence: %+v", fresh)
	}
}

func TestThreadHasItsOwnTempFolder(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("work", "")
	if err != nil {
		t.Fatal(err)
	}
	name, err := thread.ParseName(s.ID)
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
	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if exists(tmp) {
		t.Error("the temp folder is still there after the delete")
	}
}

func TestSettledThreadStays(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(s.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the row to settle", func() bool {
		got, ok := h.Thread(s.ID)
		return ok && got.State == thread.Settled && got.SettledAt > 0
	})
	if err, want := h.SettleThread(s.ID), "Thread "+s.ID+" is asleep"; err == nil || err.Error() != want {
		t.Errorf("a second Settle = %v, want %q", err, want)
	}

	const id = "main/old1"
	long := time.Now().Add(-8 * 24 * time.Hour)
	rec := thread.Record{Thread: id, State: thread.Settled, Event: "SessionEnd", At: long, Identity: thread.Identity{Title: "old"}, SettledAt: long.UnixMilli()}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := evidence.New(filepath.Join(h.State, "data"), stats.NewLog(filepath.Join(h.State, "data"), "testtest")).AddText(id, "proof", "", "agent"); err != nil {
		t.Fatal(err)
	}
	h.Threads().Refresh()
	if got, ok := h.Thread(id); !ok || got.State != thread.Settled || got.Title != "old" || got.SettledAt != long.UnixMilli() {
		t.Errorf("a thread settled 8 days ago = %+v, %v; want its settled row", got, ok)
	}
	if !exists(filepath.Join(h.State, "threads", "main", "old1.json")) || !exists(evidenceDir(h, id)) {
		t.Error("the state file or the evidence of a thread settled 8 days ago went")
	}
}
