package plugin

import (
	"encoding/json"
	"testing"
)

func TestSanitizeHistoryKeepsToolCallLinkage(t *testing.T) {
	input := []byte(`{"model":"gpt-5.5","previous_response_id":"resp_old","include":["reasoning.encrypted_content","message.output_text"],"input":[{"type":"reasoning","id":"rs_old","encrypted_content":"opaque"},{"type":"function_call","id":"fc_old","call_id":"call_123","arguments":"{}"},{"type":"item_reference","id":"msg_old"},{"type":"function_call_output","call_id":"call_123","output":"ok"}]}`)
	cleaned, err := sanitizeHistory(input)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(cleaned, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["previous_response_id"]; ok {
		t.Fatal("previous_response_id remains")
	}
	if got := body["include"].([]any); len(got) != 1 || got[0] != "message.output_text" {
		t.Fatalf("include = %#v", got)
	}
	items := body["input"].([]any)
	if len(items) != 3 {
		t.Fatalf("input length = %d", len(items))
	}
	first := items[0].(map[string]any)
	if _, ok := first["id"]; ok {
		t.Fatal("reasoning id remains")
	}
	if _, ok := first["encrypted_content"]; ok {
		t.Fatal("reasoning encrypted_content remains")
	}
	call := items[1].(map[string]any)
	if call["call_id"] != "call_123" {
		t.Fatalf("call_id = %#v", call["call_id"])
	}
	output := items[2].(map[string]any)
	if output["call_id"] != "call_123" {
		t.Fatalf("output call_id = %#v", output["call_id"])
	}
}

func TestSanitizeHistoryLeavesUnchangedPayloadUntouched(t *testing.T) {
	input := []byte(`{"model":"gpt-5.5","input":"hello"}`)
	cleaned, err := sanitizeHistory(input)
	if err != nil {
		t.Fatal(err)
	}
	if string(cleaned) != string(input) {
		t.Fatalf("payload changed from %s to %s", input, cleaned)
	}
}
