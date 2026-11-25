package host

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/nednella/agentos/internal/chat"
)

const (
	callTimeout = 10 * time.Second
	readyPoll   = 50 * time.Millisecond
)

// Refused is the host's own reason for not doing what it was asked, in words for the user. An error that wraps no
// Refused is one of reaching the host.
type Refused struct{ Reason string }

func (r *Refused) Error() string { return r.Reason }

// Message is what the user sends: text and the paths of images to go before it.
type Message struct {
	Text  string
	Files []string
}

// ErrTooOldForImages is what SendMessage returns when a host from before message ids sent the text of a message and
// dropped its images.
var ErrTooOldForImages = &Refused{"this session's host is too old for images"}

// SendMessage gives m to Claude and returns the id the host logged it under. A host from before message ids sends the
// text and drops the images, so a reply with no id is ErrTooOldForImages when m has images.
func SendMessage(ctx context.Context, socket string, m Message) (string, error) {
	rep, err := call(ctx, socket, request{Op: opSend, Text: m.Text, Files: m.Files})
	if err == nil && rep.ID == "" && len(m.Files) > 0 {
		return "", ErrTooOldForImages
	}
	return rep.ID, err
}

func Send(ctx context.Context, socket, text string) error {
	_, err := SendMessage(ctx, socket, Message{Text: text})
	return err
}

// SendNow interrupts the turn so Claude takes in the queued message id now, with any queued alongside it.
func SendNow(ctx context.Context, socket, id string) error {
	_, err := call(ctx, socket, request{Op: opSendNow, ID: id})
	return err
}

func Draft(ctx context.Context, socket, text string) error {
	_, err := call(ctx, socket, request{Op: opDraft, Text: text})
	return err
}

func Interrupt(ctx context.Context, socket string) error {
	_, err := call(ctx, socket, request{Op: opInterrupt})
	return err
}

func Answer(ctx context.Context, socket, id string, allow bool, message string) error {
	_, err := call(ctx, socket, request{Op: opAnswer, ID: id, Allow: allow, Message: message})
	return err
}

// AnswerWith allows the approval id with input in place of the tool's own, such as a question's answers.
func AnswerWith(ctx context.Context, socket, id string, input json.RawMessage) error {
	_, err := call(ctx, socket, request{Op: opAnswer, ID: id, Allow: true, Input: input})
	return err
}

// SetMode changes Claude's permission mode, plan or default, between turns. A refusal is Claude's own reason, or
// the host's while a turn runs.
func SetMode(ctx context.Context, socket, mode string) error {
	_, err := call(ctx, socket, request{Op: opMode, Mode: mode})
	return err
}

// Hold says whether the host is idle, with no turn running, no approval waiting and no message it was given still to
// run, and when it is, holds it: the host then refuses every send, answer and mode change until it exits. A restart
// holds the host before it stops it.
func Hold(ctx context.Context, socket string) (idle bool, err error) {
	rep, err := call(ctx, socket, request{Op: opHold})
	return err == nil && !rep.Busy, err
}

// Subscribe calls fn with each logged event after `after`, then with live events, until ctx ends or the host goes away. A host drops a subscriber that falls too far behind, so fn should not block for long; subscribe again after the last Seq seen to go on.
func Subscribe(ctx context.Context, socket string, after int64, fn func(chat.Event)) error {
	conn, r, _, err := dial(ctx, socket, request{Op: opSubscribe, After: after})
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("host stream ended: %w", err)
		}
		var e chat.Event
		if err := json.Unmarshal(line, &e); err != nil {
			return fmt.Errorf("read host event: %w", err)
		}
		fn(e)
	}
}

// Ready waits until the host's socket accepts a connection.
func Ready(ctx context.Context, socket string) error {
	var d net.Dialer
	for {
		conn, err := d.DialContext(ctx, "unix", socket)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("host at %s is not ready: %w", socket, err)
		case <-time.After(readyPoll):
		}
	}
}

func call(ctx context.Context, socket string, req request) (reply, error) {
	conn, _, rep, err := dial(ctx, socket, req)
	if err != nil {
		return reply{}, err
	}
	conn.Close()
	return rep, nil
}

// dial sends req to the host and reads its reply, within callTimeout. The connection stays open for a subscribe stream.
func dial(ctx context.Context, socket string, req request) (net.Conn, *bufio.Reader, reply, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, nil, reply{}, fmt.Errorf("reach host at %s: %w", socket, err)
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	fail := func(err error) (net.Conn, *bufio.Reader, reply, error) {
		stop()
		conn.Close()
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, nil, reply{}, fmt.Errorf("host at %s: %w", socket, err)
	}
	b, err := json.Marshal(req)
	if err != nil {
		return fail(err)
	}
	if _, err := conn.Write(append(b, '\n')); err != nil {
		return fail(err)
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return fail(err)
	}
	if !stop() {
		return fail(ctx.Err())
	}
	var rep reply
	if err := json.Unmarshal(line, &rep); err != nil {
		return fail(err)
	}
	if !rep.OK {
		return fail(&Refused{rep.Error})
	}
	return conn, r, rep, nil
}
