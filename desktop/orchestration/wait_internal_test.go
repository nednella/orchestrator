package orchestration

import (
	"bufio"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	chatlog "github.com/nednella/orchestrator/internal/chat"
)

func TestTurnInReadsTheLastTurn(t *testing.T) {
	t.Parallel()
	user := func(id string) chatlog.Event { return chatlog.Event{Kind: chatlog.KindUser, ID: id, Text: "go"} }
	reply := func(text string) chatlog.Event {
		return chatlog.Event{Kind: chatlog.KindMessage, ID: "msg-" + text, Text: text}
	}
	end := chatlog.Event{Kind: chatlog.KindTurnEnd}
	long := strings.Repeat("é", 3000)
	for _, tc := range []struct {
		name   string
		events []chatlog.Event
		want   turn
	}{
		{"an empty log", nil, turn{outcome: outcomeCompleted}},
		{"a turn that runs", []chatlog.Event{user("u1")}, turn{user: 1, outcome: outcomeCompleted}},
		{"a turn that ended", []chatlog.Event{user("u1"), reply("first"), reply("done"), end}, turn{user: 1, end: 4, ended: true, outcome: outcomeCompleted, reply: "done"}},
		{"a reply before the last message is not its", []chatlog.Event{user("u1"), reply("old"), end, user("u2"), end}, turn{user: 4, end: 5, ended: true, outcome: outcomeCompleted}},
		{"a long reply is cut", []chatlog.Event{user("u1"), reply(long), end}, turn{user: 1, end: 3, ended: true, outcome: outcomeCompleted, reply: strings.Repeat("é", textMost) + cutMark}},
		{"a queued message waits", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, end}, turn{user: 1, end: 3, outcome: outcomeCompleted}},
		{"a queued message taken in", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, end, user("q1"), reply("done"), end}, turn{user: 4, end: 6, ended: true, outcome: outcomeCompleted, reply: "done"}},
		{"a queued message unsent", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, {Kind: chatlog.KindUnsent, ID: "q1"}, end}, turn{user: 1, end: 4, ended: true, outcome: outcomeCompleted}},
		{"an earlier run's queued message", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, {Kind: chatlog.KindExit}, {Kind: chatlog.KindStart}, user("u2"), end}, turn{user: 5, exit: 3, end: 6, ended: true, outcome: outcomeCompleted}},
		{"a stopped turn", []chatlog.Event{user("u1"), {Kind: chatlog.KindInterrupt}, {Kind: chatlog.KindTurnEnd, Error: true}}, turn{user: 1, end: 3, ended: true, outcome: outcomeStopped}},
		{"an interrupt of an earlier turn", []chatlog.Event{user("u1"), {Kind: chatlog.KindInterrupt}, end, user("u2"), end}, turn{user: 4, end: 5, ended: true, outcome: outcomeCompleted}},
		{"a failed turn", []chatlog.Event{user("u1"), {Kind: chatlog.KindTurnEnd, Error: true, Text: "API Error: 529"}}, turn{user: 1, end: 2, ended: true, outcome: outcomeFailed, reason: "API Error: 529"}},
		{"an approval waits", []chatlog.Event{user("u1"), {Kind: chatlog.KindApproval, ID: "ask-1", Input: json.RawMessage(`{}`)}}, turn{user: 1, outcome: outcomeCompleted, asks: true}},
		{"an approval answered", []chatlog.Event{user("u1"), {Kind: chatlog.KindApproval, ID: "ask-1"}, {Kind: chatlog.KindAnswer, ID: "ask-1", Text: "allow"}}, turn{user: 1, outcome: outcomeCompleted}},
		{"claude exited", []chatlog.Event{user("u1"), {Kind: chatlog.KindExit, Error: true}}, turn{user: 1, exit: 2, outcome: outcomeCompleted, ran: true}},
		{"the thread settled", []chatlog.Event{user("u1"), end, {Kind: chatlog.KindExit, Stop: chatlog.StopSettle}}, turn{user: 1, exit: 3, end: 2, ended: true, outcome: outcomeCompleted, ran: true, settled: true}},
		{"the owner put the thread to sleep", []chatlog.Event{user("u1"), {Kind: chatlog.KindExit, Stop: chatlog.StopSettle, Text: chatlog.ByOwner}}, turn{user: 1, exit: 2, outcome: outcomeCompleted, ran: true, settled: true, byOwner: true}},
		{"the owner's sleep of an earlier run", []chatlog.Event{user("u1"), {Kind: chatlog.KindExit, Stop: chatlog.StopSettle, Text: chatlog.ByOwner}, {Kind: chatlog.KindStart}}, turn{user: 1, exit: 2, outcome: outcomeCompleted}},
		{"a turn a thread stopped", []chatlog.Event{user("u1"), {Kind: chatlog.KindInterrupt, From: "main/parentaa"}, {Kind: chatlog.KindTurnEnd, Error: true}}, turn{user: 1, end: 3, ended: true, outcome: outcomeStopped, stopper: "main/parentaa"}},
		{"a shell runs in the background", []chatlog.Event{user("u1"), {Kind: chatlog.KindTaskStart, Parent: "t1", ID: "b1", Tool: chatlog.TaskShell}, end}, turn{user: 1, end: 3, ended: true, outcome: outcomeCompleted, open: 1}},
		{"a woken run", []chatlog.Event{user("u1"), end, {Kind: chatlog.KindExit, Stop: chatlog.StopSettle}, {Kind: chatlog.KindStart}}, turn{user: 1, exit: 3, end: 2, ended: true, outcome: outcomeCompleted}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := range tc.events {
				tc.events[i].Seq = int64(i + 1)
			}
			tc.want.newest = int64(len(tc.events))
			if got := turnIn(tc.events); got != tc.want {
				t.Errorf("turnIn = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestTurnResultIsWhatEndsTheLastTurn(t *testing.T) {
	t.Parallel()
	user := chatlog.Event{Kind: chatlog.KindUser, ID: "u1", Text: "go"}
	end := chatlog.Event{Kind: chatlog.KindTurnEnd}
	settle := chatlog.Event{Kind: chatlog.KindExit, Stop: chatlog.StopSettle}
	for _, tc := range []struct {
		name   string
		events []chatlog.Event
		want   int64
	}{
		{"no turn", nil, 0},
		{"a turn that runs", []chatlog.Event{user}, 0},
		{"a turn that ended", []chatlog.Event{user, end}, 2},
		{"a turn that ended and then slept", []chatlog.Event{user, end, settle}, 2},
		{"a turn put to sleep", []chatlog.Event{user, settle}, 2},
		{"a turn whose Claude crashed", []chatlog.Event{user, {Kind: chatlog.KindExit, Error: true}}, 0},
		{"a turn put to sleep, then woken", []chatlog.Event{user, settle, {Kind: chatlog.KindStart}}, 0},
		{"a message after the end", []chatlog.Event{user, end, {Kind: chatlog.KindUser, ID: "u2"}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := range tc.events {
				tc.events[i].Seq = int64(i + 1)
			}
			if got := turnIn(tc.events).result(); got != tc.want {
				t.Errorf("result = %d, want %d", got, tc.want)
			}
		})
	}
}

// loggedFrom is the chat log a host keeps of the recorded Claude output in the file name of internal/chat's testdata, after
// the owner's message that started it.
func loggedFrom(t *testing.T, name string) []chatlog.Event {
	t.Helper()
	f, err := os.Open("../../internal/chat/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events := []chatlog.Event{{Seq: 1, Kind: chatlog.KindUser, ID: "u1", Text: "go"}}
	var tasks chatlog.Tasks
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		parsed, err := chatlog.Parse(sc.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range parsed {
			if !e.Persisted() {
				continue
			}
			// The host names a task's subagent as it logs it.
			e = tasks.Named(e)
			tasks.See(e)
			e.Seq = int64(len(events) + 1)
			events = append(events, e)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func TestTurnInWaitsForTheBackgroundSubagents(t *testing.T) {
	t.Parallel()
	events := loggedFrom(t, "background.jsonl")
	var atEnds []bool
	for i, e := range events {
		switch e.Kind {
		case chatlog.KindTurnEnd:
			atEnds = append(atEnds, turnIn(events[:i+1]).ended)
		case chatlog.KindTaskEnd:
			if e.Agent == "" && turnIn(events[:i+1]).ended {
				t.Errorf("done at the end of a task of Claude's own (seq %d), before the turn that tells of it", e.Seq)
			}
		}
	}
	// The owner's turn ends while the subagent works; the turns for the subagent's end, its second end and the shell's
	// end each end with no subagent working.
	if want := []bool{false, true, true, true}; !slices.Equal(atEnds, want) {
		t.Errorf("ended at each turn_end = %v, want %v", atEnds, want)
	}
}

func TestOutcomeTextSaysWhoPutTheChildToSleep(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		t      turn
		asleep bool
		want   string
	}{
		{"the owner's sleep", turn{settled: true, byOwner: true}, true, "was put to sleep by the owner"},
		{"the app's sleep", turn{settled: true}, true, "was put to sleep"},
		{"a turn that ended before the sleep", turn{settled: true, byOwner: true, outcome: outcomeCompleted}, false, "completed"},
		{"the owner's stop", turn{outcome: outcomeStopped}, false, "was stopped by the owner"},
		{"a failure", turn{outcome: outcomeFailed, reason: "API Error: 529"}, false, "failed: API Error: 529"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := outcomeText(tc.t, tc.asleep); got != tc.want {
				t.Errorf("outcomeText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestThreadDeletedForgetsItsResults(t *testing.T) {
	t.Parallel()
	const parent, child, other = "main/parentaa", "main/childaaa", "main/otheraaa"
	for _, tc := range []struct {
		name    string
		deleted string
		want    []waitOn
	}{
		{"the child", child, []waitOn{{parent, other}}},
		{"the parent", parent, []waitOn{{child, other}}},
		{"a thread with no results", "main/nobodyaa", []waitOn{{child, other}, {parent, child}, {parent, other}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			all := []waitOn{{parent, child}, {parent, other}, {child, other}}
			c := &Commands{given: map[waitOn]givenBy{}, waiting: map[waitOn]held{}}
			for i, w := range all {
				c.given[w] = givenBy{end: int64(i + 1)}
				c.waiting[w] = held{from: int64(i + 1)}
			}
			c.ThreadDeleted(tc.deleted)
			byName := func(a, b waitOn) int { return strings.Compare(a.waiter+a.thread, b.waiter+b.thread) }
			if got := slices.SortedFunc(maps.Keys(c.given), byName); !slices.Equal(got, tc.want) {
				t.Errorf("given has %v, want %v", got, tc.want)
			}
			if got := slices.SortedFunc(maps.Keys(c.waiting), byName); !slices.Equal(got, tc.want) {
				t.Errorf("waiting has %v, want %v", got, tc.want)
			}
		})
	}
}
