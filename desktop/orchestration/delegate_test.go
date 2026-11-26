package orchestration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/chat"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/prompts"
	"github.com/nednella/orchestrator/internal/session"
)

// child is what delegate answers.
type child struct {
	ID, Title, Project, Access, Where, Dir, Branch string
	N                                              int
}

// delegateAs calls the delegate tool over the control socket as the thread whose token it sends, and returns the child
// it started, or the error the tool gave.
func delegateAs(t *testing.T, h *apptest.Harness, caller, input string) (child, string) {
	t.Helper()
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, caller), Args: []string{"delegate"}, Opts: map[string]string{"input": input}})
	if !resp.OK {
		return child{}, resp.Error
	}
	var c child
	if err := json.Unmarshal([]byte(resp.Out), &c); err != nil {
		t.Fatalf("delegate answered %q: %v", resp.Out, err)
	}
	return c, ""
}

// mustDelegate is delegateAs for a call that must start a child.
func mustDelegate(t *testing.T, h *apptest.Harness, caller, input string) child {
	t.Helper()
	c, err := delegateAs(t, h, caller, input)
	if err != "" {
		t.Fatalf("delegate %s: %s", input, err)
	}
	return c
}

// threadWith starts a thread with the access given and no prompt.
func threadWith(t *testing.T, h *apptest.Harness, title string, access session.Access) sessions.Session {
	t.Helper()
	s, err := h.Sessions().Create(h.Sessions().Current(), title, "", false, 0, access)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// row is the thread's row.
func row(t *testing.T, h *apptest.Harness, id string) sessions.Session {
	t.Helper()
	s, ok := h.Session(id)
	if !ok {
		t.Fatalf("no row for %s", id)
	}
	return s
}

// tmuxSessions are the names of the sessions the harness's tmux server runs.
func tmuxSessions(t *testing.T, h *apptest.Harness) []string {
	t.Helper()
	out, _ := exec.Command("tmux", "-L", h.Socket, "list-sessions", "-F", "#{session_name}").Output()
	return strings.Fields(string(out))
}

// panePath is the folder the thread's tmux session started in, with its links resolved.
func panePath(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-L", h.Socket, "display-message", "-p", "-t", "="+id+":", "#{session_path}").Output()
	if err != nil {
		t.Fatalf("reading the folder of %s: %v", id, err)
	}
	return realDir(t, strings.TrimSpace(string(out)))
}

func realDir(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// claudeArgs are the arguments the thread's stand-in claude started with, once it has.
func claudeArgs(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	var args []string
	apptest.Eventually(t, "claude started", func() bool { args = h.Args(t, id); return len(args) > 0 })
	return args
}

// systemPrompt is the thread's --append-system-prompt and the arguments after it.
func systemPrompt(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	args := claudeArgs(t, h, id)
	i := slices.Index(args, "--append-system-prompt")
	if i < 0 {
		t.Fatalf("args = %q", args)
	}
	return strings.Join(args[i+1:], "\n")
}

// permissionMode is the --permission-mode the thread's stand-in claude started with.
func permissionMode(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	args := claudeArgs(t, h, id)
	i := slices.Index(args, "--permission-mode")
	if i < 0 || i+1 >= len(args) {
		t.Fatalf("args = %q", args)
	}
	return args[i+1]
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repoFixture makes the project folder a git repository with a bare origin in the test's temp folder.
func repoFixture(t *testing.T, h *apptest.Harness) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, filepath.Dir(origin), "init", "--bare", origin)
	gitIn(t, h.Dir, "init")
	if err := os.WriteFile(filepath.Join(h.Dir, "README"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, h.Dir, "add", "README")
	gitIn(t, h.Dir, "commit", "-m", "first")
	gitIn(t, h.Dir, "remote", "add", "origin", origin)
	gitIn(t, h.Dir, "push", "-u", "origin", "main")
}

func TestDelegateStartsAChild(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release")
	if _, err := h.ChatSend(parent.ID, "the parent's own words"); err != nil {
		t.Fatal(err)
	}
	c := mustDelegate(t, h, parent.ID, `{"task":"Count the files in src\nand list them","title":"Count files","access":"read"}`)
	if c.Title != "Count files" || c.Access != "read" || c.Where != "folder" || c.Project != "main" || c.N == 0 || c.Dir == "" {
		t.Errorf("delegate answered %+v", c)
	}
	if r := row(t, h, c.ID); r.Parent != parent.ID || r.Access != session.Read || r.Title != "Count files" {
		t.Errorf("the child's row: parent %q, access %q, title %q; want %s, read, Count files", r.Parent, r.Access, r.Title, parent.ID)
	}
	if p := row(t, h, parent.ID); p.Parent != "" {
		t.Errorf("the parent's row has parent %q", p.Parent)
	}

	// The chat holds the task when delegate answers.
	events, err := h.ChatHistory(c.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	first := slices.IndexFunc(events, func(e chat.Event) bool { return e.Kind == chat.KindUser })
	if first < 0 || !strings.HasPrefix(events[first].Text, "APPLICATION_PROMPT: ") || !strings.Contains(events[first].Text, "Count the files in src\nand list them") {
		t.Fatalf("the child's chat = %+v, want its task as the first message", events)
	}
	var user string
	apptest.Eventually(t, "the child's first message", func() bool {
		i := slices.IndexFunc(h.Stdin(t, c.ID), func(l string) bool { return strings.Contains(l, `"type":"user"`) })
		if i >= 0 {
			user = h.Stdin(t, c.ID)[i]
		}
		return i >= 0
	})
	if !strings.Contains(user, "APPLICATION_PROMPT:") || !strings.Contains(user, "Count the files in src") || !strings.Contains(user, `Thread \"Plan the release\"`) {
		t.Errorf("the child's first user line = %s", user)
	}

	prompt := systemPrompt(t, h, c.ID)
	if !strings.Contains(prompt, prompts.Child("Plan the release")) {
		t.Errorf("the child's system prompt lacks child.md for its parent:\n%s", prompt)
	}
	if strings.Contains(prompt, "the parent's own words") || slices.Contains(claudeArgs(t, h, c.ID), "--resume") {
		t.Errorf("the child starts with its parent's chat:\n%s", prompt)
	}
	if strings.Contains(systemPrompt(t, h, parent.ID), "started this thread for one task") {
		t.Error("the parent's system prompt says it is a child")
	}
}

func TestDelegateAboveTheCallersRankIsRefused(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	supervised := newThread(t, h, "supervised")
	read := threadWith(t, h, "read", session.Read)
	before := tmuxSessions(t, h)
	for _, where := range []string{"", `"where":"folder",`, `"where":"worktree",`, `"where":"issue","issue":8,`} {
		for _, tt := range []struct {
			caller, asked, want string
		}{
			{supervised.ID, "full", "A thread with supervised access cannot start a child with full access"},
			{read.ID, "supervised", "A thread with read access cannot start a child with supervised access"},
			{read.ID, "full", "A thread with read access cannot start a child with full access"},
		} {
			if c, err := delegateAs(t, h, tt.caller, `{`+where+`"task":"do it","access":"`+tt.asked+`"}`); err != tt.want {
				t.Errorf("%s asking %s (%s): %+v, %q; want %q", tt.caller, tt.asked, where, c, err, tt.want)
			}
		}
	}
	if after := tmuxSessions(t, h); !slices.Equal(after, before) {
		t.Errorf("tmux sessions = %q, want %q: a refused delegate started one", after, before)
	}
	if n := len(h.Sessions().List()); n != 2 {
		t.Errorf("%d rows, want the two callers'", n)
	}
}

func TestDelegateDefaultsToTheCallersAccess(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	for _, tt := range []struct {
		access session.Access
		where  string
	}{{session.Read, "folder"}, {session.Supervised, "worktree"}, {session.Full, "worktree"}} {
		parent := threadWith(t, h, string(tt.access), tt.access)
		c := mustDelegate(t, h, parent.ID, `{"task":"look"}`)
		if c.Access != string(tt.access) || c.Where != tt.where || c.Title != "look" {
			t.Errorf("a %s caller's child: %+v; want access %s, where %s, title look", tt.access, c, tt.access, tt.where)
		}
		if r := row(t, h, c.ID); r.Access != tt.access || r.Parent != parent.ID {
			t.Errorf("a %s caller's child's row: access %q, parent %q", tt.access, r.Access, r.Parent)
		}
	}
}

func TestDelegateChecksItsInput(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "parent")
	for input, want := range map[string]string{
		`{"task":"  "}`:                       "A child needs a task",
		`{"task":"x","access":"root"}`:        `access must be read, supervised or full, not "root"`,
		`{"task":"x","where":"there"}`:        `where must be folder, worktree or issue, not "there"`,
		`{"task":"x","where":"issue"}`:        "Where issue needs the issue's number",
		`{"task":"x","issue":7}`:              "An issue number goes with where issue",
		`{"task":"x","project":"elsewhere"}`:  `No project "elsewhere": the projects are main`,
		`{"task":"x","model":"nope-model"}`:   "nope-model",
		`{"task":"x","colour":"red"}`:         `json: unknown field "colour"`,
		`{"task":"x","where":"folder","n":1}`: `json: unknown field "n"`,
		`["task"]`:                            "The tool's input is not what its schema asks for",
	} {
		_, err := delegateAs(t, h, parent.ID, input)
		if err == "" || !strings.Contains(err, want) {
			t.Errorf("delegate %s: %q, want an error with %q", input, err, want)
		}
	}
	if c := mustDelegate(t, h, parent.ID, `{"task":"x","project":"main","where":"folder"}`); c.Project != "main" {
		t.Errorf("a child in the caller's own project, named: %+v", c)
	}
}

func TestChildRunsInTheParentsFolder(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	parent := newThread(t, h, "Fix the login")
	if parent.Worktree == "" {
		t.Fatalf("the parent has no worktree: %+v", parent)
	}
	c := mustDelegate(t, h, parent.ID, `{"task":"read the login code","where":"folder"}`)
	if got, want := panePath(t, h, c.ID), panePath(t, h, parent.ID); got != want {
		t.Errorf("the child runs in %s, want its parent's folder %s", got, want)
	}
	if r := row(t, h, c.ID); r.Branch != "" || r.Worktree != "" || c.Branch != "" {
		t.Errorf("the child has branch %q, worktree %q: a child in its parent's folder has none of its own", r.Branch, r.Worktree)
	}
	if realDir(t, c.Dir) != panePath(t, h, parent.ID) {
		t.Errorf("delegate answered dir %s", c.Dir)
	}
}

func TestChildWorktreeStartsAtTheParentsHead(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	parent := newThread(t, h, "Fix the login")
	if err := os.WriteFile(filepath.Join(parent.Worktree, "login.txt"), []byte("fixed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, parent.Worktree, "add", "login.txt")
	gitIn(t, parent.Worktree, "commit", "-m", "fix the login")
	head := gitIn(t, parent.Worktree, "rev-parse", "HEAD")

	c := mustDelegate(t, h, parent.ID, `{"task":"test the fix","title":"Test the login fix","where":"worktree"}`)
	dir := realDir(t, filepath.Join(h.Dir, "trees", "test-the-login-fix"))
	if c.Branch != "test-the-login-fix" || realDir(t, c.Dir) != dir || panePath(t, h, c.ID) != dir {
		t.Errorf("delegate answered %+v; want branch test-the-login-fix in %s", c, dir)
	}
	if got := gitIn(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("the child's worktree is at %s, want its parent's commit %s", got, head)
	}
	if got := gitIn(t, dir, "branch", "--show-current"); got != "test-the-login-fix" || got == parent.Branch {
		t.Errorf("the child's worktree is on %q, want a branch of its own", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "login.txt")); err != nil {
		t.Errorf("the child's worktree lacks its parent's commit: %v", err)
	}
	if r := row(t, h, c.ID); r.Branch != "test-the-login-fix" || r.Worktree != dir || r.Parent != parent.ID {
		t.Errorf("the child's row: branch %q, worktree %q, parent %q", r.Branch, r.Worktree, r.Parent)
	}
}

func TestDelegateToAnIssueWithAThreadIsRefused(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "parent")
	issue, err := h.StartIssue(7)
	if err != nil {
		t.Fatal(err)
	}
	want := "Issue #7 has thread " + issue.ID + ": send to it instead"
	if _, err := delegateAs(t, h, parent.ID, `{"task":"fix it","where":"issue","issue":7}`); err != want {
		t.Errorf("delegate to #7 while its thread runs: %q, want %q", err, want)
	}
	if err := h.SettleThread(issue.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := delegateAs(t, h, parent.ID, `{"task":"fix it","where":"issue","issue":7}`); err != want {
		t.Errorf("delegate to #7 once its thread settled: %q, want %q", err, want)
	}

	c := mustDelegate(t, h, parent.ID, `{"task":"fix it","where":"issue","issue":8}`)
	if r := row(t, h, c.ID); r.Issue != 8 || r.Parent != parent.ID || c.Where != "issue" {
		t.Errorf("the child for #8: issue %d, parent %q, where %q", r.Issue, r.Parent, c.Where)
	}
	if got := h.Sessions().IssueSessions()[8]; got != c.ID {
		t.Errorf("the thread of #8 is %q, want the child %s", got, c.ID)
	}
}

func TestDelegateDeeperThanTheLimitIsRefused(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	top := newThread(t, h, "top")
	ids := []string{top.ID}
	for _, title := range []string{"child", "grandchild", "great-grandchild"} {
		c := mustDelegate(t, h, ids[len(ids)-1], `{"task":"go deeper, keep working","title":"`+title+`","where":"folder"}`)
		ids = append(ids, c.ID)
	}
	deepest := ids[len(ids)-1]
	before := tmuxSessions(t, h)
	want := "This thread is 3 delegations from one the owner started, and a thread that deep cannot delegate"
	if _, err := delegateAs(t, h, deepest, `{"task":"deeper still","where":"folder"}`); err != want {
		t.Errorf("a grandchild's child delegates: %q, want %q", err, want)
	}
	if after := tmuxSessions(t, h); len(after) != len(before) {
		t.Errorf("tmux sessions = %q, want %q", after, before)
	}
	s := h.Sessions()
	if got := s.Descendants(top.ID); !slices.Equal(got, ids[1:]) {
		t.Errorf("Descendants = %q, want %q", got, ids[1:])
	}
	if got := s.ChildrenOf(top.ID); !slices.Equal(got, ids[1:2]) {
		t.Errorf("ChildrenOf = %q, want %q", got, ids[1:2])
	}
	for i, id := range ids[1:] {
		if got := s.ParentOf(id); got != ids[i] {
			t.Errorf("ParentOf(%s) = %q, want %s", id, got, ids[i])
		}
	}
}

func TestLiveChildrenLimit(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "parent")
	var children []string
	// Each child works until stopped: one that finishes gives its result and goes to sleep.
	for range 8 {
		children = append(children, mustDelegate(t, h, parent.ID, `{"task":"keep working","where":"folder","access":"read"}`).ID)
	}
	want := "This thread has 8 children running, the most it may have: wait for one to finish or stop one"
	if _, err := delegateAs(t, h, parent.ID, `{"task":"one more","where":"folder","access":"read"}`); err != want {
		t.Errorf("a ninth child: %q, want %q", err, want)
	}
	if err := h.SettleThread(children[0]); err != nil {
		t.Fatal(err)
	}
	if got := h.Sessions().ChildrenOf(parent.ID); len(got) != 8 || !slices.Contains(got, children[0]) {
		t.Errorf("ChildrenOf = %q, want all eight, the settled one too", got)
	}
	c := mustDelegate(t, h, parent.ID, `{"task":"one more","where":"folder","access":"read"}`)
	if r := row(t, h, c.ID); r.Parent != parent.ID {
		t.Errorf("the child after a settle has parent %q", r.Parent)
	}
}

func TestWokenChildKeepsItsAccess(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := threadWith(t, h, "Lead", session.Full)
	c := mustDelegate(t, h, parent.ID, `{"task":"write the tests","access":"supervised","where":"folder"}`)
	if got := permissionMode(t, h, c.ID); got != "default" {
		t.Fatalf("the child starts with --permission-mode %s, want default", got)
	}
	if err := h.SettleThread(c.ID); err != nil {
		t.Fatal(err)
	}
	if r := row(t, h, c.ID); r.State != session.Settled || r.Parent != parent.ID {
		t.Errorf("the settled child: state %q, parent %q", r.State, r.Parent)
	}
	if err := os.Remove(filepath.Join(h.ClaudeDir(), strings.ReplaceAll(c.ID, "/", "_")+".args")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.WakeThread(c.ID); err != nil {
		t.Fatal(err)
	}
	if got := permissionMode(t, h, c.ID); got != "default" {
		t.Errorf("the woken child runs with --permission-mode %s, want default: its own access, not its parent's", got)
	}
	if r := row(t, h, c.ID); r.Access != session.Supervised || r.Parent != parent.ID {
		t.Errorf("the woken child: access %q, parent %q", r.Access, r.Parent)
	}
	if !strings.Contains(systemPrompt(t, h, c.ID), prompts.Child("Lead")) {
		t.Error("the woken child's system prompt does not name its parent")
	}
}

func TestParentLinkSurvivesARestart(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "parent")
	running := mustDelegate(t, h, parent.ID, `{"task":"keep working","where":"folder"}`)
	settled := mustDelegate(t, h, parent.ID, `{"task":"stop soon","where":"folder"}`)
	if err := h.SettleThread(settled.ID); err != nil {
		t.Fatal(err)
	}
	h.App.Stop()
	fresh := h.Restart(t)
	for _, id := range []string{running.ID, settled.ID} {
		if r := row(t, fresh, id); r.Parent != parent.ID {
			t.Errorf("after a restart %s has parent %q, want %s", id, r.Parent, parent.ID)
		}
	}
	if got := fresh.Sessions().ChildrenOf(parent.ID); len(got) != 2 {
		t.Errorf("after a restart ChildrenOf = %q, want both children", got)
	}
	if got := fresh.Sessions().AccessOf(running.ID); got != session.Supervised {
		t.Errorf("after a restart the child's access is %q", got)
	}
}

func TestChildInTheParentsFolderLeavesTheParentsBranch(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	parent := newThread(t, h, "Fix the login")
	c := mustDelegate(t, h, parent.ID, `{"task":"read the login code","where":"folder","access":"supervised"}`)
	gitIn(t, parent.Worktree, "checkout", "-b", "login-v2")
	if err := h.Sessions().Track(parent.ID, "login-v2"); err != nil {
		t.Fatalf("the parent tracks the branch its worktree is on: %v; a child in its folder must not hold it", err)
	}
	if r := row(t, h, parent.ID); r.Branch != "login-v2" {
		t.Errorf("the parent's branch is %q, want login-v2", r.Branch)
	}
	if r := row(t, h, c.ID); r.Branch != "" {
		t.Errorf("the child's branch is %q, want none", r.Branch)
	}
}

func TestChildInTheParentsFolderTracksNoBranch(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	parent := newThread(t, h, "Fix the login")
	inFolder := mustDelegate(t, h, parent.ID, `{"task":"read the login code","where":"folder","access":"supervised"}`)
	want := "session " + inFolder.ID + " is a child with no worktree, which works on no branch of its own"
	for _, branch := range []string{"side-work", parent.Branch} {
		if err := h.Sessions().Track(inFolder.ID, branch); err == nil || err.Error() != want {
			t.Errorf("the child in its parent's folder tracks %s: %v, want %q", branch, err, want)
		}
	}
	inTree := mustDelegate(t, h, parent.ID, `{"task":"test the login","title":"Test the login","where":"worktree"}`)
	if err := h.Sessions().Track(inTree.ID, "test-the-login"); err != nil {
		t.Errorf("a child in its own worktree tracks its branch: %v", err)
	}
}

func TestCancelledDelegateStartsNoChild(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "parent")
	before := tmuxSessions(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.Sessions().CreateChild(ctx, sessions.ChildOptions{Parent: parent.ID, Task: "too late", Access: session.Read, Where: sessions.WhereFolder})
	if err == nil || !strings.Contains(err.Error(), "no child started") {
		t.Errorf("a cancelled delegate: %v, want no child started", err)
	}
	if after := tmuxSessions(t, h); !slices.Equal(after, before) {
		t.Errorf("tmux sessions = %q, want %q", after, before)
	}
	if got := h.Sessions().ChildrenOf(parent.ID); len(got) != 0 {
		t.Errorf("ChildrenOf = %q, want none", got)
	}
}

// otherProject adds a second project, in a folder of its own, and makes main current again, so the other one is not on
// screen. It returns the other project's name and key.
func otherProject(t *testing.T, h *apptest.Harness, dir string) (name, key string) {
	t.Helper()
	snap, err := h.AddProjectDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.SwitchProject("main"); err != nil {
		t.Fatal(err)
	}
	return snap.Project.Name, snap.Project.Key
}

// Not parallel: it redirects the process-wide log.
func TestDelegateToAnotherProject(t *testing.T) {
	logged := &apptest.LockedBuffer{}
	before := log.Writer()
	log.SetOutput(logged)
	t.Cleanup(func() { log.SetOutput(before) })
	h := apptest.New(t)
	parent := newThread(t, h, "Plan the release")
	main7, err := h.StartIssue(7)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name, key := otherProject(t, h, dir)

	c := mustDelegate(t, h, parent.ID, `{"task":"read the docs","title":"Read the docs","access":"read","project":"`+strings.ToUpper(name)+`"}`)
	if c.Project != key || c.Where != "folder" || realDir(t, c.Dir) != realDir(t, dir) {
		t.Errorf("delegate answered %+v; want project %s, folder %s", c, key, dir)
	}
	if got := panePath(t, h, c.ID); got != realDir(t, dir) {
		t.Errorf("the child runs in %s, want the other project's folder %s", got, dir)
	}
	if r := row(t, h, c.ID); r.Project != key || r.Parent != parent.ID || r.Branch != "" {
		t.Errorf("the child's row: project %q, parent %q, branch %q; want %s, %s, none", r.Project, r.Parent, r.Branch, key, parent.ID)
	}
	if session.ProjectKey(c.ID) != key {
		t.Errorf("the child's id %s is not of project %s", c.ID, key)
	}
	apptest.Eventually(t, "the other project's sessions event with the child", func() bool {
		return slices.ContainsFunc(h.Rec.LastSessionsOf(key), func(s sessions.Session) bool { return s.ID == c.ID && s.Parent == parent.ID })
	})
	if slices.ContainsFunc(h.Rec.LastSessionsOf("main"), func(s sessions.Session) bool { return s.ID == c.ID }) {
		t.Error("main's sessions event lists the child")
	}
	if got := h.Sessions().ChildrenOf(parent.ID); !slices.Equal(got, []string{c.ID}) {
		t.Errorf("ChildrenOf = %q, want the child in the other project", got)
	}

	issue := mustDelegate(t, h, parent.ID, `{"task":"fix it","where":"issue","issue":7,"project":"`+key+`"}`)
	if r := row(t, h, issue.ID); r.Project != key || r.Issue != 7 || r.Parent != parent.ID || issue.ID == main7.ID {
		t.Errorf("the child for the other project's #7: %+v; main's #7 is %s", r, main7.ID)
	}
	if _, err := delegateAs(t, h, parent.ID, `{"task":"again","where":"issue","issue":7,"project":"`+key+`"}`); err != "Issue #7 has thread "+issue.ID+": send to it instead" {
		t.Errorf("a second child for the other project's #7: %q", err)
	}

	lines := strings.Count(logged.String(), "started child")
	want := fmt.Sprintf("thread %s %q in project main started child %s %q in project %s", parent.ID, "Plan the release", c.ID, "Read the docs", name)
	if lines != 2 || !strings.Contains(logged.String(), want) {
		t.Errorf("logged %d delegations, want 2 with %q:\n%s", lines, want, logged.String())
	}
	mustDelegate(t, h, parent.ID, `{"task":"here","where":"folder","project":"main"}`)
	if n := strings.Count(logged.String(), "started child"); n != 2 {
		t.Errorf("after a delegation in the caller's own project %d delegations are logged, want 2", n)
	}
}

func TestDelegateToAnUnknownProjectNamesTheProjects(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	parent := newThread(t, h, "parent")
	dir := t.TempDir()
	name, _ := otherProject(t, h, dir)
	before := tmuxSessions(t, h)
	for _, project := range []string{"nowhere", dir, "../" + name, "main/../" + name} {
		want := fmt.Sprintf("No project %q: the projects are main, %s", project, name)
		if c, err := delegateAs(t, h, parent.ID, `{"task":"x","access":"read","project":"`+project+`"}`); err != want {
			t.Errorf("delegate to project %q: %+v, %q; want %q", project, c, err, want)
		}
	}
	if after := tmuxSessions(t, h); !slices.Equal(after, before) {
		t.Errorf("tmux sessions = %q, want %q: a refused delegate started one", after, before)
	}
}

func TestCrossProjectWorktreeStartsAtTheDefaultBranch(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	repoFixture(t, h)
	parent := newThread(t, h, "Fix the login")
	if err := os.WriteFile(filepath.Join(parent.Worktree, "login.txt"), []byte("fixed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, parent.Worktree, "add", "login.txt")
	gitIn(t, parent.Worktree, "commit", "-m", "fix the login")
	parentHead := gitIn(t, parent.Worktree, "rev-parse", "HEAD")

	// The other project's clone is on a branch of its own, and its copy of origin's main is behind origin.
	root := t.TempDir()
	origin, dir, pusher := filepath.Join(root, "docs.git"), filepath.Join(root, "docs"), filepath.Join(root, "pusher")
	gitIn(t, root, "init", "--bare", origin)
	gitIn(t, root, "init", dir)
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("docs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "README")
	gitIn(t, dir, "commit", "-m", "first")
	gitIn(t, dir, "remote", "add", "origin", origin)
	gitIn(t, dir, "push", "-u", "origin", "main")
	gitIn(t, dir, "checkout", "-b", "draft")
	gitIn(t, dir, "commit", "--allow-empty", "-m", "a draft")
	localHead := gitIn(t, dir, "rev-parse", "HEAD")
	gitIn(t, root, "clone", origin, pusher)
	gitIn(t, pusher, "commit", "--allow-empty", "-m", "newer on origin")
	gitIn(t, pusher, "push", "origin", "main")
	originMain := gitIn(t, pusher, "rev-parse", "HEAD")
	_, key := otherProject(t, h, dir)

	c := mustDelegate(t, h, parent.ID, `{"task":"write the docs","title":"Write the login docs","where":"worktree","project":"`+key+`"}`)
	tree := realDir(t, filepath.Join(dir, "trees", "write-the-login-docs"))
	if c.Branch != "write-the-login-docs" || c.Project != key || realDir(t, c.Dir) != tree || panePath(t, h, c.ID) != tree {
		t.Errorf("delegate answered %+v; want branch write-the-login-docs in %s", c, tree)
	}
	if got := gitIn(t, tree, "rev-parse", "HEAD"); got != originMain || got == localHead || got == parentHead {
		t.Errorf("the child's worktree is at %s, want origin's main %s (the clone's own HEAD is %s, the parent's %s)", got, originMain, localHead, parentHead)
	}
	if r := row(t, h, c.ID); r.Project != key || r.Branch != "write-the-login-docs" || r.Worktree != tree || r.Parent != parent.ID {
		t.Errorf("the child's row: project %q, branch %q, worktree %q, parent %q", r.Project, r.Branch, r.Worktree, r.Parent)
	}
	if got := gitIn(t, h.Dir, "branch", "--list", "write-the-login-docs"); got != "" {
		t.Errorf("the caller's project has the child's branch: %q", got)
	}
}

func TestCrossProjectRankStillApplies(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	supervised := newThread(t, h, "supervised")
	read := threadWith(t, h, "read", session.Read)
	_, key := otherProject(t, h, t.TempDir())
	before := tmuxSessions(t, h)
	for _, where := range []string{"", `"where":"folder",`, `"where":"worktree",`, `"where":"issue","issue":8,`} {
		for _, tt := range []struct {
			caller, asked, want string
		}{
			{supervised.ID, "full", "A thread with supervised access cannot start a child with full access"},
			{read.ID, "supervised", "A thread with read access cannot start a child with supervised access"},
			{read.ID, "full", "A thread with read access cannot start a child with full access"},
		} {
			input := `{` + where + `"task":"do it","access":"` + tt.asked + `","project":"` + key + `"}`
			if c, err := delegateAs(t, h, tt.caller, input); err != tt.want {
				t.Errorf("%s: %+v, %q; want %q", input, c, err, tt.want)
			}
		}
	}
	if after := tmuxSessions(t, h); !slices.Equal(after, before) {
		t.Errorf("tmux sessions = %q, want %q: a refused delegate started one", after, before)
	}
	c := mustDelegate(t, h, read.ID, `{"task":"look","project":"`+key+`"}`)
	if r := row(t, h, c.ID); r.Access != session.Read || r.Project != key || c.Where != "folder" {
		t.Errorf("a read caller's child in the other project: access %q, project %q, where %q", r.Access, r.Project, c.Where)
	}
}
