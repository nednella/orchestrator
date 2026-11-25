package issues_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/prompts"
	"github.com/nednella/agentos/internal/session"
)

func TestIssueCommand(t *testing.T) {
	h := newHarness(t)
	tests := []struct {
		name string
		args []string
		want string
		fail bool
	}{
		{"two issues", []string{"7", "#8"}, "started 2 sessions: #7 (1), #8 (2)", false},
		{"one running one new", []string{"7", "11"}, "started 1 session: #11 (3); already running: #7 (1)", false},
		{"some fail", []string{"x", "999", "8"}, "already running: #8 (2); failed: x (not a number), #999 (issue #999 is not open in this project)", false},
		{"all fail", []string{"x"}, "failed: x (not a number)", true},
		{"none", nil, "usage:", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: tt.args})
			if resp.OK == tt.fail || !strings.Contains(resp.Out+resp.Error, tt.want) {
				t.Errorf("issue %v = %+v, want %q", tt.args, resp, tt.want)
			}
		})
	}
}

func TestIssueCommandHandsThePromptOver(t *testing.T) {
	h := apptest.New(t)
	resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "the patch is in /tmp/x"}})
	if !resp.OK {
		t.Fatalf("issue 7 = %+v", resp)
	}
	list := h.Sessions().List()
	if len(list) != 1 || list[0].Issue != 7 {
		t.Fatalf("sessions = %+v, want one for #7", list)
	}
	if args := argsOf(t, h.ClaudeDir(), list[0].ID); !strings.Contains(args, prompts.Handover("the patch is in /tmp/x")) {
		t.Errorf("the system prompt lacks the handover:\n%s", args)
	}

	resp = h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "more"}})
	if !resp.OK || !strings.Contains(resp.Out, "already running: #7 (1, the prompt was not given to it)") {
		t.Errorf("issue 7 again = %+v", resp)
	}
}

func TestWokenThreadGetsTheHandoverAgain(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
	gitRepo(t, h)
	if resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "the patch is in /tmp/x"}}); !resp.OK {
		t.Fatalf("issue 7 = %+v", resp)
	}
	old := h.Sessions().List()[0]
	argsOf(t, h.ClaudeDir(), old.ID)
	h.GH.SetPR(`[{"number":12,"url":"https://github.com/acme/widgets/pull/12","state":"OPEN","isDraft":false,"statusCheckRollup":[],"comments":[],"reviews":[],"updatedAt":"2026-10-01T12:00:00Z"}]`)
	h.RefreshPRs()
	if err := h.SettleThread(old.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(h.ClaudeDir(), strings.ReplaceAll(old.ID, "/", "_")+".args")); err != nil {
		t.Fatal(err)
	}
	h.GH.SetPR(`[{"number":12,"url":"https://github.com/acme/widgets/pull/12","state":"OPEN","isDraft":false,"statusCheckRollup":[],"comments":[],"reviews":[{"id":"x"}],"updatedAt":"2026-10-01T12:00:00Z"}]`)
	h.RefreshPRs()
	eventually(t, "the thread woken for the review", func() bool { got, _ := h.Session(old.ID); return got.State != "settled" })
	if args := argsOf(t, h.ClaudeDir(), old.ID); !strings.Contains(args, prompts.Handover("the patch is in /tmp/x")) {
		t.Errorf("the woken thread's system prompt lacks the handover:\n%s", args)
	}
}

func TestIssueCommandWakesTheSettledThreadWithTheNewHandover(t *testing.T) {
	h := apptest.New(t)
	if resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "the patch is in /tmp/x"}}); !resp.OK {
		t.Fatalf("issue 7 = %+v", resp)
	}
	old := h.Sessions().List()[0]
	argsOf(t, h.ClaudeDir(), old.ID)
	if err := h.SettleThread(old.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(h.ClaudeDir(), strings.ReplaceAll(old.ID, "/", "_")+".args")); err != nil {
		t.Fatal(err)
	}

	resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "now see /tmp/y"}})
	if !resp.OK || resp.Out != "woke the settled: #7 (1, with the prompt)" {
		t.Errorf("issue 7 on a settled thread = %+v", resp)
	}
	if list := h.Sessions().List(); len(list) != 1 || list[0].ID != old.ID || list[0].State == "settled" {
		t.Errorf("sessions = %+v, want %s woken", list, old.ID)
	}
	args := argsOf(t, h.ClaudeDir(), old.ID)
	if !strings.Contains(args, prompts.Handover("now see /tmp/y")) || strings.Contains(args, "/tmp/x") {
		t.Errorf("the woken thread's system prompt does not hold the new handover alone:\n%s", args)
	}
	eventually(t, "the action's command on the woken thread's stdin", func() bool {
		n := 0
		for _, line := range h.Stdin(t, old.ID) {
			if strings.Contains(line, `"text":"/ship 7"`) {
				n++
			}
		}
		return n == 2
	})
}

