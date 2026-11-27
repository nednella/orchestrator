package threads

import (
	"cmp"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/thread"
)

// sideChatTitle is what a side chat is called until its first message names it.
const sideChatTitle = "Side chat"

var (
	errSideChatStarting = errors.New("Another side chat is still starting. Try again in a moment.")
	errSideOfSide       = errors.New("Side chats cannot start side chats.")
	errSideOfSubagent   = errors.New("Subagent threads cannot start side chats.")
	errSideBeforeReply  = errors.New("Side chats start after the first message.")
	errSideWorktree     = errors.New("Available once the thread's worktree is set up.")
	errSideCompacted    = errors.New("This thread's last reply is from before its chat was compacted. Side chats start after its next reply.")
)

// StartSideChat starts a side chat of the thread parent, for a question that must not disturb it: a fork of the parent's
// real conversation taken at its latest finished reply, so it starts while the parent runs. It runs in the parent's
// folder with its model, effort, access and handover, is titled "Side chat" until its first message names it, has no
// issue and no branch, and hears no pull request. Only one start runs at a time. It is refused from a side chat and from
// a thread another thread delegated to, before the parent's first reply, while the parent wakes and gets its worktree
// back, and while it is cleaned up or deleted. A start that fails removes what it made.
func (s *Threads) StartSideChat(parent string) (Thread, error) {
	if !s.sideMu.TryLock() {
		return Thread{}, errSideChatStarting
	}
	defer s.sideMu.Unlock()
	s.mu.Lock()
	in, found := s.infoOf(parent)
	conversation := s.conversationOf(parent)
	proj, known := s.projectOf(in)
	deleting, waking, deleted := s.deleting[parent], s.waking[parent], s.deleted[parent]
	s.mu.Unlock()
	switch {
	case !found || deleted:
		return Thread{}, fmt.Errorf("No thread %s", parent)
	case in.Side:
		return Thread{}, errSideOfSide
	case in.Parent != "":
		return Thread{}, errSideOfSubagent
	case deleting:
		return Thread{}, fmt.Errorf("Thread %d is being deleted", s.Number(parent))
	case waking:
		return Thread{}, errSideWorktree
	case !known:
		return Thread{}, fmt.Errorf("The project of thread %s is gone", parent)
	case s.life.CleaningUp(parent):
		return Thread{}, fmt.Errorf("Thread %d is being cleaned up; start a side chat when that ends", s.Number(parent))
	case conversation == "":
		return Thread{}, errSideBeforeReply
	}
	events, err := chat.ReadLog(s.chatLogPath(in.Name), 0)
	if err != nil {
		log.Printf("orchestrator: reading the chat log of %s: %v", parent, err)
		return Thread{}, errors.New("Could not read the thread's chat")
	}
	cut, compacted := latestReply(events)
	switch {
	case compacted:
		return Thread{}, errSideCompacted
	case cut < 0:
		return Thread{}, errSideBeforeReply
	}
	if _, err := os.Stat(in.Path); err != nil {
		return Thread{}, fmt.Errorf("%s is gone, so a side chat cannot start there", in.Path)
	}
	fork := forkStart{of: parent, title: cmp.Or(in.Title, defaultTitle), conversation: conversation, at: events[cut].UUID, events: events[:cut+1], side: true}
	model := modelOf(in)
	return s.createIn(proj, sideChatTitle, "", false, 0, "", s.handoverOf(parent), "", fork, StartOptions{Model: model, Access: in.Access, Place: InProject, dir: in.Path})
}

// latestReply is the index in a thread's chat log of the last block of its own latest reply that a turn's end follows,
// -1 when there is none. A rewind takes back its message and what came after it. compacted says that the latest such
// reply is from before the last compaction, where Claude cannot resume.
func latestReply(events []chat.Event) (at int, compacted bool) {
	ended := false
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		switch {
		case e.Kind == chat.KindCompact:
			return -1, slices.ContainsFunc(events[:i], func(e chat.Event) bool { return e.Kind == chat.KindMessage && e.UUID != "" })
		case e.Kind == chat.KindRewind:
			if from := slices.IndexFunc(events[:i], func(u chat.Event) bool { return u.Kind == chat.KindUser && u.ID == e.ID }); from >= 0 {
				i = from
			}
		case e.Kind == chat.KindTurnEnd:
			ended = true
		case ended && e.Kind == chat.KindMessage && e.UUID != "" && e.Agent == "":
			return i, false
		}
	}
	return -1, false
}

// sideChatsOf are the side chats of the thread id, live or asleep, that are not deleted or being deleted. It needs mu.
func (s *Threads) sideChatsOf(id string) []string {
	var out []string
	add := func(in thread.Name, side bool, fork string) {
		if name := in.String(); side && fork == id && !s.deleted[name] && !s.deleting[name] {
			out = append(out, name)
		}
	}
	for _, in := range s.info {
		add(in.Name, in.Side, in.Fork)
	}
	for _, e := range s.ended {
		add(e.info.Name, e.info.Side, e.info.Fork)
	}
	slices.Sort(out)
	return out
}

// SideChatsOf are the side chats of the thread id that are not deleted. It takes mu.
func (s *Threads) SideChatsOf(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sideChatsOf(id)
}

// deleteSideChats deletes the side chats of the thread id, which serve only that thread while it runs. A side chat a
// delete takes already is left to it.
func (s *Threads) deleteSideChats(id string) {
	for _, side := range s.SideChatsOf(id) {
		if err := s.DeleteAndWait(side, false); err != nil {
			log.Printf("orchestrator: deleting side chat %s of thread %s: %v", side, id, err)
		}
	}
}

// DeleteSideChat deletes the side chat id. With unused it deletes it only when nobody has sent it a message, as when
// the owner closes the tab of one that never ran.
func (s *Threads) DeleteSideChat(id string, unused bool) error {
	s.mu.Lock()
	in, found := s.infoOf(id)
	deleted := s.deleted[id]
	s.mu.Unlock()
	if !found || deleted || !in.Side {
		return fmt.Errorf("No side chat %s", id)
	}
	if unused {
		asked, err := s.sideChatAsked(in.Name)
		if err != nil || asked {
			return err
		}
	}
	return s.DeleteAndWait(id, false)
}

// sideChatAsked says whether the side chat was sent a message of its own: one after its host first started, since the
// chat it was forked with holds its parent's.
func (s *Threads) sideChatAsked(name thread.Name) (bool, error) {
	events, err := chat.ReadLog(s.chatLogPath(name), 0)
	if err != nil {
		log.Printf("orchestrator: reading the chat log of %s: %v", name, err)
		return false, errors.New("Could not read the side chat")
	}
	started := slices.IndexFunc(events, func(e chat.Event) bool { return e.Kind == chat.KindStart })
	return started >= 0 && slices.ContainsFunc(events[started:], func(e chat.Event) bool { return e.Kind == chat.KindUser && e.Parent == "" }), nil
}

// NameSideChat titles the side chat id from text, its first message, while it is still called "Side chat". Any other
// thread keeps its title.
func (s *Threads) NameSideChat(id, text string) error {
	title := titleFromPrompt(text)
	s.mu.Lock()
	in, found := s.infoOf(id)
	s.mu.Unlock()
	if !found || !in.Side || in.Title != sideChatTitle || title == "" {
		return nil
	}
	return s.Rename(id, title)
}
