package chat

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/internal/agent"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/session"
)

// configDir is the folder the recorded run's transcript paths start with.
const configDir = "/tmp/claude-config"

// logged is an event of the host's log made from a line of the recorded stream, and the line it came from.
type logged struct {
	chatlog.Event
	line int
}

// recordedLog is the log a host would have kept of the run in testdata/hooks.jsonl: its start, what Claude wrote
// that the log keeps, the user's message where Claude echoed it, the answer where Claude took one, and its exit.
func recordedLog(t *testing.T) []logged {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "internal", "chat", "testdata", "hooks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []logged
	add := func(line int, e chatlog.Event) {
		e.Seq = int64(len(out) + 1)
		e.At = time.Date(2026, 11, 26, 3, 0, 0, 0, time.UTC).UnixMilli() + e.Seq
		out = append(out, logged{e, line})
	}
	add(0, chatlog.Event{Kind: chatlog.KindStart})
	asked := map[string]bool{}
	lines := bufio.NewScanner(f)
	lines.Buffer(nil, 1<<22)
	for n := 1; lines.Scan(); n++ {
		events, err := chatlog.Parse(lines.Bytes())
		if err != nil {
			t.Fatalf("line %d: %v", n, err)
		}
		for _, e := range events {
			switch e.Kind {
			case chatlog.KindEcho:
				add(n, chatlog.Event{Kind: chatlog.KindUser, ID: e.ID})
			case chatlog.KindApproval:
				asked[e.ID] = true
				add(n, e)
			case chatlog.KindReply:
				if asked[e.ID] {
					add(n, chatlog.Event{Kind: chatlog.KindAnswer, ID: e.ID, Text: "allow"})
				}
			case chatlog.KindInit, chatlog.KindToolUse, chatlog.KindToolResult, chatlog.KindMessage, chatlog.KindTurnEnd:
				add(n, e)
			}
		}
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	add(0, chatlog.Event{Kind: chatlog.KindExit})
	return out
}

func claudeTranscripts() *turn {
	return newTurn(agent.Claude{ConfigDir: configDir}.Transcript)
}

func TestStateFollowsTheStream(t *testing.T) {
	// The state after the first event of a kind that the recorded stream has at a line.
	want := []struct {
		line  int
		kind  chatlog.Kind
		state session.State
		why   string
	}{
		{0, chatlog.KindStart, session.Idle, "idle from the host's start"},
		{5, chatlog.KindInit, session.Idle, "system/init comes after the prompt and starts nothing"},
		{7, chatlog.KindUser, session.Working, "working at the prompt"},
		{10, chatlog.KindToolUse, session.Working, "working at a tool call"},
		{14, chatlog.KindToolResult, session.Working, "working at its result"},
		{23, chatlog.KindTurnEnd, session.Idle, "idle at the turn's end"},
		{29, chatlog.KindUser, session.Working, "working at the next prompt"},
		{52, chatlog.KindApproval, session.Waiting, "waiting at a permission"},
		{53, chatlog.KindAnswer, session.Working, "working once it is answered"},
		{54, chatlog.KindToolResult, session.Working, "working at the result of the tool it allowed"},
		{69, chatlog.KindTurnEnd, session.Idle, "idle at the end of that turn"},
		{89, chatlog.KindTurnEnd, session.Waiting, "waiting at a reply that ends in a question"},
		{95, chatlog.KindUser, session.Working, "working at the prompt after it"},
		{110, chatlog.KindApproval, session.Waiting, "waiting at AskUserQuestion"},
		{111, chatlog.KindAnswer, session.Working, "working once the question is answered"},
		{134, chatlog.KindTurnEnd, session.Idle, "idle at the end of that turn"},
		{158, chatlog.KindToolUse, session.Working, "working at a subagent's tool call"},
		{179, chatlog.KindTurnEnd, session.Idle, "idle at the end of the turn with a subagent"},
		{192, chatlog.KindApproval, session.Waiting, "waiting at the permission of the call to be interrupted"},
		{197, chatlog.KindToolResult, session.Working, "working at the interrupted call's result"},
		{199, chatlog.KindTurnEnd, session.Idle, "idle at an interrupted turn, which has no Stop"},
		{0, chatlog.KindExit, session.Idle, "idle at the exit: the row sleeps once tmux has lost the session"},
	}
	var rec session.Record
	type point struct {
		line int
		kind chatlog.Kind
	}
	got := map[point]session.State{}
	state := claudeTranscripts()
	for _, e := range recordedLog(t) {
		ev, ok := stateEvent(e.Event, state)
		if !ok {
			continue
		}
		rec = session.Apply("main/abcdefgh", rec, ev, ev.At)
		if _, seen := got[point{e.line, e.Kind}]; !seen {
			got[point{e.line, e.Kind}] = rec.State
		}
	}
	for _, w := range want {
		if s, ok := got[point{w.line, w.kind}]; !ok || s != w.state {
			t.Errorf("line %d, %s: state %q (seen %v), want %q: %s", w.line, w.kind, s, ok, w.state, w.why)
		}
	}
}

func TestTurnEndNamesTheReplyAndWhetherItAsks(t *testing.T) {
	var ends []session.Event
	state := claudeTranscripts()
	for _, e := range recordedLog(t) {
		if ev, ok := stateEvent(e.Event, state); ok && (ev.Name == "Stop" || ev.Name == session.TurnError) {
			ends = append(ends, ev)
		}
	}
	if len(ends) != 6 {
		t.Fatalf("%d turn ends, want 6", len(ends))
	}
	for i, wantAsks := range []bool{false, false, true, false, false} {
		if ends[i].Name != "Stop" || ends[i].Asks != wantAsks || ends[i].Detail == "" {
			t.Errorf("turn %d ended with %+v, want a Stop that asks %v", i+1, ends[i], wantAsks)
		}
	}
	if !strings.HasPrefix(ends[2].Detail, "I wrote `hi` to `a.txt`") {
		t.Errorf("turn 3's reply = %q", ends[2].Detail)
	}
	if last := ends[5]; last.Name != session.TurnError || last.Asks || last.Detail != "" {
		t.Errorf("the interrupted turn ended with %+v, want a TurnError", last)
	}
}

func TestToolEventsNameTheToolAndItsCommand(t *testing.T) {
	var pre, post session.Event
	state := claudeTranscripts()
	for _, e := range recordedLog(t) {
		ev, ok := stateEvent(e.Event, state)
		switch {
		case !ok:
		case ev.Name == "PreToolUse" && pre.Name == "":
			pre = ev
		case ev.Name == "PostToolUse" && post.Name == "":
			post = ev
		}
	}
	if pre.Tool != "Bash" || pre.Command != "cd sub && pwd" || pre.Detail != "Bash cd sub && pwd" {
		t.Errorf("PreToolUse = %+v", pre)
	}
	if post.Tool != "Bash" || post.Command != "cd sub && pwd" {
		t.Errorf("PostToolUse = %+v, want the tool of the call it answers", post)
	}
}

func TestInitGivesTheTranscriptHooksReported(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "chat", "testdata", "hooks-payloads.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var p struct {
			Payload struct {
				Transcript string `json:"transcript_path"`
			}
		}
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			t.Fatal(err)
		}
		paths[p.Payload.Transcript] = true
	}
	if len(paths) != 1 {
		t.Fatalf("the hooks named %d transcripts, want one", len(paths))
	}
	state := claudeTranscripts()
	inits := 0
	for _, e := range recordedLog(t) {
		ev, ok := stateEvent(e.Event, state)
		if !ok || ev.Name != session.Init {
			continue
		}
		inits++
		if !paths[ev.Transcript] || ev.Conversation == "" || ev.Model != "claude-haiku-5-5" {
			t.Errorf("init at line %d = %+v, want the transcript %v", e.line, ev, paths)
		}
	}
	if inits != 6 {
		t.Errorf("%d inits, want 6", inits)
	}
	// After the turn that went into sub, the folder changes and the transcript's does not.
	var inSub session.Event
	state = claudeTranscripts()
	for _, e := range recordedLog(t) {
		if ev, ok := stateEvent(e.Event, state); ok && ev.Name == session.Init && e.line == 27 {
			inSub = ev
		}
	}
	if inSub.Cwd != "/tmp/rec/sub" || !strings.Contains(inSub.Transcript, "/projects/-tmp-rec/") {
		t.Errorf("init in sub = %+v, want the folder sub and the transcript of the start folder", inSub)
	}
}

