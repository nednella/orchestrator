package threads_test

import (
	"os"
	"testing"

	"github.com/nednella/orchestrator/desktop/chat"
)

func TestDeleteWithTheWorktreeGoesOnWhenTheFolderWasRemovedByHand(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	s := issueThread(h, t, 7)
	if err := os.RemoveAll(s.Worktree); err != nil {
		t.Fatal(err)
	}
	if err := h.DeleteThread(s.ID, true); err != nil {
		t.Fatalf("Delete = %v, want a worktree that is gone to keep nothing", err)
	}
	if _, listed := h.Thread(s.ID); listed || tmuxHas(h, s.ID) || hasState(t, h, s.ID) {
		t.Errorf("the thread is still there: listed %v, running %v, state file %v", listed, tmuxHas(h, s.ID), hasState(t, h, s.ID))
	}
	if records := madeRecords(t, h); len(records) != 0 {
		t.Errorf("records = %+v, want the record of the worktree gone with the thread", records)
	}
}

func TestDeleteOfAThreadAtAPromptLeavesNothingWaiting(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.StartThread("work", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(s.ID, "ask"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the approval to wait", func() bool { return len(h.ChatPending()) == 1 })

	if err := h.DeleteThread(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := h.ChatPending(); len(got) != 0 {
		t.Errorf("pending = %+v, want no approval left for a thread that is gone", got)
	}
	eventually(t, "the front end to be told nothing waits", func() bool {
		u, ok := h.Rec.Last("chat:pending").(chat.PendingUpdate)
		return ok && u.ID == s.ID && u.Pending == nil
	})
}
