package threads_test

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/threads"
	"github.com/nednella/orchestrator/internal/bus"
	"github.com/nednella/orchestrator/internal/project"
	"github.com/nednella/orchestrator/internal/thread"
)

// startSettings is the settings claude was started with.
func startSettings(args []string) string {
	if i := slices.Index(args, "--settings"); i >= 0 {
		return args[i+1]
	}
	return ""
}

// flagsAre says whether the thread's row and state file hold the effort and fast mode.
func flagsAre(h *apptest.Harness, id, effort string, fast bool) func() bool {
	return func() bool {
		row, _ := h.Thread(id)
		rec, _ := bus.ReadState(h.State, id)
		return row.Effort == effort && row.Fast == fast && rec.Effort == effort && rec.Fast == fast
	}
}

func TestAThreadStartsWithFastModeAndUltracode(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewThreadFast("fast", "opus", "ultracode", true)
	if err != nil {
		t.Fatal(err)
	}
	args := claudeArgs(t, h, s.ID)
	if i := slices.Index(args, "--effort"); i < 0 || args[i+1] != "xhigh" {
		t.Errorf("args = %q, want --effort xhigh", args)
	}
	settings := startSettings(args)
	if !strings.Contains(settings, `"fastMode":true`) || !strings.Contains(settings, `"ultracode":true`) {
		t.Errorf("settings = %s, want fast mode and ultracode", settings)
	}
	if s.Effort != "ultracode" || !s.Fast || s.Model != "opus" {
		t.Errorf("row = %+v, want opus at ultracode, fast", s)
	}
	eventually(t, "the choice in the row and the state file", flagsAre(h, s.ID, "ultracode", true))
}

func TestAThreadStartsWithoutFastModeByDefault(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	s, err := h.NewThreadFast("plain", "opus", "high", false)
	if err != nil {
		t.Fatal(err)
	}
	if settings := startSettings(claudeArgs(t, h, s.ID)); strings.Contains(settings, "fastMode") || strings.Contains(settings, "ultracode") {
		t.Errorf("settings = %s, want neither", settings)
	}
	if s.Fast {
		t.Errorf("row = %+v, want no fast mode", s)
	}
}

func TestAForkKeepsFastModeAndUltracode(t *testing.T) {
	t.Parallel()
	h := apptest.New(t)
	src, err := h.Threads().CreateThread("ask", threads.StartOptions{Model: project.Model{Model: "opus", Effort: "ultracode", Fast: true}, Access: thread.Supervised})
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, src.ID)
	writeChat(t, h, src.ID, oneReply...)
	fork, err := h.ForkThread(src.ID, "r1b")
	if err != nil {
		t.Fatal(err)
	}
	if !fork.Fast || fork.Effort != "ultracode" {
		t.Errorf("fork = %+v, want fast mode and ultracode", fork)
	}
	settings := startSettings(claudeArgs(t, h, fork.ID))
	if !strings.Contains(settings, `"fastMode":true`) || !strings.Contains(settings, `"ultracode":true`) {
		t.Errorf("settings = %s, want both", settings)
	}
}

func TestUltracodeStaysInTheRowWhenTheTranscriptNamesItsXhigh(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	s, err := h.NewThreadFast("ultra", "opus", "ultracode", false)
	if err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(t.TempDir(), "conv.jsonl")
	reply := time.Now().Add(100 * time.Millisecond)
	appendLines(t, transcript, fmt.Sprintf(`{"type":"assistant","timestamp":%q,"effort":"xhigh","message":{"id":"msg_1","model":"claude-opus-5-5","usage":{"input_tokens":1,"output_tokens":1}}}`+"\n", reply.UTC().Format(time.RFC3339Nano)))
	h.Observe(t, s.ID, "Stop", fmt.Sprintf(`{"transcript_path":%q}`, transcript))
	eventually(t, "the row showing the model the turn used", func() bool {
		row, _ := h.Thread(s.ID)
		return row.Model == "claude-opus-5-5"
	})
	if row, _ := h.Thread(s.ID); row.Effort != "ultracode" {
		t.Errorf("row effort = %q, want ultracode", row.Effort)
	}
}
