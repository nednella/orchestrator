package threads

import (
	"testing"

	"github.com/nednella/orchestrator/internal/thread"
)

func TestOnlyAPromptAwaitsTheOwnerInTheMiddleOfATurn(t *testing.T) {
	for _, tt := range []struct {
		state thread.State
		event string
		want  bool
	}{
		{thread.Waiting, "Notification", true},
		{thread.Waiting, "Stop", false},
		{thread.Waiting, thread.TurnError, false},
		{thread.Waiting, "SessionEnd", false},
		{thread.Working, "Notification", false},
		{thread.Idle, "Stop", false},
	} {
		if got := awaitsOwner(thread.Record{State: tt.state, Event: tt.event}); got != tt.want {
			t.Errorf("awaitsOwner(%s after %s) = %v, want %v", tt.state, tt.event, got, tt.want)
		}
	}
}
