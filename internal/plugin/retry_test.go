package plugin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestExecuteWithRetrySanitizesOnlyAfterResourceMismatch(t *testing.T) {
	request := modelRequest{HostModelExecutionRequest: pluginapi.HostModelExecutionRequest{
		EntryProtocol: "openai-response",
		ExitProtocol:  "openai-response",
		Model:         "gpt-5.5",
		Body:          []byte(`{"previous_response_id":"resp_old","input":[{"type":"reasoning","id":"rs_old"},{"type":"message","content":"hello"}]}`),
	}}
	var calls [][]byte
	p := New(func(method string, raw []byte) ([]byte, error) {
		if method != pluginabi.MethodHostModelExecute {
			t.Fatalf("method = %q", method)
		}
		calls = append(calls, append([]byte(nil), raw...))
		if len(calls) == 1 {
			return pluginabi.NewErrorEnvelope("host_call_failed", "OpenAI Responses bad request: The requested item was created under a different OpenAI resource.", http.StatusBadRequest)
		}
		return hostSuccessEnvelope(pluginapi.HostModelExecutionResponse{StatusCode: http.StatusOK, Body: []byte(`{"id":"resp_new"}`)})
	})
	response, err := executeWithRetry[pluginapi.HostModelExecutionResponse](p, pluginabi.MethodHostModelExecute, request)
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Body) != `{"id":"resp_new"}` {
		t.Fatalf("response body = %s", response.Body)
	}
	if len(calls) != 2 {
		t.Fatalf("host calls = %d, want original call plus one retry", len(calls))
	}
	var first, second modelRequest
	if err := json.Unmarshal(calls[0], &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(calls[1], &second); err != nil {
		t.Fatal(err)
	}
	if string(first.Body) == string(second.Body) {
		t.Fatal("retry body was not sanitized")
	}
	var cleaned map[string]any
	if err := json.Unmarshal(second.Body, &cleaned); err != nil {
		t.Fatal(err)
	}
	if _, ok := cleaned["previous_response_id"]; ok {
		t.Fatal("retry kept previous_response_id")
	}
}

func TestExecuteWithRetryDoesNotRetryOtherErrors(t *testing.T) {
	request := modelRequest{HostModelExecutionRequest: pluginapi.HostModelExecutionRequest{Body: []byte(`{"input":"hello"}`)}}
	calls := 0
	p := New(func(string, []byte) ([]byte, error) {
		calls++
		return pluginabi.NewErrorEnvelope("host_call_failed", "invalid parameter", http.StatusBadRequest)
	})
	_, err := executeWithRetry[pluginapi.HostModelExecutionResponse](p, pluginabi.MethodHostModelExecute, request)
	if err == nil {
		t.Fatal("request unexpectedly succeeded")
	}
	if calls != 1 {
		t.Fatalf("host calls = %d, want 1", calls)
	}
}

func hostSuccessEnvelope(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(pluginabi.Envelope{OK: true, Result: raw})
}
