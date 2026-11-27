package threads_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/thread"
)

func TestTimelineAndWorkOutliveTheApp(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.Threads().Create(h.Threads().Current(), "#5 work", "", false, 5, thread.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	// The hooks of a thread that worked twice while the app was closed.
	t0 := time.Now().Add(-10 * time.Second).UnixMilli()
	changes := []thread.Change{{State: thread.Idle, At: t0}, {State: thread.Working, At: t0 + 1000}, {State: thread.Waiting, At: t0 + 3000}, {State: thread.Working, At: t0 + 4000}, {State: thread.Idle, At: t0 + 6000}}
	rec := thread.Record{Thread: s.ID, State: thread.Idle, Event: "Stop", At: time.UnixMilli(t0 + 6000), Changes: changes}
	if err := bus.WriteState(h.State, rec); err != nil {
		t.Fatal(err)
	}

	second := h.Restart(t)
	if got := second.Threads().History(s.ID); !reflect.DeepEqual(got, changes) {
		t.Errorf("history = %+v, want the one the hooks kept: %+v", got, changes)
	}
	eventually(t, "the work done while the app was closed", func() bool {
		led, err := second.Ledger(30, "")
		return err == nil && led.Totals.WorkMs == 4000
	})

	third := second.Restart(t)
	time.Sleep(300 * time.Millisecond)
	led, err := third.Ledger(30, "")
	if err != nil {
		t.Fatal(err)
	}
	if led.Totals.WorkMs != 4000 {
		t.Errorf("worked %dms after a second restart, want 4000: a stretch was counted twice", led.Totals.WorkMs)
	}
}
