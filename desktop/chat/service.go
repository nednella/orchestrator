package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nednella/agentos/desktop/internal/run"
	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/host"
)

// hostStarting is how long a message waits for the host of a thread that is still starting to listen.
const hostStarting = 3 * time.Second

// Service is bound to the front end.
type Service struct {
	c     *Chats
	files *files
	ctx   func() context.Context
}

// NewService binds the chats; runner runs git, and ctx gives the app's context at call time.
func NewService(c *Chats, runner run.Runner, ctx func() context.Context) *Service {
	return &Service{c: c, files: newFiles(runner), ctx: ctx}
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
		log.Printf("agentos: reading the chat log of %s: %v", id, err)
		return nil, fmt.Errorf("Could not read the chat of thread %s", id)
	}
	return events, nil
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
// thread is idle, as queued while a turn runs, and as the reason to deny each approval that waits.
func (v *Service) ChatSend(id, text string, images []Image) (Sent, error) {
	ch, err := v.live(id)
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

// ChatDraft leaves text in the thread's composer, unsent.
func (v *Service) ChatDraft(id, text string) error {
	ch, err := v.live(id)
	if err != nil {
		return err
	}
	return v.failed(id, "the draft", host.Draft(v.ctx(), ch.Socket, text))
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
	if ch.Dir == "" {
		return []string{}, nil
	}
	paths, err := v.files.list(v.ctx(), ch.Dir)
	if err != nil {
		log.Printf("agentos: listing the files of thread %s in %s: %v", id, ch.Dir, err)
		return nil, fmt.Errorf("Could not list the files of thread %s", id)
	}
	return rankFiles(paths, query, fileLimit), nil
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
		return sessions.Chat{}, fmt.Errorf("Thread %s has settled", id)
	}
	return ch, nil
}

// failed is the host's own reason when it refused, else why the thread did not take what, in plain words, with the
// error that reaching the host gave in the log only.
func (v *Service) failed(id, what string, err error) error {
	if err == nil {
		return nil
	}
	log.Printf("agentos: thread %s did not take %s: %v", id, what, err)
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
