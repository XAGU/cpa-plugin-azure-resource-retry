package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func sanitizeHistory(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("历史请求正文不是有效 JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("历史请求正文必须只包含一个 JSON 值")
	}
	body, ok := value.(map[string]any)
	if !ok {
		return raw, nil
	}

	changed := false
	if _, exists := body["previous_response_id"]; exists {
		delete(body, "previous_response_id")
		changed = true
	}
	if include, ok := body["include"].([]any); ok {
		kept := include[:0]
		for _, entry := range include {
			if text, ok := entry.(string); ok && text == "reasoning.encrypted_content" {
				changed = true
				continue
			}
			kept = append(kept, entry)
		}
		if len(kept) == 0 {
			delete(body, "include")
			changed = true
		} else {
			body["include"] = kept
		}
	}
	if input, exists := body["input"]; exists {
		cleaned, inputChanged := sanitizeInput(input)
		body["input"] = cleaned
		changed = changed || inputChanged
	}
	if !changed {
		return raw, nil
	}
	return json.Marshal(body)
}

func sanitizeInput(value any) (any, bool) {
	items, ok := value.([]any)
	if !ok {
		return value, false
	}
	changed := false
	kept := items[:0]
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			kept = append(kept, item)
			continue
		}
		if object["type"] == "item_reference" {
			changed = true
			continue
		}
		// 只清理历史 item 自身的资源标识，保留 call_id、工具定义和嵌套业务内容。
		for key := range object {
			if key == "id" || strings.HasPrefix(key, "encrypted") {
				delete(object, key)
				changed = true
			}
		}
		kept = append(kept, object)
	}
	return kept, changed
}
