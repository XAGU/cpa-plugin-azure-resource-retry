package plugin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestExecutorPreservesOriginalToolSchemasOnRetry(t *testing.T) {
	for _, format := range []string{"openai-response", "codex"} {
		for _, stream := range []bool{false, true} {
			name := format + "/非流式"
			method := pluginabi.MethodExecutorExecute
			if stream {
				name = format + "/流式"
				method = pluginabi.MethodExecutorExecuteStream
			}
			t.Run(name, func(t *testing.T) {
				original := []byte(`{"model":"azure-test","previous_response_id":"resp_old","tools":[{"type":"namespace","name":"collaboration","tools":[{"type":"function","name":"wait_agent","parameters":{"type":"object","properties":{"timeout_ms":{"type":"number"}}}}]}],"input":[{"type":"function_call","id":"fc_old","call_id":"call_1","name":"wait_agent","arguments":"{\"timeout_ms\":1000}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
				req := executorRequest{
					ExecutorRequest: pluginapi.ExecutorRequest{
						Model:           "azure-test",
						SourceFormat:    format,
						Format:          format,
						Stream:          stream,
						OriginalRequest: original,
						Payload:         bytes.ReplaceAll(original, []byte(`"number"`), []byte(`"integer"`)),
					},
					HostCallbackID: "callback-test",
					StreamID:       "downstream-test",
				}
				var calls []modelRequest
				closed := make(chan struct{})
				p := New(func(method string, raw []byte) ([]byte, error) {
					switch method {
					case pluginabi.MethodHostModelExecute, pluginabi.MethodHostModelExecuteStream:
						var request modelRequest
						if err := json.Unmarshal(raw, &request); err != nil {
							return nil, err
						}
						calls = append(calls, request)
						if len(calls) == 1 {
							return pluginabi.NewErrorEnvelope("host_call_failed", "The requested item was created under a different Azure OpenAI resource.", http.StatusBadRequest)
						}
						if stream {
							return hostSuccessEnvelope(pluginapi.HostModelStreamResponse{StreamID: "upstream-test"})
						}
						return hostSuccessEnvelope(pluginapi.HostModelExecutionResponse{StatusCode: http.StatusOK, Body: []byte(`{"id":"resp_new"}`)})
					case pluginabi.MethodHostModelStreamRead:
						return hostSuccessEnvelope(pluginapi.HostModelStreamReadResponse{Done: true})
					case pluginabi.MethodHostStreamClose:
						close(closed)
					}
					return hostSuccessEnvelope(struct{}{})
				})
				raw, err := json.Marshal(req)
				if err != nil {
					t.Fatal(err)
				}
				result, err := p.Handle(method, raw)
				if err != nil {
					t.Fatal(err)
				}
				var envelope pluginabi.Envelope
				if err := json.Unmarshal(result, &envelope); err != nil || !envelope.OK {
					t.Fatalf("执行失败: %s, %v", result, err)
				}
				if stream {
					<-closed
				}
				if len(calls) != 2 {
					t.Fatalf("宿主请求次数 = %d，预期 2", len(calls))
				}
				if !bytes.Equal(calls[0].Body, original) {
					t.Fatalf("首次请求未使用原始正文: %s", calls[0].Body)
				}
				var first, second map[string]any
				if err := json.Unmarshal(calls[0].Body, &first); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(calls[1].Body, &second); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(first["tools"], second["tools"]) {
					t.Fatalf("重试改写了工具 schema: %s", calls[1].Body)
				}
				if _, exists := second["previous_response_id"]; exists || bytes.Contains(calls[1].Body, []byte(`"fc_old"`)) {
					t.Fatalf("重试未清理历史标识: %s", calls[1].Body)
				}
				for _, call := range calls {
					if call.Stream != stream || call.HostCallbackID != req.HostCallbackID {
						t.Fatalf("转发的流标识或宿主上下文不正确: %+v", call)
					}
				}
			})
		}
	}
}
