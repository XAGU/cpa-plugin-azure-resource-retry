package plugin

import (
	"fmt"
	"net/http"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

type streamResponse struct {
	Headers http.Header `json:"headers"`
}

type streamEmit struct {
	StreamID string `json:"stream_id"`
	Payload  []byte `json:"payload"`
}

type streamClose struct {
	StreamID    string           `json:"stream_id"`
	ErrorDetail *pluginabi.Error `json:"error_detail,omitempty"`
}

func (p *Plugin) executeStream(req executorRequest) (streamResponse, error) {
	if req.StreamID == "" {
		return streamResponse{}, fmt.Errorf("缺少下游流标识")
	}
	upstream, err := executeWithRetry[pluginapi.HostModelStreamResponse](p, pluginabi.MethodHostModelExecuteStream, executionRequest(req, true))
	if err != nil {
		return streamResponse{}, err
	}
	if upstream.StreamID == "" {
		return streamResponse{}, fmt.Errorf("宿主未返回上游流标识")
	}
	go p.forwardStream(req.StreamID, upstream.StreamID)
	return streamResponse{Headers: upstream.Headers}, nil
}

func (p *Plugin) forwardStream(downstreamID, upstreamID string) {
	var terminal *pluginabi.Error
	defer func() {
		if recover() != nil {
			terminal = pluginabi.NewError("plugin_stream_error", "转发上游流时发生内部错误", http.StatusInternalServerError)
		}
		_ = p.call(pluginabi.MethodHostModelStreamClose, pluginapi.HostModelStreamCloseRequest{StreamID: upstreamID}, nil)
		_ = p.call(pluginabi.MethodHostStreamClose, streamClose{StreamID: downstreamID, ErrorDetail: terminal}, nil)
	}()

	// 流打开后只转发；已开始输出的请求不能重放，以免重复执行工具调用。
	for {
		var chunk pluginapi.HostModelStreamReadResponse
		if err := p.call(pluginabi.MethodHostModelStreamRead, pluginapi.HostModelStreamReadRequest{StreamID: upstreamID}, &chunk); err != nil {
			terminal = errorDetail(err)
			return
		}
		if len(chunk.Payload) > 0 {
			if err := p.call(pluginabi.MethodHostStreamEmit, streamEmit{StreamID: downstreamID, Payload: chunk.Payload}, nil); err != nil {
				terminal = errorDetail(err)
				return
			}
		}
		if chunk.Error != "" {
			terminal = pluginabi.NewError("upstream_stream_error", chunk.Error, http.StatusBadGateway)
			return
		}
		if chunk.Done {
			return
		}
	}
}
