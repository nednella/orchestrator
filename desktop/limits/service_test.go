package limits_test

import (
	"context"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/limits"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/session"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

// standInWindows are the windows the stand-in claude answers get_usage with.
var standInWindows = []limits.Window{
	{Name: "five_hour", Label: "Session", Left: 75, ResetsAt: 1795705200000},
	{Name: "seven_day", Label: "Weekly", Left: 4, ResetsAt: 1796079600000},
}

// usageRequests are the get_usage requests among lines, and whether any line is a user message.
func usageRequests(lines []string) (asks int, user bool) {
	for _, l := range lines {
		asks += strings.Count(l, `"subtype":"get_usage"`)
		user = user || strings.Contains(l, `"type":"user"`)
	}
	return asks, user
}

func TestRefreshAsksALiveThread(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	chat, err := h.Sessions().ChatOf(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := host.Ready(ctx, chat.Socket); err != nil {
		t.Fatal(err)
	}
	got, err := h.RefreshLimits()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Windows, standInWindows) || got.Error != "" || got.Unsupported {
		t.Errorf("limits = %+v, want the stand-in's windows", got)
	}
	if asks, user := usageRequests(h.Stdin(t, s.ID)); asks != 1 || user {
		t.Errorf("the thread's claude read %d get_usage requests, a user message: %v; want one request and no message", asks, user)
	}
	if sent := h.Rec.All("limits"); len(sent) == 0 || !reflect.DeepEqual(sent[len(sent)-1], got) {
		t.Errorf("limits events = %+v, want the last to be the limits read", sent)
	}
}

func TestARateLimitInATurnReachesTheLimits(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the thread's follower", func() bool { return h.ChatWatch(s.ID) == nil })
	want := limits.Window{Name: "five_hour", Label: "Session", Left: 50, ResetsAt: 1795705200000}
	// A rate_limit event is live only: one Claude streams before the follower subscribes is gone, so the test sends
	// again until one arrives.
	deadline := time.Now().Add(30 * time.Second)
	for !slices.Contains(h.Limits().Windows, want) {
		if time.Now().After(deadline) {
			t.Fatalf("limits = %+v, want the turn's session window", h.Limits())
		}
		if _, err := h.ChatSend(s.ID, "limit"); err != nil {
			t.Logf("send: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func TestRefreshWithNoThreadProbes(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	got, err := h.RefreshLimits()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Windows, standInWindows) || got.Error != "" {
		t.Errorf("limits = %+v, want the stand-in's windows", got)
	}
	// The probe's claude runs with no thread, so its files have no session id in their names.
	want := []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--strict-mcp-config", "--tools", "", "--setting-sources", "project,local"}
	if args := h.Args(t, ""); !slices.Equal(args, want) {
		t.Errorf("the probe ran claude %q, want %q", args, want)
	}
	if asks, user := usageRequests(h.Stdin(t, "")); asks != 1 || user {
		t.Errorf("the probe's claude read %d get_usage requests, a user message: %v; want one request and no message", asks, user)
	}
	if again, err := h.RefreshLimits(); err != nil || !reflect.DeepEqual(again.Windows, standInWindows) {
		t.Fatalf("second refresh = %+v, %v", again, err)
	}
	if asks, _ := usageRequests(h.Stdin(t, "")); asks != 1 {
		t.Errorf("%d probes, want the fresh values kept", asks)
	}
}
