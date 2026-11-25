package host

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/nednella/agentos/internal/chat"
)

const (
	callTimeout = 10 * time.Second
	// rewindTimeout covers the control requests a rewind writes, each of which waits up to the host's requestTimeout.
	rewindTimeout = 4 * requestTimeout
	readyPoll     = 50 * time.Millisecond
)

// Refused is the host's own reason for not doing what it was asked, in words for the user. An error that wraps no
// Refused is one of reaching the host. Unsupported says that the Claude Code the host runs does not know a control
// request the op needs.
type Refused struct {
	Reason      string
	Unsupported bool
}

func (r *Refused) Error() string { return r.Reason }

// Message is what the user sends: text and the paths of images to go before it. From is the thread that sends it, ""
// for the owner or the app.
type Message struct {
	Text  string
	Files []string
	From  string
}

// ErrTooOldForImages is what SendMessage returns when a host from before message ids sent the text of a message and
// dropped its images.
var ErrTooOldForImages = &Refused{Reason: "this session's host is too old for images"}

// ErrApprovalWaits is what SendMessage returns for a message from a thread while an approval waits: the host sends
// none, since a message then would deny the approval with its text.
var ErrApprovalWaits = &Refused{Reason: "this thread waits on an approval"}

// SendMessage gives m to Claude and returns the id the host logged it under. A host from before message ids sends the
// text and drops the images, so a reply with no id is ErrTooOldForImages when m has images. A host from before From
// sends the message as the owner's, also while an approval waits.
func SendMessage(ctx context.Context, socket string, m Message) (string, error) {
	rep, err := call(ctx, socket, request{Op: opSend, Text: m.Text, Files: m.Files, From: m.From})
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

func Interrupt(ctx context.Context, socket string) error { return InterruptFrom(ctx, socket, "") }

// InterruptFrom stops the turn at the ask of the thread from, which the host logs on the interrupt.
func InterruptFrom(ctx context.Context, socket, from string) error {
	_, err := call(ctx, socket, request{Op: opInterrupt, From: from})
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

// PlanStop tells the host that it is about to be stopped on purpose, for reason: chat.StopSettle, or chat.StopRestart
// with the access the thread starts again with. The host then logs its exit as that stop and not as an error.
func PlanStop(ctx context.Context, socket, reason, access string) error {
	_, err := call(ctx, socket, request{Op: opStop, Reason: reason, Access: access})
	return err
}

// Release undoes a hold and a planned stop, for a restart or a settle that did not happen: the host takes input again,
// and a later exit is logged as it comes.
func Release(ctx context.Context, socket string) error {
	_, err := call(ctx, socket, request{Op: opRelease})
	return err
}

// PreviewRewind says what a rewind of the files to before the user message id would change, and changes nothing. A
// file outside within is the answer's reason.
func PreviewRewind(ctx context.Context, socket, id, within string) (FileChanges, error) {
	rep, err := callWithin(ctx, socket, request{Op: opRewindPreview, ID: id, Within: within}, rewindTimeout)
	if err != nil || rep.Changes == nil {
		return FileChanges{}, err
	}
	return *rep.Changes, nil
}

// Rewind takes Claude's conversation back to before the user message id and returns that message's text. With within
// set, it first puts back the files Claude's turns edited since then, all of which must be inside within.
func Rewind(ctx context.Context, socket, id, within string) (string, error) {
	rep, err := callWithin(ctx, socket, request{Op: opRewind, ID: id, Within: within}, rewindTimeout)
	return rep.Prefill, err
}

// Subscribe calls fn with each logged event after `after`, then with live events, until ctx ends or the host goes away. A host drops a subscriber that falls too far behind, so fn should not block for long; subscribe again after the last Seq seen to go on.
func Subscribe(ctx context.Context, socket string, after int64, fn func(chat.Event)) error {
	conn, r, _, err := dial(ctx, socket, request{Op: opSubscribe, After: after}, callTimeout)
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

// Gone waits until no host listens on socket: one that stops logs its last event before it closes the socket.
func Gone(ctx context.Context, socket string) error {
	var d net.Dialer
	for {
		conn, err := d.DialContext(ctx, "unix", socket)
		// Only a refused dial or a missing socket means the host is gone: a busy listener's full backlog also fails a dial.
		if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
			return nil
		}
		if err == nil {
			conn.Close()
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("host at %s still listens: %w", socket, ctx.Err())
		case <-time.After(readyPoll):
		}
	}
}

func call(ctx context.Context, socket string, req request) (reply, error) {
	return callWithin(ctx, socket, req, callTimeout)
}

func callWithin(ctx context.Context, socket string, req request, timeout time.Duration) (reply, error) {
	conn, _, rep, err := dial(ctx, socket, req, timeout)
	if err != nil {
		return reply{}, err
	}
	conn.Close()
	return rep, nil
}

// dial sends req to the host and reads its reply, within timeout. The connection stays open for a subscribe stream.
func dial(ctx context.Context, socket string, req request, timeout time.Duration) (net.Conn, *bufio.Reader, reply, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
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
	if rep.Approval {
		return fail(ErrApprovalWaits)
	}
	if !rep.OK {
		return fail(&Refused{Reason: rep.Error, Unsupported: rep.Unsupported})
	}
	return conn, r, rep, nil
}
