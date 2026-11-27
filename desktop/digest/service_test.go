package digest_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/digest"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
)

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], key+"="); ok {
			return v
		}
	}
	return ""
}

func lastDigest(h *apptest.Harness) digest.Digest {
	d, _ := h.Rec.Last("digest").(digest.Digest)
	return d
}

const wailsReply = `[
  {"title": "Wails 2.15 released", "why": "you build on Wails", "url": "https://example.com/wails", "source": "wails"},
  {"title": "Duplicate", "why": "same link", "url": "https://example.com/wails", "source": "wails"},
  {"title": "Claude Code hooks", "why": "hooks can match commands", "url": "https://example.com/hooks", "source": "Claude Code"}
]`

func TestDigestRun(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{Env: []string{
		"ORCHESTRATOR_THREAD=main/9", // a run must not inherit a thread
		"DEPLOY_TOKEN=secret",
		"ANTHROPIC_API_KEY=claude-needs-this",
	}})
	if err := os.WriteFile(filepath.Join(h.Dir, "package.json"), []byte(`{"dependencies":{"wails-ish":"1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	h.Claude.SetReply(wailsReply)
	h.Claude.SetDo(func(call apptest.DigestCall) {
		for _, leaked := range []string{"ORCHESTRATOR_THREAD", "DEPLOY_TOKEN", "ORCHESTRATOR_SOCKET", "ORCHESTRATOR_DIGEST_PROJECT"} {
			if envValue(call.Env, leaked) != "" {
				t.Errorf("%s leaked into the digest run", leaked)
			}
		}
		if entries, err := os.ReadDir(call.Dir); err != nil || len(entries) != 0 {
			t.Errorf("the run's folder %s is not empty: %v, %v", call.Dir, entries, err)
		}
	})
	if err := h.RunDigest(); err != nil {
		t.Fatal(err)
	}
	if err := h.RunDigest(); err == nil {
		t.Error("a second run started while one was going")
	}
	eventually(t, "the digest to finish", func() bool { d := lastDigest(h); return !d.Running && d.LastRunAt > 0 })

	call := h.Claude.Calls()[0]
	if call.Dir == h.Dir || !strings.Contains(call.Dir, "orchestrator-digest") || exists(call.Dir) {
		t.Errorf("the run's folder = %s (the project is %s): want a fresh folder that is removed afterwards", call.Dir, h.Dir)
	}
	if envValue(call.Env, "ANTHROPIC_API_KEY") != "claude-needs-this" || envValue(call.Env, "PATH") == "" || envValue(call.Env, "HOME") == "" {
		t.Errorf("claude's own environment was dropped: %v", call.Env)
	}
	if !strings.Contains(call.Args[2], "wails-ish") {
		t.Errorf("the prompt does not list the project's dependencies:\n%s", call.Args[2])
	}
	if len(call.Args) != 5 || call.Args[0] != "claude" || call.Args[1] != "-p" || call.Args[3] != "--allowedTools" ||
		call.Args[4] != "WebSearch WebFetch" || strings.Contains(call.Args[2], "orchestrator") {
		t.Errorf("command = %q in %s", call.Args, call.Dir)
	}
	if got := lastDigest(h).Project; got != "main" {
		t.Errorf("digest event project = %q", got)
	}
	d := h.Digest()
	if len(d.Items) != 2 || d.Items[0].Title != "Wails 2.15 released" || d.Items[1].Source != "Claude Code" || d.Error != "" || d.NextRunAt <= d.LastRunAt {
		t.Fatalf("digest = %+v", d)
	}
	if d.StartedAt == 0 || d.StartedAt > d.LastRunAt || d.StoppedAt != 0 {
		t.Errorf("run times: started %d, finished %d, stopped %d", d.StartedAt, d.LastRunAt, d.StoppedAt)
	}
	if d.Tools != 1 {
		t.Errorf("tools = %d, want the 1 dependency in package.json", d.Tools)
	}
	note, err := h.DigestToNote(d.Items[0].ID)
	if err != nil || !strings.HasPrefix(note.Text, "Wails 2.15 released\n\nyou build on Wails\nhttps://example.com/wails") {
		t.Fatalf("DigestToNote = %+v, %v", note, err)
	}
	if d = h.Digest(); d.Items[0].NoteID != note.ID {
		t.Errorf("the item does not know its note: %+v", d.Items[0])
	}
	if err := h.DismissDigestItem(d.Items[1].ID); err != nil || len(h.Digest().Items) != 1 {
		t.Errorf("dismiss: %v", err)
	}
	if err := h.DismissDigestItem("nope"); err == nil {
		t.Error("dismissed an unknown item")
	}
}

func runDigest(t *testing.T, h *apptest.Harness) digest.Digest {
	t.Helper()
	if err := h.RunDigest(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the digest run to end", func() bool { d := lastDigest(h); return !d.Running && (d.LastRunAt > 0 || d.Error != "") })
	return h.Digest()
}

func TestDigestReadsItemsFromTheReply(t *testing.T) {
	t.Parallel()
	item := func(n int) string {
		return fmt.Sprintf(`{"title":"Item %d","why":"why %d","url":"https://example.com/%d","source":"src"}`, n, n, n)
	}
	tests := []struct {
		name, reply string
		want        []string
	}{
		{"the array alone", "[" + item(1) + "," + item(2) + "]", []string{"Item 1", "Item 2"}},
		{"words and a fenced array", "Here is what I found.\n\n```json\n[" + item(1) + "]\n```", []string{"Item 1"}},
		{"a fence with no language", "```\n[" + item(1) + "]\n```\n", []string{"Item 1"}},
		{"words and an array on the last lines", "Done.\n[\n" + item(1) + "\n]\n", []string{"Item 1"}},
		{"nothing worth it", "Nothing this time.\n[]", nil},
		{"more than five", "[" + strings.Join([]string{item(1), item(2), item(3), item(4), item(5), item(6), item(7)}, ",") + "]",
			[]string{"Item 1", "Item 2", "Item 3", "Item 4", "Item 5"}},
		{"one item without a web link", `[{"title":"bad","why":"","url":"javascript:alert(1)","source":""},` + item(2) + `]`, []string{"Item 2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.Claude.SetReply(tt.reply)
			d := runDigest(t, h)
			var got []string
			for _, it := range d.Items {
				got = append(got, it.Title)
			}
			if d.Error != "" || !slices.Equal(got, tt.want) || d.LastRunAt == 0 {
				t.Errorf("digest = %+v, titles %v, want %v", d, got, tt.want)
			}
		})
	}
}

func TestDigestKeepsOnlyWebLinks(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var items []string
	for url, ok := range map[string]bool{
		"https://example.com/a": true, "http://example.com/b": true,
		"javascript:alert(1)": false, "file:///etc/passwd": false, "ftp://example.com/x": false, "example.com/c": false, "https://": false,
	} {
		if ok {
			items = append(items, fmt.Sprintf(`{"title":"t","url":%q}`, url))
		}
	}
	for _, url := range []string{"javascript:alert(1)", "file:///etc/passwd", "ftp://example.com/x", "example.com/c", "https://"} {
		items = append(items, fmt.Sprintf(`{"title":"t","url":%q}`, url))
	}
	h.Claude.SetReply("[" + strings.Join(items, ",") + "]")
	d := runDigest(t, h)
	if len(d.Items) != 2 || d.Error != "" {
		t.Errorf("digest = %+v, want the 2 web links", d)
	}
}

func TestDigestRefusesABadReply(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, reply, want string }{
		{"words", "I found some things but forgot the list.", "did not end in a JSON array"},
		{"an object", `{"title":"x","url":"https://example.com"}`, "did not end in a JSON array"},
		{"a broken array", `[{"title":"x","url":"https://example.com"}`, "did not end in a JSON array"},
		{"an array and then words", "[]\nHope that helps.", "did not end in a JSON array"},
		{"only items with no web link", `[{"title":"x","url":"file:///etc/passwd"},{"url":"https://example.com"}]`, "no item with a title and a web link"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			good := `[{"title":"kept","why":"w","url":"https://example.com/kept","source":"s"}]`
			h.Claude.SetReply(good)
			if d := runDigest(t, h); len(d.Items) != 1 {
				t.Fatalf("the first run kept %+v", d.Items)
			}
			h.Claude.SetReply(tt.reply)
			d := runDigest(t, h)
			if !strings.Contains(d.Error, tt.want) || d.StoppedAt == 0 {
				t.Errorf("error = %q, want one with %q", d.Error, tt.want)
			}
			if len(d.Items) != 1 || d.Items[0].Title != "kept" {
				t.Errorf("a refused reply changed the digest: %+v", d.Items)
			}
		})
	}
}

func TestDigestFailures(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		authErr, err  error
		want          string
		wantRunCalled bool
	}{
		{"claude missing", fmt.Errorf("claude: %w", exec.ErrNotFound), nil, "no agent is installed and signed in (tried claude)", false},
		{"claude signed out", errors.New("exit status 1"), nil, "no agent is installed and signed in (tried claude)", false},
		{"claude fails", nil, context.Canceled, "context canceled", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			h.Claude.AuthErr, h.Claude.Err = tt.authErr, tt.err
			if err := h.RunDigest(); err != nil {
				t.Fatal(err)
			}
			eventually(t, "the failure", func() bool { d := lastDigest(h); return !d.Running && d.Error != "" })
			if d := lastDigest(h); !strings.Contains(d.Error, tt.want) || d.LastRunAt != 0 || d.StartedAt == 0 || d.StoppedAt < d.StartedAt {
				t.Errorf("digest = %+v", d)
			}
			if got := len(h.Claude.Calls()) > 0; got != tt.wantRunCalled {
				t.Errorf("claude ran = %v, want %v", got, tt.wantRunCalled)
			}
		})
	}
}

func TestDigestRunAfterAFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.Claude.Err = context.Canceled
	if err := h.RunDigest(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the failure", func() bool { d := lastDigest(h); return !d.Running && d.StoppedAt > 0 })
	h.Claude.Err = nil
	if err := h.RunDigest(); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the second run to finish", func() bool { d := lastDigest(h); return !d.Running && d.LastRunAt > 0 })
	if d := h.Digest(); d.Error != "" || d.StoppedAt != 0 {
		t.Errorf("after a good run the failure stays: %+v", d)
	}
}

func TestDigestAutomaticRuns(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{DigestFirst: 50 * time.Millisecond, DigestTick: 50 * time.Millisecond})
	eventually(t, "the automatic run", func() bool { return len(h.Claude.Calls()) == 1 && !h.Digest().Running })
	time.Sleep(300 * time.Millisecond)
	if n := len(h.Claude.Calls()); n != 1 {
		t.Errorf("ran %d times; a run in the last week should stop the next", n)
	}
	if h.Digest().NextRunAt == 0 {
		t.Error("no next run time")
	}

	off := apptest.NewWith(t, apptest.Options{ProjectExtra: "    digest_schedule: off\n", DigestFirst: 50 * time.Millisecond, DigestTick: 50 * time.Millisecond})
	time.Sleep(400 * time.Millisecond)
	if n := len(off.Claude.Calls()); n != 0 || off.Digest().NextRunAt != 0 {
		t.Errorf("a project with the digest off ran %d times, next run %d", n, off.Digest().NextRunAt)
	}
}