func TestResumeAndForkNameTheirOwnTranscripts(t *testing.T) {
	for _, tt := range []struct {
		name string
		init chatlog.Event
		want string
	}{
		{"a resume keeps the conversation and the folder it started in", chatlog.Event{Kind: chatlog.KindInit, ID: "f2b5a017", Cwd: "/tmp/rec", Origin: "/tmp/rec"}, configDir + "/projects/-tmp-rec/f2b5a017.jsonl"},
		{"a fork is a new conversation", chatlog.Event{Kind: chatlog.KindInit, ID: "d6a30985", Cwd: "/tmp/rec", Origin: "/tmp/rec"}, configDir + "/projects/-tmp-rec/d6a30985.jsonl"},
		{"a worktree is its own folder", chatlog.Event{Kind: chatlog.KindInit, ID: "c5ccf729", Cwd: "/tmp/trees/wt", Origin: "/tmp/trees/wt"}, configDir + "/projects/-tmp-trees-wt/c5ccf729.jsonl"},
		{"a folder the thread moved to is not the transcript's", chatlog.Event{Kind: chatlog.KindInit, ID: "s1", Cwd: "/tmp/rec/sub", Origin: "/tmp/rec"}, configDir + "/projects/-tmp-rec/s1.jsonl"},
		{"no folder, no transcript", chatlog.Event{Kind: chatlog.KindInit, ID: "s1"}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ev, ok := stateEvent(tt.init, claudeTranscripts())
			if !ok || ev.Name != session.Init || ev.Transcript != tt.want || ev.Conversation != tt.init.ID {
				t.Errorf("stateEvent = %+v, %v, want the transcript %q", ev, ok, tt.want)
			}
		})
	}
}

