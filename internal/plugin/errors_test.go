package plugin

import (
	"errors"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
)

func TestIsResourceMismatchRequiresBadRequest(t *testing.T) {
	matching := pluginabi.NewError("host_call_failed", "OpenAI Responses bad request: The requested item was created under a different OpenAI resource. Use the same resource that created the item to access it. [trace_id=abc]", http.StatusBadRequest)
	if !isResourceMismatch(matching) {
		t.Fatal("matching resource error was not recognized")
	}
	wrapped := errors.Join(errors.New("host callback"), matching)
	if !isResourceMismatch(wrapped) {
		t.Fatal("wrapped resource error was not recognized")
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
		err := pluginabi.NewError("host_call_failed", matching.Message, status)
		if isResourceMismatch(err) {
			t.Fatalf("status %d incorrectly triggered cleanup", status)
		}
	}
	other := pluginabi.NewError("host_call_failed", "The requested item does not exist.", http.StatusBadRequest)
	if isResourceMismatch(other) {
		t.Fatal("unrelated bad request triggered cleanup")
	}
}
