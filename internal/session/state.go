package session

import (
	"fmt"
	"slices"
	"sort"
	"time"
)

type State string

const (
	Idle    State = "idle"    // started, or a turn ended: the user's move
	Working State = "working" // the agent is running
	Waiting State = "waiting" // blocked on the user
	Settled State = "settled" // the agent's session is gone; the row stays until deleted
)

// Record is a session's state as the state file holds it.
type Record struct {
	Session      string    `json:"session"`
	State        State     `json:"state"`
	Event        string    `json:"event"`
	At           time.Time `json:"at"`
	Detail       string    `json:"detail,omitempty"`
	Tool         string    `json:"tool,omitempty"`         // the latest tool call, kept through the prompt it leads to
	Command      string    `json:"command,omitempty"`      // that call's shell command
	Notify       string    `json:"notify,omitempty"`       // notification_type of a Notification
	Cwd          string    `json:"cwd,omitempty"`          // where the agent was working at the latest event
	Conversation string    `json:"conversation,omitempty"` // the agent's id for its conversation, to resume it
	Transcript   string    `json:"transcript,omitempty"`   // the file the agent writes its conversation to
	AgentModel   string    `json:"agentModel,omitempty"`   // the model the agent says it runs, when its stream says; prices a transcript that names none
	Changes      []Change  `json:"changes,omitempty"`      // the latest state changes, oldest first, so the timeline outlives the app
	Observed     int64     `json:"observed,omitempty"`     // the latest seq of the chat log whose event the state holds, so a restart folds in each only once

	Identity
	SettledAt int64 `json:"settledAt,omitempty"` // unix ms, set by the desktop app when it sees the session end
}

// Identity is what the desktop app started a session as. The app writes it when the session starts and wakes, so a
// session that ends while the app is closed comes back as itself. Model and Effort follow the agent: the start
// choice until its first turn, then what its transcript or its stream says it used last.
type Identity struct {
	N       int    `json:"n,omitempty"` // the thread's number in its project: given once, never changed or given again
	Title   string `json:"title,omitempty"`
	Issue   int    `json:"issue,omitempty"`
	Access  Access `json:"access,omitempty"`
	Action  string `json:"action,omitempty"`
	Model   string `json:"model,omitempty"`
	Effort  string `json:"effort,omitempty"`
	Fast    bool   `json:"fast,omitempty"` // the agent runs in its faster mode
	Path    string `json:"path,omitempty"`
	Fork    string `json:"fork,omitempty"`    // the session a fork was forked from
	ForkAt  string `json:"forkAt,omitempty"`  // the uuid of the reply the fork's chat was cut at
	Side    bool   `json:"side,omitempty"`    // the fork is a side chat of the thread Fork names
	Parent  string `json:"parent,omitempty"`  // the thread that delegated to this one, "" for a thread the owner started
	Branch  string `json:"branch,omitempty"`  // the branch the thread works on, in the worktree it started in; "" in the project folder
	Created int64  `json:"created,omitempty"` // unix ms of the first start
	Woke    int64  `json:"woke,omitempty"`    // unix ms of the latest start: the first one or a wake
	// Reported is the seq, in the child's chat log, of the latest turn_end whose result went to its parent, so a restart
	// never sends it again.
	Reported int64 `json:"reported,omitempty"`
}

// Access is what a thread may do without asking the owner.
type Access string

const (
	Read       Access = "read"       // it reads and plans; Claude asks before it changes anything
	Supervised Access = "supervised" // it writes; the owner's allow rules apply and Claude asks for the rest
	Full       Access = "full"       // every tool runs without asking
)

// ParseAccess reads an access level; "" is Supervised.
func ParseAccess(s string) (Access, error) {
	switch a := Access(s); a {
	case "":
		return Supervised, nil
	case Read, Supervised, Full:
		return a, nil
	}
	return "", fmt.Errorf("access must be %s, %s or %s, not %q", Read, Supervised, Full, s)
}

// StoredAccess is the access a tag or state file holds: Read for none or for a value this build does not know, so a
// thread never gains access it was not given.
func StoredAccess(s string) Access {
	switch a := Access(s); a {
	case Supervised, Full:
		return a
	}
	return Read
}

// Rank orders the levels by what they allow: Read, then Supervised, then Full.
func (a Access) Rank() int {
	switch a {
	case Supervised:
		return 1
	case Full:
		return 2
	}
	return 0
}

// Change is one entry of a session's state timeline.
type Change struct {
	State State `json:"state"`
	At    int64 `json:"at"` // unix ms
}

// ChangesCap is how many changes a timeline keeps.
const ChangesCap = 200

// AddChange appends c to changes and keeps the latest ChangesCap of them. It never writes into the array of changes.
func AddChange(changes []Change, c Change) []Change {
	out := append(slices.Clip(changes), c)
	if len(out) > ChangesCap {
		out = slices.Clone(out[len(out)-ChangesCap:])
	}
	return out
}

// Session is one agent as the screen sees it.
type Session struct {
	Name   Name
	N      int // the number shown, 0 when none was given
	Title  string
	State  State
	At     time.Time
	Detail string
}

func (s State) rank() int {
	switch s {
	case Waiting:
		return 0
	case Idle:
		return 1
	case Working:
		return 2
	case Settled:
		return 3
	}
	return 4
}

// Sort orders sessions by who needs the user most: waiting, idle, working, settled, then most recent event first.
func Sort(ss []Session) {
	sort.SliceStable(ss, func(i, j int) bool {
		a, b := ss[i], ss[j]
		if a.State.rank() != b.State.rank() {
			return a.State.rank() < b.State.rank()
		}
		if !a.At.Equal(b.At) {
			return a.At.After(b.At)
		}
		if a.N != b.N {
			return a.N < b.N
		}
		return a.Name.String() < b.Name.String()
	})
}
