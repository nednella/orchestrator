package threads

import (
	"testing"

	"github.com/nednella/orchestrator/internal/chat"
)

func TestLatestReplyIsTheLastFinishedOne(t *testing.T) {
	user := func(id string) chat.Event { return chat.Event{Kind: chat.KindUser, ID: id} }
	reply := func(uuid string) chat.Event { return chat.Event{Kind: chat.KindMessage, UUID: uuid} }
	end := chat.Event{Kind: chat.KindTurnEnd}
	tests := []struct {
		name      string
		events    []chat.Event
		want      int
		compacted bool
	}{
		{"none", []chat.Event{user("u1")}, -1, false},
		{"a turn that runs", []chat.Event{user("u1"), reply("r1")}, -1, false},
		{"the last block of the last ended turn", []chat.Event{user("u1"), reply("r1a"), reply("r1b"), end, user("u2"), reply("r2")}, 2, false},
		{"a subagent's reply", []chat.Event{user("u1"), reply("r1"), {Kind: chat.KindMessage, UUID: "s1", Agent: "call"}, end}, 1, false},
		{"a rewind takes its message back", []chat.Event{user("u1"), reply("r1"), end, user("u2"), reply("r2"), end, {Kind: chat.KindRewind, ID: "u2"}}, 1, false},
		{"from before a compaction", []chat.Event{user("u1"), reply("r1"), end, {Kind: chat.KindCompact}, user("u2")}, -1, true},
		{"after a compaction", []chat.Event{user("u1"), reply("r1"), end, {Kind: chat.KindCompact}, user("u2"), reply("r2"), end}, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, compacted := latestReply(tt.events); got != tt.want || compacted != tt.compacted {
				t.Errorf("latestReply = %d, %v; want %d, %v", got, compacted, tt.want, tt.compacted)
			}
		})
	}
}
