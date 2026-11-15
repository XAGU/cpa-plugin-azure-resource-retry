package plugin

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
)

var differentResource = regexp.MustCompile(`(?i)\bthe requested item was created under a different\s+[^.\r\n]{0,128}\bopenai resource\b`)

func isResourceMismatch(err error) bool {
	var detail *pluginabi.Error
	if !errors.As(err, &detail) || detail.HTTPStatus != http.StatusBadRequest {
		return false
	}
	message := detail.Message
	var body struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(message), &body) == nil {
		if body.Error.Message != "" {
			message = body.Error.Message
		} else if body.Message != "" {
			message = body.Message
		} else {
			return false
		}
	}
	return differentResource.MatchString(strings.Join(strings.Fields(message), " "))
}

func errorDetail(err error) *pluginabi.Error {
	var detail *pluginabi.Error
	if errors.As(err, &detail) {
		return detail
	}
	return pluginabi.NewError("plugin_error", err.Error(), http.StatusInternalServerError)
}
