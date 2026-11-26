package chat_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nednella/orchestrator/desktop/chat"
	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/internal/bus"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/session"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

var eventually = apptest.Eventually

// newThread starts a hosted session on the stand-in claude.
func newThread(t *testing.T) (*apptest.Harness, string) {
	t.Helper()
	h := apptest.New(t)
	s, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	return h, s.ID
}

func history(t *testing.T, h *apptest.Harness, id string, after int64) []chatlog.Event {
	t.Helper()
	events, err := h.ChatHistory(id, after)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func kinds(events []chatlog.Event) []chatlog.Kind {
	out := []chatlog.Kind{}
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}

func seqs(events []chatlog.Event) []int64 {
	out := []int64{}
	for _, e := range events {
		out = append(out, e.Seq)
	}
	return out
}

func countTurnEnds(events []chatlog.Event) int {
	return len(slices.DeleteFunc(slices.Clone(events), func(e chatlog.Event) bool { return e.Kind != chatlog.KindTurnEnd }))
}

// turnEnds counts the turn_ends of the thread's log.
func turnEnds(t *testing.T, h *apptest.Harness, id string) int {
	t.Helper()
	return countTurnEnds(history(t, h, id, 0))
}

// emitted are the events of the thread's chat:<id> events, in the order they went out.
func emitted(rec *apptest.Recorder, id string) []chatlog.Event {
	var out []chatlog.Event
	for _, p := range rec.All("chat:" + id) {
		out = append(out, p.(chat.Update).Events...)
	}
	return out
}

func persisted(events []chatlog.Event) []chatlog.Event {
	return slices.DeleteFunc(slices.Clone(events), func(e chatlog.Event) bool { return !e.Persisted() })
}

func emittedTurnEnds(rec *apptest.Recorder, id string) int { return countTurnEnds(emitted(rec, id)) }

func send(t *testing.T, h *apptest.Harness, id, text string) string {
	t.Helper()
	sent, err := h.ChatSend(id, text)
	if err != nil {
		t.Fatal(err)
	}
	return sent.ID
}

func TestHistoryReadsTheChatLog(t *testing.T) {
	h, id := newThread(t)
	msg := send(t, h, id, "hi")
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })

	events := history(t, h, id, 0)
	want := []chatlog.Kind{chatlog.KindStart, chatlog.KindUser, chatlog.KindInit, chatlog.KindMessage, chatlog.KindTurnEnd}
	if !slices.Equal(kinds(events), want) {
		t.Fatalf("history kinds = %v, want %v", kinds(events), want)
	}
	if user := events[1]; user.ID != msg || user.Text != "hi" {
		t.Errorf("user event = %+v, want id %s", user, msg)
	}
	if after := history(t, h, id, events[1].Seq); !slices.Equal(kinds(after), want[2:]) {
		t.Errorf("history after the user's seq = %v, want %v", kinds(after), want[2:])
	}
}

func TestHistoryOfAnEndedThread(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "hi")
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to settle", func() bool { s, _ := h.Session(id); return s.State == "settled" })

	if events := history(t, h, id, 0); len(events) < 5 || events[1].Kind != chatlog.KindUser {
		t.Errorf("history of the ended thread = %v", kinds(events))
	}
	want := "Thread " + id + " is asleep"
	if err := h.ChatInterrupt(id); err == nil || err.Error() != want {
		t.Errorf("ChatInterrupt = %v, want %q", err, want)
	}
	if err := h.ChatWatch(id); err == nil || err.Error() != want {
		t.Errorf("ChatWatch = %v, want %q", err, want)
	}
}

func TestWatchedThreadGetsLiveEvents(t *testing.T) {
	h, id := newThread(t)
	if err := h.ChatWatch(id); err != nil {
		t.Fatal(err)
	}
	send(t, h, id, "hi")
	eventually(t, "the turn's end in a chat event", func() bool { return emittedTurnEnds(h.Rec, id) == 1 })

	got := seqs(persisted(emitted(h.Rec, id)))
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("emitted seqs %v do not strictly increase", got)
		}
	}
	// The run's start, seq 1, was logged before the watch.
	if want := seqs(history(t, h, id, 1)); !slices.Equal(got, want) {
		t.Errorf("emitted seqs = %v, want the history's %v", got, want)
	}
}

