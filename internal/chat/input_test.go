package chat

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

// assertLine checks that got is one JSON line equal to want, key order aside.
func assertLine(t *testing.T, got []byte, want string) {
	t.Helper()
	if n := bytes.Count(got, []byte("\n")); n != 1 || got[len(got)-1] != '\n' {
		t.Fatalf("want one line ending in a newline, got %q", got)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("bad test fixture: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestUserMessageShape(t *testing.T) {
	assertLine(t, UserMessage("hi"),
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"hi"}]},"parent_tool_use_id":null,"session_id":"default"}`)
}

func TestUserMessageEscapesText(t *testing.T) {
	text := "first line\nsecond \"quoted\" line 🎉"
	got := UserMessage(text)
	assertLine(t, got, `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"first line\nsecond \"quoted\" line 🎉"}]},"parent_tool_use_id":null,"session_id":"default"}`)

	var sent struct {
		Message struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(got, &sent); err != nil {
		t.Fatal(err)
	}
	if gotText := sent.Message.Content[0].Text; gotText != text {
		t.Fatalf("text did not round-trip: got %q, want %q", gotText, text)
	}
}

func TestControlRequestShapes(t *testing.T) {
	assertLine(t, Initialize("req_1"),
		`{"type":"control_request","request_id":"req_1","request":{"subtype":"initialize"}}`)
	assertLine(t, Interrupt("req_2"),
		`{"type":"control_request","request_id":"req_2","request":{"subtype":"interrupt","spare_background_tasks":true}}`)
}

func TestSetPermissionModeShape(t *testing.T) {
	assertLine(t, SetPermissionMode("aos-3", "plan"),
		`{"type":"control_request","request_id":"aos-3","request":{"subtype":"set_permission_mode","mode":"plan"}}`)
}

func TestPermissionAnswerShapes(t *testing.T) {
	assertLine(t, Allow("ffd4", json.RawMessage(`{"command":"ls"}`)),
		`{"type":"control_response","response":{"subtype":"success","request_id":"ffd4","response":{"behavior":"allow","updatedInput":{"command":"ls"}}}}`)
	assertLine(t, Deny("ffd4", "not now"),
		`{"type":"control_response","response":{"subtype":"success","request_id":"ffd4","response":{"behavior":"deny","message":"not now"}}}`)
}

func TestDenyWithoutReasonSendsTheDefault(t *testing.T) {
	want := `{"type":"control_response","response":{"subtype":"success","request_id":"ffd4","response":{"behavior":"deny","message":"The user denied this tool call."}}}`
	assertLine(t, Deny("ffd4", ""), want)
	assertLine(t, Deny("ffd4", " \n\t "), want)
}

func TestAllowWithoutInputSendsEmptyObject(t *testing.T) {
	want := `{"type":"control_response","response":{"subtype":"success","request_id":"ffd4","response":{"behavior":"allow","updatedInput":{}}}}`
	assertLine(t, Allow("ffd4", nil), want)
	assertLine(t, Allow("ffd4", json.RawMessage{}), want)
}

func TestUserMessageCarriesTheID(t *testing.T) {
	assertLine(t, UserMessageWith("abc", "hi", nil),
		`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"hi"}]},"parent_tool_use_id":null,"session_id":"default","uuid":"abc"}`)
}

func TestUserMessageWithImages(t *testing.T) {
	got := UserMessageWith("abc", "look", []Image{{Mime: "image/png", Data: "QUJD"}})
	assertLine(t, got,
		`{"type":"user","message":{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"QUJD"}},{"type":"text","text":"look"}]},"parent_tool_use_id":null,"session_id":"default","uuid":"abc"}`)
	assertLine(t, UserMessageWith("abc", "", []Image{{Mime: "image/png", Data: "QUJD"}}),
		`{"type":"user","message":{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"QUJD"}}]},"parent_tool_use_id":null,"session_id":"default","uuid":"abc"}`)
}

func TestRewindRequestShapes(t *testing.T) {
	assertLine(t, RewindFiles("aos-4", "5d0150ad", true),
		`{"type":"control_request","request_id":"aos-4","request":{"subtype":"rewind_files","user_message_id":"5d0150ad","dry_run":true}}`)
	assertLine(t, RewindFiles("aos-5", "5d0150ad", false),
		`{"type":"control_request","request_id":"aos-5","request":{"subtype":"rewind_files","user_message_id":"5d0150ad","dry_run":false}}`)
	assertLine(t, RewindConversation("aos-6", "5d0150ad", "9d957459"),
		`{"type":"control_request","request_id":"aos-6","request":{"subtype":"rewind_conversation","target_message_uuid":"5d0150ad","last_seen_user_message_uuid":"9d957459"}}`)
}
