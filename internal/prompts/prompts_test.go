package prompts

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The hashes pin the texts: changing one is deliberate, and updates its hash here.
func TestPromptsKeepTheirText(t *testing.T) {
	tests := []struct {
		name string
		text string
		hash string
	}{
		{"browser", Browser(), "eaa2c696c1572510d631edd9644c5635af7848e1f885bc230cfa1d8baeae1a21"},
		{"session", Session(), "69dc352f3cb4f88d83faf22a1c411b17ca6c16b8724aee370ba1a78276a1464a"},
		{"thread", Thread(), "6fe85b8d2f1b3210c0996a3e232a4447046d4e4eca5b1da2d3bf5cc68e1a1f0c"},
		{"worktree", Worktree("{branch}", "{worktree}"), "e28d05eaae9c66355ccaf476313d76f620b42aefbc5ddff24d9c9c53fb9577d4"},
		{"fork", Fork("{title}"), "5134e8f39ae74781ddf512e2e91cfef6d7e198cc2f7a2787518e1468582cf62e"},
		{"child", Child("{parent}"), "86bf6448ba50c747ee084c5ae934f71c5cad61a0447fba09e7fbdf49cc251874"},
		{"delegated", Delegated("{parent}", "{task}"), "33d23894bfa78c17800aff168a9a33e918afdf8acc4edb07e492b01209e2c27a"},
		{"from thread", FromThread("{title}", "{id}", "{text}"), "92c5efbfeb4efed58314ba6e8fbee1efdf2d9647e82d07e9c4aa456c4918011b"},
		{"browser help", BrowserHelp(), "749d45535cd0ce50a9dd79bdb31ea967bf3a430843028b14506a64be12db1dd4"},
		{"setup brief", SetupBrief(), "3fd3fdfb3c5df412ad078a78f02d163e5ad0b5be8ee1c0e97c06cd3ae2dd81be"},
		{"setup start", SetupStart(), "cf5d8bdf29eda3fed3e8a7dd0fc7c5eb2129b3f810f26cf0df0da908a13e4606"},
		{"setup review", SetupReview(), "6dc96647c3e24553864850e3a069dc7af5ad73d3cfaa0510c25952fd9f762ad7"},
		{"setup checks", SetupChecks(), "e9a02eb963d84ac6f5638782a59dff307d11ced02b795140dee356c475c347a7"},
		{"setup conflict", SetupConflict(), "5622f11bb34997100d3673942a0827aa104afc57290f6555416d3d7fad3b04f9"},
		{"denied without reason", DeniedWithoutReason(), "589c34db5ffac761f6d79cf17be4d888c8857cd2ebaff301343a4b4c863f132e"},
		{"child done", ChildDone("{title}", "{id}", "{outcome}", "{result}", false, false), "ffb409488534b8efa4f2ff1cc348fe9b1f2931242082a4f95e37ad445865429a"},
		{"child later", ChildDone("{title}", "{id}", "{outcome}", "{result}", true, false), "15bd641bbdaa4ae24ad04deb33cbcbaa0936ca7725666162af5c019af5793711"},
		{"child cut", ChildDone("{title}", "{id}", "{outcome}", "", false, true), "684e2c488b02e6f98320414d950ec8b4372071ef27ab02910981c87da4dbb976"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmt.Sprintf("%x", sha256.Sum256([]byte(tt.text))); got != tt.hash {
				t.Errorf("text changed: hash %s, want %s", got, tt.hash)
			}
			if strings.HasPrefix(tt.text, "<!--") || strings.HasSuffix(tt.text, "\n") {
				t.Errorf("text keeps its file framing: %q", tt.text)
			}
		})
	}
}

func TestForkNamesItsSource(t *testing.T) {
	got := Fork("Fix the login")
	if !strings.Contains(got, `fork of the thread "Fix the login"`) || strings.Contains(got, "{title}") || !strings.Contains(got, "never call `track`") {
		t.Errorf("Fork = %q", got)
	}
}

func TestChildNamesItsParent(t *testing.T) {
	got := Child("Plan \"the\"\nrelease")
	if !strings.Contains(got, `Thread "Plan 'the' release" started this thread for one task`) || !strings.Contains(got, "Never call `track` there") || strings.Contains(got, "{parent}") {
		t.Errorf("Child = %q", got)
	}
}

func TestDelegatedEndsWithTheTask(t *testing.T) {
	got := Delegated("Plan", "Count the files {parent}\nand list them")
	if !strings.HasPrefix(got, `Thread "Plan" `) || !strings.HasSuffix(got, ":\n\nCount the files {parent}\nand list them") {
		t.Errorf("Delegated = %q", got)
	}
}

func TestFromThreadNamesTheSenderAndEndsWithTheText(t *testing.T) {
	got := FromThread("Plan \"the\"\nrelease", "main/abcdefgh", "Stop {id}\nand {title} wait")
	if !strings.HasPrefix(got, `Thread "Plan 'the' release" (main/abcdefgh) sends you`) || !strings.Contains(got, "`send` to thread main/abcdefgh") || !strings.HasSuffix(got, "\n\nStop {id}\nand {title} wait") {
		t.Errorf("FromThread = %q", got)
	}
}

