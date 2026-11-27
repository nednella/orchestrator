package thread

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNext(t *testing.T) {
	tests := []struct {
		name string
		cur  State
		ev   Event
		want State
	}{
		{"start", Idle, Event{Name: "SessionStart"}, Idle},
		{"start after a reply", Idle, Event{Name: "SessionStart"}, Idle},
		{"prompt", Idle, Event{Name: "UserPromptSubmit"}, Working},
		{"pre tool", Idle, Event{Name: "PreToolUse"}, Working},
		{"post tool answers a prompt", Waiting, Event{Name: "PostToolUse"}, Working},
		{"permission prompt", Working, Event{Name: "Notification", NotificationType: "permission_prompt"}, Waiting},
		{"notification without type", Working, Event{Name: "Notification", Message: "Claude needs your permission to use Bash"}, Waiting},
		{"stop", Working, Event{Name: "Stop"}, Idle},
		{"stop while waiting", Waiting, Event{Name: "Stop"}, Idle},
		{"stop with a question", Working, Event{Name: "Stop", Asks: true}, Waiting},
		{"turn error", Working, Event{Name: TurnError}, Idle},
		{"turn error while waiting", Waiting, Event{Name: TurnError}, Idle},
		{"unknown event", Working, Event{Name: "PreCompact"}, Working},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Next(tt.cur, tt.ev); got != tt.want {
				t.Errorf("Next(%s, %s) = %s, want %s", tt.cur, tt.ev.Name, got, tt.want)
			}
		})
	}
}

func TestApply(t *testing.T) {
	t0 := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)
	stop := Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0, Detail: "done"}
	chat := Identity{Title: "ask", Issue: 7, Access: Read, Action: "plan", Model: "opus", Effort: "high", Path: "/p", Created: t0.UnixMilli()}
	tests := []struct {
		name string
		prev Record
		ev   Event
		want Record
	}{
		{"first event", Record{}, Event{Name: "PreToolUse", Detail: "Bash ls"}, Record{Thread: "p/1", State: Working, Event: "PreToolUse", At: t1, Detail: "Bash ls"}},
		{"permission prompt", stop, Event{Name: "Notification", NotificationType: "permission_prompt", Detail: "needs Bash"}, Record{Thread: "p/1", State: Waiting, Event: "Notification", At: t1, Detail: "needs Bash", Notify: "permission_prompt"}},
		{"prompt keeps the last tool", Record{Thread: "p/1", State: Working, Event: "PreToolUse", Tool: "Bash", Command: "yarn test"}, Event{Name: "Notification", NotificationType: "permission_prompt"}, Record{Thread: "p/1", State: Waiting, Event: "Notification", At: t1, Notify: "permission_prompt", Tool: "Bash", Command: "yarn test"}},
		{"new prompt forgets the tool", Record{Thread: "p/1", State: Working, Event: "PreToolUse", Tool: "Bash", Command: "yarn test"}, Event{Name: "UserPromptSubmit"}, Record{Thread: "p/1", State: Working, Event: "UserPromptSubmit", At: t1}},
		{"cwd from the event", Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0, Cwd: "/a"}, Event{Name: "PreToolUse", Cwd: "/b"}, Record{Thread: "p/1", State: Working, Event: "PreToolUse", At: t1, Cwd: "/b"}},
		{"conversation from the event", Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0, Conversation: "a"}, Event{Name: "UserPromptSubmit", Conversation: "b"}, Record{Thread: "p/1", State: Working, Event: "UserPromptSubmit", At: t1, Conversation: "b"}},
		{"conversation kept when the event has none", Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0, Conversation: "a"}, Event{Name: "UserPromptSubmit"}, Record{Thread: "p/1", State: Working, Event: "UserPromptSubmit", At: t1, Conversation: "a"}},
		{"transcript kept when the event has none", Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0, Transcript: "/t/a.jsonl"}, Event{Name: "UserPromptSubmit"}, Record{Thread: "p/1", State: Working, Event: "UserPromptSubmit", At: t1, Transcript: "/t/a.jsonl"}},
		{"cwd kept when the event has none", Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0, Cwd: "/a"}, Event{Name: "UserPromptSubmit"}, Record{Thread: "p/1", State: Working, Event: "UserPromptSubmit", At: t1, Cwd: "/a"}},
		{"identity kept", Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0, Identity: chat}, Event{Name: "UserPromptSubmit"}, Record{Thread: "p/1", State: Working, Event: "UserPromptSubmit", At: t1, Identity: chat}},
		{"identity kept on a record with no state", Record{Thread: "p/1", At: t0, Identity: chat}, Event{Name: "SessionStart"}, Record{Thread: "p/1", State: Idle, Event: "SessionStart", At: t1, Identity: chat}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Apply("p/1", tt.prev, tt.ev, t1)
			got.Changes = nil // TestApplyChanges covers them
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseAccess(t *testing.T) {
	tests := []struct {
		in      string
		want    Access
		wantErr bool
	}{
		{"", Supervised, false},
		{"read", Read, false},
		{"supervised", Supervised, false},
		{"full", Full, false},
		{"write", "", true},
		{"Read", "", true},
	}
	for _, tt := range tests {
		got, err := ParseAccess(tt.in)
		if got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("ParseAccess(%q) = %q, %v", tt.in, got, err)
		}
	}
	if !(Read.Rank() < Supervised.Rank() && Supervised.Rank() < Full.Rank()) {
		t.Errorf("ranks = %d, %d, %d, want Read < Supervised < Full", Read.Rank(), Supervised.Rank(), Full.Rank())
	}
}

