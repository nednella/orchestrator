package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nednella/agentos/desktop/sessions"
	chatlog "github.com/nednella/agentos/internal/chat"
	"github.com/nednella/agentos/internal/host"
)

// hostStarting is how long a message waits for the host of a thread that is still starting to listen.
const hostStarting = 3 * time.Second

// Service is bound to the front end.
type Service struct {
	c   *Chats
	ctx func() context.Context
}

// NewService binds the chats; ctx gives the app's context at call time.
func NewService(c *Chats, ctx func() context.Context) *Service { return &Service{c: c, ctx: ctx} }

// ChatHistory is the thread's logged events with a seq above after, live or ended. Call ChatWatch first and merge the
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
	if _, err := v.live(id); err != nil {
		return err
	}
	v.c.watch(id)
	return nil
}

// ChatUnwatch stops the thread's chat:<id> events.
func (v *Service) ChatUnwatch(id string) { v.c.unwatch(id) }

// ChatSend gives the message to Claude and returns the id it is logged under: as the user's at once when the thread
// is idle, as queued while a turn runs, and as the reason to deny each approval that waits.
func (v *Service) ChatSend(id, text string) (string, error) {
	ch, err := v.live(id)
	if err != nil {
		return "", err
	}
	ctx := v.ctx()
	ready, cancel := context.WithTimeout(ctx, hostStarting)
	err = host.Ready(ready, ch.Socket)
	cancel()
	var msg string
	if err == nil {
		msg, err = host.SendMessage(ctx, ch.Socket, host.Message{Text: text})
	}
	return msg, v.failed(id, "the message", err)
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

// ChatPending is the approval each live thread waits on.
func (v *Service) ChatPending() []Pending { return v.c.pending() }

// live is the chat of a thread that runs.
func (v *Service) live(id string) (sessions.Chat, error) {
	ch, err := v.c.threads.ChatOf(id)
	if err != nil {
		return sessions.Chat{}, err
	}
	if !ch.Live {
		return sessions.Chat{}, fmt.Errorf("Thread %s has ended", id)
	}
	return ch, nil
}

// failed logs why the thread's host did not take what, and says so in plain words.
func (v *Service) failed(id, what string, err error) error {
	if err == nil {
		return nil
	}
	log.Printf("agentos: thread %s did not take %s: %v", id, what, err)
	return fmt.Errorf("Thread %s did not take %s", id, what)
}
