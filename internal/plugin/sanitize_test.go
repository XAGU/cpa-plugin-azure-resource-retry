package plugin

import (
	"encoding/json"
	"reflect"
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

func TestSanitizeHistoryPreservesNestedContentAndToolSchemas(t *testing.T) {
	input := []byte(`{"previous_response_id":"resp_old","tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{"id":{"type":"string"},"timeout_ms":{"type":"number"}}}}],"input":[{"type":"function_call","id":"fc_old","call_id":"call_1","arguments":"{\"id\":\"business-1\"}"},{"type":"function_call_output","id":"out_old","call_id":"call_1","encrypted_content":"opaque","encrypted_metadata":"opaque","output":[{"type":"input_text","text":"result","id":"business-2","encrypted_content":"business-content","metadata":{"id":"business-3"}}]},{"type":"additional_tools","tools":[{"type":"function","name":"lookup","parameters":{"properties":{"id":{"type":"string"},"timeout_ms":{"type":"number"}}}}]}]}`)
	cleaned, err := sanitizeHistory(input)
	if err != nil {
		t.Fatal(err)
	}
	var before, after map[string]any
	if err := json.Unmarshal(input, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cleaned, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before["tools"], after["tools"]) {
		t.Fatalf("顶层工具 schema 发生变化: %s", cleaned)
	}
	itemsBefore, itemsAfter := before["input"].([]any), after["input"].([]any)
	for i, item := range itemsAfter {
		object := item.(map[string]any)
		for _, key := range []string{"id", "encrypted_content", "encrypted_metadata"} {
			if _, exists := object[key]; exists {
				t.Fatalf("第 %d 个历史 item 仍包含 %s", i, key)
			}
		}
		original := itemsBefore[i].(map[string]any)
		for _, key := range []string{"call_id", "arguments", "output", "tools"} {
			if !reflect.DeepEqual(object[key], original[key]) {
				t.Fatalf("第 %d 个 item 的 %s 发生变化: %s", i, key, cleaned)
			}
		}
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
