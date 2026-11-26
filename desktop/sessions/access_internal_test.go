package sessions

import (
	"testing"
	"time"

	"github.com/nednella/orchestrator/internal/session"
)

// A delegation holds childMu from its check of the parent's access to the child's start, so a change of access waits
// for it: the parent cannot lose access between the check and a child that has the old one.
func TestSetAccessWaitsForADelegation(t *testing.T) {
	s := newSessions(Options{})
	s.childMu.Lock()
	done := make(chan error, 1)
	go func() { done <- s.SetAccess("main/abcdefgh", session.Read) }()
	select {
	case err := <-done:
		t.Fatalf("SetAccess returned %v while a delegation held childMu", err)
	case <-time.After(200 * time.Millisecond):
	}
	s.childMu.Unlock()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SetAccess did not return once the delegation ended")
	}
}
