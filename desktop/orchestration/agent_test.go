package orchestration_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/threads"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/thread"
)

const badInput = "The tool's input is not what its schema asks for"

// noteTexts are the texts of the project's notes.
func noteTexts(t *testing.T, h *apptest.Harness, key string) []string {
	t.Helper()
	list, err := h.Notes().List(key)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, n := range list {
		texts = append(texts, n.Text)
	}
	slices.Sort(texts)
	return texts
}

// threadIn starts a Supervised thread in the project of that name, and makes main the current project again.
func threadIn(t *testing.T, h *apptest.Harness, project, title string) threads.Thread {
	t.Helper()
	if _, err := h.SwitchProject(project); err != nil {
		t.Fatal(err)
	}
	s := newThread(t, h, title)
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}
	return s
}

// threadTitled is the row of the thread with that title.
func threadTitled(h *apptest.Harness, title string) (threads.Thread, bool) {
	list := h.Threads().List()
	i := slices.IndexFunc(list, func(s threads.Thread) bool { return s.Title == title })
	if i < 0 {
		return threads.Thread{}, false
	}
	return list[i], true
}

func TestNoteToolActsAsTheCaller(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a := newThread(t, h, "a")
	name, key := otherProject(t, h, t.TempDir())
	b := threadIn(t, h, name, "b")

	tests := []struct {
		caller, text string
	}{
		{a.ID, "the build is slow"},
		{b.ID, "the docs are old"},
	}
	for _, tt := range tests {
		resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, tt.caller), Args: []string{"note"}, Opts: map[string]string{"input": `{"text":"` + tt.text + `"}`}})
		if !resp.OK || resp.Out != "note saved: "+tt.text {
			t.Errorf("note as %s = %+v", tt.caller, resp)
		}
	}
	if got := noteTexts(t, h, "main"); !slices.Equal(got, []string{"the build is slow"}) {
		t.Errorf("main's notes = %q, want only A's", got)
	}
	if got := noteTexts(t, h, key); !slices.Equal(got, []string{"the docs are old"}) {
		t.Errorf("%s's notes = %q, want only B's", key, got)
	}
	if err := useTool(t, h, a.ID, "note", `{"text":"  "}`, nil); err != "The note is empty" {
		t.Errorf("an empty note: %q", err)
	}
}

func TestToolIgnoresAThreadInItsInput(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a := newThread(t, h, "a")
	name, key := otherProject(t, h, t.TempDir())
	b := threadIn(t, h, name, "b")

	// The request's own fields name B, as a forged call would.
	asB := func(tool, input string) ctl.Response {
		return h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, a.ID), Thread: b.ID, Project: key, Args: []string{tool},
			Opts: map[string]string{"input": input, "thread": b.ID, "project": key}})
	}
	if resp := asB("note", `{"text":"from a"}`); !resp.OK {
		t.Fatalf("note = %+v", resp)
	}
	if resp := asB("track", `{"branch":"feature-a"}`); !resp.OK || resp.Out != "tracking feature-a" {
		t.Fatalf("track = %+v", resp)
	}
	if resp := asB("show", `{"text":"done"}`); !resp.OK {
		t.Fatalf("show = %+v", resp)
	}
	if got := noteTexts(t, h, "main"); !slices.Equal(got, []string{"from a"}) {
		t.Errorf("main's notes = %q, want A's", got)
	}
	if got := noteTexts(t, h, key); len(got) != 0 {
		t.Errorf("B's project got the notes %q", got)
	}
	apptest.Eventually(t, "A on feature-a", func() bool { return row(t, h, a.ID).Branch == "feature-a" })
	if got := row(t, h, b.ID).Branch; got != "" {
		t.Errorf("B's branch = %q, want none", got)
	}
	if len(h.Evidence(a.ID)) != 1 || len(h.Evidence(b.ID)) != 0 {
		t.Errorf("evidence: A %+v, B %+v; want one for A", h.Evidence(a.ID), h.Evidence(b.ID))
	}

	// A field in the input that names a thread or a project is refused, never followed.
	for _, input := range []string{
		`{"text":"x","thread":"` + b.ID + `"}`,
		`{"text":"x","thread":"` + b.ID + `"}`,
		`{"text":"x","project":"` + key + `"}`,
	} {
		if err := useTool(t, h, a.ID, "note", input, nil); !strings.HasPrefix(err, badInput) {
			t.Errorf("note %s: %q, want it refused", input, err)
		}
	}
	if got := noteTexts(t, h, key); len(got) != 0 {
		t.Errorf("B's project got the notes %q", got)
	}
}

