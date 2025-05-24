package engine

import (
	"app/model"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Common error definitions
var (
	ErrWorkflowNotFound      = errors.New("workflow not found")
	ErrWorkflowAlreadyExists = errors.New("workflow already exists with this ID")
	ErrInvalidWorkflowState  = errors.New("invalid workflow state")
	ErrTaskQueueNotFound     = errors.New("task queue not found")
)

type (
	// Engine is the central workflow execution orchestrator
	Engine struct {
		// Configuration options
		config Config

		// Registration and execution data
		workflows         map[string]*model.WorkflowDefinition
		workflowInstances map[string]*WorkflowInstance
		taskQueues        map[string]*TaskQueue

		// NATS connection for state persistence and pub/sub
		natsConn *nats.Conn

		// Engine state
		isRunning bool
		stopCh    chan struct{}

		// Concurrency controls
		mu         sync.RWMutex
		instanceMu sync.RWMutex
	}

	// Config holds configuration options for the workflow engine
	Config struct {
		// NATS connection details
		NatsURL   string
		ClusterID string

		// Engine settings
		MaxConcurrentExecutions int
		StateCheckInterval      time.Duration
		TaskTimeout             time.Duration
	}

	// WorkflowInstance represents a running workflow
	WorkflowInstance struct {
		ID           string
		RunID        string
		WorkflowType string
		Status       WorkflowStatus
		State        interface{} // Current state of the workflow
		Input        interface{} // Initial input
		Result       interface{} // Final result when completed
		Error        error       // Error if failed
		TaskQueue    string      // Queue this workflow is assigned to
		StartedAt    time.Time
		CompletedAt  time.Time
		History      []HistoryEvent
	}

	// TaskQueue represents a queue where workers poll for tasks
	TaskQueue struct {
		Name      string
		Workers   []string // Worker IDs connected to this queue
		TaskCount int      // Number of pending tasks
	}

	// WorkflowStatus represents the current status of a workflow instance
	WorkflowStatus string

	// HistoryEvent represents a significant event in the workflow execution
	HistoryEvent struct {
		EventType  string
		EventID    int64
		Timestamp  time.Time
		Attributes interface{} // Event-specific data
	}

	// RegisteredWorker represents a worker instance that has registered with the engine
	RegisteredWorker struct {
		ID            string    // Unique identifier for the worker
		TaskQueue     string    // Task queue this worker listens on
		Workflows     []string  // Workflow types this worker can handle
		Activities    []string  // Activity types this worker can handle
		RegisteredAt  time.Time // When this worker registered
		LastHeartbeat time.Time // Last heartbeat received from this worker
	}

	// TaskResult represents the result of a task execution
	TaskResult struct {
		TaskID     string      `json:"taskId"`
		WorkflowID string      `json:"workflowId"`
		RunID      string      `json:"runId"`
		Result     interface{} `json:"result,omitempty"`
		Error      string      `json:"error,omitempty"`
		Timestamp  int64       `json:"timestamp"`
	}
)

// WorkflowStatus constants
const (
	WorkflowStatusCreated    WorkflowStatus = "CREATED"
	WorkflowStatusRunning    WorkflowStatus = "RUNNING"
	WorkflowStatusCompleted  WorkflowStatus = "COMPLETED"
	WorkflowStatusFailed     WorkflowStatus = "FAILED"
	WorkflowStatusCanceled   WorkflowStatus = "CANCELED"
	WorkflowStatusTerminated WorkflowStatus = "TERMINATED"
)

// NewEngine creates a new workflow engine instance
func NewEngine(config Config) *Engine {
	// Set reasonable defaults
	if config.MaxConcurrentExecutions <= 0 {
		config.MaxConcurrentExecutions = 100
	}
	if config.StateCheckInterval <= 0 {
		config.StateCheckInterval = 5 * time.Second
	}
	if config.TaskTimeout <= 0 {
		config.TaskTimeout = 30 * time.Second
	}

	nc, err := nats.Connect(config.NatsURL)
	if err != nil {
		return nil
	}

	return &Engine{
		config:            config,
		workflows:         make(map[string]*model.WorkflowDefinition),
		workflowInstances: make(map[string]*WorkflowInstance),
		taskQueues:        make(map[string]*TaskQueue),
		stopCh:            make(chan struct{}),
		natsConn:          nc,
	}
}

// Start initializes and starts the workflow engine
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning {
		return nil
	}

	if e.natsConn == nil {
		nc, err := nats.Connect(e.config.NatsURL)
		if err != nil {
			return fmt.Errorf("failed to connect to NATS: %w", err)
		}
		e.natsConn = nc
	}

	// Start background workers
	go e.processScheduledTasks(ctx)
	go e.monitorWorkflowState(ctx)

	// Set up NATS subscriptions for API requests and commands
	// Subscribe to worker registration requests
	if _, err := e.natsConn.Subscribe("WORKFLOW.Command.RegisterWorker", e.registerWorkerHandler); err != nil {
		return fmt.Errorf("failed to subscribe to worker registrations: %w", err)
	}

	// Additional subscriptions for API requests would go here
	// nc.Subscribe("WORKFLOW.Api.StartWorkflow", e.handleStartWorkflow)
	// nc.Subscribe("WORKFLOW.Api.GetResult", e.handleGetResult)

	fmt.Println("Workflow engine started successfully")
	e.isRunning = true

	return nil
}

