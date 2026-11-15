package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	changed := false
	switch current := value.(type) {
	case []any:
		kept := current[:0]
		for _, item := range current {
			if object, ok := item.(map[string]any); ok && object["type"] == "item_reference" {
				changed = true
				continue
			}
			cleaned, itemChanged := sanitizeInput(item)
			kept = append(kept, cleaned)
			changed = changed || itemChanged
		}
		return kept, changed
	case map[string]any:
		// 跨资源重放只移除资源内标识；call_id 继续关联工具调用与工具结果。
		for _, key := range []string{"id", "encrypted_content"} {
			if _, exists := current[key]; exists {
				delete(current, key)
				changed = true
			}
		}
		for key, child := range current {
			cleaned, childChanged := sanitizeInput(child)
			current[key] = cleaned
			changed = changed || childChanged
		}
	}
	return value, changed
}
