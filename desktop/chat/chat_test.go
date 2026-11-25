package chat_test

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nednella/agentos/desktop/chat"
	"github.com/nednella/agentos/desktop/internal/apptest"
	"github.com/nednella/agentos/internal/bus"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/host"
)

func TestMain(m *testing.M) { os.Exit(apptest.Main(m)) }

var eventually = apptest.Eventually

// newThread starts a hosted session on the stand-in claude.
func newThread(t *testing.T) (*apptest.Harness, string) {
	t.Helper()
	h := apptest.New(t)
	s, err := h.Sessions().Create("work", "", false, 0, "")
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
	want := []chatlog.Kind{chatlog.KindUser, chatlog.KindInit, chatlog.KindMessage, chatlog.KindTurnEnd}
	if !slices.Equal(kinds(events), want) {
		t.Fatalf("history kinds = %v, want %v", kinds(events), want)
	}
	if user := events[0]; user.ID != msg || user.Text != "hi" {
		t.Errorf("user event = %+v, want id %s", user, msg)
	}
	if after := history(t, h, id, events[0].Seq); !slices.Equal(kinds(after), want[1:]) {
		t.Errorf("history after the user's seq = %v, want %v", kinds(after), want[1:])
	}
}

func TestHistoryOfAnEndedThread(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "hi")
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	if err := h.KillSession(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to end", func() bool { s, _ := h.Session(id); return s.State == "ended" })

	if events := history(t, h, id, 0); len(events) < 4 || events[0].Kind != chatlog.KindUser {
		t.Errorf("history of the ended thread = %v", kinds(events))
	}
	want := "Thread " + id + " has ended"
	if _, err := h.ChatSend(id, "again"); err == nil || err.Error() != want {
		t.Errorf("ChatSend = %v, want %q", err, want)
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
	if want := seqs(history(t, h, id, 0)); !slices.Equal(got, want) {
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
	if want := (chat.Pending{ID: id, Request: "ask-1", Tool: "Bash", Summary: "touch made.txt", Kind: "tool"}); p != want {
		t.Errorf("pending = %+v, want %+v", p, want)
	}
	if u, ok := h.Rec.Last("chat:pending").(chat.PendingUpdate); !ok || u.ID != id || u.Pending == nil || *u.Pending != p {
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

func TestEndedHostedSessionKeepsItsHostedMark(t *testing.T) {
	h, id := newThread(t)
	send(t, h, id, "hi")
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	if err := h.KillSession(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to end", func() bool { s, _ := h.Session(id); return s.State == "ended" })
	if s, _ := h.Session(id); !s.Hosted {
		t.Errorf("ended row = %+v, want hosted", s)
	}

	second := h.Restart(t)
	s, ok := second.Session(id)
	if !ok || s.State != "ended" || !s.Hosted {
		t.Fatalf("ended row after a restart = %+v (%v), want hosted", s, ok)
	}
	if events := history(t, second, id, 0); len(events) < 4 {
		t.Errorf("history after a restart = %v", kinds(events))
	}
}

func TestATurnThatEndedWhileTheAppWasClosedEndsAtTheNextStart(t *testing.T) {
	h, id := newThread(t)
	h.Hook(t, id, "UserPromptSubmit", `{"prompt":"go"}`)
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
	user := history(t, h, id, 0)[0]
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

func TestDismissRemovesChatImages(t *testing.T) {
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
	if err := h.KillSession(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread to end", func() bool { s, _ := h.Session(id); return s.State == "ended" })
	if !apptest.Exists(ch.Media) {
		t.Fatalf("%s went when the thread ended, want it kept until the row is dismissed", ch.Media)
	}
	if err := h.DismissSession(id); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the thread's images to go", func() bool { return !apptest.Exists(ch.Media) })
	if !apptest.Exists(filepath.Dir(ch.Media)) {
		t.Errorf("the project's %s went with the thread's", filepath.Dir(ch.Media))
	}
}

func TestAnImageIsSavedAsTheTypeItIs(t *testing.T) {
	h, id := newThread(t)
	sent, err := h.ChatSend(id, "see", chat.Image{Base64: dot, Mime: "image/jpeg"})
	if err != nil {
		t.Fatalf("ChatSend of a png given as a jpeg = %v, want it sent", err)
	}
	eventually(t, "the turn's end in the log", func() bool { return turnEnds(t, h, id) == 1 })
	user := history(t, h, id, 0)[0]
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
	wt := filepath.Join(h.Dir, "trees", "issue-7")
	gitIn(t, h.Dir, "worktree", "add", "-q", "-b", "issue-7", wt)
	if err := os.WriteFile(filepath.Join(wt, "only-here.go"), []byte("package here\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := h.Sessions().Create("#7 work", "", false, 7, "")
	if err != nil {
		t.Fatal(err)
	}
	h.RefreshPRs()
	eventually(t, "the thread's worktree", func() bool { got, _ := h.Session(s.ID); return got.Worktree != "" })

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
