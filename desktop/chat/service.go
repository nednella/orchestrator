package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nednella/orchestrator/desktop/internal/run"
	"github.com/nednella/orchestrator/desktop/sessions"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	"github.com/nednella/orchestrator/internal/host"
)

// hostStarting is how long a message waits for the host of a thread that is still starting to listen.
const hostStarting = 3 * time.Second

// Service is bound to the front end.
type Service struct {
	c          *Chats
	files      *files
	projectDir func(project string) string
	ctx        func() context.Context
	plans      planCache
	// noRewind is why Edit from here is off for this run of the app, once Claude Code did not know a rewind request.
	noRewind atomic.Pointer[string]
}

// NewService binds the chats; runner runs git, projectDir gives a project's folder, "" for none, and ctx gives the
// app's context at call time.
func NewService(c *Chats, runner run.Runner, projectDir func(project string) string, ctx func() context.Context) *Service {
	return &Service{c: c, files: newFiles(runner), projectDir: projectDir, ctx: ctx}
}

// ChatHistory is the thread's logged events with a seq above after, live or settled. Call ChatWatch first and merge the
// two by seq: an event logged in between comes in both.
func (v *Service) ChatHistory(id string, after int64) ([]chatlog.Event, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return nil, err
	}
	events, err := chatlog.ReadLog(ch.Log, after)
	if err != nil {
		log.Printf("orchestrator: reading the chat log of %s: %v", id, err)
		return nil, fmt.Errorf("Could not read the chat of thread %s", id)
	}
	return shown(events), nil
}

// ChatWatch sends the thread's events from now on as chat:<id> events, deltas among them.
func (v *Service) ChatWatch(id string) error {
	ch, err := v.live(id)
	if err != nil {
		return err
	}
	return v.c.watch(ch)
}

// ChatUnwatch stops the thread's chat:<id> events.
func (v *Service) ChatUnwatch(id string) { v.c.unwatch(id) }

// Sent is a message ChatSend gave to Claude: the id it is logged under, and what the user should know when only its
// text went out.
type Sent struct {
	ID     string `json:"id"`
	Notice string `json:"notice,omitempty"`
}

// ChatSend gives the message, its images saved in the thread's media folder, to Claude: as the user's at once when the
// thread is idle, as queued while a turn runs, and as the reason to deny each approval that waits. A settled thread is
// woken for it.
func (v *Service) ChatSend(id, text string, images []Image) (Sent, error) {
	ch, err := v.awake(id)
	if err != nil {
		return Sent{}, err
	}
	files, err := saveImages(ch.Media, images)
	if err != nil {
		return Sent{}, err
	}
	ctx := v.ctx()
	ready, cancel := context.WithTimeout(ctx, hostStarting)
	err = host.Ready(ready, ch.Socket)
	cancel()
	var msg string
	if err == nil {
		msg, err = host.SendMessage(ctx, ch.Socket, host.Message{Text: text, Files: files})
	}
	if errors.Is(err, host.ErrTooOldForImages) {
		return Sent{Notice: "Your text was sent, but this thread's host is too old for images. Restart the thread to send images."}, nil
	}
	return Sent{ID: msg}, v.failed(id, "the message", err)
}

// ChatInterrupt stops the thread's turn.
func (v *Service) ChatInterrupt(id string) error {
	ch, err := v.live(id)
	if err != nil {
		return err
	}
	return v.failed(id, "the interrupt", host.Interrupt(v.ctx(), ch.Socket))
}

// ChatSendNow interrupts the turn so Claude takes in the queued message now.
func (v *Service) ChatSendNow(id, message string) error {
	ch, err := v.live(id)
	if err != nil {
		return err
	}
	return v.failed(id, "Send now", host.SendNow(v.ctx(), ch.Socket, message))
}

// ChatAnswer allows or denies the approval request; a deny tells Claude message.
func (v *Service) ChatAnswer(id, request string, allow bool, message string) error {
	ch, err := v.live(id)
	if err != nil {
		return err
	}
	return v.failed(id, "the answer", host.Answer(v.ctx(), ch.Socket, request, allow, message))
}

