package chat

import (
	"slices"
	"testing"
)

func TestPromptsAreTheMessagesTheConversationHolds(t *testing.T) {
	var ids []string
	for _, e := range []Event{
		{Kind: KindUser, ID: "u1"},
		{Kind: KindMessage, ID: "msg_1", UUID: "r1"},
		{Kind: KindQueued, ID: "u2"},
		{Kind: KindUser, ID: "u2"},
		{Kind: KindUser, ID: "u3", Parent: "ask-1"}, // it only denied a tool
		{Kind: KindUser, ID: "u4"},
		{Kind: KindUser, ID: "u5"},
		{Kind: KindUnsent, ID: "u5", Error: true}, // Claude failed before it read it
		{Kind: KindRewind, ID: "u2"},
		{Kind: KindUser, ID: "u6"},
		{Kind: KindRewind, ID: "gone"},
	} {
		ids = Prompts(ids, e)
	}
	if want := []string{"u1", "u6"}; !slices.Equal(ids, want) {
		t.Errorf("prompts = %q, want %q", ids, want)
	}
}

func TestPromptsChangeOnlyForPromptKinds(t *testing.T) {
	kinds := []Kind{KindStart, KindInit, KindUser, KindDelta, KindMessage, KindToolUse, KindToolResult, KindApproval,
		KindAnswer, KindTurnEnd, KindDraft, KindQueued, KindInterrupt, KindCommands, KindCompact, KindThinking,
		KindThought, KindEcho, KindStillQueued, KindUnsent, KindMode, KindReply, KindTaskStart, KindTaskEnd, KindRewind,
		KindExit, KindTasks, KindCancel, KindRateLimit}
	for _, k := range kinds {
		if slices.Contains(PromptKinds, k) {
			continue
		}
		ids := []string{"a", "b"}
		if got := Prompts(ids, Event{Kind: k, ID: "b", Error: true}); !slices.Equal(got, ids) {
			t.Errorf("an event of kind %s changed the prompts to %v; add it to PromptKinds", k, got)
		}
	}
}
