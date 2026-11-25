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

func TestPersistedSkipsWhatOnlyStreams(t *testing.T) {
	for _, tt := range []struct {
		e    Event
		want bool
	}{
		{Event{Kind: KindDelta, Text: "a"}, false},
		{Event{Kind: KindThinking, Text: "a"}, false},
		{Event{Kind: KindThought}, false},
		{Event{Kind: KindThought, Text: "a"}, true},
		{Event{Kind: KindMessage, Text: "a"}, true},
	} {
		if got := tt.e.Persisted(); got != tt.want {
			t.Errorf("%+v: Persisted() = %v, want %v", tt.e, got, tt.want)
		}
	}
}

func TestParseThinkingStreamsThenLogsTheThought(t *testing.T) {
	events := parseFile(t, "thinking.jsonl")
	var thoughts []Event
	streamed := map[string]*strings.Builder{}
	ended := map[string]bool{}
	for _, e := range events {
		switch e.Kind {
		case KindThinking:
			if ended[e.ID] {
				t.Errorf("block %s streams after its thought", e.ID)
			}
			if streamed[e.ID] == nil {
				if e.Text != "" {
					t.Errorf("block %s streamed %q before its start", e.ID, e.Text)
				}
				streamed[e.ID] = &strings.Builder{}
			}
			streamed[e.ID].WriteString(e.Text)
		case KindThought:
			thoughts = append(thoughts, e)
			ended[e.ID] = true
		case KindDelta:
			if !ended[e.ID] {
				t.Errorf("message %s streams its reply before its thought", e.ID)
			}
		}
	}
	if len(thoughts) != 2 || thoughts[0].ID == thoughts[1].ID {
		t.Fatalf("want a thought for each of the two messages, got %+v", thoughts)
	}
	for _, th := range thoughts {
		if th.Text == "" || streamed[th.ID] == nil || th.Text != streamed[th.ID].String() {
			t.Errorf("thought %s is %q, its stream %v", th.ID, th.Text, streamed[th.ID])
		}
		if !th.Persisted() {
			t.Errorf("thought %s is not logged", th.ID)
		}
	}
	if !strings.HasPrefix(thoughts[0].Text, "I need to read notes.txt") {
		t.Errorf("first thought %q", thoughts[0].Text)
	}
}

