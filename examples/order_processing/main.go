package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ngnhng/diy-temporal/engine"
	"github.com/ngnhng/diy-temporal/examples/order_processing/activities"
	"github.com/ngnhng/diy-temporal/sdk/worker"
	"github.com/ngnhng/diy-temporal/types"
)

const (
	defaultNatsURL           = "nats://localhost:4222"
	defaultWorkerConcurrency = 3
	clientWaitTimeout        = 30 * time.Second
	monitorTickInterval      = 2 * time.Second
	maxMonitorIterations     = 15
	orderDelayBetween        = 2 * time.Second
)

type ServingFn func(ctx context.Context) error

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [engine|worker|client]")
		os.Exit(1)
	}

	mode := os.Args[1]
	natsURL := getEnvOrDefault("NATS_URL", defaultNatsURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var serveFn ServingFn
	switch mode {
	case "engine":
		serveFn = createEngineServeFn(natsURL)
	case "worker":
		serveFn = createWorkerServeFn(natsURL)
	case "client":
		if err := runClient(ctx, natsURL); err != nil {
			log.Fatalf("Client failed: %v", err)
		}
		return
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
		os.Exit(1)
	}

	if err := runWithGracefulShutdown(ctx, cancel, serveFn); err != nil {
		log.Fatalf("Service failed: %v", err)
	}
}

func createEngineServeFn(natsURL string) ServingFn {
	return func(ctx context.Context) error {
		log.Println("Starting Workflow Engine...")

		engine, err := engine.NewWorkflowEngine(ctx, natsURL)
		if err != nil {
			return fmt.Errorf("failed to create engine: %w", err)
		}
		defer func() {
			if err := engine.Close(); err != nil {
				log.Printf("Error closing engine: %v", err)
			}
		}()

		if err := registerOrderProcessingWorkflow(engine); err != nil {
			return fmt.Errorf("failed to register workflow: %w", err)
		}

		log.Println("Workflow Engine started successfully!")
		log.Println("Registered workflows:")
		log.Println("  - order_processing: A complete order fulfillment workflow")

		<-ctx.Done()
		log.Println("Shutting down workflow engine...")
		return nil
	}
}

func createWorkerServeFn(natsURL string) ServingFn {
	return func(ctx context.Context) error {
		workerID := getEnvOrDefault("WORKER_ID", fmt.Sprintf("worker-%d", time.Now().Unix()))
		log.Printf("Starting Worker: %s", workerID)

		worker, err := worker.NewWorker(ctx, worker.WorkerConfig{
			NatsURL:     natsURL,
			WorkerID:    workerID,
			Concurrency: defaultWorkerConcurrency,
		})
		if err != nil {
			return fmt.Errorf("failed to create worker: %w", err)
		}
		defer worker.Stop()

		registerActivityHandlers(worker)
		logRegisteredActivities(workerID)

		var wg sync.WaitGroup
		errChan := make(chan error, 1)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := worker.Start(); err != nil {
				select {
				case errChan <- fmt.Errorf("worker error: %w", err):
				default:
				}
			}
		}()

		select {
		case <-ctx.Done():
			log.Printf("Shutting down worker %s...", workerID)
		case err := <-errChan:
			return err
		}

		wg.Wait()
		return nil
	}
}

func registerActivityHandlers(w *worker.Worker) {
	w.RegisterActivityHandler(&activities.ValidateOrderActivity{})
	w.RegisterActivityHandler(&activities.ChargePaymentActivity{})
	w.RegisterActivityHandler(&activities.ShipOrderActivity{})
	w.RegisterActivityHandler(&activities.SendNotificationActivity{})
}

func logRegisteredActivities(workerID string) {
	log.Printf("Worker %s registered activity handlers:", workerID)
	log.Println("  - validate_order")
	log.Println("  - charge_payment")
	log.Println("  - ship_order")
	log.Println("  - send_notification")
}

