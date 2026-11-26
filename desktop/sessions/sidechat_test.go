package sessions_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/nednella/orchestrator/desktop/internal/apptest"
	"github.com/nednella/orchestrator/desktop/sessions"
	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/session"
)

// talkedThread is a live thread in the project folder whose chat has one finished reply, oneReply's.
func talkedThread(t *testing.T, h *apptest.Harness) sessions.Session {
	t.Helper()
	s, err := h.Sessions().Create(h.Sessions().Current(), "work", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, s.ID)
	writeChat(t, h, s.ID, append([]chat.Event{{Kind: chat.KindInit, ID: "abc"}}, oneReply...)...)
	return s
}

// sideChat starts a side chat of parent with no message.
func sideChat(t *testing.T, h *apptest.Harness, parent string) sessions.Session {
	t.Helper()
	side, err := h.SideChatStart(parent, "")
	if err != nil {
		t.Fatal(err)
	}
	return side
}

// gone says whether the thread has no row, no tmux session and no state file.
func gone(t *testing.T, h *apptest.Harness, id string) func() bool {
	return func() bool {
		_, listed := h.Session(id)
		return !listed && !tmuxHas(h, id) && !hasState(t, h, id)
	}
}

func TestSideChatForksTheParentAtItsLatestFinishedReply(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	repoFixture(t, h)
	parent := issueThread(h, t, 7)
	withConversation(t, h, parent.ID)
	// The parent runs a second turn as the side chat starts: its reply so far is not finished.
	writeChat(t, h, parent.ID,
		chat.Event{Kind: chat.KindInit, ID: "abc"},
		chat.Event{Kind: chat.KindUser, ID: "u1", Text: "first"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_1", UUID: "r1a", Text: "one"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_1", UUID: "r1b", Text: "two"},
		chat.Event{Kind: chat.KindTurnEnd},
		chat.Event{Kind: chat.KindUser, ID: "u2", Text: "second"},
		chat.Event{Kind: chat.KindMessage, ID: "msg_2", UUID: "r2", Text: "half"},
	)

	side := sideChat(t, h, parent.ID)
	args := claudeArgs(t, h, side.ID)
	if i := slices.Index(args, "--resume"); i < 0 || !slices.Equal(args[i:i+5], []string{"--resume", "abc", "--fork-session", "--resume-session-at", "r1b"}) {
		t.Errorf("args = %q", args)
	}
	sideLog, _ := hostFiles(h, side.ID)
	events, err := chat.ReadLog(sideLog, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := kindsAndTexts(events), []string{"init:", "user:first", "message:one", "message:two", "turn_end:", "start:"}; !slices.Equal(got, want) {
		t.Errorf("side chat's chat = %q, want %q", got, want)
	}
	if !side.Side || side.ForkOf != parent.ID || side.Title != "Side chat" || side.Issue != 0 || side.Branch != "" || side.Worktree != "" || side.Access != parent.Access {
		t.Errorf("side chat = %+v", side)
	}
	if got := realDir(t, panePath(t, h, side.ID)); got != parent.Worktree {
		t.Errorf("the side chat runs in %s, want the parent's worktree %s", got, parent.Worktree)
	}
	if got := h.Sessions().IssueSessions()[7]; got != parent.ID {
		t.Errorf("issue 7's thread = %q, want the parent", got)
	}
	for _, p := range h.Sessions().Projects() {
		if p.Key == "main" && p.Sessions != 1 {
			t.Errorf("the project counts %d threads, want the parent alone", p.Sessions)
		}
	}
}

func TestSideChatIsRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	fresh, err := h.Sessions().Create(h.Sessions().Current(), "fresh", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.SideChatStart(fresh.ID, ""); err == nil || err.Error() != "Side chats start after the first message." {
		t.Errorf("a thread with no reply: %v", err)
	}

	compacted, err := h.Sessions().Create(h.Sessions().Current(), "compacted", "", false, 0, session.Supervised)
	if err != nil {
		t.Fatal(err)
	}
	withConversation(t, h, compacted.ID)
	writeChat(t, h, compacted.ID, append(append([]chat.Event{{Kind: chat.KindInit, ID: "abc"}}, oneReply...), chat.Event{Kind: chat.KindCompact})...)
	if _, err := h.SideChatStart(compacted.ID, ""); err == nil || !strings.Contains(err.Error(), "compacted") {
		t.Errorf("a thread whose reply is from before its compaction: %v", err)
	}

	parent := talkedThread(t, h)
	side := sideChat(t, h, parent.ID)
	withConversation(t, h, side.ID)
	writeChat(t, h, side.ID, oneReply...)
	if _, err := h.SideChatStart(side.ID, ""); err == nil || err.Error() != "Side chats cannot start side chats." {
		t.Errorf("a side chat's side chat: %v", err)
	}

	child := childOf(t, h, parent, "Helper", sessions.WhereFolder)
	withConversation(t, h, child.ID)
	writeChat(t, h, child.ID, oneReply...)
	if _, err := h.SideChatStart(child.ID, ""); err == nil || err.Error() != "Subagent threads cannot start side chats." {
		t.Errorf("a delegated thread's side chat: %v", err)
	}
}

func TestSideChatShowsNoAttention(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	parent := talkedThread(t, h)
	side := sideChat(t, h, parent.ID)
	before := h.Rec.CountAttentionFor(side.ID, "waiting")

	h.Observe(t, side.ID, "Notification", `{"message":"Claude needs your permission"}`)
	eventually(t, "the side chat waits", func() bool { got, _ := h.Session(side.ID); return got.State == "waiting" })
	h.Sessions().Touch()
	if got := h.Rec.CountAttentionFor(side.ID, "waiting"); got != before {
		t.Errorf("%d waiting alerts for the side chat", got-before)
	}
	for _, p := range h.Sessions().Projects() {
		if p.Key == "main" && (p.NeedsYou != 0 || p.Sessions != 1) {
			t.Errorf("project counts = %+v, want only the parent", p)
		}
	}
}

func TestSideChatsFirstMessageNamesIt(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	parent := talkedThread(t, h)

	asked, err := h.SideChatStart(parent.ID, "What does main.go do?\nIn short.")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the question reaches the side chat", func() bool {
		return slices.ContainsFunc(h.Stdin(t, asked.ID), func(line string) bool { return strings.Contains(line, "What does main.go do?") })
	})
	eventually(t, "the side chat named by its question", func() bool { got, _ := h.Session(asked.ID); return got.Title == "What does main.go do?" })

	empty := sideChat(t, h, parent.ID)
	if _, err := h.ChatSend(empty.ID, "Why is the build slow?"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the empty side chat named by its first message", func() bool { got, _ := h.Session(empty.ID); return got.Title == "Why is the build slow?" })
	if _, err := h.ChatSend(empty.ID, "And the tests?"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the second message reaches it", func() bool {
		return slices.ContainsFunc(h.Stdin(t, empty.ID), func(line string) bool { return strings.Contains(line, "And the tests?") })
	})
	if got, _ := h.Session(empty.ID); got.Title != "Why is the build slow?" {
		t.Errorf("a later message renamed the side chat %q", got.Title)
	}
	if got, _ := h.Session(parent.ID); got.Title != "work" {
		t.Errorf("the parent is called %q", got.Title)
	}
}

