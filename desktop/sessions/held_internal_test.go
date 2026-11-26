package sessions

import (
	"testing"

	"github.com/nednella/agentos/internal/session"
)

func TestHeldPromptsWaitForTheirTurn(t *testing.T) {
	const id = "main/abcdefgh"
	told, command := heldPrompt{Told: Told{From: "main/bbbbbbbb", Text: "Check the tests"}}, heldPrompt{Told: Told{Text: "/review"}, command: true}
	for _, tt := range []struct {
		name       string
		rec        session.Record
		background Background
		idling     bool
		told, cmd  bool // whether each is held
	}{
		{"idle", session.Record{State: session.Idle, Event: "Stop"}, Background{}, false, false, false},
		{"working", session.Record{State: session.Working, Event: "PreToolUse"}, Background{}, false, false, true},
		{"waiting mid-turn", session.Record{State: session.Waiting, Event: "Notification"}, Background{}, false, true, true},
		{"a reply that asks", session.Record{State: session.Waiting, Event: "Stop"}, Background{}, false, false, true},
		{"a subagent's approval after the turn", session.Record{State: session.Idle, Event: "Stop"}, Background{Subagents: 1, Asks: true}, false, true, true},
		{"an approval only the log shows", session.Record{State: session.Idle, Event: "Stop"}, Background{Asks: true}, false, true, false},
		{"going to sleep", session.Record{State: session.Idle, Event: "Stop"}, Background{}, true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSessions(Options{})
			s.records[id], s.background[id], s.idling[id] = tt.rec, tt.background, tt.idling
			if got := s.holds(id, told); got != tt.told {
				t.Errorf("a told message held %v, want %v", got, tt.told)
			}
			if got := s.holds(id, command); got != tt.cmd {
				t.Errorf("a command held %v, want %v", got, tt.cmd)
			}
		})
	}
}