func TestNewThreadToolRefusesMoreAccessThanTheCaller(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		caller thread.Access
		asked  string
		want   thread.Access
		err    string
	}{
		{"supervised asks full", thread.Supervised, "full", "", "A thread with supervised access cannot start a thread with full access"},
		{"read asks supervised", thread.Read, "supervised", "", "A thread with read access cannot start a thread with supervised access"},
		{"read asks full", thread.Read, "full", "", "A thread with read access cannot start a thread with full access"},
		{"read gets read by default", thread.Read, "", thread.Read, ""},
		{"supervised asks read", thread.Supervised, "read", thread.Read, ""},
		{"supervised gets supervised by default", thread.Supervised, "", thread.Supervised, ""},
		{"full asks full", thread.Full, "full", thread.Full, ""},
		{"an access orchestrator does not know", thread.Full, "write", "", `access must be read, supervised or full, not "write"`},
	}
	h := apptest.New(t)
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := threadWith(t, h, fmt.Sprintf("caller %d", i), tt.caller)
			title := fmt.Sprintf("started %d", i)
			input := `{"title":"` + title + `","prompt":"read the code"`
			if tt.asked != "" {
				input += `,"access":"` + tt.asked + `"`
			}
			err := useTool(t, h, caller.ID, "new_thread", input+"}", nil)
			got, started := threadTitled(h, title)
			if tt.err != "" {
				if err != tt.err || started {
					t.Errorf("new_thread = %q, started %v; want %q and no thread", err, started, tt.err)
				}
				return
			}
			if err != "" || !started || got.Access != tt.want || got.Project != "main" {
				t.Errorf("new_thread = %q, started %+v; want a %s thread in main", err, got, tt.want)
			}
		})
	}
	caller := newThread(t, h, "asker")
	for _, input := range []string{`{"title":"x"}`, `{"title":" ","prompt":"p"}`, `{"prompt":"p"}`} {
		if err := useTool(t, h, caller.ID, "new_thread", input, nil); err == "" {
			t.Errorf("new_thread %s started a thread", input)
		}
	}
}

func TestIssueToolStartsTheIssueThread(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	caller := threadWith(t, h, "plan", thread.Read)
	issueThread := func() (threads.Thread, bool) {
		list := h.Threads().List()
		i := slices.IndexFunc(list, func(s threads.Thread) bool { return s.Issue == 7 })
		if i < 0 {
			return threads.Thread{}, false
		}
		return list[i], true
	}

	for _, tt := range []struct{ input, err string }{
		{`{"number":0}`, "number is an issue's number, 1 or more"},
		{`{"prompt":"x"}`, "number is an issue's number, 1 or more"},
		{`{"number":7,"prompt":"` + strings.Repeat("x", 6<<10+1) + `"}`, "The prompt is 6145 bytes, more than the 6144 a thread takes: put the long parts in a file and give its path"},
		{`{"number":"7"}`, badInput},
	} {
		if err := useTool(t, h, caller.ID, "issue", tt.input, nil); !strings.HasPrefix(err, tt.err) {
			t.Errorf("issue %.40s: %q, want %q", tt.input, err, tt.err)
		}
	}
	if s, ok := issueThread(); ok {
		t.Fatalf("a refused call started %+v", s)
	}

	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, caller.ID), Args: []string{"issue"}, Opts: map[string]string{"input": `{"number":7,"prompt":"the patch is in /tmp/x"}`}})
	s, ok := issueThread()
	if !resp.OK || !ok || resp.Out != fmt.Sprintf("started 1 thread: #7 (%d)", s.N) {
		t.Fatalf("issue 7 = %+v, thread %+v", resp, s)
	}
	if s.Access != thread.Read {
		t.Errorf("the issue thread's access = %q, want Read, the caller's, since the action's is Supervised", s.Access)
	}
	if got := systemPrompt(t, h, s.ID); !strings.Contains(got, prompts.Handover("the patch is in /tmp/x")) {
		t.Errorf("the issue thread's system prompt lacks the handover:\n%s", got)
	}
	again := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, caller.ID), Args: []string{"issue"}, Opts: map[string]string{"input": `{"number":7,"prompt":"more"}`}})
	if want := fmt.Sprintf("already running: #7 (%d, the prompt was not given to it)", s.N); !again.OK || again.Out != want {
		t.Errorf("issue 7 again = %+v, want %q", again, want)
	}
}