// Stop gracefully shuts down the workflow engine
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return nil
	}

	// Signal all background workers to stop
	close(e.stopCh)

	if e.natsConn != nil {
		e.natsConn.Close()
	}

	e.isRunning = false
	fmt.Println("Workflow engine stopped")

	return nil
}

// RegisterWorkflowType registers a new workflow definition with the engine
// TODO: complete
func (e *Engine) RegisterWorkflowType(workflowType string, definition interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.workflows[workflowType]; exists {
		// Overwrite existing definition
		fmt.Printf("Overwriting existing workflow definition: %s\n", workflowType)
	}

	e.workflows[workflowType] = &model.WorkflowDefinition{
		Type:         workflowType,
		StateMachine: definition,
		CreatedAt:    time.Now(),
	}

	fmt.Printf("Registered workflow type: %s\n", workflowType)
	return nil
}

// RegisterTaskQueue registers a new task queue
func (e *Engine) RegisterTaskQueue(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.taskQueues[name]; exists {
		return nil
	}

	e.taskQueues[name] = &TaskQueue{
		Name: name,
	}

	fmt.Printf("Registered task queue: %s\n", name)
	return nil
}

// StartWorkflow initiates a new workflow execution
func (e *Engine) StartWorkflow(
	ctx context.Context,
	workflowType string,
	workflowID string,
	taskQueue string,
	input interface{},
) (string, error) {
	e.mu.RLock()
	_, exists := e.workflows[workflowType]
	if !exists {
		e.mu.RUnlock()
		return "", ErrWorkflowNotFound
	}

	_, queueExists := e.taskQueues[taskQueue]
	e.mu.RUnlock()

	if !queueExists {
		return "", ErrTaskQueueNotFound
	}

	// Generate a workflow ID if not provided
	if workflowID == "" {
		workflowID = uuid.New().String()
	}

	// Generate a run ID for this execution
	runID := uuid.New().String()

	instance := &WorkflowInstance{
		ID:           workflowID,
		RunID:        runID,
		WorkflowType: workflowType,
		Status:       WorkflowStatusCreated,
		Input:        input,
		TaskQueue:    taskQueue,
		StartedAt:    time.Now(),
		History:      []HistoryEvent{},
	}

	// Add initial event to history
	instance.History = append(instance.History, HistoryEvent{
		EventType: "WorkflowExecutionStarted",
		EventID:   1,
		Timestamp: time.Now(),
		Attributes: map[string]interface{}{
			"workflowType": workflowType,
			"taskQueue":    taskQueue,
		},
	})

	// Store the new workflow instance
	e.instanceMu.Lock()
	if _, exists := e.workflowInstances[workflowID]; exists {
		e.instanceMu.Unlock()
		return "", ErrWorkflowAlreadyExists
	}
	e.workflowInstances[workflowID] = instance
	e.instanceMu.Unlock()

	// In a real implementation, persist the workflow state to NATS JetStream
	key := fmt.Sprintf("WORKFLOW.State.%s", workflowID)
	js, err := e.natsConn.JetStream()
	if err != nil {
		return "", ErrInvalidWorkflowState
	}

	js.Publish(key, encodeWorkflowState(instance))

	// Schedule the first workflow task
	workflowTask := &model.WorkflowTask{
		TaskID:       uuid.New().String(),
		WorkflowID:   workflowID,
		RunID:        runID,
		WorkflowType: workflowType,
		TaskType:     "WorkflowExecutionTask",
		Input:        input,
		ScheduledAt:  time.Now(),
	}

	taskQueueSubject := fmt.Sprintf("WORKFLOW.TaskQueue.%s", taskQueue)
	e.natsConn.Publish(taskQueueSubject, encodeTask(workflowTask))

	fmt.Printf("Started workflow %s (ID: %s, Run: %s) on queue %s\n",
		workflowType, workflowID, runID, taskQueue)

	// Update status
	e.updateWorkflowStatus(workflowID, WorkflowStatusRunning)

	return runID, nil
}

