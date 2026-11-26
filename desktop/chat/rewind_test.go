package chat_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nednella/agentos/desktop/internal/apptest"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/session"
)

// talked sends each text as a turn of its own to the thread, waits for each turn's end and returns the messages' ids.
func talked(t *testing.T, h *apptest.Harness, id string, texts ...string) []string {
	t.Helper()
	var ids []string
	for _, text := range texts {
		before := turnEnds(t, h, id)
		ids = append(ids, send(t, h, id, text))
		eventually(t, "the turn's end", func() bool { return turnEnds(t, h, id) == before+1 })
	}
	return ids
}

// rewinds are the rewind requests the thread's stand-in claude read, as "<subtype> dry=<dry run> <message>
// last=<newest message named>".
func rewinds(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	var out []string
	for _, line := range h.Stdin(t, id) {
		var l struct {
			Request struct {
				Subtype  string `json:"subtype"`
				Message  string `json:"user_message_id"`
				DryRun   bool   `json:"dry_run"`
				Target   string `json:"target_message_uuid"`
				LastSeen string `json:"last_seen_user_message_uuid"`
			} `json:"request"`
		}
		if json.Unmarshal([]byte(line), &l) != nil || !strings.HasPrefix(l.Request.Subtype, "rewind_") {
			continue
		}
		r := l.Request
		out = append(out, fmt.Sprintf("%s dry=%v %s%s last=%s", r.Subtype, r.DryRun, r.Message, r.Target, r.LastSeen))
	}
	return out
}

func loggedRewinds(t *testing.T, h *apptest.Harness, id string) []string {
	t.Helper()
	var out []string
	for _, e := range history(t, h, id, 0) {
		if e.Kind == chatlog.KindRewind {
			out = append(out, e.ID)
		}
	}
	return out
}

