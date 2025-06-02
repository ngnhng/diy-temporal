package types

import "time"

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