func TestUnwatchedThreadEmitsNoChatEvents(t *testing.T) {
	h, id := newThread(t)
	if err := h.ChatWatch(id); err != nil {
		t.Fatal(err)
	}
	h.ChatUnwatch(id)
	send(t, h, id, "stream")
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	time.Sleep(100 * time.Millisecond) // past a batch's wait
	if n := h.Rec.Count("chat:" + id); n != 0 {
		t.Errorf("%d chat events for an unwatched thread, want none", n)
	}
}

func TestRestartResumesWithoutGapsOrDoubles(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "slow")

	second := h.Restart(t)
	if err := second.ChatWatch(id); err != nil {
		t.Fatal(err)
	}
	before := history(t, second, id, 0)
	eventually(t, "turn 1's end", func() bool { return turnEnds(t, second, id) == 1 })
	send(t, second, id, "hi")
	// Turn 1 may end before the watch or after it.
	eventually(t, "turn 2's end in a chat event", func() bool {
		logged, got := history(t, second, id, 0), persisted(emitted(second.Rec, id))
		return countTurnEnds(logged) == 2 && len(got) > 0 && got[len(got)-1].Seq == logged[len(logged)-1].Seq
	})

	live := seqs(persisted(emitted(second.Rec, id)))
	for i := 1; i < len(live); i++ {
		if live[i] <= live[i-1] {
			t.Fatalf("emitted seqs %v do not strictly increase", live)
		}
	}
	// An event logged between the watch and the history read comes in both; the front end's merge drops the double.
	all := append(seqs(before), live...)
	slices.Sort(all)
	all = slices.Compact(all)
	last := history(t, second, id, 0)
	want := make([]int64, len(last))
	for i := range want {
		want[i] = int64(i + 1)
	}
	if !slices.Equal(all, want) {
		t.Errorf("the history read after the watch and the chat events hold seqs %v, want 1 to %d with no gap", all, len(want))
	}
	if countTurnEnds(last) != 2 {
		t.Errorf("log kinds = %v, want two turns", kinds(last))
	}
}

func TestPendingApprovalIsReported(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "ask")
	eventually(t, "chat:pending", func() bool { return h.Rec.Count("chat:pending") > 0 })
	pending := h.ChatPending()
	if len(pending) != 1 {
		t.Fatalf("ChatPending = %+v, want one", pending)
	}
	p := pending[0]
	if want := (chat.Pending{ID: id, Request: "ask-1", Tool: "Bash", Summary: "touch made.txt", Kind: "tool", Input: json.RawMessage(`{"command":"touch made.txt"}`)}); !samePending(p, want) {
		t.Errorf("pending = %+v, want %+v", p, want)
	}
	if u, ok := h.Rec.Last("chat:pending").(chat.PendingUpdate); !ok || u.ID != id || u.Pending == nil || !samePending(*u.Pending, p) {
		t.Errorf("chat:pending = %+v, want %+v", h.Rec.Last("chat:pending"), p)
	}

	if err := h.ChatAnswer(id, p.Request, true, ""); err != nil {
		t.Fatal(err)
	}
	eventually(t, "nothing waiting", func() bool { return len(h.ChatPending()) == 0 })
	if u, ok := h.Rec.Last("chat:pending").(chat.PendingUpdate); !ok || u.ID != id || u.Pending != nil {
		t.Errorf("chat:pending after the answer = %+v, want none waiting", h.Rec.Last("chat:pending"))
	}
	eventually(t, "the reply after the answer", func() bool { return turnEnds(t, h, id) == 1 })
}

func TestADenyWithoutReasonReachesClaudeWithOne(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "ask")
	eventually(t, "the approval", func() bool { return len(h.ChatPending()) == 1 })
	if err := h.ChatAnswer(id, "ask-1", false, ""); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the reply after the deny", func() bool { return turnEnds(t, h, id) == 1 })

	var answers []string
	for _, line := range h.Stdin(t, id) {
		var sent struct {
			Type     string `json:"type"`
			Response struct {
				Response struct {
					Behavior string `json:"behavior"`
					Message  string `json:"message"`
				} `json:"response"`
			} `json:"response"`
		}
		if json.Unmarshal([]byte(line), &sent) != nil || sent.Type != "control_response" {
			continue
		}
		answers = append(answers, sent.Response.Response.Behavior+": "+sent.Response.Response.Message)
	}
	if len(answers) != 1 || answers[0] != "deny: The user denied this tool call." {
		t.Errorf("answers claude read = %q, want one deny with the default reason", answers)
	}
}

