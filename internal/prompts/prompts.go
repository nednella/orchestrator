// Package prompts holds the texts agentos gives to agents and prints for them, as Markdown files.
// Each file starts with an HTML comment that names its {token} placeholders; the callers fill them in.
package prompts

import (
	_ "embed"
	"strconv"
	"strings"
)

var (
	//go:embed digest.md
	digest string
	//go:embed browser.md
	browser string
	//go:embed setup-brief.md
	setupBrief string
	//go:embed setup-start.md
	setupStart string
	//go:embed setup-review.md
	setupReview string
	//go:embed setup-checks.md
	setupChecks string
	//go:embed setup-conflict.md
	setupConflict string
	//go:embed start-issue-branch.md
	startIssueBranch string
	//go:embed session.md
	session string
	//go:embed thread.md
	thread string
	//go:embed issue.md
	issue string
	//go:embed worktree.md
	worktree string
	//go:embed fork.md
	fork string
	//go:embed child.md
	child string
	//go:embed delegated.md
	delegated string
	//go:embed from-thread.md
	fromThread string
	//go:embed child-done.md
	childDone string
	//go:embed child-later.md
	childLater string
	//go:embed child-cut.md
	childCut string
	//go:embed child-no-reply.md
	childNoReply string
	//go:embed handover.md
	handover string
	//go:embed denied.md
	denied string
	//go:embed denied-no-reason.md
	deniedNoReason string
	//go:embed tool-delegate.md
	toolDelegate string
	//go:embed tool-threads.md
	toolThreads string
	//go:embed tool-read.md
	toolRead string
	//go:embed tool-send.md
	toolSend string
	//go:embed tool-wait.md
	toolWait string
	//go:embed tool-stop.md
	toolStop string
	//go:embed tool-note.md
	toolNote string
	//go:embed tool-issue.md
	toolIssue string
	//go:embed tool-new_thread.md
	toolNewThread string
	//go:embed tool-show.md
	toolShow string
	//go:embed tool-track.md
	toolTrack string
	//go:embed tool-refresh.md
	toolRefresh string
	//go:embed tool-browser.md
	toolBrowser string
)

// Digest is what the weekly digest run asks Claude to do, given the names the project depends on.
func Digest(dependencies []string) string {
	return strings.Replace(body(digest), "{dependencies}", strings.Join(dependencies, "\n"), 1)
}

// Browser is appended to a thread's system prompt when it has a browser.
func Browser() string { return body(browser) }

// Session starts the system prompt of every Claude thread: where it runs and the tools every thread uses.
func Session() string { return body(session) }

// Thread follows Session in the system prompt of every thread, whatever its access: what each access allows and the rules for branches.
func Thread() string { return body(thread) }

// Issue follows Thread in the system prompt of a thread that works on the GitHub issue number.
func Issue(number int) string {
	return strings.Replace(body(issue), "{number}", strconv.Itoa(number), 1)
}

// Worktree follows Thread in the system prompt of a thread that agentos started in the worktree it made, on branch.
func Worktree(branch, dir string) string {
	return strings.NewReplacer("{branch}", branch, "{worktree}", dir).Replace(body(worktree))
}

// Fork follows Thread in the system prompt of a fork of the thread called title: it has no branch of its own.
func Fork(title string) string { return strings.Replace(body(fork), "{title}", quotable(title), 1) }

// Child follows Thread, and Worktree when there is one, in the system prompt of a thread that the thread called parent
// started with delegate: its first message is its task, and its last reply of each turn is its result.
func Child(parent string) string {
	return strings.Replace(body(child), "{parent}", quotable(parent), 1)
}

// Delegated is the first message of a child thread: the task that the thread called parent gave it.
func Delegated(parent, task string) string {
	return strings.NewReplacer("{parent}", quotable(parent), "{task}", task).Replace(body(delegated))
}

// FromThread is a message that the thread called title, of that id, sends to another thread with the send tool.
func FromThread(title, id, text string) string {
	return strings.NewReplacer("{title}", quotable(title), "{id}", id, "{text}", text).Replace(body(fromThread))
}

// ChildDone tells a thread the result of a turn of its child, the thread of that id that does the task called title:
// how the turn ended, as outcome says ("completed", "failed: <why>" or "was stopped by the owner"), and the child's last
// reply, result. later marks a turn after the one whose result came first, so the result is not taken for a copy, and
// cut a result cut short, which read gives whole.
func ChildDone(title, id, outcome, result string, later, cut bool) string {
	text := childDone
	if later {
		text = childLater
	}
	if result == "" {
		result = body(childNoReply)
	}
	if cut {
		result += "\n\n" + strings.Replace(body(childCut), "{id}", id, 1)
	}
	return strings.NewReplacer("{title}", quotable(title), "{id}", id, "{outcome}", outcome, "{result}", result).Replace(body(text))
}

// quotable is a thread's title as a text may quote it. A title is the owner's to edit, and a thread may set its child's,
// so it is kept to one line and its double quotes become single ones: it cannot add a line of its own to the prompt or
// end the quotes it stands in.
func quotable(title string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(title), " "), `"`, "'")
}

// Handover ends the system prompt of a thread that another thread asked for, with the context that thread gave.
func Handover(context string) string { return strings.Replace(body(handover), "{context}", context, 1) }

// Denied is the reason a tool call is denied with when the user sends a message while it waits for their approval.
func Denied(text string) string { return strings.Replace(body(denied), "{text}", text, 1) }

// DeniedWithoutReason is the reason a tool call is denied with when the user gives none.
func DeniedWithoutReason() string { return body(deniedNoReason) }

// Tool is the description of an agentos tool, by its name; empty for a name that is not one.
func Tool(name string) string {
	switch name {
	case "delegate":
		return body(toolDelegate)
	case "threads":
		return body(toolThreads)
	case "read":
		return body(toolRead)
	case "send":
		return body(toolSend)
	case "wait":
		return body(toolWait)
	case "stop":
		return body(toolStop)
	case "note":
		return body(toolNote)
	case "issue":
		return body(toolIssue)
	case "new_thread":
		return body(toolNewThread)
	case "show":
		return body(toolShow)
	case "track":
		return body(toolTrack)
	case "refresh":
		return body(toolRefresh)
	case "browser":
		return body(toolBrowser)
	}
	return ""
}

// SetupBrief is added to the system prompt of the session that sets up a project's repository.
func SetupBrief() string { return body(setupBrief) }

// SetupStart is what the session that sets up a project's repository starts with.
func SetupStart() string { return body(setupStart) }

// SetupReview is the pr_review_command that setting up a project writes; {n} is the PR number.
func SetupReview() string { return body(setupReview) }

// SetupChecks is the pr_checks_command that setting up a project writes; {n} is the PR number.
func SetupChecks() string { return body(setupChecks) }

// SetupConflict is the pr_conflict_command that setting up a project writes; {n} is the PR number.
func SetupConflict() string { return body(setupConflict) }

// StartIssue is what a session for an issue is started with when the project defines no action for it.
// {n} is the issue number, {title} its title and {branch} the branch agentos makes for it.
func StartIssue() string { return body(startIssueBranch) }

// body drops the leading comment and the final newline of a file.
func body(file string) string {
	if strings.HasPrefix(file, "<!--") {
		_, file, _ = strings.Cut(file, "-->\n\n")
	}
	return strings.TrimSuffix(file, "\n")
}