func TestStoredAccess(t *testing.T) {
	for in, want := range map[string]Access{"": Read, "read": Read, "supervised": Supervised, "full": Full, "root": Read, "Full": Read} {
		if got := StoredAccess(in); got != want {
			t.Errorf("StoredAccess(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStateFileSavesSettledAt(t *testing.T) {
	data, err := json.Marshal(Record{Thread: "p/abcdefgh", State: Settled, SettledAt: 1234})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"settledAt":1234`) || strings.Contains(string(data), "endedAt") {
		t.Errorf("record written as %s, want settledAt and no endedAt", data)
	}
	var back Record
	if err := json.Unmarshal(data, &back); err != nil || back.SettledAt != 1234 {
		t.Errorf("record read back as %+v, %v", back, err)
	}
}

func TestApplyChanges(t *testing.T) {
	t0 := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	at := func(m int) time.Time { return t0.Add(time.Duration(m) * time.Minute) }
	ms := func(m int) int64 { return at(m).UnixMilli() }

	rec := Apply("p/1", Record{}, Event{Name: "SessionStart"}, at(0))
	rec = Apply("p/1", rec, Event{Name: "UserPromptSubmit"}, at(1))
	rec = Apply("p/1", rec, Event{Name: "PreToolUse"}, at(2))
	rec = Apply("p/1", rec, Event{Name: "Notification", NotificationType: "permission_prompt"}, at(3))
	rec = Apply("p/1", rec, Event{Name: "Stop"}, at(4))
	want := []Change{{Idle, ms(0)}, {Working, ms(1)}, {Waiting, ms(3)}, {Idle, ms(4)}}
	if !reflect.DeepEqual(rec.Changes, want) {
		t.Errorf("changes = %+v, want a change only when the state changes: %+v", rec.Changes, want)
	}

	old := Record{Thread: "p/1", State: Idle, Event: "Stop", At: t0}
	if got := Apply("p/1", old, Event{Name: "SessionStart"}, at(1)).Changes; !reflect.DeepEqual(got, []Change{{Idle, ms(1)}}) {
		t.Errorf("a record saved before changes were kept got %+v, want its timeline to start at the next event", got)
	}

	rec = Record{Thread: "p/1", State: Idle, At: t0}
	for i := range ChangesCap + 10 {
		ev := Event{Name: "UserPromptSubmit"}
		if rec.State == Working {
			ev = Event{Name: "Stop"}
		}
		rec = Apply("p/1", rec, ev, at(i+1))
	}
	if len(rec.Changes) != ChangesCap || rec.Changes[ChangesCap-1].At != ms(ChangesCap+10) {
		t.Errorf("kept %d changes ending at %d, want the latest %d", len(rec.Changes), rec.Changes[len(rec.Changes)-1].At, ChangesCap)
	}
}

func TestSort(t *testing.T) {
	t0 := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	mk := func(n int, s State, ago time.Duration) Thread {
		return Thread{Name: Name{Project: "p", Token: "t"}, N: n, State: s, At: t0.Add(-ago)}
	}
	ss := []Thread{
		mk(1, Idle, 0),
		mk(2, Working, time.Minute),
		mk(3, Idle, time.Hour),
		mk(4, Waiting, time.Hour),
		mk(5, Working, time.Second),
		mk(6, Idle, time.Minute),
		mk(7, Idle, 0),
		mk(8, Settled, 0),
	}
	Sort(ss)
	var got []int
	for _, s := range ss {
		got = append(got, s.N)
	}
	want := []int{4, 1, 7, 6, 3, 5, 2, 8}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestParseName(t *testing.T) {
	tests := []struct {
		in      string
		want    Name
		wantErr bool
	}{
		{"api/3", Name{Project: "api", Token: "3"}, false},
		{"my-app/12", Name{Project: "my-app", Token: "12"}, false},
		{"api/k3x9q0ab", Name{Project: "api", Token: "k3x9q0ab"}, false},
		{"api/shells", Name{Project: "api", Token: "shells"}, false},
		{"a/b/7", Name{}, true},
		{"../7", Name{}, true},
		{"a..b/7", Name{}, true},
		{"../../x/shell", Name{}, true},
		{"api", Name{}, true},
		{"/3", Name{}, true},
		{"api/", Name{}, true},
		{"api/X", Name{}, true},
		{"api/a_b", Name{}, true},
		{"api/a b", Name{}, true},
		{"api/a:b", Name{}, true},
		{"api/a.b", Name{}, true},
		{"api/shell", Name{Project: "api", Shell: 1}, false},
		{"api/shell-2", Name{Project: "api", Shell: 2}, false},
		{"api/shell-12", Name{Project: "api", Shell: 12}, false},
		{"api/shell-1", Name{}, true},
		{"api/shell-02", Name{}, true},
		{"api/shell-x", Name{}, true},
		{"a/b/shell", Name{}, true},
		{"/shell", Name{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseName(tt.in)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("ParseName(%q) = %+v, %v", tt.in, got, err)
			}
			if err == nil && got.String() != tt.in {
				t.Errorf("round trip = %q, want %q", got.String(), tt.in)
			}
			if err == nil && got.IsShell() != (strings.Contains(tt.in, "/shell") && got.Token == "") {
				t.Errorf("IsShell = %v for %q", got.IsShell(), tt.in)
			}
		})
	}
}

func TestNextN(t *testing.T) {
	tests := []struct {
		used []int
		want int
	}{{nil, 1}, {[]int{1, 2}, 3}, {[]int{2, 3}, 1}, {[]int{1, 3}, 2}}
	for _, tt := range tests {
		if got := NextN(tt.used); got != tt.want {
			t.Errorf("NextN(%v) = %d, want %d", tt.used, got, tt.want)
		}
	}
}

func TestNewToken(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		tok := NewToken()
		name, err := ParseName("p/" + tok)
		if err != nil || len(tok) != 8 || name.Token != tok || seen[tok] {
			t.Fatalf("NewToken() = %q, %+v, %v", tok, name, err)
		}
		seen[tok] = true
	}
}

func TestProjectKey(t *testing.T) {
	for in, want := range map[string]string{"api/3": "api", "api/shell": "api", "../3": "project", "nonsense": "project"} {
		if got := ProjectKey(in); got != want {
			t.Errorf("ProjectKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInitTellsTheConversationAndChangesNothingElse(t *testing.T) {
	at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	for _, state := range []State{Idle, Working, Waiting} {
		t.Run(string(state), func(t *testing.T) {
			prev := Record{Thread: "demo/abcdefgh", State: state, Event: "PreToolUse", At: at, Detail: "Bash make", Tool: "Bash", Command: "make", Changes: []Change{{State: state, At: 1}}, Observed: 4}
			got := Apply("demo/abcdefgh", prev, Event{Name: Init, Conversation: "c1", Cwd: "/w", Transcript: "/t/c1.jsonl", Model: "claude-haiku-5-5", Seq: 5}, at.Add(time.Second))
			want := prev
			want.At, want.Conversation, want.Cwd, want.Transcript, want.AgentModel, want.Observed = at.Add(time.Second), "c1", "/w", "/t/c1.jsonl", "claude-haiku-5-5", 5
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Apply = %+v, want %+v", got, want)
			}
		})
	}
	// An init that names less keeps what the record holds.
	prev := Record{Thread: "demo/abcdefgh", State: Working, At: at, Conversation: "c1", Transcript: "/t/c1.jsonl", Cwd: "/w"}
	if got := Apply("demo/abcdefgh", prev, Event{Name: Init}, at.Add(time.Second)); got.Conversation != "c1" || got.Transcript != "/t/c1.jsonl" || got.Cwd != "/w" {
		t.Errorf("an init that names nothing gave %+v", got)
	}
}

func TestApplyKeepsTheLatestSeq(t *testing.T) {
	at := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	rec := Apply("demo/abcdefgh", Record{}, Event{Name: "SessionStart", Seq: 1}, at)
	rec = Apply("demo/abcdefgh", rec, Event{Name: "UserPromptSubmit", Seq: 2}, at.Add(time.Second))
	if rec.Observed != 2 || rec.State != Working {
		t.Fatalf("record = %+v, want the seq 2 and working", rec)
	}
	// An event with no seq, one a test or an old build made, takes none back.
	if rec = Apply("demo/abcdefgh", rec, Event{Name: "PreToolUse"}, at.Add(2*time.Second)); rec.Observed != 2 {
		t.Errorf("Observed = %d after an event with no seq, want 2", rec.Observed)
	}
	if rec = Apply("demo/abcdefgh", rec, Event{Name: "Stop", Seq: 7}, at.Add(3*time.Second)); rec.Observed != 7 || rec.State != Idle {
		t.Errorf("record = %+v, want the seq 7 and idle", rec)
	}
}

func TestObservedSurvivesTheStateFile(t *testing.T) {
	data, err := json.Marshal(Record{Thread: "demo/abcdefgh", State: Idle, Observed: 12})
	if err != nil {
		t.Fatal(err)
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil || rec.Observed != 12 {
		t.Errorf("record = %+v, %v", rec, err)
	}
}
