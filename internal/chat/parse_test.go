package chat

import (
	"bufio"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

func parseFile(t *testing.T, name string) []Event {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		evs, err := Parse(sc.Bytes())
		if err != nil {
			t.Fatalf("%s: %v", sc.Text(), err)
		}
		events = append(events, evs...)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func first(events []Event, kind Kind) (Event, bool) {
	for _, e := range events {
		if e.Kind == kind {
			return e, true
		}
	}
	return Event{}, false
}

func TestParseReplyStreamsDeltasThenMessage(t *testing.T) {
	events := parseFile(t, "reply.jsonl")
	if len(events) == 0 || events[0].Kind != KindInit || events[0].ID == "" {
		t.Fatalf("want init with a session id first, got %+v", events)
	}
	if last := events[len(events)-1]; last.Kind != KindTurnEnd || last.Cost <= 0 || last.Error {
		t.Fatalf("want a costed turn_end last, got %+v", last)
	}
	var deltas strings.Builder
	n := 0
	for _, e := range events {
		if e.Kind == KindDelta {
			n++
			deltas.WriteString(e.Text)
		}
	}
	if n == 0 {
		t.Fatal("no delta")
	}
	msg, ok := first(events, KindMessage)
	if !ok || msg.ID == "" {
		t.Fatalf("no message with an id in %+v", events)
	}
	if msg.UUID == "" {
		t.Error("message has no uuid")
	}
	for _, e := range events {
		if e.Kind == KindDelta && e.ID != msg.ID {
			t.Errorf("delta id %q, message id %q", e.ID, msg.ID)
		}
	}
	if msg.Text != deltas.String() {
		t.Errorf("message %q, deltas %q", msg.Text, deltas.String())
	}
}

func TestParseToolCallPairsUseAndResult(t *testing.T) {
	events := parseFile(t, "tool.jsonl")
	use, ok := first(events, KindToolUse)
	if !ok || use.Tool != "Bash" || use.ID == "" {
		t.Fatalf("want a Bash tool_use, got %+v", use)
	}
	res, ok := first(events, KindToolResult)
	if !ok {
		t.Fatal("no tool_result")
	}
	if res.Parent != use.ID || res.Error || !strings.Contains(res.Text, "note.txt") {
		t.Errorf("tool_result %+v does not answer %+v", res, use)
	}
	for _, e := range events {
		if e.Kind == KindDelta && strings.Contains(e.Text, "command") {
			t.Errorf("tool input leaked as a delta: %+v", e)
		}
	}
}

func TestParseApprovalRequest(t *testing.T) {
	events := parseFile(t, "approval.jsonl")
	ap, ok := first(events, KindApproval)
	if !ok || ap.ID == "" || ap.Tool != "Bash" || !strings.Contains(string(ap.Input), `"command"`) {
		t.Fatalf("want a Bash approval with a command, got %+v", ap)
	}
	use, _ := first(events, KindToolUse)
	if ap.Parent == "" || ap.Parent != use.ID {
		t.Errorf("want the approval's parent to be its tool call %q, got %q", use.ID, ap.Parent)
	}
}

func TestParseInterruptEndsTurnWithError(t *testing.T) {
	events := parseFile(t, "interrupt.jsonl")
	last := events[len(events)-1]
	if last.Kind != KindTurnEnd || !last.Error {
		t.Errorf("want an error turn_end last, got %+v", last)
	}
	if e, ok := first(events, KindUser); ok {
		t.Errorf("the host records user lines itself, got %+v", e)
	}
}

func TestParseGivesTheErrorsOfAFailedStart(t *testing.T) {
	events, err := Parse([]byte(`{"type":"result","subtype":"error_during_execution","is_error":true,"num_turns":0,"errors":["No message found with message.uuid of: 1203a261"]}`))
	if err != nil || len(events) != 1 || events[0].Kind != KindTurnEnd || !events[0].Error || events[0].Text != "No message found with message.uuid of: 1203a261" {
		t.Errorf("got %+v, %v", events, err)
	}
}

func TestParseIgnoresUnknownTypes(t *testing.T) {
	events, err := Parse([]byte(`{"type":"rate_limit_event"}`))
	if err != nil || len(events) != 0 {
		t.Errorf("got %v, %v", events, err)
	}
}

func TestParseRejectsBrokenJSON(t *testing.T) {
	if _, err := Parse([]byte(`{"type":`)); err == nil {
		t.Error("want an error")
	}
}

func TestPersistedSkipsOnlyDeltas(t *testing.T) {
	if (Event{Kind: KindDelta}).Persisted() || !(Event{Kind: KindMessage}).Persisted() {
		t.Error("only a delta is not persisted")
	}
}

func TestParseAcceptsStringUserContent(t *testing.T) {
	for _, in := range []string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		`{"type":"control_request","request_id":"r"}`,
		`{"type":"stream_event"}`,
	} {
		events, err := Parse([]byte(in))
		if err != nil || len(events) != 0 {
			t.Errorf("%s: got %v, %v", in, events, err)
		}
	}
}

func TestParseToolResultWithTextBlocks(t *testing.T) {
	events, err := Parse([]byte(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}]}}`))
	if err != nil || len(events) != 1 || events[0].Parent != "t1" || events[0].Text != "a\nb" {
		t.Errorf("got %+v, %v", events, err)
	}
}

func TestParseAssistantKeepsBlockOrder(t *testing.T) {
	events, err := Parse([]byte(`{"type":"assistant","uuid":"u","message":{"id":"m","content":[{"type":"text","text":"one"},{"type":"tool_use","id":"t","name":"Bash","input":{}},{"type":"text","text":"two"}]}}`))
	if err != nil || len(events) != 3 {
		t.Fatalf("got %+v, %v", events, err)
	}
	kinds := []Kind{events[0].Kind, events[1].Kind, events[2].Kind}
	if kinds[0] != KindMessage || kinds[1] != KindToolUse || kinds[2] != KindMessage || events[0].Text != "one" || events[2].Text != "two" {
		t.Errorf("got %+v", events)
	}
}

func TestParseInitializeListsCommands(t *testing.T) {
	events := parseFile(t, "initialize.jsonl")
	var got []Event
	for _, e := range events {
		if e.Kind == KindCommands {
			got = append(got, e)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one commands event, got %d", len(got))
	}
	var commands []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(got[0].Input, &commands); err != nil || len(commands) == 0 || commands[0].Name == "" {
		t.Fatalf("want an array of commands with names, got %s (%v)", got[0].Input, err)
	}
}

func TestParseQuestionApproval(t *testing.T) {
	events := parseFile(t, "question.jsonl")
	ev, ok := first(events, KindApproval)
	if !ok || ev.Tool != "AskUserQuestion" || !strings.Contains(string(ev.Input), `"questions"`) {
		t.Fatalf("want an AskUserQuestion approval with questions, got %+v", ev)
	}
	if use, _ := first(events, KindToolUse); ev.Parent != use.ID {
		t.Errorf("want the approval's parent to be its tool call %q, got %q", use.ID, ev.Parent)
	}
}

func TestParseCompactBoundaryIsACompaction(t *testing.T) {
	var kinds []Kind
	for _, e := range parseFile(t, "compact.jsonl") {
		kinds = append(kinds, e.Kind)
	}
	if want := []Kind{KindCompact, KindInit, KindTurnEnd}; !slices.Equal(kinds, want) {
		t.Errorf("kinds = %v, want %v", kinds, want)
	}
}

func TestParseReplayedUserIsAnEcho(t *testing.T) {
	events := parseFile(t, "queued.jsonl")
	var echoes []Event
	for _, e := range events {
		if e.Kind == KindEcho {
			echoes = append(echoes, e)
		}
	}
	if len(echoes) != 2 || echoes[0].ID != "11111111-1111-4111-8111-111111111111" || echoes[1].ID != "22222222-2222-4222-8222-222222222222" {
		t.Fatalf("want an echo of each message by its uuid, got %+v", echoes)
	}
}

func TestParseInterruptReplyListsWhatIsStillQueued(t *testing.T) {
	ev, ok := first(parseFile(t, "interrupt.jsonl"), KindStillQueued)
	if !ok || ev.ID != "req_int" || string(ev.Input) != "[]" {
		t.Fatalf("want the interrupt's reply with an empty list, got %+v", ev)
	}
	events, err := Parse([]byte(`{"type":"control_response","response":{"subtype":"success","request_id":"i2","response":{"still_queued":["2222"]}}}`))
	if err != nil || len(events) != 1 || events[0].Kind != KindStillQueued || string(events[0].Input) != `["2222"]` {
		t.Fatalf("got %+v, %v", events, err)
	}
	if events, _ := Parse([]byte(`{"type":"control_response","response":{"subtype":"success","request_id":"i3","response":{}}}`)); len(events) != 0 {
		t.Fatalf("an older claude's empty reply gave %+v, want none", events)
	}
}
