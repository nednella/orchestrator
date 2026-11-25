package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/spf13/cobra"

	"github.com/nednella/agentos/internal/agent"
	"github.com/nednella/agentos/internal/bus"
	"github.com/nednella/agentos/internal/session"
)

const hookBudget = 200 * time.Millisecond

// newHookCmd is what the agent runs on each hook event. It must never break a turn, so it always exits 0 and
// prints only the reply its agent reads.
func newHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "hook <event>",
		Short:  "Record an agent hook event (called by the agents)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reportHook(args[0], cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

// reportHook saves the session's new state and tells the app, within a short
// budget: the file is already saved, so a miss costs only a delay.
func reportHook(event string, stdin io.Reader, stdout io.Writer) {
	reportHookAt(event, stdin, stdout, time.Now())
}

// noSessionReply is what a hook prints outside a session: the owner's own agy still calls agentos from its global
// hooks, and reads a JSON object from each, but takes an empty object on PreToolUse for a deny. Only no output
// leaves that decision to it.
func noSessionReply(event string) string {
	if event == "PreToolUse" {
		return ""
	}
	return "{}"
}

// reportHookAt records the event as of at, the moment the hook started: a hook that waited
// for the lock must not undo a later event that got in first.
func reportHookAt(event string, stdin io.Reader, stdout io.Writer, at time.Time) {
	name := os.Getenv("AGENTOS_SESSION")
	socket := os.Getenv("AGENTOS_SOCKET")
	if _, err := session.ParseName(name); err != nil || socket == "" {
		fmt.Fprint(stdout, noSessionReply(event))
		return
	}
	dir := bus.DirOf(socket)
	ev, reply, ok := agent.Claude{}.ParseEvent(event, stdin)
	fmt.Fprint(stdout, reply)
	if !ok {
		return
	}
	rec, ok := saveHook(dir, name, ev, at)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), hookBudget)
	defer cancel()
	_ = bus.Send(ctx, socket, rec)
}

// saveHook folds the event into the saved state under the file's lock. It reports whether the state changed.
func saveHook(dir, name string, ev session.Event, at time.Time) (session.Record, bool) {
	unlock, err := bus.LockState(dir, name)
	if err != nil {
		return session.Record{}, false
	}
	defer unlock()
	prev, _ := bus.ReadState(dir, name)
	if prev.Session == "" && ev.Name != "SessionStart" || prev.State == session.Archived {
		return session.Record{}, false // a hook that fires after the thread was deleted, or archived by an older build, must not bring it back
	}
	if prev.At.After(at) {
		return session.Record{}, false
	}
	rec := session.Apply(name, prev, ev, at)
	if reflect.DeepEqual(rec, prev) || bus.WriteState(dir, rec) != nil {
		return session.Record{}, false
	}
	return rec, true
}