func TestIssueToolGivesTheLowerOfTheActionsAndTheCallersAccess(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	caller := threadWith(t, h, "boss", thread.Full)
	mustUse(t, h, caller.ID, "issue", `{"number":7}`, nil)
	list := h.Threads().List()
	i := slices.IndexFunc(list, func(s threads.Thread) bool { return s.Issue == 7 })
	if i < 0 {
		t.Fatal("no thread for #7")
	}
	if list[i].Access != thread.Supervised {
		t.Errorf("a Full caller's issue thread has %q access, want Supervised, the action's", list[i].Access)
	}
}

func TestTrackToolTakesTheBranchForTheCaller(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a, b := newThread(t, h, "a"), newThread(t, h, "b")
	reader := threadWith(t, h, "reader", thread.Read)

	mustUse(t, h, a.ID, "track", `{"branch":" feature-a "}`, nil)
	apptest.Eventually(t, "A on feature-a", func() bool { return row(t, h, a.ID).Branch == "feature-a" })
	if got := row(t, h, b.ID).Branch; got != "" {
		t.Errorf("B's branch = %q, want none", got)
	}
	for _, tt := range []struct{ caller, input, err string }{
		{b.ID, `{"branch":"feature-a"}`, fmt.Sprintf("Thread %d works on feature-a", row(t, h, a.ID).N)},
		{reader.ID, `{"branch":"feature-r"}`, "A Read thread works on no branch"},
		{a.ID, `{"branch":""}`, "track needs a branch"},
		{a.ID, `{}`, "track needs a branch"},
	} {
		if err := useTool(t, h, tt.caller, "track", tt.input, nil); err != tt.err {
			t.Errorf("track %s as %s: %q, want %q", tt.input, tt.caller, err, tt.err)
		}
	}
}

