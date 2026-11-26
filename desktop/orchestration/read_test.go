package orchestration_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/internal/chat"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/session"
)

type readEntry struct {
	Seq, At    int64
	From, Text string
	Error      bool
}

type readPage struct {
	Items  []readEntry
	Before int64
}

// settledWithLog starts a thread, settles it so no host writes its chat log, and appends events to the log.
func settledWithLog(t *testing.T, h *apptest.Harness, events ...chat.Event) string {
	t.Helper()
	id := threadWith(t, h, "Fix the login", session.Supervised).ID
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	c, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	l, err := chat.OpenLog(c.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, e := range events {
		if _, err := l.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func readAs(t *testing.T, h *apptest.Harness, caller, input string) readPage {
	t.Helper()
	var p readPage
	mustUse(t, h, caller, "read", input, &p)
	return p
}

func texts(p readPage) []string {
	out := []string{}
	for _, e := range p.Items {
		out = append(out, e.Text)
	}
	return out
}

func TestReadPagesBackwards(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	var events []chat.Event
	for i := 1; i <= 25; i++ {
		e := chat.Event{Kind: chat.KindUser, ID: fmt.Sprintf("u%d", i), Text: fmt.Sprint("m", i)}
		if i%2 == 0 {
			e = chat.Event{Kind: chat.KindMessage, ID: fmt.Sprintf("msg_%d", i), Text: fmt.Sprint("m", i)}
		}
		events = append(events, e)
	}
	target := settledWithLog(t, h, events...)
	reader := threadWith(t, h, "Read it", session.Read).ID

	seen := map[int64]bool{}
	var pages [][]string
	before := int64(0)
	for range 3 {
		p := readAs(t, h, reader, fmt.Sprintf(`{"thread":%q,"limit":10,"before":%d}`, target, before))
		for i, e := range p.Items {
			if seen[e.Seq] {
				t.Errorf("seq %d came twice", e.Seq)
			}
			seen[e.Seq] = true
			if i > 0 && e.Seq <= p.Items[i-1].Seq {
				t.Errorf("page %v is not oldest first", texts(p))
			}
		}
		pages = append(pages, texts(p))
		before = p.Before
		if before != 0 && before != p.Items[0].Seq {
			t.Errorf("before = %d, want the oldest item's seq %d", before, p.Items[0].Seq)
		}
	}
	want := [][]string{
		{"m16", "m17", "m18", "m19", "m20", "m21", "m22", "m23", "m24", "m25"},
		{"m6", "m7", "m8", "m9", "m10", "m11", "m12", "m13", "m14", "m15"},
		{"m1", "m2", "m3", "m4", "m5"},
	}
	if fmt.Sprint(pages) != fmt.Sprint(want) {
		t.Errorf("pages = %v, want %v", pages, want)
	}
	if before != 0 {
		t.Errorf("the last page's before = %d, want 0: nothing is older", before)
	}
	if p := readAs(t, h, reader, fmt.Sprintf(`{"thread":%q}`, target)); len(p.Items) != 20 || p.Items[19].Text != "m25" {
		t.Errorf("the default page = %v, want the newest 20", texts(p))
	}
}

func TestReadSaysWhoEachMessageIsFrom(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	target := settledWithLog(t, h,
		chat.Event{Kind: chat.KindUser, ID: "u1", Text: "fix the login"},
		chat.Event{Kind: chat.KindUser, ID: "u2", Text: "APPLICATION_PROMPT: PR #4 has a review"},
		chat.Event{Kind: chat.KindUser, ID: "u3", Text: "APPLICATION_PROMPT: Thread \"Plan\" (main/abcdefgh) sends you", From: "main/abcdefgh"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_1", Text: "first block"},
		chat.Event{Kind: chat.KindToolUse, ID: "t1", Tool: "Bash", Input: json.RawMessage(`{"command":"ls"}`)},
		chat.Event{Kind: chat.KindMessage, ID: "msg_1", Text: "second block"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_2", Text: "another reply"},
	)
	reader := threadWith(t, h, "Read it", session.Read).ID
	p := readAs(t, h, reader, fmt.Sprintf(`{"thread":%q,"view":"messages"}`, target))
	got := []string{}
	for _, e := range p.Items {
		got = append(got, e.From+": "+e.Text)
	}
	want := []string{
		"owner: fix the login",
		"orchestrator: APPLICATION_PROMPT: PR #4 has a review",
		"main/abcdefgh: APPLICATION_PROMPT: Thread \"Plan\" (main/abcdefgh) sends you",
		"agent: first block\n\nsecond block",
		"agent: another reply",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("messages = %q, want %q", got, want)
	}
}

func TestReadCutsLongTexts(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	reader := threadWith(t, h, "Read it", session.Read).ID

	t.Run("a text", func(t *testing.T) {
		target := settledWithLog(t, h, chat.Event{Kind: chat.KindUser, ID: "u1", Text: strings.Repeat("é", 3000)})
		p := readAs(t, h, reader, fmt.Sprintf(`{"thread":%q}`, target))
		text := p.Items[len(p.Items)-1].Text
		kept, cut := strings.CutSuffix(text, "… (cut)")
		if !cut || utf8.RuneCountInString(kept) != 2000 || strings.Trim(kept, "é") != "" {
			t.Errorf("the long text came as %d characters, cut %v; want its first 2000 and the mark", utf8.RuneCountInString(text), cut)
		}
	})

	t.Run("the answer", func(t *testing.T) {
		var events []chat.Event
		for i := 1; i <= 30; i++ {
			events = append(events, chat.Event{Kind: chat.KindMessage, ID: fmt.Sprintf("msg_%d", i), Text: strings.Repeat("x", 1990)})
		}
		target := settledWithLog(t, h, events...)
		seen := map[int64]bool{}
		for before := int64(-1); before != 0; {
			input := fmt.Sprintf(`{"thread":%q,"limit":100,"before":%d}`, target, max(before, 0))
			resp := h.Ask(t, ctl.Request{Cmd: "orchestrate", Token: tokenOf(t, h, reader), Args: []string{"read"}, Opts: map[string]string{"input": input}})
			if !resp.OK || len(resp.Out) > 40000 {
				t.Fatalf("read answered %d bytes (%s), want at most 40000", len(resp.Out), resp.Error)
			}
			var p readPage
			if err := json.Unmarshal([]byte(resp.Out), &p); err != nil {
				t.Fatal(err)
			}
			if before == -1 && len(p.Items) >= 30 {
				t.Fatalf("the first page holds %d items, want fewer than 30 to stay under 40000 bytes", len(p.Items))
			}
			for _, e := range p.Items {
				if seen[e.Seq] {
					t.Fatalf("seq %d came twice", e.Seq)
				}
				seen[e.Seq] = true
			}
			if p.Before != 0 && p.Before != p.Items[0].Seq {
				t.Fatalf("before = %d, want the oldest item's seq %d", p.Before, p.Items[0].Seq)
			}
			before = p.Before
		}
		if len(seen) != 30 {
			t.Errorf("the pages hold %d replies, want all 30", len(seen))
		}
	})
}

func TestReadActivityShowsToolCalls(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	target := settledWithLog(t, h,
		chat.Event{Kind: chat.KindUser, ID: "u1", Text: "run the tests"},
		chat.Event{Kind: chat.KindToolUse, ID: "t1", Tool: "Bash", Input: json.RawMessage(`{"command":"go test ./...\ngo vet ./..."}`)},
		chat.Event{Kind: chat.KindToolResult, Parent: "t1", Text: "FAIL", Error: true},
		chat.Event{Kind: chat.KindToolUse, ID: "t2", Tool: "Read", Input: json.RawMessage(`{"file_path":"/repo/main.go"}`)},
		chat.Event{Kind: chat.KindToolResult, Parent: "t2", Text: "package main"},
		chat.Event{Kind: chat.KindToolUse, ID: "t3", Tool: "TodoWrite", Input: json.RawMessage(`{"todos":[]}`)},
		chat.Event{Kind: chat.KindMessage, ID: "msg_1", Text: "One test fails."},
	)
	reader := threadWith(t, h, "Read it", session.Read).ID
	tests := []struct {
		view string
		want []string
	}{
		{"messages", []string{"owner run the tests", "agent One test fails."}},
		{"activity", []string{"owner run the tests", "agent Bash: go test ./... error", "agent Read: /repo/main.go", "agent TodoWrite", "agent One test fails."}},
	}
	for _, tt := range tests {
		t.Run(tt.view, func(t *testing.T) {
			p := readAs(t, h, reader, fmt.Sprintf(`{"thread":%q,"view":%q}`, target, tt.view))
			got := []string{}
			for _, e := range p.Items {
				line := e.From + " " + e.Text
				if e.Error {
					line += " error"
				}
				got = append(got, line)
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("%s = %q, want %q", tt.view, got, tt.want)
			}
		})
	}
}

func TestReadChecksItsInput(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	reader := threadWith(t, h, "Read it", session.Read).ID
	gone := threadWith(t, h, "Gone", session.Supervised).ID
	if err := h.DeleteThread(gone, false); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ input, want string }{
		{`{}`, "read needs the id of a thread"},
		{`{"thread":"` + reader + `","view":"all"}`, `view is messages or activity, not "all"`},
		{`{"thread":"` + reader + `","limit":101}`, "limit is 1 to 100, not 101"},
		{`{"thread":"` + reader + `","limit":-1}`, "limit is 1 to 100, not -1"},
		{`{"thread":"main/zzzzzzzz"}`, "No thread main/zzzzzzzz"},
		{`{"thread":"` + gone + `"}`, "Thread " + gone + " was deleted"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if err := useTool(t, h, reader, "read", tt.input, nil); err != tt.want {
				t.Errorf("read %s = %q, want %q", tt.input, err, tt.want)
			}
		})
	}
	if p := readAs(t, h, reader, `{"thread":"`+reader+`"}`); p.Items == nil || p.Before != 0 {
		t.Errorf("a thread's own empty chat = %+v, want no items and no before", p)
	}
}
