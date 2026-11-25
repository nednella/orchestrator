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

// Message is what the user sends: text and the paths of images to go before it.
type Message struct {
	Text  string
	Files []string
}

// SendMessage gives m to Claude and returns the id the host logged it under.
func SendMessage(ctx context.Context, socket string, m Message) (string, error) {
	rep, err := call(ctx, socket, request{Op: opSend, Text: m.Text, Files: m.Files})
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
		return fail(fmt.Errorf("%s", rep.Error))
	}
	return conn, r, rep, nil
}