// GetWorkflowResult retrieves the result or error of a workflow execution
func (e *Engine) GetWorkflowResult(workflowID string) (interface{}, error) {
	e.instanceMu.RLock()
	instance, exists := e.workflowInstances[workflowID]
	e.instanceMu.RUnlock()

	if !exists {
		// In a real implementation, try to load from NATS JetStream first
		// key := fmt.Sprintf("WORKFLOW.State.%s", workflowID)
		// msg, err := e.natsConn.JetStream().GetMsg(key)
		// if err != nil {
		//     return nil, ErrWorkflowNotFound
		// }
		// instance = decodeWorkflowState(msg.Data)

		return nil, ErrWorkflowNotFound
	}

	// Check if the workflow is completed
	if instance.Status == WorkflowStatusCompleted {
		return instance.Result, nil
	} else if instance.Status == WorkflowStatusFailed {
		if instance.Error != nil {
			return nil, instance.Error
		}
		return nil, fmt.Errorf("workflow execution failed")
	} else if instance.Status == WorkflowStatusCanceled {
		return nil, fmt.Errorf("workflow execution was canceled")
	} else if instance.Status == WorkflowStatusTerminated {
		return nil, fmt.Errorf("workflow execution was terminated")
	}

	// Workflow is still running
	return nil, fmt.Errorf("workflow execution is still in progress: %s", instance.Status)
}

// CompleteWorkflowTask processes the result of a workflow task
func (e *Engine) CompleteWorkflowTask(taskID string, workflowID string, result interface{}, err error) error {
	e.instanceMu.Lock()
	defer e.instanceMu.Unlock()

	instance, exists := e.workflowInstances[workflowID]
	if !exists {
		return ErrWorkflowNotFound
	}

	if err != nil {
		// Task failed, update workflow status and record error
		instance.Status = WorkflowStatusFailed
		instance.Error = err
		instance.CompletedAt = time.Now()

		// Add event to history
		instance.History = append(instance.History, HistoryEvent{
			EventType: "WorkflowExecutionFailed",
			EventID:   int64(len(instance.History) + 1),
			Timestamp: time.Now(),
			Attributes: map[string]interface{}{
				"error": err.Error(),
			},
		})
	} else {
		// Check if this is the final result
		isFinal := true // In a real implementation, this would be determined by the task result

		if isFinal {
			// Final task completed, update workflow status and record result
			instance.Status = WorkflowStatusCompleted
			instance.Result = result
			instance.CompletedAt = time.Now()

			// Add event to history
			instance.History = append(instance.History, HistoryEvent{
				EventType: "WorkflowExecutionCompleted",
				EventID:   int64(len(instance.History) + 1),
				Timestamp: time.Now(),
			})
		} else {
			// Not final, schedule next task based on workflow state machine
			// This would create and schedule the next WorkflowTask
			// For simplicity, this is not implemented here

			// Add event to history
			instance.History = append(instance.History, HistoryEvent{
				EventType: "WorkflowTaskCompleted",
				EventID:   int64(len(instance.History) + 1),
				Timestamp: time.Now(),
			})
		}
	}

	// In a real implementation, persist the updated state to NATS JetStream
	// key := fmt.Sprintf("WORKFLOW.State.%s", workflowID)
	// e.natsConn.JetStream().Publish(key, encodeWorkflowState(instance))

	return nil
}

