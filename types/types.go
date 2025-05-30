package types

import (
	"encoding/json"
	"time"
)

// WorkflowState represents the current state of a workflow
type WorkflowState string

const (
	WorkflowStatePending   WorkflowState = "pending"
	WorkflowStateRunning   WorkflowState = "running"
	WorkflowStateCompleted WorkflowState = "completed"
	WorkflowStateFailed    WorkflowState = "failed"
	WorkflowStatePaused    WorkflowState = "paused"
)

// TaskState represents the current state of a task
type TaskState string

const (
	TaskStatePending   TaskState = "pending"
	TaskStateRunning   TaskState = "running"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
	TaskStateRetrying  TaskState = "retrying"
)

// RetryPolicy defines how tasks should be retried
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialInterval time.Duration `json:"initial_interval"`
	BackoffFactor   float64       `json:"backoff_factor"`
	MaxInterval     time.Duration `json:"max_interval"`
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		BackoffFactor:   2.0,
		MaxInterval:     30 * time.Second,
	}
}

// ActivityDefinition defines a single activity/task in a workflow
type ActivityDefinition struct {
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Input        map[string]any `json:"input"`
	Timeout      time.Duration  `json:"timeout"`
	RetryPolicy  RetryPolicy    `json:"retry_policy"`
	Dependencies []string       `json:"dependencies"` // Names of activities this depends on
}

// WorkflowDefinition defines the structure of a workflow
type WorkflowDefinition struct {
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Description string               `json:"description"`
	Activities  []ActivityDefinition `json:"activities"`
	Timeout     time.Duration        `json:"timeout"`
}

// WorkflowInstance represents a running instance of a workflow
type WorkflowInstance struct {
	ID           string         `json:"id"`
	WorkflowName string         `json:"workflow_name"`
	State        WorkflowState  `json:"state"`
	Input        map[string]any `json:"input"`
	Output       map[string]any `json:"output,omitempty"`
	Error        string         `json:"error,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Tasks        []TaskInstance `json:"tasks"`
}

// TaskInstance represents a running instance of a task/activity
type TaskInstance struct {
	ID           string         `json:"id"`
	WorkflowID   string         `json:"workflow_id"`
	ActivityName string         `json:"activity_name"`
	ActivityType string         `json:"activity_type"`
	State        TaskState      `json:"state"`
	Input        map[string]any `json:"input"`
	Output       map[string]any `json:"output,omitempty"`
	Error        string         `json:"error,omitempty"`
	RetryCount   int            `json:"retry_count"`
	MaxRetries   int            `json:"max_retries"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
}

// TaskMessage represents a message sent to workers
type TaskMessage struct {
	TaskID       string         `json:"task_id"`
	WorkflowID   string         `json:"workflow_id"`
	ActivityName string         `json:"activity_name"`
	ActivityType string         `json:"activity_type"`
	Input        map[string]any `json:"input"`
	Timeout      time.Duration  `json:"timeout"`
	RetryCount   int            `json:"retry_count"`
	MaxRetries   int            `json:"max_retries"`
}

// TaskResult represents the result of task execution
type TaskResult struct {
	TaskID      string         `json:"task_id"`
	WorkflowID  string         `json:"workflow_id"`
	Success     bool           `json:"success"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	CompletedAt time.Time      `json:"completed_at"`
}

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
