package issues_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/session"
)

func TestIssueCommand(t *testing.T) {
	t.Parallel()
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
		{"none", nil, "issue needs a number", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := h.AskTool(t, ctl.Request{Cmd: "issue", Args: tt.args})
			if resp.OK == tt.fail || !strings.Contains(resp.Out+resp.Error, tt.want) {
				t.Errorf("issue %v = %+v, want %q", tt.args, resp, tt.want)
			}
		})
	}
}

func TestIssueCommandHandsThePromptOver(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	resp := h.AskTool(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "the patch is in /tmp/x"}})
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

	resp = h.AskTool(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "more"}})
	if !resp.OK || !strings.Contains(resp.Out, "already running: #7 (1, the prompt was not given to it)") {
		t.Errorf("issue 7 again = %+v", resp)
	}
}

func TestWokenThreadGetsTheHandoverAgain(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{ProjectExtra: "    pr_review_command: \"/address-review {n}\"\n"})
	gitRepo(t, h)
	if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "the patch is in /tmp/x"}}); !resp.OK {
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
	t.Parallel()
	h := apptest.New(t)
	if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "the patch is in /tmp/x"}}); !resp.OK {
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

	resp := h.AskTool(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": "now see /tmp/y"}})
	if !resp.OK || resp.Out != "woke the sleeping: #7 (1, with the prompt)" {
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
	t.Parallel()
	h := apptest.New(t)
	handover := strings.Repeat(`it's "quoted" $HOME \ `, 6<<10)[:6<<10] // the most orchestrator issue --prompt takes
	if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Args: []string{"7"}, Opts: map[string]string{"prompt": handover}}); !resp.OK {
		t.Fatalf("issue 7 with the longest prompt = %+v", resp)
	}
	if args := argsOf(t, h.ClaudeDir(), h.Sessions().List()[0].ID); !strings.Contains(args, strings.TrimSpace(handover)) {
		t.Error("the system prompt lacks the handover")
	}
}

func TestRefreshCommand(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if _, err := h.Issues(false); err != nil {
		t.Fatal(err)
	}
	resp := h.AskTool(t, ctl.Request{Cmd: "refresh"})
	if !resp.OK || resp.Out != "refreshed 6 issues and the pull requests" || h.GH.IssueCalls() != 2 {
		t.Errorf("refresh = %+v, gh issue list ran %d times", resp, h.GH.IssueCalls())
	}
}

const fullActionQueue = `    queue_sections:
      - name: Ready
        labels: [ready]
        actions:
          - {name: Work, command: "/ship {n}", access: full}
      - name: Inbox
        actions:
          - {name: Investigate, command: "/investigate {n}"}
`

func TestIssueCommandGivesTheLowerOfTheActionsAndTheThreadsAccess(t *testing.T) {
	t.Parallel()
	tests := []struct {
		asker         session.Access
		full, regular session.Access // what issue 7, whose action is Full, and issue 11, whose action is Supervised, get
	}{
		{session.Read, session.Read, session.Read},
		{session.Supervised, session.Supervised, session.Supervised},
		{session.Full, session.Full, session.Supervised},
	}
	for _, tt := range tests {
		t.Run(string(tt.asker), func(t *testing.T) {
			h := apptest.NewWith(t, apptest.Options{Queue: fullActionQueue})
			asker, err := h.NewThread("asker", "", "", "", string(tt.asker))
			if err != nil {
				t.Fatal(err)
			}
			for issue, want := range map[string]session.Access{"7": tt.full, "11": tt.regular} {
				if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: asker.ID, Args: []string{issue}}); !resp.OK {
					t.Fatalf("issue %s from a %s thread = %+v", issue, tt.asker, resp)
				}
				n, _ := strconv.Atoi(issue)
				i := slices.IndexFunc(h.Sessions().List(), func(s sessions.Session) bool { return s.Issue == n })
				if i < 0 {
					t.Fatalf("issue %s started no thread", issue)
				}
				if got := h.Sessions().List()[i].Access; got != want {
					t.Errorf("issue %s from a %s thread: access %q, want %q", issue, tt.asker, got, want)
				}
			}
		})
	}
}

func TestIssueCommandRefusesAnAskerThatIsNotARunningThread(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{Queue: fullActionQueue})
	for from, asker := range map[string]string{"a session that is not a thread": "garbage", "a thread that is gone": "main/abcdefgh"} {
		for _, issue := range []string{"7", "11"} {
			resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: asker, Args: []string{issue}})
			if resp.OK || !strings.Contains(resp.Error, "Only a running thread can start an issue's thread") {
				t.Errorf("issue %s from %s = %+v", issue, from, resp)
			}
		}
	}
	if list := h.Sessions().List(); len(list) != 0 {
		t.Errorf("a refused asker started %+v", list)
	}
}