func TestStateEventForEachKind(t *testing.T) {
	approval := chatlog.Event{Kind: chatlog.KindApproval, ID: "r1", Tool: "Bash", Input: json.RawMessage(`{"command":"make test"}`)}
	for _, tt := range []struct {
		name   string
		before []chatlog.Event
		e      chatlog.Event
		want   session.Event
		ok     bool
	}{
		{"a message of the owner", nil, chatlog.Event{Kind: chatlog.KindUser, Text: "fix\n  the bug"}, session.Event{Name: "UserPromptSubmit", Detail: "fix the bug"}, true},
		{"a message that denies an approval", nil, chatlog.Event{Kind: chatlog.KindUser, Parent: "r1", Text: "no"}, session.Event{}, false},
		{"a tool call", nil, chatlog.Event{Kind: chatlog.KindToolUse, ID: "t1", Tool: "Edit", Input: json.RawMessage(`{"file_path":"a.go"}`)}, session.Event{Name: "PreToolUse", Tool: "Edit", Detail: "Edit a.go"}, true},
		{"a tool call of a subagent", nil, chatlog.Event{Kind: chatlog.KindToolUse, ID: "t1", Agent: "a1", Tool: "Bash", Input: json.RawMessage(`{"command":"ls"}`)}, session.Event{Name: "PreToolUse", Tool: "Bash", Command: "ls", Detail: "Bash ls"}, true},
		{"the result of a call the log has no start of", nil, chatlog.Event{Kind: chatlog.KindToolResult, Parent: "t9"}, session.Event{Name: "PostToolUse"}, true},
		{"an approval", nil, approval, session.Event{Name: "Notification", NotificationType: "permission_prompt", Message: "Bash make test", Detail: "Bash make test"}, true},
		{"a subagent's approval", nil, chatlog.Event{Kind: chatlog.KindApproval, ID: "r2", Agent: "a1", Tool: "Write"}, session.Event{}, false},
		{"an answer to a subagent's approval", []chatlog.Event{{Kind: chatlog.KindApproval, ID: "r2", Agent: "a1", Tool: "Write"}}, chatlog.Event{Kind: chatlog.KindAnswer, ID: "r2"}, session.Event{}, false},
		{"an answer", []chatlog.Event{approval}, chatlog.Event{Kind: chatlog.KindAnswer, ID: "r1"}, session.Event{Name: "PreToolUse", Tool: "Bash", Command: "make test", Detail: "Bash make test"}, true},
		{"an answer while another approval waits", []chatlog.Event{approval, {Kind: chatlog.KindApproval, ID: "r2", Tool: "Edit"}}, chatlog.Event{Kind: chatlog.KindAnswer, ID: "r1"}, session.Event{}, false},
		{"an answer to nothing", nil, chatlog.Event{Kind: chatlog.KindAnswer, ID: "r7"}, session.Event{}, false},
		{"a reply of a subagent is not the turn's", []chatlog.Event{{Kind: chatlog.KindMessage, Text: "Ready?"}, {Kind: chatlog.KindMessage, Agent: "a1", Text: "done"}}, chatlog.Event{Kind: chatlog.KindTurnEnd}, session.Event{Name: "Stop", Detail: "Ready?", Asks: true}, true},
		{"a reply that ends in a question under markup", []chatlog.Event{{Kind: chatlog.KindMessage, Text: "Done.\n\n**Do you want me to push?**\n"}}, chatlog.Event{Kind: chatlog.KindTurnEnd}, session.Event{Name: "Stop", Detail: "Done. **Do you want me to push?**", Asks: true}, true},
		{"a question inside the reply", []chatlog.Event{{Kind: chatlog.KindMessage, Text: "Why? The test was flaky. I fixed it."}}, chatlog.Event{Kind: chatlog.KindTurnEnd}, session.Event{Name: "Stop", Detail: "Why? The test was flaky. I fixed it."}, true},
		{"the reply of an earlier turn is not this one's", []chatlog.Event{{Kind: chatlog.KindMessage, Text: "Ready?"}, {Kind: chatlog.KindTurnEnd}}, chatlog.Event{Kind: chatlog.KindTurnEnd}, session.Event{Name: "Stop"}, true},
		{"a turn that ended in an error", []chatlog.Event{{Kind: chatlog.KindMessage, Text: "Ready?"}}, chatlog.Event{Kind: chatlog.KindTurnEnd, Error: true}, session.Event{Name: session.TurnError}, true},
		{"the exit", nil, chatlog.Event{Kind: chatlog.KindExit}, session.Event{Name: "SessionEnd"}, true},
		{"a failed exit", nil, chatlog.Event{Kind: chatlog.KindExit, Error: true}, session.Event{Name: "SessionEnd"}, true},
		{"the host's start", nil, chatlog.Event{Kind: chatlog.KindStart}, session.Event{Name: "SessionStart"}, true},
		{"a delta", nil, chatlog.Event{Kind: chatlog.KindDelta, Text: "x"}, session.Event{}, false},
		{"a queued message", nil, chatlog.Event{Kind: chatlog.KindQueued, Text: "x"}, session.Event{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := claudeTranscripts()
			for _, e := range tt.before {
				stateEvent(e, state)
			}
			tt.e.Seq, tt.e.At = 7, 1234
			got, ok := stateEvent(tt.e, state)
			if tt.ok {
				tt.want.Seq, tt.want.At = 7, time.UnixMilli(1234)
			}
			if ok != tt.ok || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("stateEvent = %+v, %v, want %+v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestEveryStateEventCarriesItsSeq(t *testing.T) {
	var seqs []int64
	state := claudeTranscripts()
	for _, e := range recordedLog(t) {
		if ev, ok := stateEvent(e.Event, state); ok {
			if ev.Seq != e.Seq || ev.At.UnixMilli() != e.At {
				t.Errorf("event %+v from log entry %d at %d", ev, e.Seq, e.At)
			}
			seqs = append(seqs, ev.Seq)
		}
	}
	if !slices.IsSorted(seqs) || len(slices.Compact(slices.Clone(seqs))) != len(seqs) {
		t.Errorf("seqs %v are not strictly increasing", seqs)
	}
}