func TestShowToolTakesFilesOnlyFromTheThreadsFolders(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a := newThread(t, h, "a")
	roots := h.Threads().ShowRoots(a.ID)
	if len(roots) != 3 {
		t.Fatalf("roots = %q, want the thread's folder, the project's and the temp folder", roots)
	}
	write := func(dir, name string) string {
		t.Helper()
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, name)
		if err := os.WriteFile(file, []byte("inside "+name), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	outside := t.TempDir()
	secret := write(outside, "id_rsa")
	link := filepath.Join(roots[0], "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	fileLink := filepath.Join(roots[0], "key.txt")
	if err := os.Symlink(secret, fileLink); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"thread.txt", "project.txt", "temp.txt"} {
		mustUse(t, h, a.ID, "show", `{"file":"`+write(roots[i], name)+`"}`, nil)
	}
	if n := len(h.Evidence(a.ID)); n != 3 {
		t.Fatalf("%d pieces of evidence after the three allowed files, want 3", n)
	}
	refusal := "is not in the thread's folder, the project folder or the thread's temp folder, so it cannot be shown"
	for name, file := range map[string]string{
		"a file outside":               secret,
		"a path with ..":               filepath.Join(roots[0], "..", filepath.Base(outside), "id_rsa"),
		"a path that climbs out":       roots[0] + "/../../../../../../etc/hosts",
		"a folder link that leads out": filepath.Join(link, "id_rsa"),
		"a file link that leads out":   fileLink,
		"a sibling of the folder":      roots[0] + "-other/x.txt",
		"a file that is not there":     filepath.Join(outside, "gone.txt"),
	} {
		if err := useTool(t, h, a.ID, "show", `{"file":"`+file+`"}`, nil); !strings.Contains(err, refusal) {
			t.Errorf("show %s: %q, want a refusal", name, err)
		}
	}
	if n := len(h.Evidence(a.ID)); n != 3 {
		t.Errorf("%d pieces of evidence after the refused calls, want 3", n)
	}
}

func TestShowToolFilesEvidenceForTheCaller(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a, b := newThread(t, h, "a"), newThread(t, h, "b")
	file := filepath.Join(h.Threads().ShowRoots(a.ID)[0], "result.txt")
	if err := os.WriteFile(file, []byte("all 12 tests pass\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	mustUse(t, h, a.ID, "show", `{"text":"the build passed","caption":"the build"}`, nil)
	mustUse(t, h, a.ID, "show", `{"file":"`+file+`"}`, nil)
	items := h.Evidence(a.ID)
	if len(items) != 2 || len(h.Evidence(b.ID)) != 0 {
		t.Fatalf("evidence: A %+v, B %+v; want two for A", items, h.Evidence(b.ID))
	}
	texts := map[string]string{}
	for _, it := range items {
		if it.Source != "agent" || it.Kind != "text" {
			t.Errorf("evidence %+v, want a text card from the agent", it)
		}
		texts[it.Caption] = it.Text
	}
	if texts["the build"] != "the build passed" || !strings.Contains(texts["result.txt"], "all 12 tests pass") {
		t.Errorf("evidence by caption = %q", texts)
	}

	for _, tt := range []struct{ input, err string }{
		{`{"file":"result.txt"}`, "result.txt is not an absolute path"},
		{`{"file":"` + file + `","text":"both"}`, "Give file or text, not both"},
		{`{"caption":"none"}`, "Give file or text, not both"},
		{`{"file":"` + filepath.Join(filepath.Dir(file), "gone.png") + `"}`, "is not a file"},
	} {
		if err := useTool(t, h, a.ID, "show", tt.input, nil); !strings.Contains(err, tt.err) {
			t.Errorf("show %s: %q, want %q", tt.input, err, tt.err)
		}
	}
	if n := len(h.Evidence(a.ID)); n != 2 {
		t.Errorf("A has %d pieces of evidence after the refused calls, want 2", n)
	}
}

// pageServer serves a page titled Demo on a port of the test range.
func pageServer(t *testing.T) string {
	t.Helper()
	for port := 18100; port < 18200; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `<!doctype html><title>Demo</title><h1>Demo</h1>`)
		})}
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(func() { _ = srv.Close() })
		return "http://" + ln.Addr().String() + "/"
	}
	t.Fatal("no free port in 18100-18199")
	return ""
}

