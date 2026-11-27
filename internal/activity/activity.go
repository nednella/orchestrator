// Package activity is what the app records of what it does, one event at a time.
package activity

import "github.com/nednella/orchestrator/internal/usage"

// The kinds of event.
const (
	Prompt      = "prompt"
	ThreadStart = "thread_start" // label "resumed" when a wake starts the thread again
	ThreadEnd   = "thread_end"   // ms is how long the thread ran
	Worked      = "worked"       // ms is how long a thread worked
	PROpened    = "pr_opened"
	PRMerged    = "pr_merged" // ms is the time from first seen open
	PRClosed    = "pr_closed" // ms is the time from first seen open
	Review      = "review"    // a review command was typed into the thread
	Checks      = "checks"    // a checks command was typed into the thread
	Conflict    = "conflict"  // a conflict command was typed into the thread
	IssueFiled  = "issue_filed"
	Cleanup     = "cleanup" // label done or blocked
	Evidence    = "evidence"
	Usage       = "usage" // the tokens one model used since the last count; label "subagent" for a thread's subagents
)

// Subagent is the label of usage by a thread's subagents.
const Subagent = "subagent"

// Resumed is the label of a thread's start when a wake starts it again under its own id.
const Resumed = "resumed"

// Event is one thing the app did or saw.
type Event struct {
	At     int64  `json:"at"` // unix ms
	Kind   string `json:"kind"`
	Thread string `json:"thread,omitempty"`
	Issue  int    `json:"issue,omitempty"`
	PR     int    `json:"pr,omitempty"`
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	Ms     int64  `json:"ms,omitempty"`
	Label  string `json:"label,omitempty"`
	Title  string `json:"title,omitempty"` // the thread title, on usage

	usage.Tokens // on usage; At is the time of the latest message counted
}
