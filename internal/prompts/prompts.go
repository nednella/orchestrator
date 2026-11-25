// Package prompts holds the texts agentos gives to agents and prints for them, as Markdown files.
// Each file starts with an HTML comment that names its {token} placeholders; the callers fill them in.
package prompts

import (
	_ "embed"
	"strings"
)

var (
	//go:embed digest.md
	digest string
	//go:embed browser.md
	browser string
	//go:embed browser-help.md
	browserHelp string
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
	//go:embed fork.md
	fork string
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
)

// Digest is what the weekly digest run asks Claude to do, given the names the project depends on.
func Digest(dependencies []string) string {
	return strings.Replace(body(digest), "{dependencies}", strings.Join(dependencies, "\n"), 1)
}

// Browser is appended to a thread's system prompt when it has a browser.
func Browser() string { return body(browser) }

// Session starts the system prompt of every Claude thread: where it runs and the commands every thread uses.
func Session() string { return body(session) }

// Thread follows Session in the system prompt of every thread, whatever its access: what each access allows and the rules for branches.
func Thread() string { return body(thread) }

// Fork follows Thread in the system prompt of a fork of the thread called title: it has no branch of its own. The
// title is the owner's to edit, so it is kept to one line and its double quotes become single ones: it cannot add a
// line of its own to the prompt or end the quotes it stands in.
func Fork(title string) string {
	title = strings.ReplaceAll(strings.Join(strings.Fields(title), " "), `"`, "'")
	return strings.Replace(body(fork), "{title}", title, 1)
}

// Handover ends the system prompt of a thread that another thread asked for, with the context that thread gave.
func Handover(context string) string { return strings.Replace(body(handover), "{context}", context, 1) }

// Denied is the reason a tool call is denied with when the user sends a message while it waits for their approval.
func Denied(text string) string { return strings.Replace(body(denied), "{text}", text, 1) }

// DeniedWithoutReason is the reason a tool call is denied with when the user gives none.
func DeniedWithoutReason() string { return body(deniedNoReason) }

// Tool is the description of an orchestration tool, by its name; empty for a name that is not one.
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
	}
	return ""
}

// BrowserHelp is what agentos browser help prints.
func BrowserHelp() string { return body(browserHelp) }

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