func runWithGracefulShutdown(ctx context.Context, cancel context.CancelFunc, fn ServingFn) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)

	go func() {
		errChan <- fn(ctx)
	}()

	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, initiating shutdown...", sig)
		cancel()

		// Wait for graceful shutdown with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		select {
		case err := <-errChan:
			return err
		case <-shutdownCtx.Done():
			return fmt.Errorf("shutdown timeout exceeded")
		}
	case err := <-errChan:
		return err
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func runClient(ctx context.Context, natsURL string) error {
	log.Println("Starting test client...")

	engine, err := engine.NewWorkflowEngine(ctx, natsURL)
	if err != nil {
		return fmt.Errorf("failed to create engine client: %w", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			log.Printf("Error closing engine client: %v", err)
		}
	}()

	if err := registerOrderProcessingWorkflow(engine); err != nil {
		log.Printf("Warning: Failed to register workflow (might already exist): %v", err)
	}
	log.Println("Workflow Engine started successfully!")
	log.Println("Registered workflows:")
	log.Println("  - order_processing: A complete order fulfillment workflow")

	orders := generateSampleOrders(3)

	for i, order := range orders {
		log.Printf("\n--- Processing Order %d ---", i+1)

		workflow, err := engine.StartWorkflow(ctx, "order_processing", map[string]any{
			"order":          order,
			"payment_method": "credit_card",
		})
		if err != nil {
			log.Printf("Failed to start workflow: %v", err)
			continue
		}

		log.Printf("[MONITOR] Started workflow %s for order %s", workflow.ID, order["id"])

		go monitorWorkflow(ctx, engine, workflow.ID)

		time.Sleep(orderDelayBetween)
	}

	log.Println("\nWaiting for workflows to complete...")

	waitCtx, waitCancel := context.WithTimeout(ctx, clientWaitTimeout)
	defer waitCancel()

	<-waitCtx.Done()
	if waitCtx.Err() == context.DeadlineExceeded {
		log.Println("Wait timeout reached, checking final status...")
	}

	return printFinalWorkflowStatus(ctx, engine)
}

func printFinalWorkflowStatus(ctx context.Context, engine *engine.WorkflowEngine) error {
	workflows, err := engine.ListWorkflows()
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	log.Println("\nFinal workflow statuses:")
	for _, wf := range workflows {
		log.Printf("⏱️  Workflow %s (%s): %s", wf.ID[:8], wf.WorkflowName, wf.State)
		if wf.Error != "" {
			log.Printf("    Error: %s", wf.Error)
		}
	}
	return nil
}

// TODO: registering workflows must be initiated from client
func registerOrderProcessingWorkflow(engine *engine.WorkflowEngine) error {
	// Define the order processing workflow
	workflow := &types.WorkflowDefinition{
		Name:        "order_processing",
		Version:     "1.0.0",
		Description: "Complete order fulfillment workflow: validate -> charge -> ship -> notify",
		Timeout:     10 * time.Minute,
		Activities: []types.ActivityDefinition{
			{
				Name:    "validate_order",
				Type:    "validate_order",
				Input:   map[string]any{},
				Timeout: 30 * time.Second,
				RetryPolicy: types.RetryPolicy{
					MaxRetries:      2,
					InitialInterval: 1 * time.Second,
					BackoffFactor:   2.0,
					MaxInterval:     10 * time.Second,
				},
				Dependencies: []string{}, // No dependencies - first activity
			},
			{
				Name:    "charge_payment",
				Type:    "charge_payment",
				Input:   map[string]any{},
				Timeout: 45 * time.Second,
				RetryPolicy: types.RetryPolicy{
					MaxRetries:      3,
					InitialInterval: 2 * time.Second,
					BackoffFactor:   2.0,
					MaxInterval:     30 * time.Second,
				},
				Dependencies: []string{"validate_order"},
			},
			{
				Name:    "ship_order",
				Type:    "ship_order",
				Input:   map[string]any{},
				Timeout: 60 * time.Second,
				RetryPolicy: types.RetryPolicy{
					MaxRetries:      2,
					InitialInterval: 5 * time.Second,
					BackoffFactor:   1.5,
					MaxInterval:     15 * time.Second,
				},
				Dependencies: []string{"charge_payment"},
			},
			{
				Name: "send_notification",
				Type: "send_notification",
				Input: map[string]any{
					"notification_type": "order_shipped",
				},
				Timeout: 30 * time.Second,
				RetryPolicy: types.RetryPolicy{
					MaxRetries:      1,
					InitialInterval: 1 * time.Second,
					BackoffFactor:   1.0,
					MaxInterval:     5 * time.Second,
				},
				Dependencies: []string{"ship_order"},
			},
		},
	}

	return engine.RegisterWorkflowDefinition(workflow)
}

