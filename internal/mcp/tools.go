package mcp

import (
	"encoding/json"

	"github.com/nednella/agentos/internal/prompts"
)

const (
	threadField = `{"type":"string","description":"The thread's id."}`
	noExtra     = `"additionalProperties":false`
)

// Tools are the orchestration tools: what a thread uses to start, watch and stop other threads.
func Tools() []Tool {
	tools := []struct {
		name     string
		schema   string
		readOnly bool
	}{
		{"delegate", `{"type":"object","properties":{
			"task":{"type":"string","description":"What the child must do."},
			"title":{"type":"string","description":"The child's title."},
			"access":{"type":"string","enum":["read","supervised","full"],"description":"The child's access; never more than yours. Default: yours."},
			"model":{"type":"string","description":"The child's model."},
			"effort":{"type":"string","description":"The child's effort."},
			"project":{"type":"string","description":"A project's name or key. Default: yours."},
			"where":{"type":"string","enum":["folder","worktree","issue"],"description":"Where the child works. Default: folder for a read child, worktree for the others."},
			"issue":{"type":"integer","description":"The issue number; needed with where issue."}
		},"required":["task"],` + noExtra + `}`, false},
		{"threads", `{"type":"object","properties":{
			"project":{"type":"string","description":"A project's name, a key, or all. Default: yours."}
		},` + noExtra + `}`, true},
		{"read", `{"type":"object","properties":{
			"thread":` + threadField + `,
			"view":{"type":"string","enum":["messages","activity"],"description":"What to read. Default: messages."},
			"before":{"type":"integer","description":"A sequence number: read the entries before it. Default: the newest."},
			"limit":{"type":"integer","minimum":1,"maximum":100,"description":"How many entries. Default: 20."}
		},"required":["thread"],` + noExtra + `}`, true},
		{"send", `{"type":"object","properties":{
			"thread":` + threadField + `,
			"text":{"type":"string","description":"The message."},
			"steer":{"type":"boolean","description":"True: stop the running turn and send now. Default false: the thread takes it in at its next step."}
		},"required":["thread","text"],` + noExtra + `}`, false},
		{"wait", `{"type":"object","properties":{
			"thread":` + threadField + `,
			"timeout_seconds":{"type":"integer","minimum":1,"maximum":3600,"description":"How long to wait. Default: 300."}
		},"required":["thread"],` + noExtra + `}`, true},
		{"stop", `{"type":"object","properties":{
			"thread":` + threadField + `
		},"required":["thread"],` + noExtra + `}`, false},
	}
	out := make([]Tool, len(tools))
	for i, t := range tools {
		out[i] = Tool{Name: t.name, Description: prompts.Tool(t.name), Schema: json.RawMessage(t.schema), ReadOnly: t.readOnly}
	}
	return out
}
