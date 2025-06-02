package types

import "time"

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
