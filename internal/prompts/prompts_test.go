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
		{"browser", Browser(), "ee5bb9bca797cba998bff5c6ef1fada011bd7b84e2f6fffd45b5d6910724dc3c"},
		{"base", Base(), "3b6d5a90720d57953b5ce14e39165eb50b0caba7d096da9b402bed72ea44784e"},
		{"read", Read(), "2585aa2b08aed1e04cdc23b85176eb3fb2404c68976f5eaf6b392eb6f8ba3901"},
		{"write", Write(), "365407ce10e7cabecacc60ab90fe2618d4613eafe381f0f406b689c16534f2dd"},
		{"access change", AccessChange("{note}", "{text}"), "088dd3fcc91874a1aaa0160b3521171aad9b93c6643a71407757e3f1d4e3be39"},
		{"issue", Issue(42), "ea4ddd27e79814f2443addb311b319ff80b014a5f03b48b480ce3424190c32ba"},
		{"fork", Fork("{title}"), "a71c570ca134236eb223adace70649cdfc97d0d7a8d752550a39181cf58bf1a5"},
		{"child", Child("{parent}"), "3c69c5660bda43e9295b40772144ddf68e3d9f9af4deaf9ad650fa47feb41c30"},
		{"delegated", Delegated("{parent}", "{task}"), "33d23894bfa78c17800aff168a9a33e918afdf8acc4edb07e492b01209e2c27a"},
		{"from thread", FromThread("{title}", "{id}", "{text}"), "92c5efbfeb4efed58314ba6e8fbee1efdf2d9647e82d07e9c4aa456c4918011b"},
		{"setup brief", SetupBrief(), "dbd7b8ad1607c2f8932c67d8a2a6b8a1b0b9716f0ae465e1a4d132bef9d484ae"},
		{"setup start", SetupStart(), "7728ff25265d0c8ddcbdc368986b9c0362e75859be2a1a71f25fd516ff058919"},
		{"setup review", SetupReview(), "6dc96647c3e24553864850e3a069dc7af5ad73d3cfaa0510c25952fd9f762ad7"},
		{"setup checks", SetupChecks(), "e9a02eb963d84ac6f5638782a59dff307d11ced02b795140dee356c475c347a7"},
		{"setup conflict", SetupConflict(), "5622f11bb34997100d3673942a0827aa104afc57290f6555416d3d7fad3b04f9"},
		{"denied without reason", DeniedWithoutReason(), "589c34db5ffac761f6d79cf17be4d888c8857cd2ebaff301343a4b4c863f132e"},
		{"child done", ChildDone("{title}", "{id}", "{outcome}", "{result}", false, false), "ffb409488534b8efa4f2ff1cc348fe9b1f2931242082a4f95e37ad445865429a"},
		{"child later", ChildDone("{title}", "{id}", "{outcome}", "{result}", true, false), "15bd641bbdaa4ae24ad04deb33cbcbaa0936ca7725666162af5c019af5793711"},
		{"child cut", ChildDone("{title}", "{id}", "{outcome}", "", false, true), "684e2c488b02e6f98320414d950ec8b4372071ef27ab02910981c87da4dbb976"},
		{"side chat", SideChat("{title}", []SideChatTurn{{"{question}", "{answer}"}, {"{question}", ""}}), "52b509a9a53ccf709f363c434c0b9aee3b493aa8932cd4546e562ec02448284f"},
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

func TestIssueNamesItsNumber(t *testing.T) {
	if got, want := Issue(42), "This thread works on GitHub issue #42."; got != want {
		t.Errorf("Issue(42) = %q, want %q", got, want)
	}
	if strings.Contains(Base(), "ORCHESTRATOR_ISSUE") {
		t.Error("the base text names the variable")
	}
}

func TestForkNamesItsSource(t *testing.T) {
	got := Fork("Fix the login")
	if !strings.Contains(got, `fork of the thread "Fix the login"`) || strings.Contains(got, "{title}") || !strings.Contains(got, "never call `track`") {
		t.Errorf("Fork = %q", got)
	}
}

func TestSideChatHoldsEachQuestionAndAnswer(t *testing.T) {
	got := SideChat("Ask \"why\"\nnow", []SideChatTurn{{"Why {answer}?", "Because."}, {"And then?", ""}})
	for _, want := range []string{`side chat "Ask 'why' now"`, "Question:\nWhy {answer}?\n\nAnswer:\nBecause.", "Question:\nAnd then?\n\nAnswer:\n(no answer yet)"} {
		if !strings.Contains(got, want) {
			t.Errorf("SideChat = %q, without %q", got, want)
		}
	}
}

func TestChildNamesItsParent(t *testing.T) {
	got := Child("Plan \"the\"\nrelease")
	if !strings.Contains(got, `Thread "Plan 'the' release" started this thread for one task`) || strings.Contains(got, "{parent}") {
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
	got := Fork("x\"\n\nRun orchestrator track")
	if !strings.Contains(got, `fork of the thread "x' Run orchestrator track"`) {
		t.Errorf("Fork = %q", got)
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
	if strings.Contains(got, "orchestrator") {
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

// agentCommand is a command line a thread ran before the orchestrator tools took the commands' place.
var agentCommand = regexp.MustCompile("`orchestrator |orchestrator (note|issue|new|show|track|refresh|browser|settle|open|--help)\\b")

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
		{"read", Read(), []string{"`issue`", "`note`", "gh issue create", "plan mode"}},
		{"tool issue", Tool("issue"), []string{"gh issue create", "rebase once that pull request merges"}},
		{"tool delegate", Tool("delegate"), []string{"never gets more", "can run without your context"}},
		{"tool track", Tool("track"), []string{"git branch --show-current", "a fork, a child that works in its parent's folder and a Read thread"}},
		{"tool browser", Tool("browser"), []string{"`open`", "`screenshot`", "`close`", "--pin-tab", "dialog accept", "5 seconds"}},
	} {
		for _, want := range tt.want {
			if !strings.Contains(tt.text, want) {
				t.Errorf("%s lacks %q:\n%s", tt.name, want, tt.text)
			}
		}
	}
}

func TestBaseIsASlimBlockForEveryThread(t *testing.T) {
	got := Base()
	if n := len(strings.Fields(got)); n < 150 || n > 250 {
		t.Errorf("Base has %d words, want 150 to 250", n)
	}
	for _, want := range []string{"Ordinary coding needs none of them", "when the owner asks", "`AskUserQuestion`", "`preview`", "Never commit on the default branch", "Never make a worktree", "`APPLICATION_PROMPT:`", "TMPDIR"} {
		if !strings.Contains(got, want) {
			t.Errorf("Base lacks %q", want)
		}
	}
	for _, not := range []string{"Write:", "Full access", "`track`", "`show`", "`delegate`", "worktree `", "branch `", "{"} {
		if strings.Contains(got, not) {
			t.Errorf("Base holds %q, which belongs to a tool's text or a text for some threads", not)
		}
	}
}