// worktreeThread is the thread of issue 7, in the worktree agentos makes for it in a new git repository, and a file
// in that worktree that its stand-in claude's rewind of the files puts back.
func worktreeThread(t *testing.T) (h *apptest.Harness, id, file string) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	h = apptest.New(t)
	gitIn(t, h.Dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(h.Dir, "README"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, h.Dir, "add", "README")
	gitIn(t, h.Dir, "commit", "-q", "-m", "first")
	s, err := h.Sessions().Create("#7 work", "", false, 7, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if s.Worktree == "" {
		t.Fatalf("the thread has no worktree: %+v", s)
	}
	file = filepath.Join(s.Worktree, "made.txt")
	if err := os.WriteFile(file, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.RewindAs(t, s.ID, "", file)
	return h, s.ID, file
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRewindTheConversationOnly(t *testing.T) {
	h, id := newThread(t)
	ids := talked(t, h, id, "one", "two")

	if offer := h.RewindFilesOffer(id); offer.Offered {
		t.Errorf("RewindFilesOffer = %+v; want no files offered in the project folder", offer)
	}
	preview, err := h.RewindPreview(id, ids[1])
	if err != nil || preview.Offered || preview.Files == nil {
		t.Fatalf("RewindPreview = %+v, %v; want no files offered in the project folder", preview, err)
	}
	if _, err := h.RewindTo(id, ids[1], true); err == nil || err.Error() != "The files of the project folder are never rewound" {
		t.Errorf("RewindTo with the files = %v", err)
	}
	prefill, err := h.RewindTo(id, ids[1], false)
	if err != nil || prefill != "two" {
		t.Fatalf("RewindTo = %q, %v; want the message's text", prefill, err)
	}
	if got, want := rewinds(t, h, id), []string{"rewind_conversation dry=false " + ids[1] + " last=" + ids[1]}; !slices.Equal(got, want) {
		t.Errorf("requests = %q, want %q", got, want)
	}
	if got := loggedRewinds(t, h, id); !slices.Equal(got, []string{ids[1]}) {
		t.Errorf("logged rewinds = %q", got)
	}
}

func TestRewindWithTheFiles(t *testing.T) {
	h, id, file := worktreeThread(t)
	ids := talked(t, h, id, "one", "two")

	preview, err := h.RewindPreview(id, ids[0])
	if err != nil || !preview.Offered || preview.Reason != "" || !slices.Equal(preview.Files, []string{file}) || preview.Insertions != 2 || preview.Deletions != 1 {
		t.Fatalf("RewindPreview = %+v, %v", preview, err)
	}
	if got := read(t, file); got != "after" {
		t.Fatalf("the dry run changed the file: %q", got)
	}
	if got := loggedRewinds(t, h, id); len(got) != 0 {
		t.Fatalf("the dry run logged %q", got)
	}

	prefill, err := h.RewindTo(id, ids[0], true)
	if err != nil || prefill != "one" {
		t.Fatalf("RewindTo = %q, %v", prefill, err)
	}
	if got := read(t, file); got != "before" {
		t.Errorf("file = %q, want it put back", got)
	}
	dry, real, conversation := "rewind_files dry=true "+ids[0]+" last=", "rewind_files dry=false "+ids[0]+" last=", "rewind_conversation dry=false "+ids[0]+" last="+ids[1]
	if got, want := rewinds(t, h, id), []string{dry, dry, real, conversation}; !slices.Equal(got, want) {
		t.Errorf("requests =\n%s\nwant the preview, a dry run, the files, then the conversation:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got := loggedRewinds(t, h, id); !slices.Equal(got, []string{ids[0]}) {
		t.Errorf("logged rewinds = %q", got)
	}
}

func TestRewindToAStaleTargetChangesNothing(t *testing.T) {
	h, id, file := worktreeThread(t)
	ids := talked(t, h, id, "one")
	h.RewindAs(t, id, "stale")

	if _, err := h.RewindTo(id, ids[0], false); err == nil || err.Error() != "A message arrived since; look at the chat and edit again" {
		t.Errorf("RewindTo = %v", err)
	}
	if got := loggedRewinds(t, h, id); len(got) != 0 {
		t.Errorf("logged rewinds = %q, want none", got)
	}
	if got := read(t, file); got != "after" {
		t.Errorf("file = %q, want it left", got)
	}
}

func TestRewindOfAThreadWithoutCheckpoints(t *testing.T) {
	h, id, file := worktreeThread(t)
	ids := talked(t, h, id, "one")
	h.RewindAs(t, id, "nocheckpoint", file)

	preview, err := h.RewindPreview(id, ids[0])
	if err != nil || !preview.Offered || preview.Reason != "This thread started before file checkpoints" {
		t.Fatalf("RewindPreview = %+v, %v; want the files offered with the reason they cannot rewind", preview, err)
	}
	if _, err := h.RewindTo(id, ids[0], true); err == nil || err.Error() != "This thread started before file checkpoints" {
		t.Errorf("RewindTo with the files = %v", err)
	}
	if got := loggedRewinds(t, h, id); len(got) != 0 {
		t.Fatalf("logged rewinds = %q, want none", got)
	}
	if prefill, err := h.RewindTo(id, ids[0], false); err != nil || prefill != "one" {
		t.Errorf("RewindTo the conversation alone = %q, %v", prefill, err)
	}
	if got := read(t, file); got != "after" {
		t.Errorf("file = %q, want it left", got)
	}
}

func TestEditIsOffForTheRunWhenClaudeCannotRewind(t *testing.T) {
	h, id := newThread(t)
	ids := talked(t, h, id, "one")
	h.RewindAs(t, id, "unsupported")

	const off = "This version of Claude Code cannot rewind a thread"
	if _, err := h.RewindTo(id, ids[0], false); err == nil || err.Error() != off {
		t.Fatalf("RewindTo = %v", err)
	}
	if got := h.RewindOff(); got != off {
		t.Errorf("RewindOff = %q, want %q", got, off)
	}
	h.RewindAs(t, id, "")
	before := len(rewinds(t, h, id))
	if _, err := h.RewindPreview(id, ids[0]); err == nil || err.Error() != off {
		t.Errorf("RewindPreview later in the run = %v", err)
	}
	if _, err := h.RewindTo(id, ids[0], false); err == nil || err.Error() != off {
		t.Errorf("RewindTo later in the run = %v", err)
	}
	if got := len(rewinds(t, h, id)); got != before {
		t.Errorf("%d more requests went to Claude, want none", got-before)
	}
	if got := h.Restart(t).RewindOff(); got != "" {
		t.Errorf("RewindOff in the next run = %q, want it on again", got)
	}
}

func TestASettledThreadIsWokenThenRewound(t *testing.T) {
	h, id := newThread(t)
	ids := talked(t, h, id, "one", "two", "three")
	if _, err := h.RewindTo(id, ids[2], false); err != nil {
		t.Fatal(err)
	}
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { s, _ := h.Session(id); return s.State == "settled" })

	prefill, err := h.RewindTo(id, ids[0], false)
	if err != nil || prefill != "one" {
		t.Fatalf("RewindTo = %q, %v", prefill, err)
	}
	if s, _ := h.Session(id); s.State == "settled" {
		t.Errorf("the thread is still settled")
	}
	// The woken host reads the log: three went with the first rewind, so two is the newest message.
	if got := rewinds(t, h, id); len(got) != 2 || got[1] != "rewind_conversation dry=false "+ids[0]+" last="+ids[1] {
		t.Errorf("requests = %q", got)
	}
	if got := loggedRewinds(t, h, id); !slices.Equal(got, []string{ids[2], ids[0]}) {
		t.Errorf("logged rewinds = %q", got)
	}
}

func TestTheRewindOutlivesARestart(t *testing.T) {
	h, id := newThread(t)
	ids := talked(t, h, id, "one", "two")
	if _, err := h.RewindTo(id, ids[1], false); err != nil {
		t.Fatal(err)
	}
	again := h.Restart(t)
	var rewound []string
	for _, e := range history(t, again, id, 0) {
		rewound = chatlog.Prompts(rewound, e)
	}
	if !slices.Equal(rewound, ids[:1]) {
		t.Errorf("the log after a restart holds the messages %q, want %q", rewound, ids[:1])
	}
	if _, err := again.RewindTo(id, ids[1], false); err == nil || err.Error() != "Claude's conversation no longer holds this message" {
		t.Errorf("RewindTo the rewound message = %v", err)
	}
}

func TestRewindIsRefusedWhileATurnRuns(t *testing.T) {
	h, id := newThread(t)
	ids := talked(t, h, id, "one")
	send(t, h, id, "slow")
	if _, err := h.RewindTo(id, ids[0], false); err == nil || err.Error() != "Edit when the turn ends" {
		t.Errorf("RewindTo = %v", err)
	}
	if got := rewinds(t, h, id); len(got) != 0 {
		t.Errorf("requests = %q, want none", got)
	}
}

// typeIn types the line into the shell id and presses Enter.
func typeIn(t *testing.T, h *apptest.Harness, id, line string) {
	t.Helper()
	if err := exec.Command("tmux", "-L", h.Socket, "send-keys", "-t", "="+id+":", line, "Enter").Run(); err != nil {
		t.Fatal(err)
	}
}

func TestFilesAreNotRewoundInASharedWorktree(t *testing.T) {
	h, id, file := worktreeThread(t)
	ids := talked(t, h, id, "one")
	s, _ := h.Session(id)
	h.Observe(t, id, "Stop", fmt.Sprintf(`{"cwd":%q}`, s.Worktree))
	shell, err := h.ShellNew("")
	if err != nil {
		t.Fatal(err)
	}
	typeIn(t, h, shell.ID, fmt.Sprintf("cd %q", s.Worktree))
	eventually(t, "the shell in the worktree", func() bool {
		_, _, reason := h.Sessions().FilesRewind(id)
		return reason != ""
	})

	if offer := h.RewindFilesOffer(id); !offer.Offered || offer.Reason != "A shell is open in this worktree" {
		t.Errorf("RewindFilesOffer = %+v", offer)
	}
	preview, err := h.RewindPreview(id, ids[0])
	if err != nil || !preview.Offered || preview.Reason != "A shell is open in this worktree" {
		t.Errorf("RewindPreview = %+v, %v", preview, err)
	}
	if _, err := h.RewindTo(id, ids[0], true); err == nil || err.Error() != "A shell is open in this worktree" {
		t.Errorf("RewindTo with the files = %v", err)
	}
	if got := rewinds(t, h, id); len(got) != 0 {
		t.Errorf("requests = %q, want none", got)
	}
	if got := read(t, file); got != "after" {
		t.Errorf("file = %q, want it left", got)
	}
	if err := h.ShellClose(shell.ID); err != nil {
		t.Fatal(err)
	}
}

func TestFilesAreRewoundBesideTheThreadsOwnIdleShell(t *testing.T) {
	h, id, _ := worktreeThread(t)
	s, _ := h.Session(id)
	h.Observe(t, id, "Stop", fmt.Sprintf(`{"cwd":%q}`, s.Worktree))
	shell, err := h.ShellNew(id)
	if err != nil {
		t.Fatal(err)
	}
	if offer := h.RewindFilesOffer(id); !offer.Offered || offer.Reason != "" {
		t.Errorf("RewindFilesOffer beside an idle shell of the thread = %+v", offer)
	}

	typeIn(t, h, shell.ID, "sleep 30")
	eventually(t, "the shell's command keeps the files", func() bool {
		_, _, reason := h.Sessions().FilesRewind(id)
		return reason != ""
	})
	if offer := h.RewindFilesOffer(id); !offer.Offered || offer.Reason != "A shell of this thread runs a command" {
		t.Errorf("RewindFilesOffer while a shell of the thread runs a command = %+v", offer)
	}
	if err := h.ShellClose(shell.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTheFilesOfferWakesNoThread(t *testing.T) {
	h, id, _ := worktreeThread(t)
	ids := talked(t, h, id, "one")
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread settled", func() bool { s, _ := h.Session(id); return s.State == "settled" })

	if offer := h.RewindFilesOffer(id); !offer.Offered || offer.Reason != "" {
		t.Errorf("RewindFilesOffer = %+v", offer)
	}
	if s, _ := h.Session(id); s.State != "settled" {
		t.Errorf("the offer woke the thread: %s", s.State)
	}
	if preview, err := h.RewindPreview(id, ids[0]); err != nil || len(preview.Files) != 1 {
		t.Errorf("RewindPreview = %+v, %v", preview, err)
	}
	if s, _ := h.Session(id); s.State == "settled" {
		t.Errorf("the dry run did not wake the thread")
	}
}
