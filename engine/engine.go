package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"maps"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	storage "github.com/ngnhng/diy-temporal/nats"
	"github.com/ngnhng/diy-temporal/types"
)

const (
	TaskQueueSubject   = "workflow.tasks"
	ResultQueueSubject = "workflow.results"
	WorkflowStreamName = "WORKFLOWS"
	TaskConsumerName   = "task-processor"
	ResultConsumerName = "result-processor"
)

type WorkflowEngine struct {
	storage      *storage.Client
	definitions  map[string]*types.WorkflowDefinition
	mu           sync.RWMutex
	ctx          context.Context
	taskStream   jetstream.Stream
	resultStream jetstream.Stream
	taskKV       jetstream.KeyValue
	// Holds workflow definitions, engine API Writes here, Orchestrator reads here
	workflowKV jetstream.KeyValue
}

// TODO: use some logging code that supports output logger detail (which line, fn, etc.)
func NewWorkflowEngine(ctx context.Context, natsURL string) (*WorkflowEngine, error) {
	store, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	engine := &WorkflowEngine{
		storage:     store,
		definitions: make(map[string]*types.WorkflowDefinition),
		ctx:         ctx,
	}

	// Initialize streams
	if err := engine.initializeStreams(); err != nil {
		return nil, fmt.Errorf("failed to initialize streams: %w", err)
	}

	// Initialzie KVs
	if err := engine.initializeKVs(); err != nil {
		return nil, fmt.Errorf("failed to initialize KVs: %w", err)
	}

	// Start result processor
	go engine.processResults()

	return engine, nil
}

func (e *WorkflowEngine) initializeStreams() error {
	// Create task stream
	taskStream, err := e.storage.EnsureStream(e.ctx, jetstream.StreamConfig{
		Name:       "TASKS",
		Subjects:   []string{TaskQueueSubject, TaskQueueSubject + ".>"},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.WorkQueuePolicy,
		MaxAge:     24 * time.Hour,
		Duplicates: 5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create task stream: %w", err)
	}
	e.taskStream = taskStream

	// Create result stream
	resultStream, err := e.storage.EnsureStream(e.ctx, jetstream.StreamConfig{
		Name:       "RESULTS",
		Subjects:   []string{ResultQueueSubject},
		Storage:    jetstream.FileStorage,
		Retention:  jetstream.LimitsPolicy,
		MaxAge:     24 * time.Hour,
		Duplicates: 5 * time.Minute,
	})
	if err != nil {
		return fmt.Errorf("failed to create result stream: %w", err)
	}
	e.resultStream = resultStream

	return nil
}

func (e *WorkflowEngine) initializeKVs() error {
	taskKV, err := e.storage.EnsureKV(e.ctx, jetstream.KeyValueConfig{
		Bucket:      "tasks",
		Description: "Executing activity instance storage",
		TTL:         1 * time.Hour, // TODO: do I even need the TTL
	})
	if err != nil {
		return fmt.Errorf("failed to create JetStream KV: %w", err)
	}

	e.taskKV = taskKV

	workflowKV, err := e.storage.EnsureKV(e.ctx, jetstream.KeyValueConfig{
		Bucket:      "workflows",
		Description: "Workflow definition storage",
	})
	if err != nil {
		return fmt.Errorf("failed to create JetStream KV: %w", err)
	}

	e.workflowKV = workflowKV
	return nil
}

func (e *WorkflowEngine) Close() error {
	return e.storage.Close()
}

// RegisterWorkflowDefinition registers a new workflow definition
func (e *WorkflowEngine) RegisterWorkflowDefinition(def *types.WorkflowDefinition) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate definition
	if err := e.validateWorkflowDefinition(def); err != nil {
		return fmt.Errorf("invalid workflow definition: %w", err)
	}

	e.definitions[def.Name] = def
	log.Printf("Registered workflow definition: %s (version: %s)", def.Name, def.Version)
	return nil
}

func (e *WorkflowEngine) validateWorkflowDefinition(def *types.WorkflowDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(def.Activities) == 0 {
		return fmt.Errorf("workflow must have at least one activity")
	}

	// Validate dependencies
	activityNames := make(map[string]bool)
	for _, activity := range def.Activities {
		if activity.Name == "" {
			return fmt.Errorf("activity name is required")
		}
		if activity.Type == "" {
			return fmt.Errorf("activity type is required for %s", activity.Name)
		}
		activityNames[activity.Name] = true
	}

	// Check that all dependencies exist
	for _, activity := range def.Activities {
		for _, dep := range activity.Dependencies {
			if !activityNames[dep] {
				return fmt.Errorf("activity %s depends on non-existent activity %s", activity.Name, dep)
			}
		}
	}

	return nil
}