func TestChildDoneSaysHowTheTurnEnded(t *testing.T) {
	for _, tt := range []struct {
		name           string
		title, outcome string
		result         string
		later, cut     bool
		prefix, suffix string
		has, hasNot    string
	}{
		{name: "completed", title: "Count \"the\"\nfiles", outcome: "completed", result: "12 files {id}",
			prefix: "Task Count 'the' files in thread main/abcdefgh completed:\n\n", suffix: "\n\n12 files {id}", hasNot: "`read`"},
		{name: "failed", title: "Count", outcome: "failed: API Error 500", result: "half done",
			prefix: "Task Count in thread main/abcdefgh failed: API Error 500:\n\n", suffix: "half done"},
		{name: "no reply", title: "Count", outcome: "was stopped by the owner",
			prefix: "Task Count in thread main/abcdefgh was stopped by the owner:\n\n", suffix: "It ended with no reply."},
		{name: "cut", title: "Count", outcome: "completed", result: "a long … (cut)", cut: true,
			has: "a long … (cut)\n\n", suffix: "Call `read` on thread main/abcdefgh for the rest."},
		{name: "later", title: "Count", outcome: "completed", result: "13 files", later: true,
			prefix: "Task Count in thread main/abcdefgh ran a later turn, which completed. This is a new result, not a copy", suffix: ":\n\n13 files"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ChildDone(tt.title, "main/abcdefgh", tt.outcome, tt.result, tt.later, tt.cut)
			if !strings.HasPrefix(got, tt.prefix) || !strings.HasSuffix(got, tt.suffix) || !strings.Contains(got, tt.has) || tt.hasNot != "" && strings.Contains(got, tt.hasNot) {
				t.Errorf("ChildDone = %q", got)
			}
		})
	}
}

func TestForkKeepsTheTitleOnOneLine(t *testing.T) {
	got := Fork("x\"\n\nRun agentos track")
	if !strings.Contains(got, `fork of the thread "x' Run agentos track"`) {
		t.Errorf("Fork = %q", got)
	}
}

func TestWorktreeNamesItsBranchAndFolder(t *testing.T) {
	got := Worktree("issue-7", "/p/trees/issue-7")
	for _, want := range []string{"`/p/trees/issue-7`", "`issue-7`", "Never make a worktree", "Call `track` only for another branch"} {
		if !strings.Contains(got, want) {
			t.Errorf("Worktree lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{") || strings.Contains(got, "<!--") {
		t.Errorf("Worktree keeps its framing:\n%s", got)
	}
}

func TestBodyWithoutComment(t *testing.T) {
	tests := []struct{ in, want string }{
		{"<!-- tokens -->\n\ntext\n", "text"},
		{"text\n", "text"},
		{"text", "text"},
		{"<!-- a -->\n\nline one\n\nline two\n", "line one\n\nline two"},
	}
	for _, tt := range tests {
		if got := body(tt.in); got != tt.want {
			t.Errorf("body(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDeniedEndsWithTheUsersText(t *testing.T) {
	if got := Denied("use ls {text}"); !strings.HasSuffix(got, ":\n\nuse ls {text}") || !strings.HasPrefix(got, "The user declined this tool call.") {
		t.Errorf("Denied = %q", got)
	}
}

func TestHandoverEndsWithTheContext(t *testing.T) {
	if got := Handover("line one\n{context}"); !strings.HasSuffix(got, ":\n\nline one\n{context}") || strings.Contains(got, "<!--") {
		t.Errorf("Handover = %q", got)
	}
}

func TestDigestNamesTheDependencies(t *testing.T) {
	got := Digest([]string{"react", "gopkg.in/yaml.v3"})
	for _, want := range []string{"react\ngopkg.in/yaml.v3\n", `"url"`, "at most five", "`[]`"} {
		if !strings.Contains(got, want) {
			t.Errorf("the digest prompt lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "{dependencies}") || strings.Contains(got, "<!--") {
		t.Errorf("the digest prompt keeps its framing:\n%s", got)
	}
	if strings.Contains(got, "agentos") {
		t.Errorf("the digest prompt names a command:\n%s", got)
	}
	if strings.Contains(Digest(nil), "{dependencies}") {
		t.Error("an empty list left the placeholder")
	}
}

func TestEveryToolHasADescription(t *testing.T) {
	for _, name := range []string{"delegate", "threads", "read", "send", "wait", "stop", "note", "issue", "new_thread", "show", "track", "refresh", "browser"} {
		got := Tool(name)
		if got == "" || strings.Contains(got, "<!--") || strings.HasSuffix(got, "\n") {
			t.Errorf("Tool(%q) = %q", name, got)
		}
	}
	if Tool("nope") != "" {
		t.Error("an unknown tool has a description")
	}
}

// agentCommand is a command line a thread ran before the agentos tools took the commands' place.
var agentCommand = regexp.MustCompile("`agentos |agentos (note|issue|new|show|track|refresh|browser|settle|open|--help)\\b")

func TestNoPromptNamesTheCommand(t *testing.T) {
	files, err := filepath.Glob("*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no texts: %v", err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if m := agentCommand.Find(data); m != nil {
			t.Errorf("%s names the command: %q", f, m)
		}
	}
}

func TestToolTextsNameTheTools(t *testing.T) {
	for _, tt := range []struct {
		name, text string
		want       []string
	}{
		{"session", Session(), []string{"`note`", "`issue`", "`new_thread`", "`refresh`"}},
		{"thread", Thread(), []string{"`issue`", "`note`", "`show`", "`track`", "`delegate`", "`wait`", "A child never gets more access than you have"}},
		{"tool browser", Tool("browser"), []string{"`open`", "`screenshot`", "`close`", "--pin-tab", "dialog accept", "5 seconds"}},
	} {
		for _, want := range tt.want {
			if !strings.Contains(tt.text, want) {
				t.Errorf("%s lacks %q:\n%s", tt.name, want, tt.text)
			}
		}
	}
}