func TestParseHiddenThinkingOnlyStartsAndEnds(t *testing.T) {
	var events []Event
	for _, in := range []string{
		`{"type":"stream_event","api_message_id":"m","event":{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}}`,
		`{"type":"stream_event","api_message_id":"m","event":{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"","estimated_tokens":50}}}`,
		`{"type":"stream_event","api_message_id":"m","event":{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"c2ln"}}}`,
		`{"type":"assistant","uuid":"u","message":{"id":"m","content":[{"type":"thinking","thinking":"","signature":"c2ln"}]}}`,
		`{"type":"stream_event","api_message_id":"m","event":{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}}`,
		`{"type":"assistant","uuid":"u","message":{"id":"m","content":[{"type":"redacted_thinking","data":"x"}]}}`,
	} {
		evs, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		events = append(events, evs...)
	}
	want := []Event{{Kind: KindThinking, ID: "m"}, {Kind: KindThought, ID: "m"}}
	if !slices.EqualFunc(events, want, func(a, b Event) bool { return a.Kind == b.Kind && a.ID == b.ID && a.Text == b.Text }) {
		t.Errorf("got %+v, want %+v", events, want)
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
	if events, _ := Parse([]byte(`{"type":"control_response","response":{"subtype":"success","request_id":"i3","response":{}}}`)); len(events) != 1 || events[0].Kind != KindReply {
		t.Fatalf("an older claude's empty reply gave %+v, want a plain reply", events)
	}
}

func TestParseModeChange(t *testing.T) {
	events := parseFile(t, "mode.jsonl")
	var replies, modes []Event
	for _, e := range events {
		switch e.Kind {
		case KindReply:
			if strings.HasPrefix(e.ID, "mode-") {
				replies = append(replies, e)
			}
		case KindMode:
			modes = append(modes, e)
		}
	}
	if len(replies) != 3 || replies[0].ID != "mode-1" || replies[0].Error || replies[1].ID != "mode-2" || replies[1].Error {
		t.Fatalf("want a plain reply to mode-1 and mode-2, got %+v", replies)
	}
	if r := replies[2]; r.ID != "mode-3" || !r.Error || !strings.Contains(r.Text, "bypassPermissions") {
		t.Errorf("want mode-3 refused with Claude's reason, got %+v", r)
	}
	if len(modes) != 2 || modes[0].Text != "default" || modes[1].Text != "plan" {
		t.Errorf("want the modes default then plan, got %+v", modes)
	}
}

func TestParsePlanExit(t *testing.T) {
	events := parseFile(t, "planexit.jsonl")
	exit := slices.IndexFunc(events, func(e Event) bool { return e.Kind == KindApproval && e.Tool == "ExitPlanMode" })
	if exit < 0 {
		t.Fatal("no ExitPlanMode approval")
	}
	if e, ok := first(events[:exit], KindMode); ok {
		t.Errorf("a mode before the plan was approved: %+v", e)
	}
	if e, ok := first(events[exit:], KindMode); !ok || e.Text != "default" {
		t.Errorf("want the mode default after the plan, got %+v (%v)", e, ok)
	}
}

func TestParseStatusWithoutAModeGivesNone(t *testing.T) {
	events, err := Parse([]byte(`{"type":"system","subtype":"status","status":"requesting","session_id":"s"}`))
	if err != nil || len(events) != 0 {
		t.Errorf("got %+v, %v", events, err)
	}
}

// agentCalls is the Agent tool calls of the thread's own Claude, by id.
func agentCalls(t *testing.T, events []Event) map[string]Event {
	t.Helper()
	calls := map[string]Event{}
	for _, e := range events {
		if e.Kind == KindToolUse && e.Tool == "Agent" && e.Agent == "" {
			calls[e.ID] = e
		}
	}
	if len(calls) != 2 {
		t.Fatalf("want two Agent calls, got %+v", calls)
	}
	return calls
}

func TestParseSubagentEventsNameTheirAgentCall(t *testing.T) {
	events := parseFile(t, "subagents.jsonl")
	calls := agentCalls(t, events)
	for _, call := range calls {
		var input struct {
			Description, Prompt string
			Type                string `json:"subagent_type"`
		}
		if err := json.Unmarshal(call.Input, &input); err != nil || input.Description == "" || input.Prompt == "" || input.Type != "Explore" {
			t.Errorf("Agent call input %s: %+v, %v", call.Input, input, err)
		}
	}
	kinds := map[string][]Kind{}
	for _, e := range events {
		if e.Agent == "" {
			continue
		}
		if _, ok := calls[e.Agent]; !ok {
			t.Errorf("%+v names no Agent call", e)
		}
		kinds[e.Agent] = append(kinds[e.Agent], e.Kind)
	}
	for id := range calls {
		for _, want := range []Kind{KindToolUse, KindToolResult, KindMessage} {
			if !slices.Contains(kinds[id], want) {
				t.Errorf("subagent %s sent no %s: %v", id, want, kinds[id])
			}
		}
		// Claude streams no partial message of a subagent's.
		if slices.Contains(kinds[id], KindDelta) || slices.Contains(kinds[id], KindThinking) {
			t.Errorf("subagent %s streamed: %v", id, kinds[id])
		}
	}
	for _, e := range events {
		if e.Kind == KindTurnEnd && e.Agent != "" {
			t.Errorf("a turn's end names a subagent: %+v", e)
		}
	}
}

func TestParseBackgroundSubagentsEndAfterTheTurn(t *testing.T) {
	events := parseFile(t, "subagents.jsonl")
	calls := agentCalls(t, events)
	turnEnd := slices.IndexFunc(events, func(e Event) bool { return e.Kind == KindTurnEnd })
	ended := map[string]string{}
	for i, e := range events {
		if e.Kind != KindTaskEnd {
			continue
		}
		if e.Error || e.Stop != "" || e.Agent != "" {
			t.Errorf("want a completed end, got %+v", e)
		}
		if i < turnEnd {
			t.Errorf("a background subagent ended before the turn that started it: %+v", e)
		}
		ended[e.Parent] = e.Text
	}
	for id := range calls {
		if ended[id] == "" {
			t.Errorf("subagent %s has no end with its report: %v", id, ended)
		}
	}
}

func TestParseWaitedForSubagentsEndBeforeTheirCallsResult(t *testing.T) {
	events := parseFile(t, "subagents-wait.jsonl")
	calls := agentCalls(t, events)
	for id := range calls {
		end := slices.IndexFunc(events, func(e Event) bool { return e.Kind == KindTaskEnd && e.Parent == id })
		result := slices.IndexFunc(events, func(e Event) bool { return e.Kind == KindToolResult && e.Parent == id })
		if end < 0 || result < 0 || end > result || events[end].Text == "" {
			t.Errorf("subagent %s: end at %d, result at %d", id, end, result)
		}
	}
}

func TestParseTaskEndSaysHowTheTaskEnded(t *testing.T) {
	for _, tt := range []struct {
		status string
		want   []Event
	}{
		{"completed", []Event{{Kind: KindTaskEnd, Parent: "toolu_1", Text: "done"}}},
		{"failed", []Event{{Kind: KindTaskEnd, Parent: "toolu_1", Text: "done", Error: true}}},
		{"stopped", []Event{{Kind: KindTaskEnd, Parent: "toolu_1", Text: "done", Error: true, Stop: TaskStopped}}},
	} {
		got, err := Parse([]byte(`{"type":"system","subtype":"task_notification","task_id":"a1","tool_use_id":"toolu_1","status":"` + tt.status + `","summary":"done"}`))
		if err != nil || !slices.EqualFunc(got, tt.want, func(a, b Event) bool {
			return a.Kind == b.Kind && a.Parent == b.Parent && a.Text == b.Text && a.Error == b.Error && a.Stop == b.Stop
		}) {
			t.Errorf("%s: got %+v, %v", tt.status, got, err)
		}
	}
	if got, _ := Parse([]byte(`{"type":"system","subtype":"task_notification","task_id":"a1","status":"completed"}`)); len(got) != 0 {
		t.Errorf("a task no call started: got %+v", got)
	}
}
