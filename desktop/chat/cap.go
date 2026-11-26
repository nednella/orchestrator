package chat

import (
	"encoding/json"
	"unicode/utf8"

	chatlog "github.com/nednella/orchestrator/internal/chat"
)

// maxShownText is the most text of one tool result, and of one string in a tool call's input, that the front end gets.
// The log keeps the whole text; a thread that runs for days would otherwise send hundreds of megabytes on each open.
const maxShownText = 64 << 10

// cutMarker ends a text that was cut, so the chat shows where.
const cutMarker = "\n(cut at 64 KB)"

// shown is the events as the front end gets them: a tool result's text and each string of a tool call's input cut at
// maxShownText, with the marker at their end and Cut set. The log's own events are left as they are.
func shown(events []chatlog.Event) []chatlog.Event {
	out, copied := events, false
	for i, e := range events {
		cut, ok := shownEvent(e)
		if !ok {
			continue
		}
		if !copied {
			out, copied = append([]chatlog.Event(nil), events...), true
		}
		out[i] = cut
	}
	return out
}

func shownEvent(e chatlog.Event) (chatlog.Event, bool) {
	switch e.Kind {
	case chatlog.KindToolResult:
		if len(e.Text) <= maxShownText {
			return e, false
		}
		e.Text, e.Cut = cutText(e.Text), true
		return e, true
	case chatlog.KindToolUse:
		if len(e.Input) <= maxShownText {
			return e, false
		}
		e.Input, e.Cut = cutInput(e.Input), true
		return e, true
	}
	return e, false
}

// cutText is the first maxShownText bytes of s, ended where a character ends, and the marker.
func cutText(s string) string {
	if len(s) <= maxShownText {
		return s
	}
	n := maxShownText
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + cutMarker
}

// cutInput cuts each top-level value of a tool call's input that is over the limit into a string of its first part.
// An input that is no object becomes one string.
func cutInput(input json.RawMessage) json.RawMessage {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return jsonString(cutText(string(input)))
	}
	for k, v := range fields {
		if len(v) <= maxShownText {
			continue
		}
		var text string
		if err := json.Unmarshal(v, &text); err != nil {
			text = string(v)
		}
		fields[k] = jsonString(cutText(text))
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return jsonString(cutText(string(input)))
	}
	return out
}

func jsonString(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}