// Internal method to update a workflow's status
func (e *Engine) updateWorkflowStatus(workflowID string, status WorkflowStatus) {
	e.instanceMu.Lock()
	defer e.instanceMu.Unlock()

	if instance, exists := e.workflowInstances[workflowID]; exists {
		instance.Status = status

		// In a real implementation, persist the updated state to NATS JetStream
		// key := fmt.Sprintf("WORKFLOW.State.%s", workflowID)
		// e.natsConn.JetStream().Publish(key, encodeWorkflowState(instance))
	}
}

// Background worker to process scheduled tasks
func (e *Engine) processScheduledTasks(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In a real implementation, this would check for scheduled tasks
			// and dispatch them to appropriate task queues
			// fmt.Println("Processing scheduled tasks...")
		}
	}
}

// Background worker to monitor workflow states and handle timeouts
func (e *Engine) monitorWorkflowState(ctx context.Context) {
	ticker := time.NewTicker(e.config.StateCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.instanceMu.RLock()
			for id, instance := range e.workflowInstances {
				if instance.Status == WorkflowStatusRunning {
					// Check for timeouts or other state conditions
					// This is simplified; in a real implementation, you'd have more logic
					if time.Since(instance.StartedAt) > 24*time.Hour {
						// Example: auto-timeout workflows running for too long
						fmt.Printf("Workflow %s timed out after 24h\n", id)
						e.instanceMu.RUnlock()
						e.updateWorkflowStatus(id, WorkflowStatusFailed)
						e.instanceMu.RLock()
					}
				}
			}
			e.instanceMu.RUnlock()
		}
	}
}

// registerWorkerHandler processes worker registration requests
func (e *Engine) registerWorkerHandler(msg *nats.Msg) {
	if msg.Reply == "" {
		log.Printf("Received worker registration with no reply subject")
		return
	}

	var registration struct {
		WorkerID      string   `json:"workerId"`
		TaskQueue     string   `json:"taskQueue"`
		WorkflowTypes []string `json:"workflowTypes"`
		ActivityTypes []string `json:"activityTypes"`
		Timestamp     int64    `json:"timestamp"`
	}

	var response struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	// Parse registration request
	if err := json.Unmarshal(msg.Data, &registration); err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("Failed to parse registration: %v", err)
		e.sendResponse(msg.Reply, response)
		return
	}

	// Validate request
	if registration.WorkerID == "" {
		response.Success = false
		response.Error = "Worker ID is required"
		e.sendResponse(msg.Reply, response)
		return
	}

	if registration.TaskQueue == "" {
		response.Success = false
		response.Error = "Task queue is required"
		e.sendResponse(msg.Reply, response)
		return
	}

	// Check if task queue exists, create if it doesn't
	e.mu.Lock()
	if _, exists := e.taskQueues[registration.TaskQueue]; !exists {
		e.taskQueues[registration.TaskQueue] = &TaskQueue{
			Name:    registration.TaskQueue,
			Workers: []string{},
		}
		log.Printf("Created new task queue: %s", registration.TaskQueue)
	}

	// Register the worker
	// worker := &RegisteredWorker{
	// 	ID:            registration.WorkerID,
	// 	TaskQueue:     registration.TaskQueue,
	// 	Workflows:     registration.WorkflowTypes,
	// 	Activities:    registration.ActivityTypes,
	// 	RegisteredAt:  time.Now(),
	// 	LastHeartbeat: time.Now(),
	// }

	// Add worker ID to task queue's workers list if not already there
	taskQueue := e.taskQueues[registration.TaskQueue]
	workerExists := false
	for _, wid := range taskQueue.Workers {
		if wid == registration.WorkerID {
			workerExists = true
			break
		}
	}

	if !workerExists {
		taskQueue.Workers = append(taskQueue.Workers, registration.WorkerID)
	}
	e.mu.Unlock()

	// Log the registration
	log.Printf("Worker registered: %s on queue %s with %d workflow types and %d activity types",
		registration.WorkerID, registration.TaskQueue, len(registration.WorkflowTypes), len(registration.ActivityTypes))

	// Send successful response
	response.Success = true
	e.sendResponse(msg.Reply, response)

	// Publish worker registration event
	e.publishEvent("WorkerRegistered", map[string]interface{}{
		"workerID":   registration.WorkerID,
		"taskQueue":  registration.TaskQueue,
		"workflows":  registration.WorkflowTypes,
		"activities": registration.ActivityTypes,
	})
}