func TestDeltasAreBatched(t *testing.T) {
	h, id := newThread(t)
	if err := h.ChatWatch(id); err != nil {
		t.Fatal(err)
	}
	// Deltas are not logged, so only a follower that is already subscribed gets them: a first turn shows it is.
	send(t, h, id, "hi")
	eventually(t, "the first turn's end in a chat event", func() bool { return emittedTurnEnds(h.Rec, id) == 1 })
	first := h.Rec.Count("chat:" + id)
	send(t, h, id, "stream")
	eventually(t, "the streamed turn's end in a chat event", func() bool { return emittedTurnEnds(h.Rec, id) == 2 })

	if n := h.Rec.Count("chat:"+id) - first; n >= 40 {
		t.Errorf("%d chat events for 200 deltas, want fewer than 40", n)
	}
	var text, want strings.Builder
	for _, e := range emitted(h.Rec, id) {
		if e.Kind == chatlog.KindDelta {
			text.WriteString(e.Text)
		}
	}
	for i := range 200 {
		want.WriteString(strconv.Itoa(i) + " ")
	}
	if text.String() != want.String() {
		t.Errorf("deltas = %q, want 0 to 199 in order", text.String())
	}
}

func TestEndedSessionKeepsItsRowAfterARestart(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "hi")
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to settle", func() bool { s, _ := h.Session(id); return s.State == "settled" })

	second := h.Restart(t)
	s, ok := second.Session(id)
	if !ok || s.State != "settled" {
		t.Fatalf("ended row after a restart = %+v (%v), want settled", s, ok)
	}
	if events := history(t, second, id, 0); len(events) < 4 {
		t.Errorf("history after a restart = %v", kinds(events))
	}
}

func TestATurnThatEndedWhileTheAppWasClosedEndsAtTheNextStart(t *testing.T) {
	h, id := newThread(t)
	h.Observe(t, id, "UserPromptSubmit", `{"prompt":"go"}`)
	eventually(t, "working", func() bool { s, _ := h.Session(id); return s.State == "working" })
	ch, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	h.App.Stop()
	if err := host.Interrupt(context.Background(), ch.Socket); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the interrupted turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	time.Sleep(200 * time.Millisecond)
	if rec, err := bus.ReadState(h.State, id); err != nil || rec.State != "working" {
		t.Fatalf("state with the app closed = %+v, %v; want working", rec, err)
	}

	second := h.Restart(t)
	eventually(t, "idle after the next start", func() bool { s, _ := second.Session(id); return s.State == "idle" })
}

func TestTheHostsOwnReasonReachesTheUser(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "hi")
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	if err, want := h.ChatSendNow(id, "nope"), "No queued message nope"; err == nil || err.Error() != want {
		t.Errorf("ChatSendNow = %v, want %q", err, want)
	}
	if err, want := h.ChatAnswer(id, "nope", true, ""), `No approval "nope" is waiting`; err == nil || err.Error() != want {
		t.Errorf("ChatAnswer = %v, want %q", err, want)
	}
}

// dot is a png of one pixel.
const dot = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestImagesAreSavedAndSent(t *testing.T) {
	h, id := newThread(t)
	sent, err := h.ChatSend(id, "", chat.Image{Base64: dot, Mime: "image/png"})
	if err != nil || sent.ID == "" || sent.Notice != "" {
		t.Fatalf("ChatSend = %+v, %v; want an id and no notice", sent, err)
	}
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })

	stdin := strings.Join(h.Stdin(t, id), "\n")
	if !strings.Contains(stdin, `"type":"user"`) || !strings.Contains(stdin, `"media_type":"image/png"`) {
		t.Errorf("claude's stdin = %s, want a user message with a png", stdin)
	}
	user := history(t, h, id, 0)[1]
	if user.Kind != chatlog.KindUser || user.ID != sent.ID || len(user.Files) != 1 {
		t.Fatalf("user event = %+v, want the image's file", user)
	}
	file := user.Files[0]
	ch, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	if token := strings.Split(id, "/")[1]; filepath.Dir(file) != ch.Media || !strings.HasSuffix(ch.Media, filepath.Join("chat-media", token)) {
		t.Errorf("image saved at %s, want it in %s, under chat-media", file, ch.Media)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("saved image: %v, %v; want mode 0600", info, err)
	}
	want, _ := base64.StdEncoding.DecodeString(dot)
	if got, _ := os.ReadFile(file); string(got) != string(want) {
		t.Errorf("saved image holds %d bytes, want the png's %d", len(got), len(want))
	}
}