func TestSideChatDelete(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	parent := talkedThread(t, h)
	asked, err := h.SideChatStart(parent.ID, "a question")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "the question in the side chat's log", func() bool {
		chatLog, _ := hostFiles(h, asked.ID)
		events, _ := chat.ReadLog(chatLog, 0)
		return slices.ContainsFunc(events, func(e chat.Event) bool { return e.Kind == chat.KindUser && e.Text == "a question" })
	})
	unused := sideChat(t, h, parent.ID)

	if err := h.SideChatDelete(asked.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.Session(asked.ID); !ok {
		t.Error("closing the tab of a side chat that ran deleted it")
	}
	if err := h.SideChatDelete(unused.ID, true); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the unused side chat gone", gone(t, h, unused.ID))
	if err := h.SideChatDelete(asked.ID, false); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the side chat deleted", gone(t, h, asked.ID))
	if err := h.SideChatDelete(parent.ID, false); err == nil {
		t.Error("SideChatDelete deleted a thread that is no side chat")
	}
	if _, ok := h.Session(parent.ID); !ok {
		t.Error("the parent went with its side chats")
	}
}

func TestParentsEndDeletesItsSideChats(t *testing.T) {
	t.Parallel()
	t.Run("sleep", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		parent := talkedThread(t, h)
		side := sideChat(t, h, parent.ID)
		if err := h.SettleThread(parent.ID); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the side chat deleted", gone(t, h, side.ID))
		if got, _ := h.Session(parent.ID); got.State != "settled" {
			t.Errorf("the parent is %q", got.State)
		}
	})
	t.Run("delete", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		parent := talkedThread(t, h)
		side := sideChat(t, h, parent.ID)
		if got := h.DeletePlan(parent.ID); len(got.Children) != 0 {
			t.Errorf("the confirm lists the side chat as a child: %+v", got)
		}
		if err := h.DeleteThread(parent.ID, false); err != nil {
			t.Fatal(err)
		}
		eventually(t, "the parent and its side chat deleted", func() bool { return gone(t, h, parent.ID)() && gone(t, h, side.ID)() })
	})
	t.Run("delete with the worktree", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		withConversation(t, h, parent.ID)
		writeChat(t, h, parent.ID, append([]chat.Event{{Kind: chat.KindInit, ID: "abc"}}, oneReply...)...)
		pushWork(t, parent.Worktree)
		side := sideChat(t, h, parent.ID)
		if got := h.DeletePlan(parent.ID); got.Worktrees != 1 {
			t.Errorf("the confirm offers %d worktrees, want the parent's though its side chat runs there", got.Worktrees)
		}
		if err := h.DeleteThread(parent.ID, true); err != nil {
			t.Fatal(err)
		}
		if exists(parent.Worktree) {
			t.Error("the worktree is still there")
		}
		eventually(t, "the parent and its side chat deleted", func() bool { return gone(t, h, parent.ID)() && gone(t, h, side.ID)() })
	})
	t.Run("clean-up", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		repoFixture(t, h)
		parent := issueThread(h, t, 7)
		withConversation(t, h, parent.ID)
		writeChat(t, h, parent.ID, append([]chat.Event{{Kind: chat.KindInit, ID: "abc"}}, oneReply...)...)
		side := sideChat(t, h, parent.ID)
		cleanedUpAfterAMerge(t, h, parent)
		eventually(t, "the side chat deleted", gone(t, h, side.ID))
		if log := h.Cleanups(); len(log) != 1 || log[0].Status != "done" {
			t.Errorf("log = %+v", log)
		}
	})
}