func generateSampleOrders(count int) []map[string]any {
	orders := make([]map[string]any, count)

	products := []map[string]any{
		{"id": "prod-1", "name": "Laptop", "price": 999.99},
		{"id": "prod-2", "name": "Mouse", "price": 29.99},
		{"id": "prod-3", "name": "Keyboard", "price": 79.99},
		{"id": "prod-4", "name": "Monitor", "price": 299.99},
		{"id": "prod-5", "name": "Headphones", "price": 149.99},
	}

	addresses := []map[string]any{
		{
			"street":   "123 Main St",
			"city":     "New York",
			"state":    "NY",
			"zip_code": "10001",
			"country":  "USA",
		},
		{
			"street":   "456 Oak Ave",
			"city":     "Los Angeles",
			"state":    "CA",
			"zip_code": "90210",
			"country":  "USA",
		},
		{
			"street":   "789 Pine Rd",
			"city":     "Chicago",
			"state":    "IL",
			"zip_code": "60601",
			"country":  "USA",
		},
	}

	for i := range count {
		// Generate random items
		numItems := rand.Intn(3) + 1 // 1-3 items
		items := make([]map[string]any, numItems)

		for j := 0; j < numItems; j++ {
			product := products[rand.Intn(len(products))]
			quantity := rand.Intn(3) + 1 // 1-3 quantity

			items[j] = map[string]any{
				"product_id": product["id"],
				"name":       product["name"],
				"price":      product["price"],
				"quantity":   float64(quantity),
			}
		}

		orders[i] = map[string]any{
			"id":      fmt.Sprintf("order-%d-%d", time.Now().Unix(), i+1),
			"user_id": fmt.Sprintf("user-%d", rand.Intn(1000)+1),
			"items":   items,
			"address": addresses[i%len(addresses)],
			"status":  "pending",
		}
	}

	return orders
}

func monitorWorkflow(ctx context.Context, engine *engine.WorkflowEngine, workflowID string) {
	ticker := time.NewTicker(monitorTickInterval)
	defer ticker.Stop()

	for range maxMonitorIterations {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			workflow, err := engine.GetWorkflow(workflowID)
			if err != nil {
				log.Printf("[MONITOR] Failed to get workflow %s: %v", workflowID[:8], err)
				return
			}

			log.Printf("[MONITOR] Workflow %s status: %s", workflowID[:8], workflow.State)

			for _, task := range workflow.Tasks {
				log.Printf("[MONITOR]  Task %s (%s): %s", task.ActivityName, task.ActivityType, task.State)
				if task.Error != "" {
					log.Printf("[MONITOR]    Error: %s", task.Error)
				}
			}

			if workflow.State == types.WorkflowStateCompleted || workflow.State == types.WorkflowStateFailed {
				logWorkflowCompletion(workflow, workflowID)
				return
			}
		}
	}
}

func logWorkflowCompletion(workflow *types.WorkflowInstance, workflowID string) {
	if workflow.State == types.WorkflowStateCompleted {
		log.Printf("✅ Workflow %s completed successfully!", workflowID[:8])
		if len(workflow.Output) > 0 {
			if output, err := json.MarshalIndent(workflow.Output, "    ", "  "); err == nil {
				log.Printf("    Final output: %s", string(output))
			}
		}
	} else {
		log.Printf("❌ Workflow %s failed: %s", workflowID[:8], workflow.Error)
	}
}