func TestLongestPromptStartsTheSession(t *testing.T) {
	h := apptest.New(t)
	handover := strings.Repeat(`it's "quoted" $HOME \ `, 6<<10)[:6<<10] // the most agentos issue --prompt takes
	if resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": handover}}); !resp.OK {
		t.Fatalf("issue 7 with the longest prompt = %+v", resp)
	}
	if args := argsOf(t, h.ClaudeDir(), h.Sessions().List()[0].ID); !strings.Contains(args, strings.TrimSpace(handover)) {
		t.Error("the system prompt lacks the handover")
	}
}

func TestRefreshCommand(t *testing.T) {
	h := newHarness(t)
	if _, err := h.Issues(false); err != nil {
		t.Fatal(err)
	}
	resp := h.Ask(t, ctl.Request{Cmd: "refresh"})
	if !resp.OK || resp.Out != "refreshed 6 issues and the pull requests" || h.GH.IssueCalls() != 2 {
		t.Errorf("refresh = %+v, gh issue list ran %d times", resp, h.GH.IssueCalls())
	}
}

func TestIssueCommandRefusesFullFromAThreadThatIsNot(t *testing.T) {
	h := apptest.NewWith(t, apptest.Options{Queue: `    queue_sections:
      - name: Ready
        labels: [ready]
        actions:
          - {name: Work, command: "/ship {n}", access: full}
      - name: Inbox
        actions:
          - {name: Investigate, command: "/investigate {n}"}
`})
	supervised, err := h.NewSession("supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	read, err := h.NewThread("read", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	askers := map[string]string{"a Supervised thread": supervised.ID, "a Read thread": read.ID, "a session that is not a thread": "garbage", "a thread that is gone": "main/abcdefgh"}
	refused := func(what string) {
		t.Helper()
		for from, asker := range askers {
			resp := h.Ask(t, ctl.Request{Cmd: "issue", Session: asker, Args: []string{"7"}})
			if resp.OK || !strings.Contains(resp.Error, "A thread that is not Full access cannot start one that is") {
				t.Errorf("issue 7 %s from %s = %+v", what, from, resp)
			}
		}
	}
	issueThread := func() (sessions.Session, bool) {
		for _, s := range h.Sessions().List() {
			if s.Issue == 7 {
				return s, true
			}
		}
		return sessions.Session{}, false
	}

	refused("with a Full action")
	if s, ok := issueThread(); ok {
		t.Fatalf("a thread that is not Full started %+v", s)
	}

	if resp := h.Ask(t, ctl.Request{Cmd: "issue", Session: read.ID, Args: []string{"11"}}); !resp.OK {
		t.Errorf("issue 11, a Supervised action, from a Read thread = %+v", resp)
	}

	if resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}}); !resp.OK {
		t.Fatalf("issue 7 from the shell = %+v", resp)
	}
	full, ok := issueThread()
	if !ok || full.Access != session.Full {
		t.Fatalf("issue 7 from the shell started %+v", full)
	}
	if err := h.SettleThread(full.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the Full thread settled", func() bool { s, _ := h.Session(full.ID); return s.State == session.Settled })

	refused("with a settled Full thread")
	if s, _ := h.Session(full.ID); s.State != session.Settled {
		t.Errorf("a thread that is not Full woke the Full thread: %+v", s)
	}
	if resp := h.Ask(t, ctl.Request{Cmd: "issue", Args: []string{"7"}}); !resp.OK || !strings.HasPrefix(resp.Out, "woke the settled: #7") {
		t.Errorf("issue 7 from the shell on the settled Full thread = %+v", resp)
	}
}