func TestTooManyImagesAreRefused(t *testing.T) {
	h, id := newThread(t)
	ch, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	images := slices.Repeat([]chat.Image{{Base64: dot, Mime: "image/png"}}, 11)
	big := base64.StdEncoding.EncodeToString(make([]byte, host.MaxImage+1))
	for _, tt := range []struct {
		images []chat.Image
		want   string
	}{
		{images, "A message takes at most 10 images"},
		{[]chat.Image{images[0], {Base64: big, Mime: "image/png"}}, "Image 2 is too large (the limit is 3.75 MB)"},
		{[]chat.Image{{Base64: base64.StdEncoding.EncodeToString([]byte("<svg></svg>")), Mime: "image/svg+xml"}}, "Image 1 is not png, jpeg, gif or webp"},
	} {
		if _, err := h.ChatSend(id, "see", tt.images...); err == nil || err.Error() != tt.want {
			t.Errorf("ChatSend = %v, want %q", err, tt.want)
		}
	}
	if apptest.Exists(ch.Media) {
		t.Errorf("%s was made for refused messages", ch.Media)
	}
	if events := history(t, h, id, 0); slices.Contains(kinds(events), chatlog.KindUser) {
		t.Errorf("history after refused messages = %v, want no message", kinds(events))
	}
}