// StartWorkflow starts a new workflow instance
func (e *WorkflowEngine) StartWorkflow(ctx context.Context, workflowName string, input map[string]any) (*types.WorkflowInstance, error) {
	e.mu.RLock()
	def, exists := e.definitions[workflowName]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow definition not found: %s", workflowName)
	}

	// Create workflow instance
	workflowID := uuid.New().String()
	now := time.Now()

	workflow := &types.WorkflowInstance{
		ID:           workflowID,
		WorkflowName: workflowName,
		State:        types.WorkflowStatePending,
		Input:        input,
		CreatedAt:    now,
		UpdatedAt:    now,
		Tasks:        make([]types.TaskInstance, 0),
	}

	// Create task instances
	for _, activityDef := range def.Activities {
		task := types.TaskInstance{
			ID:           uuid.New().String(),
			WorkflowID:   workflowID,
			ActivityName: activityDef.Name,
			ActivityType: activityDef.Type,
			State:        types.TaskStatePending,
			Input:        e.prepareTaskInput(activityDef.Input, input),
			RetryCount:   0,
			MaxRetries:   activityDef.RetryPolicy.MaxRetries,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		workflow.Tasks = append(workflow.Tasks, task)
	}

	// Save workflow
	if err := e.saveWorkflow(workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	// Start workflow execution
	if err := e.executeNextTasks(workflow); err != nil {
		return nil, fmt.Errorf("failed to start workflow execution: %w", err)
	}

	log.Printf("[ENGINE] Started workflow %s with ID %s", workflowName, workflowID)
	return workflow, nil
}

func (e *WorkflowEngine) prepareTaskInput(activityInput, workflowInput map[string]any) map[string]any {
	result := make(map[string]any)

	// Copy activity input
	maps.Copy(result, activityInput)

	// Add workflow input
	maps.Copy(result, workflowInput)

	return result
}

func (e *WorkflowEngine) executeNextTasks(workflow *types.WorkflowInstance) error {
	// Find tasks that are ready to execute (dependencies satisfied)
	readyTasks := e.findReadyTasks(workflow)

	if len(readyTasks) == 0 {
		// Check if workflow is complete
		if e.isWorkflowComplete(workflow) {
			return e.completeWorkflow(e.ctx, workflow)
		}
		return nil
	}

	// Update workflow state to running
	workflow.State = types.WorkflowStateRunning
	workflow.UpdatedAt = time.Now()

	// Dispatch ready tasks
	for _, task := range readyTasks {
		if err := e.dispatchTask(e.ctx, workflow, task); err != nil {
			log.Printf("Failed to dispatch task %s: %v", task.ID, err)
			continue
		}
	}

	// Save updated workflow
	return e.saveWorkflow(workflow)
}

func (e *WorkflowEngine) findReadyTasks(workflow *types.WorkflowInstance) []*types.TaskInstance {
	e.mu.RLock()
	def := e.definitions[workflow.WorkflowName]
	e.mu.RUnlock()

	var readyTasks []*types.TaskInstance

	for i := range workflow.Tasks {
		task := &workflow.Tasks[i]
		if task.State != types.TaskStatePending {
			continue
		}

		// Find activity definition
		var activityDef *types.ActivityDefinition
		for j := range def.Activities {
			if def.Activities[j].Name == task.ActivityName {
				activityDef = &def.Activities[j]
				break
			}
		}

		if activityDef == nil {
			continue
		}

		// Check if all dependencies are satisfied
		if e.areDependenciesSatisfied(workflow, activityDef.Dependencies) {
			readyTasks = append(readyTasks, task)
		}
	}

	return readyTasks
}

// Simple check whether an activity is ready to be executed
// Check if it's dependent tasks/activities are completed
func (e *WorkflowEngine) areDependenciesSatisfied(workflow *types.WorkflowInstance, dependencies []string) bool {
	if len(dependencies) == 0 {
		return true
	}

	for _, depName := range dependencies {
		found := false
		for _, task := range workflow.Tasks {
			if task.ActivityName == depName && task.State == types.TaskStateCompleted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (e *WorkflowEngine) isWorkflowComplete(workflow *types.WorkflowInstance) bool {
	for _, task := range workflow.Tasks {
		if task.State != types.TaskStateCompleted && task.State != types.TaskStateFailed {
			return false
		}
	}
	return true
}

func (e *WorkflowEngine) completeWorkflow(ctx context.Context, workflow *types.WorkflowInstance) error {
	// Check if any tasks failed
	for _, task := range workflow.Tasks {
		if task.State == types.TaskStateFailed {
			workflow.State = types.WorkflowStateFailed
			workflow.Error = fmt.Sprintf("Task %s failed: %s", task.ActivityName, task.Error)
			now := time.Now()
			workflow.CompletedAt = &now
			workflow.UpdatedAt = now
			return e.saveWorkflow(workflow)
		}
	}

	// All tasks completed successfully
	workflow.State = types.WorkflowStateCompleted
	now := time.Now()
	workflow.CompletedAt = &now
	workflow.UpdatedAt = now

	// Collect outputs from tasks
	workflow.Output = make(map[string]any)
	for _, task := range workflow.Tasks {
		for k, v := range task.Output {
			workflow.Output[k] = v
		}
	}

	log.Printf("Workflow %s completed successfully", workflow.ID)
	return e.saveWorkflow(workflow)
}

func (e *WorkflowEngine) dispatchTask(ctx context.Context, workflow *types.WorkflowInstance, task *types.TaskInstance) error {
	// Update task state
	task.State = types.TaskStateRunning
	task.UpdatedAt = time.Now()

	// Create task message
	taskMsg := &types.TaskMessage{
		TaskID:       task.ID,
		WorkflowID:   task.WorkflowID,
		ActivityName: task.ActivityName,
		ActivityType: task.ActivityType,
		Input:        task.Input,
		Timeout:      30 * time.Second, // Default timeout
		RetryCount:   task.RetryCount,
		MaxRetries:   task.MaxRetries,
	}

	// Serialize and publish
	data, err := taskMsg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize task message: %w", err)
	}

	// Use subject pattern that will be matched by worker filters
	// Format: workflow.tasks.{activity_type} to allow for more specific filtering if needed
	subject := fmt.Sprintf("%s.%s", TaskQueueSubject, task.ActivityType)
	_, err = e.storage.JetStream().Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish task: %w", err)
	}

	log.Printf("[ENGINE] Dispatched task %s (%s) for workflow %s", task.ID, task.ActivityName, workflow.ID)
	return nil
}

// processResults processes task results from workers
func (e *WorkflowEngine) processResults() {
	consumer, err := e.resultStream.CreateOrUpdateConsumer(e.ctx, jetstream.ConsumerConfig{
		Name:          ResultConsumerName,
		Durable:       ResultConsumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		MaxAckPending: 100,
	})
	if err != nil {
		log.Printf("Failed to create result consumer: %v", err)
		return
	}

	msgs, err := consumer.Consume(func(msg jetstream.Msg) {
		if err := e.handleTaskResult(msg); err != nil {
			log.Printf("Failed to handle task result: %v", err)
			msg.Nak()
		} else {
			msg.Ack()
		}
	})
	if err != nil {
		log.Printf("Failed to start consuming results: %v", err)
		return
	}

	// Wait for context cancellation
	<-e.ctx.Done()
	msgs.Stop()
}

func (e *WorkflowEngine) handleTaskResult(msg jetstream.Msg) error {
	result := types.TaskResult{}
	if err := result.FromJSON(msg.Data()); err != nil {
		return fmt.Errorf("failed to deserialize task result: %w", err)
	}

	log.Printf("[ENGINE] <DEBUG> %v\n", result.Debug())

	// Get workflow
	workflow, err := e.GetWorkflow(result.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	// Find and update task
	taskUpdated := false
	for i := range workflow.Tasks {
		task := &workflow.Tasks[i]
		if task.ID == result.TaskID {
			if result.Success {
				task.State = types.TaskStateCompleted
				task.Output = result.Output
				now := time.Now()
				task.CompletedAt = &now
			} else {
				task.Error = result.Error
				if task.RetryCount < task.MaxRetries {
					task.State = types.TaskStateRetrying
					task.RetryCount++
				} else {
					task.State = types.TaskStateFailed
					now := time.Now()
					task.CompletedAt = &now
				}
			}
			task.UpdatedAt = time.Now()
			taskUpdated = true
			break
		}
	}

	if !taskUpdated {
		return fmt.Errorf("task not found: %s", result.TaskID)
	}

	// Save workflow
	if err := e.saveWorkflow(workflow); err != nil {
		return fmt.Errorf("failed to save workflow: %w", err)
	}

	// Continue workflow execution
	if err := e.executeNextTasks(workflow); err != nil {
		return fmt.Errorf("failed to continue workflow execution: %w", err)
	}

	log.Printf("[ENGINE] Processed result for task %s in workflow %s", result.TaskID, result.WorkflowID)
	return nil
}

func (e *WorkflowEngine) saveWorkflow(workflow *types.WorkflowInstance) error {
	data, err := workflow.ToJSON()
	if err != nil || data == nil {
		return fmt.Errorf("error serializing data %v: %w", data, err)
	}
	e.workflowKV.Put(e.ctx, workflow.ID, data)
	return nil
}

// GetWorkflow retrieves a workflow by ID
func (e *WorkflowEngine) GetWorkflow(workflowID string) (*types.WorkflowInstance, error) {
	entry, err := e.workflowKV.Get(e.ctx, workflowID)

	if err != nil || entry == nil || entry.Value() == nil {
		return nil, fmt.Errorf("error retrieving workflow %v instance: %w", workflowID, err)
	}

	instance := &types.WorkflowInstance{}
	err = instance.FromJSON(entry.Value())
	return instance, err
}

// ListWorkflows retrieves all workflows
func (e *WorkflowEngine) ListWorkflows() ([]*types.WorkflowInstance, error) {

	keyLister, err := e.workflowKV.ListKeys(e.ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching key list: %w", err)
	}

	var result []*types.WorkflowInstance
	for k := range keyLister.Keys() {
		v, err := e.GetWorkflow(k)
		if err != nil {
			return nil, fmt.Errorf("error fetching key list: %w", err)
		}

		result = append(result, v)
	}

	return result, nil
}
