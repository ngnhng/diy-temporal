package types

import (
	"encoding/json"
)

// Note: Do not use pointer type
func FromJSON[T any](data json.RawMessage) (T, error) {
	var t T
	if err := json.Unmarshal(data, &t); err != nil {
		return t, err
	}
	return t, nil
}

func ToJSON(data any) (json.RawMessage, error) {
	return json.Marshal(data)
}

func Debug(data any) string {
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ""
	}
	return string(result)
}
