package mcp

import (
	"encoding/json"

	"github.com/nednella/orchestrator/internal/prompts"
)

const (
	threadField = `{"type":"string","description":"The thread's id."}`
	noExtra     = `"additionalProperties":false`
)

const (
	// Server is the name a thread's MCP config gives the orchestrator server.
	Server = "orchestrator"
	// ToolPrefix starts the name Claude gives each of the server's tools.
	ToolPrefix = "mcp__" + Server + "__"
	// TokenEnv holds the thread's token, in the server's environment only.
	TokenEnv = "ORCHESTRATOR_MCP_TOKEN"
	// ControlEnv holds the path of the app's control socket, in the server's environment and in the thread's, which
	// it names to the app that started it.
	ControlEnv = "ORCHESTRATOR_CONTROL"
)

// spec is one tool as the server lists it. writes marks a tool that only a thread that writes runs without asking.
type spec struct {
	name     string
	schema   string
	readOnly bool
	writes   bool
}

func specs() []spec {
	return []spec{
		{name: "delegate", schema: `{"type":"object","properties":{
			"task":{"type":"string","description":"What the child must do."},
			"title":{"type":"string","description":"The child's title."},
			"access":{"type":"string","enum":["read","supervised","full"],"description":"The child's access; never more than yours. Default: yours."},
			"model":{"type":"string","description":"The child's model."},
			"effort":{"type":"string","description":"The child's effort."},
			"project":{"type":"string","description":"A project's name or key. Default: yours. In another project, folder is its folder and worktree starts from its default branch."},
			"where":{"type":"string","enum":["folder","worktree","issue"],"description":"Where the child works. Default: folder for a read child, worktree for the others."},
			"issue":{"type":"integer","description":"The issue number; needed with where issue."}
		},"required":["task"],` + noExtra + `}`},
		{name: "threads", schema: `{"type":"object","properties":{
			"project":{"type":"string","description":"A project's name, a key, or all. Default: yours."}
		},` + noExtra + `}`, readOnly: true},
		{name: "read", schema: `{"type":"object","properties":{
			"thread":` + threadField + `,
			"view":{"type":"string","enum":["messages","activity"],"description":"What to read. Default: messages."},
			"before":{"type":"integer","description":"A sequence number: read the entries before it. Default: the newest."},
			"limit":{"type":"integer","minimum":1,"maximum":100,"description":"How many entries. Default: 20."}
		},"required":["thread"],` + noExtra + `}`, readOnly: true},
		{name: "send", schema: `{"type":"object","properties":{
			"thread":` + threadField + `,
			"text":{"type":"string","description":"The message."},
			"steer":{"type":"boolean","description":"True: stop the running turn and send now. Default false: the thread takes it in at its next step."}
		},"required":["thread","text"],` + noExtra + `}`},
		{name: "wait", schema: `{"type":"object","properties":{
			"thread":` + threadField + `,
			"timeout_seconds":{"type":"integer","minimum":1,"maximum":3600,"description":"How long to wait. Default: 300."}
		},"required":["thread"],` + noExtra + `}`, readOnly: true},
		{name: "stop", schema: `{"type":"object","properties":{
			"thread":` + threadField + `
		},"required":["thread"],` + noExtra + `}`},
		{name: "note", schema: `{"type":"object","properties":{
			"text":{"type":"string","description":"The note. Its first line is its title."}
		},"required":["text"],` + noExtra + `}`, readOnly: true},
		{name: "issue", schema: `{"type":"object","properties":{
			"number":{"type":"integer","minimum":1,"description":"The issue's number."},
			"prompt":{"type":"string","description":"What the new thread starts with: your context for it, at most 6 KB."}
		},"required":["number"],` + noExtra + `}`, readOnly: true},
		{name: "new_thread", schema: `{"type":"object","properties":{
			"title":{"type":"string","description":"A short title."},
			"prompt":{"type":"string","description":"The new thread's first message, at most 6 KB."},
			"access":{"type":"string","enum":["read","supervised","full"],"description":"The new thread's access; never more than yours. Default: yours."}
		},"required":["title","prompt"],` + noExtra + `}`, readOnly: true},
		{name: "show", schema: `{"type":"object","properties":{
			"file":{"type":"string","description":"The absolute path of an image or a text file. Give file or text, not both."},
			"text":{"type":"string","description":"Words to show as a card. Give file or text, not both."},
			"caption":{"type":"string","description":"What the evidence shows."}
		},` + noExtra + `}`, writes: true},
		{name: "track", schema: `{"type":"object","properties":{
			"branch":{"type":"string","description":"The branch this thread works on."}
		},"required":["branch"],` + noExtra + `}`, writes: true},
		{name: "refresh", schema: `{"type":"object","properties":{},` + noExtra + `}`, readOnly: true},
		{name: "browser", schema: `{"type":"object","properties":{
			"action":{"type":"string","enum":["open","screenshot","close"],"description":"open loads a page in your window, screenshot files the page as evidence, close closes your window."},
			"url":{"type":"string","description":"The page to open; needed with open."},
			"front":{"type":"boolean","description":"With open: raise the window in front of the owner's. Default false."},
			"caption":{"type":"string","description":"With screenshot: what it shows."},
			"full":{"type":"boolean","description":"With screenshot: the whole page, not only what shows. Default false."}
		},"required":["action"],` + noExtra + `}`, readOnly: true},
	}
}

// Tools are the orchestrator tools a thread calls: the orchestration tools, which start, watch and stop other threads, and
// the agent tools, which note, start threads, show evidence, track a branch, refresh the queue and drive the browser.
func Tools() []Tool {
	all := specs()
	out := make([]Tool, len(all))
	for i, t := range all {
		out[i] = Tool{Name: t.name, Description: prompts.Tool(t.name), Schema: json.RawMessage(t.schema), ReadOnly: t.readOnly}
	}
	return out
}

// ClaudeNames are the orchestrator tools as Claude names them: ToolPrefix, then the tool's name.
func ClaudeNames() []string { return names(false) }

// ReadNames are the orchestrator tools that a Read thread runs without asking, as Claude names them: every one but those
// that file evidence of work or claim a branch.
func ReadNames() []string { return names(true) }

func names(read bool) []string {
	var out []string
	for _, t := range specs() {
		if !read || !t.writes {
			out = append(out, ToolPrefix+t.name)
		}
	}
	return out
}