func TestDeleteRemovesChatImages(t *testing.T) {
	h, id := newThread(t)
	ch, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ChatSend(id, "see", chat.Image{Base64: dot, Mime: "image/png"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	if !apptest.Exists(ch.Media) {
		t.Fatalf("%s was not made", ch.Media)
	}
	if err := h.SettleThread(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to settle", func() bool { s, _ := h.Session(id); return s.State == "settled" })
	if !apptest.Exists(ch.Media) {
		t.Fatalf("%s went when the thread ended", ch.Media)
	}
	if err := h.DeleteThread(id, false); err != nil {
		t.Fatal(err)
	}
	if apptest.Exists(ch.Media) {
		t.Errorf("%s is still there after the thread was deleted", ch.Media)
	}
}

func TestAnImageIsSavedAsTheTypeItIs(t *testing.T) {
	h, id := newThread(t)
	sent, err := h.ChatSend(id, "see", chat.Image{Base64: dot, Mime: "image/jpeg"})
	if err != nil {
		t.Fatalf("ChatSend of a png given as a jpeg = %v, want it sent", err)
	}
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	user := history(t, h, id, 0)[1]
	if user.ID != sent.ID || len(user.Files) != 1 || filepath.Ext(user.Files[0]) != ".png" {
		t.Errorf("user event = %+v, want the image saved as a .png", user)
	}
}

func TestAThreadWorksInTheFolderItStartedIn(t *testing.T) {
	h, id := newThread(t)
	ch, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Dir != h.Dir {
		t.Errorf("Dir = %q, want %q", ch.Dir, h.Dir)
	}
}

func TestChatFilesListsTheThreadsWorktree(t *testing.T) {
	h := apptest.New(t)
	gitIn(t, h.Dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(h.Dir, "README"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitIn(t, h.Dir, "add", "README")
	gitIn(t, h.Dir, "commit", "-q", "-m", "first")
	s, err := h.Sessions().Create(h.Sessions().Current(), "#7 work", "", false, 7, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if s.Worktree == "" {
		t.Fatalf("the thread has no worktree: %+v", s)
	}
	if err := os.WriteFile(filepath.Join(s.Worktree, "only-here.go"), []byte("package here\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := h.ChatFiles(s.ID, "")
	if want := []string{"README", "only-here.go"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("ChatFiles = %q, %v; want %q", got, err, want)
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v: %s", args, dir, err, out)
	}
}

// samePending compares two approvals field by field, their inputs as JSON values.
func samePending(a, b chat.Pending) bool {
	var ai, bi any
	_ = json.Unmarshal(a.Input, &ai)
	_ = json.Unmarshal(b.Input, &bi)
	a.Input, b.Input = nil, nil
	return reflect.DeepEqual(a, b) && reflect.DeepEqual(ai, bi)
}

func TestAThreadWorksWhileItsSubagentRuns(t *testing.T) {
	h, id := newThread(t)
	subagents := func(h *apptest.Harness) (string, int) {
		s, _ := h.Session(id)
		return string(s.State), s.Subagents
	}
	h.Observe(t, id, "UserPromptSubmit", `{"prompt":"background"}`)
	send(t, h, id, "background")
	eventually(t, "the turn that started the subagent to end", func() bool { return turnEnds(t, h, id) == 1 })
	h.Observe(t, id, "Stop", `{}`)
	eventually(t, "working with one subagent", func() bool { state, n := subagents(h); return state == "working" && n == 1 })

	if err := h.SetAccess(id, "full"); err == nil || err.Error() != "Change access when the subagents end" {
		t.Errorf("restart for Full while the subagent works: %v", err)
	}

	second := h.Restart(t)
	eventually(t, "working with one subagent after a restart of the app", func() bool { state, n := subagents(second); return state == "working" && n == 1 })

	second.Observe(t, id, "UserPromptSubmit", `{"prompt":"finish"}`)
	send(t, second, id, "finish")
	eventually(t, "the turn that ended the subagent to end", func() bool { return turnEnds(t, second, id) == 2 })
	second.Observe(t, id, "Stop", `{}`)
	eventually(t, "idle once the subagent ended", func() bool { state, n := subagents(second); return state == "idle" && n == 0 })
}

func TestASubagentsApprovalWaitsPastTheTurnUntilItIsAnswered(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "subagent asks")
	eventually(t, "the subagent's approval after the turn's end", func() bool { return turnEnds(t, h, id) == 1 && len(h.ChatPending()) == 1 })
	if p := h.ChatPending()[0]; p.Request != "sub-1" || p.Tool != "Write" || p.Summary != "sub.txt" {
		t.Fatalf("pending = %+v, want the subagent's Write", p)
	}

	second := h.Restart(t)
	eventually(t, "the approval read again after a restart of the app", func() bool { return len(second.ChatPending()) == 1 })
	if err := second.ChatAnswer(id, "sub-1", true, ""); err != nil {
		t.Fatal(err)
	}
	eventually(t, "nothing waiting", func() bool { return len(second.ChatPending()) == 0 })
	eventually(t, "Claude's report of the subagent's end", func() bool { return turnEnds(t, second, id) == 2 })
}

func TestAThreadWhoseSubagentsEndWasMissedStopsWorkingWhenClaudeListsNoTask(t *testing.T) {
	h, id := newThread(t)
	state := func() (string, int) { s, _ := h.Session(id); return string(s.State), s.Subagents }
	h.Observe(t, id, "UserPromptSubmit", `{"prompt":"background"}`)
	send(t, h, id, "background")
	eventually(t, "the turn that started the subagent to end", func() bool { return turnEnds(t, h, id) == 1 })
	h.Observe(t, id, "Stop", `{}`)
	eventually(t, "working with one subagent", func() bool { s, n := state(); return s == "working" && n == 1 })

	h.Observe(t, id, "UserPromptSubmit", `{"prompt":"list none"}`)
	send(t, h, id, "list none")
	eventually(t, "the turn that listed no task to end", func() bool { return turnEnds(t, h, id) == 2 })
	h.Observe(t, id, "Stop", `{}`)
	eventually(t, "idle with no subagent", func() bool { s, n := state(); return s == "idle" && n == 0 })
}

func TestASubagentsApprovalStopsWaitingWhenClaudeListsNoTask(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "subagent lost")
	eventually(t, "the subagent's approval after the turn's end", func() bool { return turnEnds(t, h, id) == 1 && len(h.ChatPending()) == 1 })
	eventually(t, "nothing waiting once Claude lists no task", func() bool { return len(h.ChatPending()) == 0 })
}

func TestChatHistoryCutsAHugeToolResultAndTheLogKeepsIt(t *testing.T) {
	h, id := newThread(t)
	ch, err := h.Sessions().ChatOf(id)
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread's host", func() bool { return apptest.Exists(ch.Socket) })
	log, err := chatlog.OpenLog(ch.Log)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	whole := strings.Repeat("0123456789", 20_000)
	if _, err := log.Append(chatlog.Event{Kind: chatlog.KindToolResult, ID: "toolu_big", Text: whole}); err != nil {
		t.Fatal(err)
	}

	var got chatlog.Event
	for _, e := range history(t, h, id, 0) {
		if e.Kind == chatlog.KindToolResult && e.ID == "toolu_big" {
			got = e
		}
	}
	if !got.Cut || len(got.Text) >= len(whole) || !strings.HasSuffix(got.Text, "(cut at 64 KB)") {
		t.Errorf("the result in the history is %d bytes, cut %v", len(got.Text), got.Cut)
	}
	onDisk, err := chatlog.ReadLog(ch.Log, 0)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(onDisk, func(e chatlog.Event) bool { return e.ID == "toolu_big" })
	if i < 0 || onDisk[i].Text != whole || onDisk[i].Cut {
		t.Error("the log lost the whole text")
	}
}
