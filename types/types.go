package types

import (
	"encoding/json"
)

// ToJSON converts a struct to JSON bytes
func (w *WorkflowInstance) ToJSON() ([]byte, error) {
	return json.Marshal(w)
}

// FromJSON converts JSON bytes to WorkflowInstance
func (w *WorkflowInstance) FromJSON(data []byte) error {
	return json.Unmarshal(data, w)
}

// ToJSON converts a struct to JSON bytes
func (t *TaskMessage) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// FromJSON converts JSON bytes to TaskMessage
func (t *TaskMessage) FromJSON(data []byte) error {
	return json.Unmarshal(data, t)
}

// ToJSON converts a struct to JSON bytes
func (r *TaskResult) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON converts JSON bytes to TaskResult
func (r *TaskResult) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// Debug Pretty prints to Stdout a readable string
func (r *TaskResult) Debug() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "Error marshaling TaskResult: " + err.Error()
	}
	return string(data)
}

// Debug returns a pretty-printed string representation of the workflow instance
func (w *WorkflowInstance) Debug() string {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return "Error marshaling WorkflowInstance: " + err.Error()
	}
	return string(data)
}

// Debug returns a pretty-printed string representation of the task message
func (t *TaskMessage) Debug() string {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return "Error marshaling TaskMessage: " + err.Error()
	}
	return string(data)
}

func FromJSON[T any](data json.RawMessage) (T, error) {
	var t T
	if err := json.Unmarshal(data, t); err != nil {
		return nil, err
	}
	return t, nil
}

func ToJSON(data any) (json.RawMessage, error) {
	return json.Marshal(data)
}
