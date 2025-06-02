package types

import "time"

// ActivityDefinition defines a single activity/task in a workflow
type ActivityDefinition struct {
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Input        map[string]any `json:"input"`
	Timeout      time.Duration  `json:"timeout"`
	RetryPolicy  RetryPolicy    `json:"retry_policy"`
	Dependencies []string       `json:"dependencies"` // Names of activities this depends on
}