// ChatAnswerInput allows the approval request with input, a JSON object, in place of the tool's own: a question's
// answers.
func (v *Service) ChatAnswerInput(id, request, input string) error {
	ch, err := v.live(id)
	if err != nil {
		return err
	}
	if !json.Valid([]byte(input)) {
		return errors.New("The answer is not valid JSON")
	}
	return v.failed(id, "the answer", host.AnswerWith(v.ctx(), ch.Socket, request, json.RawMessage(input)))
}

// ChatFiles is the files of the thread's folder that best match query, for an @ mention: those git tracks and those
// it does not ignore. A folder outside git has none.
func (v *Service) ChatFiles(id, query string) ([]string, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return nil, err
	}
	return v.filesIn(ch.Dir, query, "thread "+id)
}

// ProjectFiles is the files of the project's folder, where a new thread starts, that best match query, as ChatFiles.
func (v *Service) ProjectFiles(project, query string) ([]string, error) {
	dir := v.projectDir(project)
	if dir == "" {
		return nil, fmt.Errorf("No project %s", project)
	}
	return v.filesIn(dir, query, "project "+project)
}

// filesIn is the files of dir that best match query; of names whose files they are, for the error.
func (v *Service) filesIn(dir, query, of string) ([]string, error) {
	if dir == "" {
		return []string{}, nil
	}
	paths, err := v.files.list(v.ctx(), dir)
	if err != nil {
		log.Printf("orchestrator: listing the files of %s in %s: %v", of, dir, err)
		return nil, fmt.Errorf("Could not list the files of %s", of)
	}
	return rankFiles(paths, query, fileLimit), nil
}

// ProjectCommands is the slash commands the project's threads reported last, none before any of them has run.
func (v *Service) ProjectCommands(project string) ([]Command, error) {
	if v.projectDir(project) == "" {
		return nil, fmt.Errorf("No project %s", project)
	}
	commands, err := v.c.commands.list(project)
	if err != nil {
		log.Printf("orchestrator: %v", err)
		return []Command{}, nil
	}
	return commands, nil
}

// FilesRewind is what Edit from here offers for a thread's files. Offered is false for a thread in the project folder,
// whose files a rewind never touches. Reason says why they cannot be rewound, "" when they can; Files, Insertions and
// Deletions are what a rewind of them would change.
type FilesRewind struct {
	Offered    bool     `json:"offered"`
	Reason     string   `json:"reason"`
	Files      []string `json:"files"`
	Insertions int      `json:"insertions"`
	Deletions  int      `json:"deletions"`
}

// RewindFilesOffer is whether Edit from here offers to put the thread's files back, and why it cannot when it does
// not, with no dry run: the thread is not woken for it.
func (v *Service) RewindFilesOffer(id string) FilesRewind {
	_, inProject, reason := v.c.threads.FilesRewind(id)
	return FilesRewind{Offered: !inProject, Reason: sentence(reason), Files: []string{}}
}

// RewindPreview is what a rewind of the thread's files to before its user message would change, from Claude's dry run,
// with nothing changed. A settled thread is woken for it, as a message would wake it.
func (v *Service) RewindPreview(id, message string) (FilesRewind, error) {
	if off := v.RewindOff(); off != "" {
		return FilesRewind{}, errors.New(off)
	}
	preview := v.RewindFilesOffer(id)
	if !preview.Offered || preview.Reason != "" {
		return preview, nil
	}
	ch, err := v.awake(id)
	if err != nil {
		return FilesRewind{}, err
	}
	dir, _, reason := v.c.threads.FilesRewind(id)
	if reason != "" {
		preview.Reason = sentence(reason)
		return preview, nil
	}
	changes, err := host.PreviewRewind(v.ctx(), ch.Socket, message, dir)
	if err != nil {
		return FilesRewind{}, v.rewindFailed(id, "the rewind's dry run", err)
	}
	preview.Reason, preview.Insertions, preview.Deletions = changes.Reason, changes.Insertions, changes.Deletions
	if changes.Files != nil {
		preview.Files = changes.Files
	}
	return preview, nil
}