// sendResponse sends a response to a NATS request
func (e *Engine) sendResponse(subject string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	if err := e.natsConn.Publish(subject, data); err != nil {
		log.Printf("Failed to publish response to %s: %v", subject, err)
	}
}

// publishEvent publishes an event to the WORKFLOW.Events subject
func (e *Engine) publishEvent(eventType string, data map[string]interface{}) {
	event := map[string]interface{}{
		"type":      eventType,
		"timestamp": time.Now().UnixNano(),
		"data":      data,
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	subject := fmt.Sprintf("WORKFLOW.Events.%s", eventType)
	if err := e.natsConn.Publish(subject, eventData); err != nil {
		log.Printf("Failed to publish event: %v", err)
	}
}

// dispatchWorkflowTask dispatches a workflow task to an appropriate worker
func (e *Engine) dispatchWorkflowTask(ctx context.Context, task *model.WorkflowTask, taskQueue string) error {
	e.mu.RLock()
	queue, exists := e.taskQueues[taskQueue]
	e.mu.RUnlock()

	if !exists || len(queue.Workers) == 0 {
		return fmt.Errorf("no workers available for task queue: %s", taskQueue)
	}

	// Serialize the task
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow task: %w", err)
	}

	// Create subject for the task queue
	subject := fmt.Sprintf("WORKFLOW.TaskQueue.%s.Workflow", taskQueue)

	// Set up context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Send task to a worker through NATS with request-reply pattern
	resp, err := e.natsConn.RequestWithContext(reqCtx, subject, taskData)
	if err != nil {
		return fmt.Errorf("failed to dispatch workflow task: %w", err)
	}

	// Process response to see if task was accepted
	var taskResponse TaskResult
	if err := json.Unmarshal(resp.Data, &taskResponse); err != nil {
		return fmt.Errorf("failed to unmarshal task response: %w", err)
	}

	if taskResponse.Error != "" {
		return fmt.Errorf("task dispatch rejected: %s", taskResponse.Error)
	}

	// Task was accepted, wait for result asynchronously
	go e.waitForTaskResult(ctx, task.TaskID, task.WorkflowID)

	return nil
}

// waitForTaskResult waits for a task result to be published
func (e *Engine) waitForTaskResult(ctx context.Context, taskID string, workflowID string) {
	// Result subject
	resultSubject := fmt.Sprintf("WORKFLOW.TaskResult.%s", taskID)

	// Set up subscription for result
	var sub *nats.Subscription
	var err error
	sub, err = e.natsConn.Subscribe(resultSubject, func(msg *nats.Msg) {
		var result TaskResult
		if err := json.Unmarshal(msg.Data, &result); err != nil {
			log.Printf("Failed to unmarshal task result: %v", err)
			return
		}

		// Process the task result
		log.Printf("Received result for task %s (workflow %s)", taskID, workflowID)

		// In a real implementation, this would update the workflow state
		// For now we'll just complete the workflow with this result
		e.CompleteWorkflowTask(taskID, workflowID, result.Result, nil)

		// Auto-unsubscribe after receiving the result
		sub.Unsubscribe()
	})

	if err != nil {
		log.Printf("Failed to subscribe to task result: %v", err)
		return
	}

	// Set up cleanup when context is done
	go func() {
		select {
		case <-ctx.Done():
			sub.Unsubscribe()
		}
	}()
}
