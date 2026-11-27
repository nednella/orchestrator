package threads_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
)

// flagRequests are the settings changes the thread's stand-in claude has read.
func flagRequests(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	var out []string
	for _, l := range h.Stdin(t, id) {
		if strings.Contains(l, `"subtype":"apply_flag_settings"`) {
			out = append(out, l)
		}
	}
	return out
}

func TestFastModeChangesWhileTheThreadRuns(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewThreadFast("fast", "opus", "high", false)
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, s.ID)

	if err := h.SetFast(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := flagRequests(t, h, s.ID); len(got) != 1 || !strings.Contains(got[0], `"settings":{"fastMode":true}`) {
		t.Fatalf("stdin = %q, want one request for fast mode on", got)
	}
	eventually(t, "fast mode in the row and the state file", flagsAre(h, s.ID, "high", true))

	if err := h.SetFast(s.ID, false); err != nil {
		t.Fatal(err)
	}
	if got := flagRequests(t, h, s.ID); len(got) != 2 || !strings.Contains(got[1], `"settings":{"fastMode":null}`) {
		t.Fatalf("stdin = %q, want a second request taking fast mode out", got)
	}
	eventually(t, "fast mode off in the row and the state file", flagsAre(h, s.ID, "high", false))
	if got := h.Args(t, s.ID); !slices.Equal(got, args) {
		t.Errorf("claude started again with %q", got)
	}
}

func TestEffortChangesWhileTheThreadRuns(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewThreadFast("effort", "opus", "high", false)
	if err != nil {
		t.Fatal(err)
	}
	claudeArgs(t, h, s.ID)

	if err := h.SetEffort(s.ID, "ultracode"); err != nil {
		t.Fatal(err)
	}
	if got := flagRequests(t, h, s.ID); len(got) != 1 || !strings.Contains(got[0], `"settings":{"effortLevel":"xhigh","ultracode":true}`) {
		t.Fatalf("stdin = %q, want one request for xhigh with ultracode", got)
	}
	eventually(t, "ultracode in the row and the state file", flagsAre(h, s.ID, "ultracode", false))

	if err := h.SetEffort(s.ID, "low"); err != nil {
		t.Fatal(err)
	}
	if got := flagRequests(t, h, s.ID); len(got) != 2 || !strings.Contains(got[1], `"settings":{"effortLevel":"low","ultracode":false}`) {
		t.Fatalf("stdin = %q, want a second request for low without ultracode", got)
	}
	eventually(t, "low in the row and the state file", flagsAre(h, s.ID, "low", false))

	for _, bad := range []string{"", "hgih"} {
		if err := h.SetEffort(s.ID, bad); err == nil {
			t.Errorf("effort %q was taken", bad)
		}
	}
	if got := flagRequests(t, h, s.ID); len(got) != 2 {
		t.Errorf("stdin = %q, want no request for a bad effort", got)
	}
}

func TestAChangeClaudeRefusesLeavesTheThreadAsItWas(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewThreadFast("refused", "haiku", "high", false)
	if err != nil {
		t.Fatal(err)
	}
	claudeArgs(t, h, s.ID)
	h.RefuseFlags(t, s.ID, "ultracode is not available for this session (claude-haiku-4-5 does not support it)")

	err = h.SetEffort(s.ID, "ultracode")
	if err == nil || err.Error() != "ultracode is not available for this session (claude-haiku-4-5 does not support it)" {
		t.Fatalf("err = %v, want Claude's reason", err)
	}
	if err := h.SetFast(s.ID, true); err == nil {
		t.Error("a refused fast mode was taken")
	}
	eventually(t, "the row and the state file as before", flagsAre(h, s.ID, "high", false))
}

func TestFastModeAndUltracodeOutliveTheApp(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewThreadFast("lasting", "opus", "high", false)
	if err != nil {
		t.Fatal(err)
	}
	claudeArgs(t, h, s.ID)
	if err := h.SetFast(s.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := h.SetEffort(s.ID, "ultracode"); err != nil {
		t.Fatal(err)
	}
	h.App.Stop()

	second := h.Restart(t)
	eventually(t, "the row to show both", func() bool {
		row, ok := second.Thread(s.ID)
		return ok && row.Fast && row.Effort == "ultracode"
	})
}

func TestASettledThreadWakesWithFastModeAndUltracode(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	before := talkedThenSettled(t, h)
	if err := h.SetFast(before.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := h.SetEffort(before.ID, "ultracode"); err != nil {
		t.Fatal(err)
	}
	if got := flagRequests(t, h, before.ID); len(got) != 0 {
		t.Errorf("stdin = %q, want no request to a thread that sleeps", got)
	}
	if row, _ := h.Thread(before.ID); !row.Fast || row.Effort != "ultracode" || row.State != "settled" {
		t.Fatalf("row = %+v, want the settled thread with both", row)
	}
	if _, err := h.WakeThread(before.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "claude to start again with both", func() bool {
		args := h.Args(t, before.ID)
		settings := startSettings(args)
		i := slices.Index(args, "--effort")
		return strings.Contains(settings, `"fastMode":true`) && strings.Contains(settings, `"ultracode":true`) && i >= 0 && args[i+1] == "xhigh"
	})
}
