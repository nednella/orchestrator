package orchestration

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	ctl "github.com/nednella/orchestrator/internal/control"
)

const (
	viewMessages = "messages"
	viewActivity = "activity"

	readLimit  = 20
	readMost   = 100
	textMost   = 2000  // characters of one entry's text
	answerMost = 40000 // bytes of the whole answer
	cutMark    = "… (cut)"
)

// Who an entry is from, besides a thread's id.
const (
	fromOwner = "owner"
	fromApp   = "orchestrator"
	fromAgent = "agent"
)

type readInput struct {
	Thread string `json:"thread"`
	View   string `json:"view"`
	Before int64  `json:"before"`
	Limit  int    `json:"limit"`
}

// entry is one thing a thread's chat holds, at the seq of its first event. From is fromOwner, fromApp, a thread's id
// or fromAgent; Error marks a tool call that failed.
type entry struct {
	Seq   int64  `json:"seq"`
	At    int64  `json:"at"`
	From  string `json:"from"`
	Text  string `json:"text"`
	Error bool   `json:"error,omitempty"`
}

// page is what read answers: the entries, oldest first, and the seq to read before for older ones, 0 when none is.
type page struct {
	Items  []entry `json:"items"`
	Before int64   `json:"before"`
}

// read answers the read tool: the newest entries of a thread's chat before the seq the input gives. Any thread that is
// not deleted and is no side chat may be read, whatever its access or project.
func (c *Commands) read(_ context.Context, _ string, req ctl.Request) (string, error) {
	var in readInput
	if err := decode(req.Opts["input"], &in); err != nil {
		return "", err
	}
	switch {
	case in.Thread == "":
		return "", errors.New("read needs the id of a thread")
	case in.View != "" && in.View != viewMessages && in.View != viewActivity:
		return "", fmt.Errorf("view is %s or %s, not %q", viewMessages, viewActivity, in.View)
	case in.Limit < 0 || in.Limit > readMost:
		return "", fmt.Errorf("limit is 1 to %d, not %d", readMost, in.Limit)
	}
	if err := c.reachable(in.Thread); err != nil {
		return "", err
	}
	ch, err := c.threads.ChatOf(in.Thread)
	if err != nil {
		return "", err
	}
	events, err := chatlog.ReadLog(ch.Log, 0)
	if err != nil {
		log.Printf("orchestrator: reading the chat log of %s: %v", in.Thread, err)
		return "", fmt.Errorf("Could not read the chat of thread %s", in.Thread)
	}
	data, err := json.Marshal(pageOf(entries(events, in.View == viewActivity), in.Before, cmp.Or(in.Limit, readLimit)))
	if err != nil {
		return "", fmt.Errorf("writing the chat of thread %s: %w", in.Thread, err)
	}
	return string(data), nil
}

// entries are the messages of the log in order: each user message, and each reply of the agent with the text blocks
// of one message joined. With activity, each tool call too, which a failed result marks.
func entries(events []chatlog.Event, activity bool) []entry {
	var out []entry
	calls := map[string]int{} // a tool call's id to its entry
	joins := ""               // the message id the last entry holds the text of, "" when it is no reply
	for _, e := range events {
		switch e.Kind {
		case chatlog.KindUser:
			out, joins = append(out, entry{Seq: e.Seq, At: e.At, From: senderOf(e), Text: e.Text}), ""
		case chatlog.KindMessage:
			if joins != "" && joins == e.ID {
				out[len(out)-1].Text += "\n\n" + e.Text
				continue
			}
			out, joins = append(out, entry{Seq: e.Seq, At: e.At, From: fromAgent, Text: e.Text}), e.ID
		case chatlog.KindToolUse:
			if !activity {
				continue
			}
			text := e.Tool
			if summary := chat.ToolSummary(e.Tool, e.Input); summary != "" {
				text += ": " + summary
			}
			calls[e.ID] = len(out)
			out, joins = append(out, entry{Seq: e.Seq, At: e.At, From: fromAgent, Text: text}), ""
		case chatlog.KindToolResult:
			if i, ok := calls[e.Parent]; ok && e.Error {
				out[i].Error = true
			}
		}
	}
	for i := range out {
		out[i].Text = cut(out[i].Text)
	}
	return out
}

// senderOf says who sent the user message e: the thread it is from, orchestrator for a text the app wrote, else the owner.
func senderOf(e chatlog.Event) string {
	switch {
	case e.From != "":
		return e.From
	case strings.HasPrefix(e.Text, sessions.AppPrompt):
		return fromApp
	}
	return fromOwner
}

// cut keeps text to textMost characters.
func cut(text string) string {
	runes := []rune(text)
	if len(runes) <= textMost {
		return text
	}
	return string(runes[:textMost]) + cutMark
}

// pageOf is the newest limit entries before the seq before, 0 for the newest of all, kept to answerMost bytes by
// leaving out the oldest.
func pageOf(all []entry, before int64, limit int) page {
	end := len(all)
	if before > 0 {
		end = 0
		for end < len(all) && all[end].Seq < before {
			end++
		}
	}
	for start := max(0, end-limit); ; start++ {
		p := page{Items: append([]entry{}, all[start:end]...)}
		if start > 0 {
			p.Before = all[start].Seq
		}
		if data, _ := json.Marshal(p); len(data) <= answerMost || start >= end-1 {
			return p
		}
	}
}
