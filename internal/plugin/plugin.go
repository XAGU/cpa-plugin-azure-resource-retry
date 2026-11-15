package plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

const Name = "azure-resource-retry"

var Version = "0.1.1"

type HostCall func(string, []byte) ([]byte, error)

type Plugin struct {
	host HostCall
}

func New(host HostCall) *Plugin {
	return &Plugin{host: host}
}

type executorRequest struct {
	pluginapi.ExecutorRequest
	StreamID       string `json:"stream_id,omitempty"`
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type modelRequest struct {
	pluginapi.HostModelExecutionRequest
	HostCallbackID string `json:"host_callback_id,omitempty"`
}

type registration struct {
	SchemaVersion uint32             `json:"schema_version"`
	Metadata      pluginapi.Metadata `json:"metadata"`
	Capabilities  capabilities       `json:"capabilities"`
}

type capabilities struct {
	ModelRouter           bool     `json:"model_router"`
	Executor              bool     `json:"executor"`
	ExecutorModelScope    string   `json:"executor_model_scope"`
	ExecutorInputFormats  []string `json:"executor_input_formats"`
	ExecutorOutputFormats []string `json:"executor_output_formats"`
}

func (p *Plugin) Handle(method string, raw []byte) ([]byte, error) {
	var result any
	var err error
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		result = registration{
			SchemaVersion: pluginabi.SchemaVersion,
			Metadata: pluginapi.Metadata{
				Name:             Name,
				Version:          Version,
				Author:           "XAGU",
				GitHubRepository: "https://github.com/XAGU/cpa-plugin-azure-resource-retry",
				ConfigFields:     []pluginapi.ConfigField{},
			},
			Capabilities: capabilities{
				ModelRouter:           true,
				Executor:              true,
				ExecutorModelScope:    "static",
				ExecutorInputFormats:  []string{"openai-response", "codex"},
				ExecutorOutputFormats: []string{"openai-response", "codex"},
			},
		}
	case pluginabi.MethodPluginQuiesce, pluginabi.MethodPluginShutdown:
		result = struct{}{}
	case pluginabi.MethodModelRoute:
		var req pluginapi.ModelRouteRequest
		if err = json.Unmarshal(raw, &req); err == nil {
			result = route(req)
		}
	case pluginabi.MethodExecutorIdentifier:
		result = map[string]string{"identifier": Name}
	case pluginabi.MethodExecutorExecute, pluginabi.MethodExecutorExecuteStream:
		var req executorRequest
		if err = json.Unmarshal(raw, &req); err == nil {
			if req.HostCallbackID == "" {
				err = fmt.Errorf("缺少宿主请求上下文")
			} else if method == pluginabi.MethodExecutorExecuteStream {
				result, err = p.executeStream(req)
			} else {
				result, err = p.execute(req)
			}
		}
	case pluginabi.MethodExecutorCountTokens, pluginabi.MethodExecutorHTTPRequest:
		err = pluginabi.NewError(pluginabi.ErrorCodeNotImplemented, "此插件只处理 Responses 生成请求", http.StatusNotImplemented)
	default:
		err = pluginabi.NewError("unknown_method", "不支持的插件方法", http.StatusNotImplemented)
	}
	if err != nil {
		return json.Marshal(pluginabi.Envelope{OK: false, Error: errorDetail(err)})
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(pluginabi.Envelope{OK: true, Result: encoded})
}

func route(req pluginapi.ModelRouteRequest) pluginapi.ModelRouteResponse {
	format := strings.ToLower(strings.TrimSpace(req.SourceFormat))
	if format != "openai-response" && format != "codex" {
		return pluginapi.ModelRouteResponse{}
	}
	path, _ := req.Metadata["request_path"].(string)
	if path != "" && !strings.HasSuffix(strings.TrimSuffix(path, "/"), "/responses") {
		return pluginapi.ModelRouteResponse{}
	}
	return pluginapi.ModelRouteResponse{Handled: true, TargetKind: pluginapi.ModelRouteTargetSelf}
}

func executionRequest(req executorRequest, stream bool) modelRequest {
	path, _ := req.Metadata["request_path"].(string)
	callerScope, _ := req.Metadata[pluginapi.CallerScopeMetadataKey].(string)
	// 委托请求使用原始正文，工具 schema 和 payload 配置由目标执行器处理。
	return modelRequest{
		HostModelExecutionRequest: pluginapi.HostModelExecutionRequest{
			EntryProtocol: req.SourceFormat,
			ExitProtocol:  req.Format,
			Model:         req.Model,
			Stream:        stream,
			Body:          req.OriginalRequest,
			Headers:       req.Headers,
			Query:         req.Query,
			Alt:           req.Alt,
			Path:          path,
			CallerScope:   callerScope,
		},
		HostCallbackID: req.HostCallbackID,
	}
}

func (p *Plugin) execute(req executorRequest) (pluginapi.ExecutorResponse, error) {
	response, err := executeWithRetry[pluginapi.HostModelExecutionResponse](p, pluginabi.MethodHostModelExecute, executionRequest(req, false))
	if err != nil {
		return pluginapi.ExecutorResponse{}, err
	}
	return pluginapi.ExecutorResponse{Payload: response.Body, Headers: response.Headers}, nil
}

func executeWithRetry[T any](p *Plugin, method string, req modelRequest) (T, error) {
	var response T
	err := p.call(method, req, &response)
	if !isResourceMismatch(err) {
		return response, err
	}

	// 只有跨资源 item 的 400 错误允许清理历史，并且本次请求最多重试一次。
	cleaned, errClean := sanitizeHistory(req.Body)
	if errClean != nil {
		return response, err
	}
	req.Body = cleaned
	var retried T
	err = p.call(method, req, &retried)
	return retried, err
}

func (p *Plugin) call(method string, request any, result any) error {
	raw, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("编码宿主请求: %w", err)
	}
	response, err := p.host(method, raw)
	if err != nil {
		return err
	}
	var envelope pluginabi.Envelope
	if err := json.Unmarshal(response, &envelope); err != nil {
		return fmt.Errorf("解码宿主响应: %w", err)
	}
	if !envelope.OK {
		if envelope.Error != nil {
			return envelope.Error
		}
		return fmt.Errorf("宿主调用失败，未返回错误详情")
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("解码宿主结果: %w", err)
	}
	return nil
}