func TestIssueCommandRefusesWakingAFullThreadFromOneThatIsNot(t *testing.T) {
	t.Parallel()
	h := apptest.NewWith(t, apptest.Options{Queue: fullActionQueue})
	supervised, err := h.NewSession("supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	read, err := h.NewThread("read", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	askers := map[string]string{"a Supervised thread": supervised.ID, "a Read thread": read.ID}
	refused := func(what string) {
		t.Helper()
		for from, asker := range askers {
			resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: asker, Args: []string{"7"}})
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

	if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: read.ID, Args: []string{"11"}}); !resp.OK {
		t.Errorf("issue 11, a Supervised action, from a Read thread = %+v", resp)
	}

	owner, err := h.NewThread("full", "", "", "", "full")
	if err != nil {
		t.Fatal(err)
	}
	if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: owner.ID, Args: []string{"7"}}); !resp.OK {
		t.Fatalf("issue 7 from a Full thread = %+v", resp)
	}
	full, ok := issueThread()
	if !ok || full.Access != session.Full {
		t.Fatalf("issue 7 from a Full thread started %+v", full)
	}
	if err := h.SettleThread(full.ID); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the Full thread settled", func() bool { s, _ := h.Session(full.ID); return s.State == session.Settled })

	refused("with a settled Full thread")
	if s, _ := h.Session(full.ID); s.State != session.Settled {
		t.Errorf("a thread that is not Full woke the Full thread: %+v", s)
	}
	if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: owner.ID, Args: []string{"7"}}); !resp.OK || !strings.HasPrefix(resp.Out, "woke the sleeping: #7") {
		t.Errorf("issue 7 from a Full thread on the settled Full thread = %+v", resp)
	}
}

func TestIssueAndRefreshWorkInTheCallersProject(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if _, err := h.AddProjectDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other := h.Snapshot().Project.Key
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}
	here, err := h.NewSession("here", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject(other); err != nil {
		t.Fatal(err)
	}
	there, err := h.NewSession("there", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for _, c := range []struct {
		id, issue string
	}{{here.ID, "7"}, {there.ID, "8"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: c.id, Args: []string{c.issue}}); !resp.OK {
				t.Errorf("issue %s from %s = %+v", c.issue, c.id, resp)
			}
			if resp := h.AskTool(t, ctl.Request{Cmd: "refresh", Session: c.id}); !resp.OK {
				t.Errorf("refresh from %s = %+v", c.id, resp)
			}
		}()
	}
	wg.Wait()
	projectOf := map[int]string{}
	for _, s := range h.Sessions().List() {
		if s.Issue > 0 {
			projectOf[s.Issue] = s.Project
		}
	}
	if projectOf[7] != "main" || projectOf[8] != other {
		t.Errorf("project of each issue thread = %v, want #7 in main and #8 in %q", projectOf, other)
	}
	if got := h.Snapshot().Project.Key; got != "main" {
		t.Errorf("the owner's project is now %q, want main", got)
	}
}

func TestIssueCommandRefusesWakingAThreadWithMoreAccess(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	read, err := h.NewThread("read", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	supervised, err := h.NewSession("supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	asleep, err := h.StartIssue(11)
	if err != nil || asleep.Access != session.Supervised {
		t.Fatalf("issue 11 started %+v, %v", asleep, err)
	}
	if err := h.SettleThread(asleep.ID); err != nil {
		t.Fatal(err)
	}

	resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: read.ID, Args: []string{"11"}, Opts: map[string]string{"prompt": "do it my way"}})
	if resp.OK || !strings.Contains(resp.Error, "A thread with read access cannot wake a thread with supervised access") {
		t.Fatalf("issue 11 from a Read thread = %+v", resp)
	}
	if got, _ := h.Session(asleep.ID); got.State != "settled" {
		t.Errorf("the thread of #11 is %q after the refusal, want settled", got.State)
	}

	if resp := h.AskTool(t, ctl.Request{Cmd: "issue", Session: supervised.ID, Args: []string{"11"}}); !resp.OK || !strings.Contains(resp.Out, "woke the sleeping") {
		t.Errorf("issue 11 from a Supervised thread = %+v", resp)
	}
}