func TestBrowserToolOpensTheCallersWindow(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a, b := newThread(t, h, "a"), newThread(t, h, "b")
	for _, tt := range []struct{ input, err string }{
		{`{"action":"open"}`, "open needs a url"},
		{`{"action":"help"}`, `action is open, screenshot or close, not "help"`},
		{`{}`, `action is open, screenshot or close, not ""`},
		{`{"action":"open","url":"x","thread":"` + b.ID + `"}`, badInput},
	} {
		if err := useTool(t, h, a.ID, "browser", tt.input, nil); !strings.HasPrefix(err, tt.err) {
			t.Errorf("browser %s: %q, want %q", tt.input, err, tt.err)
		}
	}
	if !h.App.Browsers().Available() {
		t.Skip("no Chrome for Testing installed")
	}

	url := pageServer(t)
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, a.ID), Thread: b.ID, Args: []string{"browser"}, Opts: map[string]string{"input": `{"action":"open","url":"` + url + `"}`}, TimeoutMs: 75_000})
	if !resp.OK || !strings.Contains(resp.Out, "Demo") {
		t.Fatalf("open = %+v", resp)
	}
	if !row(t, h, a.ID).Browser || row(t, h, b.ID).Browser || h.BrowserState(b.ID).Open {
		t.Errorf("A's browser %v, B's %v; want only A's window", row(t, h, a.ID).Browser, row(t, h, b.ID).Browser)
	}
	resp = h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, a.ID), Args: []string{"browser"}, Opts: map[string]string{"input": `{"action":"screenshot","caption":"the demo"}`}, TimeoutMs: 75_000})
	if items := h.Evidence(a.ID); !resp.OK || len(items) != 1 || items[0].Kind != "image" || items[0].Caption != "the demo" || len(h.Evidence(b.ID)) != 0 {
		t.Errorf("screenshot = %+v, A's evidence %+v", resp, items)
	}
	mustUse(t, h, a.ID, "browser", `{"action":"close"}`, nil)
	apptest.Eventually(t, "A's window closed", func() bool { return !row(t, h, a.ID).Browser })
}

func TestRefreshToolReadsTheQueueAgain(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a := threadWith(t, h, "reader", thread.Read)
	before := h.GH.IssueCalls()
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, a.ID), Args: []string{"refresh"}, Opts: map[string]string{"input": `{}`}})
	if !resp.OK || !strings.HasPrefix(resp.Out, "refreshed ") {
		t.Fatalf("refresh = %+v", resp)
	}
	if h.GH.IssueCalls() <= before {
		t.Error("refresh did not list the issues again")
	}
	if err := useTool(t, h, a.ID, "refresh", `{"all":true}`, nil); !strings.HasPrefix(err, badInput) {
		t.Errorf("refresh with a field: %q", err)
	}
}

func TestMCPServerServesTheAgentToolsAsTheThread(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	a := threadWith(t, h, "reader", thread.Read)
	data, err := os.ReadFile(configOf(t, h, a.ID))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MCPServers map[string]struct {
			Command string
			Args    []string
			Env     map[string]string
		}
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	server := config.MCPServers["orchestrator"]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	// The server runs with Claude's environment, which names the app's socket, and the config's, which holds the token.
	// A thread and a project set there name another thread; the call still acts as the token's.
	cmd.Env = append(slices.Clone(h.Env), "ORCHESTRATOR_SESSION=main/bbbbbbbb", "ORCHESTRATOR_PROJECT=elsewhere")
	for k, v := range server.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"note","arguments":{"text":"the build is slow"}}}`,
	} {
		if _, err := fmt.Fprintln(stdin, line); err != nil {
			t.Fatal(err)
		}
	}
	answers := map[int]json.RawMessage{}
	r := bufio.NewReader(stdout)
	for len(answers) < 2 {
		line, err := r.ReadBytes('\n')
		if err != nil {
			t.Fatalf("reading the server: %v", err)
		}
		var resp struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Fatalf("answer %s: %v", line, err)
		}
		answers[resp.ID] = resp.Result
	}
	stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Errorf("the server exited with %v", err)
	}

	var list struct {
		Tools []struct{ Name string }
	}
	if err := json.Unmarshal(answers[1], &list); err != nil || len(list.Tools) != 13 {
		t.Errorf("tools/list = %s, %v; want the thirteen tools", answers[1], err)
	}
	var call struct {
		Content []struct{ Text string }
		IsError bool
	}
	if err := json.Unmarshal(answers[2], &call); err != nil || call.IsError || len(call.Content) != 1 || call.Content[0].Text != "note saved: the build is slow" {
		t.Errorf("note = %s, %v", answers[2], err)
	}
	if got := noteTexts(t, h, "main"); !slices.Equal(got, []string{"the build is slow"}) {
		t.Errorf("main's notes = %q, want the caller's note", got)
	}
}
