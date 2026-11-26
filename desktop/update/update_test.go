package update_test

import (
	"os"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/update"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

func TestANewerReleaseShowsInTheState(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{UpdateTick: 50 * time.Millisecond, Releases: "v9.0.0", Version: "1.0.0"})
	ready := update.State{Status: "available", Version: "9.0.0"}
	apptest.Eventually(t, "the update event", func() bool { return h.Rec.Last("update") == ready })
	apptest.Eventually(t, "a second check", func() bool { return h.GH.Calls("curl") >= 4 })
	apptest.Eventually(t, "the second check's end", func() bool { return h.App.Updates().State() == ready })
}

func TestAFailedCheckLeavesTheAppCurrent(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{UpdateTick: time.Hour, Version: "1.0.0"})
	apptest.Eventually(t, "the check", func() bool { return h.GH.Calls("curl") == 1 })
	if got := h.App.Updates().State(); got != (update.State{Status: "idle"}) {
		t.Errorf("after a failed check: %+v", got)
	}
	if n := h.Rec.Count("update"); n != 0 {
		t.Errorf("%d update events", n)
	}
}
