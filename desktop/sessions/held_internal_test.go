package sessions

import (
	"errors"
	"slices"
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
		restarting bool
		told, cmd  bool // whether each is held
	}{
		{"idle", session.Record{State: session.Idle, Event: "Stop"}, Background{}, false, false, false, false},
		{"working", session.Record{State: session.Working, Event: "PreToolUse"}, Background{}, false, false, false, true},
		{"waiting mid-turn", session.Record{State: session.Waiting, Event: "Notification"}, Background{}, false, false, true, true},
		{"a reply that asks", session.Record{State: session.Waiting, Event: "Stop"}, Background{}, false, false, false, true},
		{"a subagent's approval after the turn", session.Record{State: session.Idle, Event: "Stop"}, Background{Subagents: 1, Asks: true}, false, false, true, true},
		{"an approval only the log shows", session.Record{State: session.Idle, Event: "Stop"}, Background{Asks: true}, false, false, true, false},
		{"going to sleep", session.Record{State: session.Idle, Event: "Stop"}, Background{}, true, false, true, true},
		{"restarting for a change of access", session.Record{State: session.Idle, Event: "Stop"}, Background{}, false, true, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSessions(Options{})
			s.records[id], s.background[id], s.idling[id], s.restarting[id] = tt.rec, tt.background, tt.idling, tt.restarting
			if got := s.holds(id, told); got != tt.told {
				t.Errorf("a told message held %v, want %v", got, tt.told)
			}
			if got := s.holds(id, command); got != tt.cmd {
				t.Errorf("a command held %v, want %v", got, tt.cmd)
			}
		})
	}
}

func TestDroppedMessageTellsItsSender(t *testing.T) {
	const id = "main/abcdefgh"
	for _, tt := range []struct {
		name   string
		drop   func(s *Sessions) int
		want   error
		result bool // whether the child's result is kept for the next turn
	}{
		{"a stop", func(s *Sessions) int { return s.DropTold(id) }, errToldStopped, true},
		{"a delete", func(s *Sessions) int { return s.dropTold(id, errToldDeleted) }, errToldDeleted, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSessions(Options{})
			done, resultDone := make(chan error, 1), make(chan error, 1)
			command := heldPrompt{Told: Told{Text: "/review"}, command: true}
			result := heldPrompt{Told: Told{From: "main/dddddddd", Text: "Task done", Result: true, Done: resultDone}}
			s.wakes[id] = []heldPrompt{{Told: Told{From: "main/bbbbbbbb", Text: "Check the tests", Done: done}}, command, result, {Told: Told{From: "main/cccccccc", Text: "No one asks"}}}
			want, held := 3, []heldPrompt{command}
			if tt.result {
				kept := result
				kept.nextTurn = true
				want, held = 2, []heldPrompt{command, kept}
			}
			if n := tt.drop(s); n != want {
				t.Errorf("dropped %d, want %d", n, want)
			}
			select {
			case err := <-done:
				if !errors.Is(err, tt.want) {
					t.Errorf("Done got %v, want %v", err, tt.want)
				}
			default:
				t.Error("Done got nothing")
			}
			if got := s.wakes[id]; !slices.Equal(got, held) {
				t.Errorf("held %+v, want %+v", got, held)
			}
			if tt.result && s.HoldsTold(id) {
				t.Error("a result kept for the next turn holds the thread's turn open")
			}
		})
	}
}

func TestResultKeptForTheNextTurnGoesWithIt(t *testing.T) {
	const id = "main/abcdefgh"
	s := newSessions(Options{})
	s.seen[id] = session.Idle
	s.records[id] = session.Record{State: session.Idle, Event: "Stop"}
	kept := heldPrompt{Told: Told{From: "main/dddddddd", Text: "Task done", Result: true}, nextTurn: true}
	s.wakes[id] = []heldPrompt{kept}
	s.mu.Lock()
	s.deliverWakes()
	s.mu.Unlock()
	if got := s.wakes[id]; !slices.Equal(got, []heldPrompt{kept}) || s.delivering[id] {
		t.Fatalf("held %+v, delivering %v; want the result kept until the next turn", got, s.delivering[id])
	}
	s.mu.Lock()
	delete(s.seen, id)
	s.deliverWakes()
	s.mu.Unlock()
	if got := s.wakes[id]; !slices.Equal(got, []heldPrompt{kept}) || s.delivering[id] {
		t.Fatalf("held %+v, delivering %v; want a sleeping thread left asleep", got, s.delivering[id])
	}
	s.seen[id] = session.Idle
	s.mu.Lock()
	s.delivering[id] = true // the send itself needs a host; its start is what counts here
	s.mu.Unlock()
	s.NextTurn(id)
	if got := s.wakes[id]; len(got) != 1 || got[0].nextTurn {
		t.Errorf("held %+v, want the result released for the next turn", got)
	}
}

func TestResultsOfADeletedChildAreDropped(t *testing.T) {
	const child, parent, other = "main/childaaa", "main/parentaa", "main/otheraaa"
	for _, tt := range []struct {
		name string
		held []heldPrompt
		want []heldPrompt
		gone int // how many senders hear of the drop
	}{
		{"a kept result", []heldPrompt{{Told: Told{From: child, Text: "Task done", Result: true}, nextTurn: true}}, nil, 1},
		{"another child's", []heldPrompt{{Told: Told{From: other, Text: "Task done", Result: true}, nextTurn: true}}, []heldPrompt{{Told: Told{From: other, Text: "Task done", Result: true}, nextTurn: true}}, 0},
		{"one about to go", []heldPrompt{{Told: Told{From: child, Text: "Check this"}}}, []heldPrompt{{Told: Told{From: child, Text: "Check this"}}}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSessions(Options{})
			done := make(chan error, len(tt.held))
			for i := range tt.held {
				tt.held[i].Done = done
			}
			s.wakes[parent] = tt.held
			s.mu.Lock()
			s.dropResultsOf(child)
			s.mu.Unlock()
			got := s.wakes[parent]
			for i := range got {
				got[i].Done = nil
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("held %+v, want %+v", got, tt.want)
			}
			if len(done) != tt.gone {
				t.Errorf("%d senders heard of a drop, want %d", len(done), tt.gone)
			}
			for range tt.gone {
				if err := <-done; !errors.Is(err, errToldDeleted) {
					t.Errorf("Done got %v, want %v", err, errToldDeleted)
				}
			}
		})
	}
}
