package util

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalPretty returns indented JSON for v, or an error string if marshalling fails.
func MarshalPretty(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(b)
}

// MarshalCompact returns compact JSON for v, or an error string.
func MarshalCompact(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(b)
}

// UnmarshalAny decodes JSON bytes into a map[string]interface{}.
func UnmarshalAny(data []byte) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UnmarshalSlice decodes JSON bytes into []interface{}.
func UnmarshalSlice(data []byte) ([]interface{}, error) {
	var out []interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Reformat parses JSON and re-encodes it with indentation.
// Returns the original bytes unchanged if parsing fails.
func Reformat(data []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return data
	}
	return buf.Bytes()
}

// StringField extracts a string field from a raw JSON object.
// Returns "" if the key is absent or not a string.
func StringField(data []byte, key string) string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	raw, ok := m[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// MergeJSON merges src into dst (both must be JSON objects).
// Keys in src overwrite keys in dst.
func MergeJSON(dst, src []byte) ([]byte, error) {
	var dstMap, srcMap map[string]json.RawMessage
	if err := json.Unmarshal(dst, &dstMap); err != nil {
		return nil, fmt.Errorf("merge json dst: %w", err)
	}
	if err := json.Unmarshal(src, &srcMap); err != nil {
		return nil, fmt.Errorf("merge json src: %w", err)
	}
	for k, v := range srcMap {
		dstMap[k] = v
	}
	return json.Marshal(dstMap)
}