// RewindTo takes the thread's conversation back to before its user message and returns that message's text, for the
// composer. With files it first puts back the files Claude's turns edited since, which it does only in the worktree
// orchestrator made for the thread while nobody else works there. A settled thread is woken first, as a message would wake
// it. When anything fails, the chat log and the files are left as they were, unless the files were put back and the
// conversation then was not, which the error says.
func (v *Service) RewindTo(id, message string, files bool) (string, error) {
	if off := v.RewindOff(); off != "" {
		return "", errors.New(off)
	}
	if _, inProject, _ := v.c.threads.FilesRewind(id); files && inProject {
		return "", errors.New("The files of the project folder are never rewound")
	}
	ch, err := v.awake(id)
	if err != nil {
		return "", err
	}
	within := ""
	if files {
		dir, _, reason := v.c.threads.FilesRewind(id)
		if reason != "" {
			return "", errors.New(sentence(reason))
		}
		within = dir
	}
	prefill, err := host.Rewind(v.ctx(), ch.Socket, message, within)
	return prefill, v.rewindFailed(id, "the rewind", err)
}

// RewindOff is why Edit from here is off for this run of the app, "" while it works.
func (v *Service) RewindOff() string {
	if off := v.noRewind.Load(); off != nil {
		return *off
	}
	return ""
}

// rewindFailed is failed for a rewind op. When Claude Code does not know a rewind request, Edit from here is off for
// the rest of the app's run; a host from before the rewind ops cannot rewind until the thread starts again.
func (v *Service) rewindFailed(id, what string, err error) error {
	refused := (*host.Refused)(nil)
	switch {
	case !errors.As(err, &refused):
	case refused.Unsupported:
		off := sentence(refused.Reason)
		v.noRewind.CompareAndSwap(nil, &off)
		log.Printf("orchestrator: thread %s did not take %s: %v", id, what, err)
		return errors.New(off)
	case strings.HasPrefix(refused.Reason, "unknown op"):
		log.Printf("orchestrator: thread %s did not take %s: %v", id, what, err)
		return fmt.Errorf("Thread %s runs under a host from before Edit from here; put it to sleep and wake it to edit", id)
	}
	return v.failed(id, what, err)
}

// ChatPending is the approval each live thread waits on.
func (v *Service) ChatPending() []Pending { return v.c.pending() }

// live is the chat of a thread that runs.
func (v *Service) live(id string) (sessions.Chat, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return sessions.Chat{}, err
	}
	if !ch.Live {
		return sessions.Chat{}, fmt.Errorf("Thread %s is asleep", id)
	}
	return ch, nil
}

// awake is the chat of a thread that runs, woken first when it has settled. When another wake started it first, it is
// that wake's chat, once its host listens.
func (v *Service) awake(id string) (sessions.Chat, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil || ch.Live {
		return ch, err
	}
	woken, err := v.c.threads.Wake(id)
	if err == nil {
		return woken, nil
	}
	ch, again := v.c.threads.ChatOf(id)
	if again != nil || !ch.Live {
		return sessions.Chat{}, err
	}
	ready, cancel := context.WithTimeout(v.ctx(), hostStarting)
	defer cancel()
	return ch, v.failed(id, "the wake", host.Ready(ready, ch.Socket))
}

// failed is the host's own reason when it refused, else why the thread did not take what, in plain words, with the
// error that reaching the host gave in the log only.
func (v *Service) failed(id, what string, err error) error {
	if err == nil {
		return nil
	}
	log.Printf("orchestrator: thread %s did not take %s: %v", id, what, err)
	if refused := (*host.Refused)(nil); errors.As(err, &refused) {
		return errors.New(sentence(refused.Reason))
	}
	return fmt.Errorf("Thread %s did not take %s", id, what)
}

func sentence(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
