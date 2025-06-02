package types

import "time"

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
