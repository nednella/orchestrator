package orchestration

import (
	"encoding/json"
	"strings"
	"testing"

	chatlog "github.com/nednella/agentos/internal/chat"
)

func TestTurnInReadsTheLastTurn(t *testing.T) {
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
		{"a turn that runs", []chatlog.Event{user("u1")}, turn{outcome: outcomeCompleted}},
		{"a turn that ended", []chatlog.Event{user("u1"), reply("first"), reply("done"), end}, turn{ended: true, outcome: outcomeCompleted, reply: "done"}},
		{"a reply before the last message is not its", []chatlog.Event{user("u1"), reply("old"), end, user("u2"), end}, turn{ended: true, outcome: outcomeCompleted}},
		{"a long reply is cut", []chatlog.Event{user("u1"), reply(long), end}, turn{ended: true, outcome: outcomeCompleted, reply: strings.Repeat("é", textMost) + cutMark}},
		{"a queued message waits", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, end}, turn{outcome: outcomeCompleted}},
		{"a queued message taken in", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, end, user("q1"), reply("done"), end}, turn{ended: true, outcome: outcomeCompleted, reply: "done"}},
		{"a queued message unsent", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, {Kind: chatlog.KindUnsent, ID: "q1"}, end}, turn{ended: true, outcome: outcomeCompleted}},
		{"an earlier run's queued message", []chatlog.Event{user("u1"), {Kind: chatlog.KindQueued, ID: "q1"}, {Kind: chatlog.KindExit}, {Kind: chatlog.KindStart}, user("u2"), end}, turn{ended: true, outcome: outcomeCompleted}},
		{"a stopped turn", []chatlog.Event{user("u1"), {Kind: chatlog.KindInterrupt}, {Kind: chatlog.KindTurnEnd, Error: true}}, turn{ended: true, outcome: outcomeStopped}},
		{"an interrupt of an earlier turn", []chatlog.Event{user("u1"), {Kind: chatlog.KindInterrupt}, end, user("u2"), end}, turn{ended: true, outcome: outcomeCompleted}},
		{"a failed turn", []chatlog.Event{user("u1"), {Kind: chatlog.KindTurnEnd, Error: true, Text: "API Error: 529"}}, turn{ended: true, outcome: outcomeFailed, reason: "API Error: 529"}},
		{"an approval waits", []chatlog.Event{user("u1"), {Kind: chatlog.KindApproval, ID: "ask-1", Input: json.RawMessage(`{}`)}}, turn{outcome: outcomeCompleted, asks: true}},
		{"an approval answered", []chatlog.Event{user("u1"), {Kind: chatlog.KindApproval, ID: "ask-1"}, {Kind: chatlog.KindAnswer, ID: "ask-1", Text: "allow"}}, turn{outcome: outcomeCompleted}},
		{"claude exited", []chatlog.Event{user("u1"), {Kind: chatlog.KindExit, Error: true}}, turn{outcome: outcomeCompleted, ran: true}},
		{"the thread settled", []chatlog.Event{user("u1"), end, {Kind: chatlog.KindExit, Stop: chatlog.StopSettle}}, turn{ended: true, outcome: outcomeCompleted, ran: true, settled: true}},
		{"a woken run", []chatlog.Event{user("u1"), end, {Kind: chatlog.KindExit, Stop: chatlog.StopSettle}, {Kind: chatlog.KindStart}}, turn{ended: true, outcome: outcomeCompleted}},
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
