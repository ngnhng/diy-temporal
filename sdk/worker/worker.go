package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/ngnhng/diy-temporal/engine"
	storage "github.com/ngnhng/diy-temporal/nats"
	"github.com/ngnhng/diy-temporal/types"
)

type ActivityHandler interface {
	Execute(ctx context.Context, input map[string]any) (map[string]any, error)
	GetActivityType() string
}

// Worker represents a workflow task worker
type Worker struct {
	nc           *storage.Client
	handlers     map[string]ActivityHandler
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	workerID     string
	concurrency  int
	resultStream jetstream.Stream
}

// WorkerConfig contains configuration for the worker
type WorkerConfig struct {
	NatsURL     string
	WorkerID    string
	Concurrency int // Number of concurrent tasks to process
}

// NewWorker creates a new worker instance
func NewWorker(ctx context.Context, config WorkerConfig) (*Worker, error) {
	if config.Concurrency <= 0 {
		config.Concurrency = 5 // Default concurrency
	}

	store, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	worker := &Worker{
		nc:          store,
		handlers:    make(map[string]ActivityHandler),
		ctx:         ctx,
		cancel:      cancel,
		workerID:    config.WorkerID,
		concurrency: config.Concurrency,
	}

	// Initialize result stream
	if err := worker.initializeResultStream(); err != nil {
		return nil, fmt.Errorf("failed to initialize result stream: %w", err)
	}

	return worker, nil
}

func (w *Worker) initializeResultStream() error {
	// Get or create result stream
	stream, err := w.nc.EnsureStream(w.ctx, jetstream.StreamConfig{Name: "RESULTS"})
	if err != nil {
		// Stream might not exist yet, create it
		stream, err = w.nc.EnsureStream(w.ctx, jetstream.StreamConfig{
			Name:       "RESULTS",
			Subjects:   []string{engine.ResultQueueSubject},
			Storage:    jetstream.FileStorage,
			Retention:  jetstream.LimitsPolicy,
			MaxAge:     24 * time.Hour,
			Duplicates: 5 * time.Minute,
		})
		if err != nil {
			return fmt.Errorf("failed to create result stream: %w", err)
		}
	}
	w.resultStream = stream
	return nil
}

// RegisterActivityHandler registers an activity handler
// TODO: use Jetstream KV
func (w *Worker) RegisterActivityHandler(handler ActivityHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()

	activityType := handler.GetActivityType()
	w.handlers[activityType] = handler
	log.Printf("Worker %s registered activity handler: %s", w.workerID, activityType)
}

// Start begins processing tasks
func (w *Worker) Start() error {
	log.Printf("Starting worker %s with concurrency %d", w.workerID, w.concurrency)

	// Get task stream
	_, err := w.nc.EnsureStream(w.ctx, jetstream.StreamConfig{Name: "TASKS"})
	if err != nil {
		return fmt.Errorf("failed to get task stream: %w", err)
	}

	// Create pull consumer to achieve Competing consumer pattern
	consumer, err := w.nc.EnsureConsumer(w.ctx, "TASKS", jetstream.ConsumerConfig{
		Name:          "worker-consumer",
		Durable:       "worker-consumer",
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverAllPolicy,
		MaxAckPending: w.concurrency,
		AckWait:       30 * time.Second,
		MaxDeliver:    3,
		BackOff:       []time.Duration{1 * time.Second, 5 * time.Second, 10 * time.Second},
		// Use wildcard filter that still gets all tasks
		// For workqueue streams, each consumer must have a unique filter
		FilterSubject: fmt.Sprintf("%s.>", engine.TaskQueueSubject),
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	log.Printf("Worker %v fetching from %s\n", w.workerID, consumer.CachedInfo().Name)

	// Start consuming messages
	msgs, err := consumer.Consume(func(msg jetstream.Msg) {
		w.handleTask(msg)
	}, jetstream.ConsumeErrHandler(func(consumeCtx jetstream.ConsumeContext, err error) {
		log.Printf("Consume error: %v", err)
	}))
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	// Wait for context cancellation
	<-w.ctx.Done()
	msgs.Stop()
	return nil
}

// Stop stops the worker
func (w *Worker) Stop() {
	log.Printf("Stopping worker %s", w.workerID)
	w.cancel()
	w.nc.Close()
}

func (w *Worker) handleTask(msg jetstream.Msg) {
	// Parse task message
	var taskMsg types.TaskMessage
	if err := taskMsg.FromJSON(msg.Data()); err != nil {
		log.Printf("Failed to parse task message: %v", err)
		msg.Nak()
		return
	}

	log.Printf("[WORKER] Worker %s processing task %s (%s)", w.workerID, taskMsg.TaskID, taskMsg.ActivityType)

	// Find handler
	w.mu.RLock()
	handler, exists := w.handlers[taskMsg.ActivityType]
	w.mu.RUnlock()

	if !exists {
		log.Printf("[WORKER] No handler found for activity type: %s", taskMsg.ActivityType)
		w.sendTaskResult(taskMsg.TaskID, taskMsg.WorkflowID, false, nil, "No handler found for activity type")
		msg.Ack()
		return
	}

	// Execute task with timeout
	ctx, cancel := context.WithTimeout(w.ctx, taskMsg.Timeout)
	defer cancel()

	startTime := time.Now()
	output, err := handler.Execute(ctx, taskMsg.Input)
	duration := time.Since(startTime)

	// Send result
	if err != nil {
		log.Printf("[WORKER] Task %s failed after %v: %v", taskMsg.TaskID, duration, err)
		w.sendTaskResult(taskMsg.TaskID, taskMsg.WorkflowID, false, nil, err.Error())
	} else {
		log.Printf("[WORKER] Task %s completed successfully in %v", taskMsg.TaskID, duration)
		w.sendTaskResult(taskMsg.TaskID, taskMsg.WorkflowID, true, output, "")
	}

	// Acknowledge message
	msg.Ack()
}

func (w *Worker) sendTaskResult(taskID, workflowID string, success bool, output map[string]any, errorMsg string) {
	result := types.TaskResult{
		TaskID:      taskID,
		WorkflowID:  workflowID,
		Success:     success,
		Output:      output,
		Error:       errorMsg,
		CompletedAt: time.Now(),
	}

	data, err := result.ToJSON()
	if err != nil {
		log.Printf("[WORKER] Failed to serialize task result: %v", err)
		return
	}

	_, err = w.nc.JetStream().Publish(w.ctx, engine.ResultQueueSubject, data)
	if err != nil {
		log.Printf("[WORKER] Failed to publish task result: %v", err)
		return
	}
}
