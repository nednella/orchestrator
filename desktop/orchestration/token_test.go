package orchestration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/control"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/chat"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/session"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

const refused = "This thread token is not valid: the thread settled or was woken again"

// newThread starts a Supervised thread with no prompt.
func newThread(t *testing.T, h *apptest.Harness, title string) sessions.Session {
	t.Helper()
	s, err := h.Sessions().Create(h.Sessions().Current(), title, "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// configOf is the MCP config of the thread's host, beside its socket.
func configOf(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	c, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(c.Socket, ".sock") + ".mcp.json"
}

// tokenOf is the token the thread's host was last started with, as its MCP config gives it to the orchestrator server.
func tokenOf(t *testing.T, h *apptest.Harness, id string) string {
	t.Helper()
	data, err := os.ReadFile(configOf(t, h, id))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		MCPServers map[string]struct{ Env map[string]string }
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	token := config.MCPServers["orchestrator"].Env["ORCHESTRATOR_MCP_TOKEN"]
	if token == "" {
		t.Fatalf("no token in %s", data)
	}
	return token
}

// callerOf is the thread a call with that token and that session field acts as, or why there is none.
func callerOf(h *apptest.Harness, token, sessionField string) (string, error) {
	return control.New(h.Sessions()).Caller(ctl.Request{Cmd: "orchestrate", Token: token, Session: sessionField})
}

func wantCaller(t *testing.T, h *apptest.Harness, token, want string) {
	t.Helper()
	if got, err := callerOf(h, token, ""); err != nil || got != want {
		t.Errorf("caller = %q, %v; want %s", got, err, want)
	}
}

func wantRefused(t *testing.T, h *apptest.Harness, token string) {
	t.Helper()
	if got, err := callerOf(h, token, ""); err == nil || err.Error() != refused {
		t.Errorf("caller = %q, %v; want %q", got, err, refused)
	}
}

func TestTokenNamesItsThread(t *testing.T) {
	h := apptest.New(t)
	a, b := newThread(t, h, "a"), newThread(t, h, "b")
	wantCaller(t, h, tokenOf(t, h, a.ID), a.ID)
	wantCaller(t, h, tokenOf(t, h, b.ID), b.ID)

	// Through the control socket the caller is found first, and the tool after it.
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, a.ID), Args: []string{"nope"}})
	if resp.OK || resp.Error != `orchestrator has no tool "nope"` {
		t.Errorf("an unknown tool = %+v", resp)
	}
}

func TestTokenIgnoresTheSessionField(t *testing.T) {
	h := apptest.New(t)
	a, b := newThread(t, h, "a"), newThread(t, h, "b")
	if got, err := callerOf(h, tokenOf(t, h, a.ID), b.ID); err != nil || got != a.ID {
		t.Errorf("A's token with session B: caller = %q, %v; want A, %s", got, err, a.ID)
	}
	if got, err := callerOf(h, "", b.ID); err == nil || err.Error() != "This call carries no orchestrator thread token" {
		t.Errorf("no token with session B: caller = %q, %v", got, err)
	}
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Session: b.ID, Project: "main", Args: []string{"threads"}})
	if resp.OK || resp.Error != "This call carries no orchestrator thread token" {
		t.Errorf("a call with a session and no token = %+v", resp)
	}
}

func TestUnknownTokenIsRefused(t *testing.T) {
	h := apptest.New(t)
	a := newThread(t, h, "a")
	token := tokenOf(t, h, a.ID)
	for _, bad := range []string{"not-a-token", token[:len(token)-1], token + "x", a.ID} {
		wantRefused(t, h, bad)
	}
	resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: "not-a-token", Args: []string{"threads"}})
	if resp.OK || resp.Error != refused {
		t.Errorf("an unknown token over the socket = %+v", resp)
	}
}

