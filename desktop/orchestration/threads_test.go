package orchestration_test

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	ctl "github.com/nednella/agentos/internal/control"
	"github.com/nednella/agentos/internal/session"
)

// useTool calls the orchestrator tool over the control socket as the thread whose token it sends, and reads its answer
// into out. It returns the error the tool gave, "" for none.
func useTool(t *testing.T, h *apptest.Harness, caller, tool, input string, out any) string {
	t.Helper()
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, caller), Args: []string{tool}, Opts: map[string]string{"input": input}})
	if !resp.OK {
		return resp.Error
	}
	if out != nil {
		if err := json.Unmarshal([]byte(resp.Out), out); err != nil {
			t.Fatalf("%s answered %q: %v", tool, resp.Out, err)
		}
	}
	return ""
}

// mustUse is useTool for a call that must succeed.
func mustUse(t *testing.T, h *apptest.Harness, caller, tool, input string, out any) {
	t.Helper()
	if err := useTool(t, h, caller, tool, input, out); err != "" {
		t.Fatalf("%s %s: %s", tool, input, err)
	}
}

// listedThread is one thread as the threads tool lists it.
type listedThread struct {
	ID, Title, Project, State, Access, Parent, Branch string
	N, Issue, PR                                      int
	You                                               bool
}

func listThreads(t *testing.T, h *apptest.Harness, caller, input string) map[string]listedThread {
	t.Helper()
	var out struct{ Threads []listedThread }
	mustUse(t, h, caller, "threads", input, &out)
	byID := map[string]listedThread{}
	for _, l := range out.Threads {
		if _, twice := byID[l.ID]; twice {
			t.Errorf("thread %s is listed twice", l.ID)
		}
		byID[l.ID] = l
	}
	return byID
}

// family is a harness with threads of each kind the threads tool lists or leaves out: in main the caller, a Read
// thread, the caller's child, a settled thread and a deleted one, and one thread in a second project.
type family struct {
	h                                       *apptest.Harness
	caller, reader, child, settled, deleted string
	other, otherName, otherKey              string
}

func newFamily(t *testing.T) family {
	t.Helper()
	h := apptest.New(t)
	f := family{h: h}
	f.caller = threadWith(t, h, "Plan the release", session.Supervised).ID
	f.reader = threadWith(t, h, "Read the code", session.Read).ID
	f.child = mustDelegate(t, h, f.caller, `{"task":"count the files","access":"read"}`).ID
	// The child answers, its result goes to the caller, which answers that, and the child goes to sleep.
	quiet(t, h, f.child)
	apptest.Eventually(t, "the caller idle", func() bool { return row(t, h, f.caller).State == session.Idle })
	f.settled = threadWith(t, h, "Done already", session.Supervised).ID
	if err := h.SettleThread(f.settled); err != nil {
		t.Fatal(err)
	}
	f.deleted = threadWith(t, h, "Gone", session.Supervised).ID
	if err := h.DeleteThread(f.deleted, false); err != nil {
		t.Fatal(err)
	}
	f.otherName, f.otherKey = otherProject(t, h, t.TempDir())
	if _, err := h.SwitchProject(f.otherName); err != nil {
		t.Fatal(err)
	}
	f.other = threadWith(t, h, "Elsewhere", session.Read).ID
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestThreadsListsTheProject(t *testing.T) {
	f := newFamily(t)
	for _, input := range []string{`{}`, `{"project":"MAIN"}`, `{"project":"main"}`} {
		t.Run(input, func(t *testing.T) {
			got := listThreads(t, f.h, f.caller, input)
			want := []string{f.caller, f.reader, f.child, f.settled}
			slices.Sort(want)
			if ids := slices.Sorted(maps.Keys(got)); !slices.Equal(ids, want) {
				t.Fatalf("listed %q; want the caller, the Read thread, the child and the settled thread, and no deleted one or other project's", ids)
			}
			rows := map[string]listedThread{
				f.caller:  {ID: f.caller, Title: "Plan the release", Project: "main", State: "idle", Access: "supervised", You: true},
				f.reader:  {ID: f.reader, Title: "Read the code", Project: "main", State: "idle", Access: "read"},
				f.child:   {ID: f.child, Title: "count the files", Project: "main", State: "settled", Access: "read", Parent: f.caller},
				f.settled: {ID: f.settled, Title: "Done already", Project: "main", State: "settled", Access: "supervised"},
			}
			for id, w := range rows {
				l := got[id]
				w.N = row(t, f.h, id).N
				if l != w {
					t.Errorf("%s listed as %+v, want %+v", id, l, w)
				}
			}
		})
	}
	if err := useTool(t, f.h, f.caller, "threads", `{"project":"nowhere"}`, nil); !strings.HasPrefix(err, `No project "nowhere": the projects are `) || !strings.Contains(err, f.otherName) {
		t.Errorf("an unknown project: %q", err)
	}
}

func TestThreadsAllListsEveryProject(t *testing.T) {
	f := newFamily(t)
	got := listThreads(t, f.h, f.reader, `{"project":"all"}`)
	for _, id := range []string{f.caller, f.reader, f.child, f.settled, f.other} {
		if _, ok := got[id]; !ok {
			t.Errorf("%s is not listed", id)
		}
	}
	if _, ok := got[f.deleted]; ok {
		t.Error("the deleted thread is listed")
	}
	if l := got[f.other]; l.Project != f.otherKey || l.Access != "read" {
		t.Errorf("the other project's thread: %+v", l)
	}
	for id, l := range got {
		if l.You != (id == f.reader) {
			t.Errorf("%s has you = %v; only the caller %s has it", id, l.You, f.reader)
		}
	}
	if only := listThreads(t, f.h, f.caller, `{"project":"`+f.otherKey+`"}`); len(only) != 1 || only[f.other].ID != f.other {
		t.Errorf("the other project by key lists %+v, want only %s", only, f.other)
	}
}
