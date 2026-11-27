package threads_test

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/threads"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/thread"
)

func TestThreadCommands(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	base, err := h.StartThread("base", "")
	if err != nil {
		t.Fatal(err)
	}
	asBase := func(req ctl.Request) ctl.Request { req.Thread = base.ID; return req }

	if resp := h.AskTool(t, asBase(ctl.Request{Cmd: "new", Args: []string{"fix", "login"}})); !resp.OK || resp.Out != "started thread 2: fix login" {
		t.Fatalf("new = %+v", resp)
	}
	if resp := h.AskTool(t, asBase(ctl.Request{Cmd: "new", Args: []string{"fix logout"}})); !resp.OK || resp.Out != "started thread 3: fix logout" {
		t.Fatalf("second new = %+v", resp)
	}
	numbered := func(n int) threads.Thread {
		list := h.Threads().List()
		return list[slices.IndexFunc(list, func(s threads.Thread) bool { return s.N == n })]
	}

	if resp := h.AskTool(t, asBase(ctl.Request{Cmd: "new", Args: []string{"handed"}, Opts: map[string]string{"prompt": "what the side thread found"}})); !resp.OK || resp.Out != "started thread 4: handed" {
		t.Fatalf("new with a prompt = %+v", resp)
	}
	fourth := numbered(4)
	eventually(t, "the handover in the new thread", func() bool {
		pane := sentTo(t, h, fourth.ID)
		return strings.Contains(pane, "APPLICATION_PROMPT: Another session asked for this task") && strings.Contains(pane, "what the side thread found")
	})
	if fourth.Access != thread.Supervised {
		t.Errorf("new without access started %+v", fourth)
	}

	req := ctl.Request{Cmd: "new", Thread: fourth.ID, Args: []string{"ask"}, Opts: map[string]string{"prompt": "what the thread should look at", "access": "read"}}
	if resp := h.AskTool(t, req); !resp.OK || resp.Out != "started thread 5: ask" {
		t.Fatalf("new with access read from a thread = %+v", resp)
	}
	fifth := numbered(5)
	if fifth.Access != thread.Read {
		t.Errorf("new with access read started %+v", fifth)
	}
	eventually(t, "the handover in the new Read thread", func() bool {
		pane := sentTo(t, h, fifth.ID)
		return strings.Contains(pane, "APPLICATION_PROMPT: Another session asked") && strings.Contains(pane, "what the thread should look at")
	})
	if resp := h.AskTool(t, asBase(ctl.Request{Cmd: "new", Args: []string{"bad"}, Opts: map[string]string{"access": "write"}})); resp.OK {
		t.Errorf("new with access write = %+v", resp)
	}
}

func TestCommandsWorkInTheCallersProject(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if _, err := h.AddProjectDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other := h.Snapshot().Project.Key
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}
	here, err := h.StartThread("here", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject(other); err != nil {
		t.Fatal(err)
	}
	there, err := h.StartThread("there", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for _, c := range []struct{ id, title string }{{here.ID, "from here"}, {there.ID, "from there"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if resp := h.AskTool(t, ctl.Request{Cmd: "new", Thread: c.id, Args: []string{c.title}}); !resp.OK {
				t.Errorf("new from %s = %+v", c.id, resp)
			}
		}()
	}
	wg.Wait()
	projectOf := map[string]string{}
	for _, s := range h.Threads().List() {
		projectOf[s.Title] = s.Project
	}
	if projectOf["from here"] != "main" || projectOf["from there"] != other {
		t.Errorf("project of each new thread = %v, want from here in main and from there in %q", projectOf, other)
	}
	if got := h.Snapshot().Project.Key; got != "main" {
		t.Errorf("the owner's project is now %q, want main", got)
	}
}

func TestCommandWithoutAKnownProjectIsRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	caller, err := h.StartThread("caller", "")
	if err != nil {
		t.Fatal(err)
	}
	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Thread: caller.ID, Project: "nowhere", Args: []string{"x"}}); resp.OK {
		t.Errorf("new in an unknown project = %+v", resp)
	}
}

func TestThreadsOfEveryProjectAreListed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if _, err := h.StartThread("here", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.AddProjectDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other := h.Snapshot().Project.Key
	if _, err := h.StartThread("there", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}

	byTitle := map[string]string{}
	var id string
	for _, s := range h.Snapshot().Threads {
		byTitle[s.Title] = s.Project
		if s.Title == "there" {
			id = s.ID
		}
	}
	if byTitle["here"] != "main" || byTitle["there"] != other {
		t.Fatalf("project of each thread = %v, want here in main and there in %q", byTitle, other)
	}

	h.Observe(t, id, "Notification", `{"notification_type":"permission_prompt","message":"ok?"}`)
	eventually(t, "a threads event for the other project", func() bool {
		for _, s := range h.Rec.LastThreadsOf(other) {
			if s.ID == id && s.State == "waiting" {
				return true
			}
		}
		return false
	})
	eventually(t, "attention for the other project's thread", func() bool { return h.Rec.CountAttentionFor(id, "waiting") == 1 })
}

func twoProjectsEachWithThreadOne(t *testing.T) (h *apptest.Harness, other string) {
	t.Helper()
	h = newHarness(t)
	if _, err := h.StartThread("mine", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.AddProjectDir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other = h.Snapshot().Project.Key
	if _, err := h.StartThread("there", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}
	return h, other
}

func TestIssueThreadsAreOfTheCurrentProject(t *testing.T) {
	t.Parallel()
	h, other := twoProjectsEachWithThreadOne(t)
	if _, err := h.Threads().Create(h.Threads().Current(), "a", "", false, 5, thread.Supervised); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject(other); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Threads().IssueThreads()[5]; ok {
		t.Errorf("issue 5 of the other project maps to %v", h.Threads().IssueThreads())
	}
}

func TestThreadCannotStartFullAccess(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	supervised, err := h.StartThread("supervised", "")
	if err != nil {
		t.Fatal(err)
	}
	read, err := h.NewThread("read", "", "", "", "read")
	if err != nil {
		t.Fatal(err)
	}
	full, err := h.NewThread("full", "", "", "", "full")
	if err != nil {
		t.Fatal(err)
	}
	for _, asker := range []string{supervised.ID, read.ID, "main/abcdefgh"} {
		resp := h.AskTool(t, ctl.Request{Cmd: "new", Thread: asker, Args: []string{"more"}, Opts: map[string]string{"access": "full"}})
		if resp.OK || resp.Error != "A thread that is not Full access cannot start one that is" {
			t.Errorf("new --access full from %s = %+v", asker, resp)
		}
	}
	for _, asker := range []string{"", "garbage"} {
		resp := h.AskTool(t, ctl.Request{Cmd: "new", Thread: asker, Args: []string{"more"}})
		if resp.OK || resp.Error != "no calling thread" {
			t.Errorf("new from %q = %+v, want no calling thread", asker, resp)
		}
	}
	if got := len(h.Threads().List()); got != 3 {
		t.Errorf("%d threads, want the three, none started by a refused call", got)
	}
	if resp := h.AskTool(t, ctl.Request{Cmd: "new", Thread: full.ID, Args: []string{"from full"}, Opts: map[string]string{"access": "full"}}); !resp.OK {
		t.Errorf("new --access full from a Full thread = %+v", resp)
	}
}
