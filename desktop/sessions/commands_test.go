package sessions_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/desktop/sessions"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/session"
)

func TestSessionCommands(t *testing.T) {
	h := newHarness(t)

	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Args: []string{"fix", "login"}}); !resp.OK || resp.Out != "started session 1: fix login" {
		t.Fatalf("new = %+v", resp)
	}
	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Args: []string{"fix logout"}}); !resp.OK || resp.Out != "started session 2: fix logout" {
		t.Fatalf("second new = %+v", resp)
	}

	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Args: []string{"side"}, Opts: map[string]string{"prompt": "context for the new session"}}); !resp.OK || resp.Out != "started session 3: side" {
		t.Fatalf("new with a prompt = %+v", resp)
	}
	list := h.Sessions().List()
	third := list[slices.IndexFunc(list, func(s sessions.Session) bool { return s.N == 3 })]
	eventually(t, "the prompt in the new session", func() bool { return strings.Contains(sentTo(t, h, third.ID), "context for the new session") })
	if strings.Contains(sentTo(t, h, third.ID), "APPLICATION_PROMPT") {
		t.Errorf("the owner's prompt is framed as the app's: %q", sentTo(t, h, third.ID))
	}

	req := ctl.Request{Cmd: "new", Session: third.ID, Args: []string{"handed"}, Opts: map[string]string{"prompt": "what the side session found"}}
	if resp := h.AskTool(t, req); !resp.OK || resp.Out != "started session 4: handed" {
		t.Fatalf("new from a session = %+v", resp)
	}
	list = h.Sessions().List()
	fourth := list[slices.IndexFunc(list, func(s sessions.Session) bool { return s.N == 4 })]
	eventually(t, "the handover in the new session", func() bool {
		pane := sentTo(t, h, fourth.ID)
		return strings.Contains(pane, "APPLICATION_PROMPT: Another session asked for this task") && strings.Contains(pane, "what the side session found")
	})
	if fourth.Access != session.Supervised {
		t.Errorf("new without --access started %+v", fourth)
	}

	req = ctl.Request{Cmd: "new", Session: third.ID, Args: []string{"ask"}, Opts: map[string]string{"prompt": "what the thread should look at", "access": "read"}}
	if resp := h.AskTool(t, req); !resp.OK || resp.Out != "started session 5: ask" {
		t.Fatalf("new --access read from a session = %+v", resp)
	}
	list = h.Sessions().List()
	fifth := list[slices.IndexFunc(list, func(s sessions.Session) bool { return s.N == 5 })]
	if fifth.Access != session.Read {
		t.Errorf("new --access read started %+v", fifth)
	}
	eventually(t, "the handover in the new Read thread", func() bool {
		pane := sentTo(t, h, fifth.ID)
		return strings.Contains(pane, "APPLICATION_PROMPT: Another session asked") && strings.Contains(pane, "what the thread should look at")
	})
	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Args: []string{"bad"}, Opts: map[string]string{"access": "write"}}); resp.OK {
		t.Errorf("new --access write = %+v", resp)
	}
}

func TestCommandsWorkInTheCallersProject(t *testing.T) {
	h := newHarness(t)
	root := t.TempDir()
	if _, err := h.AddProjectDir(root); err != nil {
		t.Fatal(err)
	}
	other := h.Snapshot().Project.Name
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}

	resp := h.AskTool(t, ctl.Request{Cmd: "new", Project: other, Args: []string{"elsewhere"}})
	if !resp.OK || h.Snapshot().Project.Name != other {
		t.Fatalf("new = %+v, project now %q", resp, h.Snapshot().Project.Name)
	}
	eventually(t, "the session in the other project", func() bool {
		for _, s := range h.Sessions().List() {
			if s.Title == "elsewhere" {
				return true
			}
		}
		return false
	})
}

func TestSessionsOfEveryProjectAreListed(t *testing.T) {
	h := newHarness(t)
	if _, err := h.NewSession("here", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.AddProjectDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other := h.Snapshot().Project.Key
	if _, err := h.NewSession("there", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}

	byTitle := map[string]string{}
	var id string
	for _, s := range h.Snapshot().Sessions {
		byTitle[s.Title] = s.Project
		if s.Title == "there" {
			id = s.ID
		}
	}
	if byTitle["here"] != "main" || byTitle["there"] != other {
		t.Fatalf("project of each session = %v, want here in main and there in %q", byTitle, other)
	}

	h.Hook(t, id, "Notification", `{"notification_type":"permission_prompt","message":"ok?"}`)
	eventually(t, "a sessions event for the other project", func() bool {
		for _, s := range h.Rec.LastSessionsOf(other) {
			if s.ID == id && s.State == "waiting" {
				return true
			}
		}
		return false
	})
	eventually(t, "attention for the other project's session", func() bool { return h.Rec.CountAttentionFor(id, "waiting") == 1 })
}

func twoProjectsEachWithSessionOne(t *testing.T) (h *apptest.Harness, other string) {
	t.Helper()
	h = newHarness(t)
	if _, err := h.NewSession("mine", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.AddProjectDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other = h.Snapshot().Project.Key
	if _, err := h.NewSession("there", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}
	return h, other
}

func TestIssueSessionsAreOfTheCurrentProject(t *testing.T) {
	h, other := twoProjectsEachWithSessionOne(t)
	if _, err := h.Sessions().Create("a", "", false, 5, session.Supervised); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject(other); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Sessions().IssueSessions()[5]; ok {
		t.Errorf("issue 5 of the other project maps to %v", h.Sessions().IssueSessions())
	}
}

func TestThreadCannotStartFullAccess(t *testing.T) {
	h := newHarness(t)
	supervised, err := h.NewSession("supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	read, err := h.NewThread("read", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	for _, asker := range []string{supervised.ID, read.ID, "garbage", "main/abcdefgh"} {
		resp := h.AskTool(t, ctl.Request{Cmd: "new", Session: asker, Args: []string{"more"}, Opts: map[string]string{"access": "full"}})
		if resp.OK || resp.Error != "A thread that is not Full access cannot start one that is" {
			t.Errorf("new --access full from %s = %+v", asker, resp)
		}
	}
	if got := len(h.Sessions().List()); got != 2 {
		t.Errorf("%d threads, want the two, none started Full", got)
	}

	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Args: []string{"full"}, Opts: map[string]string{"access": "full"}}); !resp.OK {
		t.Fatalf("new --access full from the shell = %+v", resp)
	}
	list := h.Sessions().List()
	full := list[slices.IndexFunc(list, func(s sessions.Session) bool { return s.Title == "full" })]
	if full.Access != session.Full {
		t.Errorf("new --access full started %+v", full)
	}
	eventually(t, "claude without permission prompts", func() bool {
		return strings.Contains(strings.Join(h.Args(t, full.ID), " "), "--permission-mode bypassPermissions")
	})
	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Session: full.ID, Args: []string{"from full"}, Opts: map[string]string{"access": "full"}}); !resp.OK {
		t.Errorf("new --access full from a Full thread = %+v", resp)
	}
}