func TestWakeReplacesTheToken(t *testing.T) {
	h := apptest.New(t)
	a := newThread(t, h, "a")
	before := tokenOf(t, h, a.ID)
	if err := h.SettleThread(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.WakeThread(a.ID); err != nil {
		t.Fatal(err)
	}
	after := tokenOf(t, h, a.ID)
	if after == before {
		t.Fatal("the woken thread kept its token")
	}
	wantRefused(t, h, before)
	wantCaller(t, h, after, a.ID)
}

func TestSettledThreadsTokenIsRefused(t *testing.T) {
	h := apptest.New(t)
	settled, deleted := newThread(t, h, "settled"), newThread(t, h, "deleted")
	tokens := map[string]string{settled.ID: tokenOf(t, h, settled.ID), deleted.ID: tokenOf(t, h, deleted.ID)}
	paths := map[string]string{settled.ID: configOf(t, h, settled.ID), deleted.ID: configOf(t, h, deleted.ID)}
	if err := h.SettleThread(settled.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.DeleteThread(deleted.ID, false); err != nil {
		t.Fatal(err)
	}
	for id, token := range tokens {
		wantRefused(t, h, token)
		if apptest.Exists(paths[id]) {
			t.Errorf("%s: its MCP config is still there", id)
		}
	}
}

func TestTokenSurvivesAppRestart(t *testing.T) {
	h := apptest.New(t)
	a := newThread(t, h, "a")
	token := tokenOf(t, h, a.ID)
	h.App.Stop()
	fresh := h.Restart(t)
	wantCaller(t, fresh, token, a.ID)
	resp := fresh.Ask(t, ctl.Request{Cmd: "orchestrate", Token: token, Args: []string{"nope"}})
	if resp.OK || resp.Error != `orchestrator has no tool "nope"` {
		t.Errorf("the token after a restart = %+v, want it to name its thread", resp)
	}
}

func TestForkGetsItsOwnToken(t *testing.T) {
	h := apptest.New(t)
	src := newThread(t, h, "work")
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h.Observe(t, src.ID, "Stop", fmt.Sprintf(`{"session_id":"abc","transcript_path":%q}`, transcript))
	c, err := h.Sessions().ChatOf(src.ID)
	if err != nil {
		t.Fatal(err)
	}
	apptest.Eventually(t, "the thread's host", func() bool { return apptest.Exists(c.Socket) })
	l, err := chat.OpenLog(c.Log)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []chat.Event{{Kind: chat.KindUser, ID: "u1", Text: "first"}, {Kind: chat.KindMessage, ID: "msg_1", UUID: "r1", Text: "one"}, {Kind: chat.KindTurnEnd}} {
		if _, err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()
	var fork sessions.Session
	// The Stop hook reaches the app a moment after it runs; until then the thread has no conversation to fork.
	apptest.Eventually(t, "the fork", func() bool { fork, err = h.ForkSession(src.ID, "r1"); return err == nil })

	srcToken, forkToken := tokenOf(t, h, src.ID), tokenOf(t, h, fork.ID)
	if srcToken == forkToken {
		t.Fatal("the fork has its source's token")
	}
	wantCaller(t, h, srcToken, src.ID)
	wantCaller(t, h, forkToken, fork.ID)
}

func TestUserOnlyCommandsAreGone(t *testing.T) {
	h := apptest.New(t)
	for _, cmd := range []string{"open", "settle", "pr", "cleanup", "project", "relaunch", "stats", "digest", "digest-add", "queue", "notes", "evidence", "chat", "next", "filter"} {
		if resp := h.Ask(t, ctl.Request{Cmd: cmd, Args: []string{"1"}}); resp.OK || resp.Error != fmt.Sprintf("unknown command %q", cmd) {
			t.Errorf("%s over the socket = %+v, want unknown command", cmd, resp)
		}
	}
}

func TestSocketNeverRunsAnAgentCommandAsAThreadItNames(t *testing.T) {
	h := apptest.New(t)
	a, b := newThread(t, h, "a"), newThread(t, h, "b")
	for _, req := range []ctl.Request{
		{Cmd: "note", Session: b.ID, Project: "main", Opts: map[string]string{"text": "forged"}},
		{Cmd: "issue", Session: b.ID, Project: "main", Args: []string{"7"}, Opts: map[string]string{"prompt": "forged"}},
		{Cmd: "new", Session: b.ID, Project: "main", Args: []string{"forged"}, Opts: map[string]string{"access": "full"}},
		{Cmd: "show", Session: b.ID, Opts: map[string]string{"text": "forged"}},
		{Cmd: "track", Session: b.ID, Opts: map[string]string{"branch": "forged"}},
		{Cmd: "refresh", Session: b.ID, Project: "main"},
		{Cmd: "browser", Session: b.ID, Args: []string{"open", "http://127.0.0.1:1/"}},
		{Cmd: "note", Token: tokenOf(t, h, a.ID), Session: b.ID, Opts: map[string]string{"text": "forged"}},
	} {
		if resp := h.Ask(t, req); resp.OK || resp.Error != fmt.Sprintf("unknown command %q", req.Cmd) {
			t.Errorf("%s as %s over the socket = %+v, want unknown command", req.Cmd, req.Session, resp)
		}
	}
	if n := len(h.Sessions().List()); n != 2 {
		t.Errorf("%d threads, want the 2 that were there", n)
	}
	if notes, err := h.Notes().List("main"); err != nil || len(notes) != 0 {
		t.Errorf("notes = %v, %v; none was asked for through a tool", notes, err)
	}
}
