package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/nednella/orchestrator/desktop/threads"
	chatlog "github.com/nednella/orchestrator/internal/chat"
	ctl "github.com/nednella/orchestrator/internal/control"
	"github.com/nednella/orchestrator/internal/host"
	"github.com/nednella/orchestrator/internal/prompts"
)

type sendInput struct {
	Thread string `json:"thread"`
	Text   string `json:"text"`
	Steer  bool   `json:"steer"`
}

// sent is what send answers: the id the message was logged under, "" while it is held for an approval to be answered,
// and whether the thread was settled and woke for it.
type sent struct {
	ID   string `json:"id"`
	Held bool   `json:"held"`
	Woke bool   `json:"woke"`
}

// send answers the send tool: it gives the text to the thread the input names, framed as the caller's, so the thread
// never takes it for the owner's. A thread may message any other thread but a side chat whose access is not above its
// own: a thread with less access never has one with more act for it.
func (c *Commands) send(_ context.Context, caller string, req ctl.Request) (string, error) {
	var in sendInput
	if err := decode(req.Opts["input"], &in); err != nil {
		return "", err
	}
	switch {
	case in.Thread == "":
		return "", errors.New("send needs the id of a thread")
	case strings.TrimSpace(in.Text) == "":
		return "", errors.New("send needs a text")
	case in.Thread == caller:
		return "", errors.New("A thread cannot send to itself")
	}
	if err := c.reachable(in.Thread); err != nil {
		return "", err
	}
	ch, err := c.threads.ChatOf(in.Thread)
	settled := err == nil && !ch.Live
	text := threads.FromApp(prompts.FromThread(c.threads.Title(caller), caller, in.Text))
	id, err := c.threads.Tell(in.Thread, threads.Told{From: caller, Text: text, Body: in.Text, Steer: in.Steer, Ranked: true})
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(sent{ID: id, Held: id == "", Woke: settled})
	if err != nil {
		return "", fmt.Errorf("writing the answer: %w", err)
	}
	return string(data), nil
}

type stopInput struct {
	Thread string `json:"thread"`
}

// stop answers the stop tool: it ends the running turn of a descendant of the caller and of each of that thread's own
// descendants, and answers those it stopped and how many messages held for them it dropped, so none of them starts a
// new turn once its approval ends. Each interrupt is logged as from the caller, so a stop the caller asked for is told
// from the owner's. No other thread can be stopped.
func (c *Commands) stop(ctx context.Context, caller string, req ctl.Request) (string, error) {
	var in stopInput
	if err := decode(req.Opts["input"], &in); err != nil {
		return "", err
	}
	if in.Thread == "" {
		return "", errors.New("stop needs the id of a thread")
	}
	if !slices.Contains(c.threads.Descendants(caller), in.Thread) {
		return "", fmt.Errorf("Thread %s is not a child of this thread", in.Thread)
	}
	stopped, dropped := []string{}, 0
	for _, id := range append([]string{in.Thread}, c.threads.Descendants(in.Thread)...) {
		ch, err := c.threads.ChatOf(id)
		if err != nil || !ch.Live {
			continue
		}
		events, err := chatlog.ReadLog(ch.Log, 0)
		if err != nil {
			log.Printf("orchestrator: reading the chat log of %s: %v", id, err)
			continue
		}
		if !turnRuns(events) {
			continue
		}
		dropped += c.threads.DropTold(id)
		if err := host.InterruptFrom(ctx, ch.Socket, caller); err != nil {
			log.Printf("orchestrator: thread %s did not stop thread %s: %v", caller, id, err)
			continue
		}
		stopped = append(stopped, id)
	}
	data, err := json.Marshal(struct {
		Stopped []string `json:"stopped"`
		Dropped int      `json:"dropped"`
	}{stopped, dropped})
	if err != nil {
		return "", fmt.Errorf("writing the answer: %w", err)
	}
	return string(data), nil
}

// turnRuns says whether the chat's last turn is open: a message went in after the last turn ended or Claude last
// started.
func turnRuns(events []chatlog.Event) bool {
	open := false
	for _, e := range events {
		switch e.Kind {
		case chatlog.KindUser, chatlog.KindQueued:
			open = true
		case chatlog.KindTurnEnd, chatlog.KindExit, chatlog.KindStart:
			open = false
		}
	}
	return open
}
