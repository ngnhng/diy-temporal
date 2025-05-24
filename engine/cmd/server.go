package main

import (
	"app/api"
	"app/engine"
	"app/nats"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Default configuration values
const (
	DefaultNatsURL            = "nats://localhost:4222"
	DefaultClusterID          = "test-cluster"
	DefaultMaxConcurrentExec  = 100
	DefaultStateCheckInterval = 5 * time.Second
	DefaultTaskTimeout        = 30 * time.Second
	DefaultTaskQueue          = "default"
)

func main() {
	// Get NATS URL from environment or use default
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = DefaultNatsURL
	}

	// Create context that will be canceled on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize NATS client for engine
	log.Printf("Connecting to NATS at %s for engine...", natsURL)
	natsClient, err := nats.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to NATS for engine: %v", err)
	}
	defer natsClient.Close()

	// Create engine configuration
	engineConfig := engine.Config{
		NatsURL:                 natsURL,
		ClusterID:               DefaultClusterID,
		MaxConcurrentExecutions: DefaultMaxConcurrentExec,
		StateCheckInterval:      DefaultStateCheckInterval,
		TaskTimeout:             DefaultTaskTimeout,
	}

	// Initialize workflow engine
	log.Println("Initializing workflow engine...")
	workflowEngine := engine.NewEngine(engineConfig)

	// Register some default task queues
	workflowEngine.RegisterTaskQueue(DefaultTaskQueue)
	workflowEngine.RegisterTaskQueue("high-priority")

	// Create error channel to catch errors from goroutines
	errChan := make(chan error, 1)

	// Start the engine and API in a goroutine
	go func() {
		// Start the engine
		if err := workflowEngine.Start(ctx); err != nil {
			errChan <- err
			return
		}
		log.Println(" Workflow Engine started successfully")

	}()

	// TODO: migrate to gRPC

	go func() {
		// Initialize and start the API
		log.Println("Initializing workflow API...")
		workflowAPI := api.NewWorkflowAPI(natsClient, workflowEngine)

		if err := workflowAPI.Start(); err != nil {
			errChan <- err
			return
		}
	}()

	// Set up channel to listen for termination signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal or error
	select {
	case err := <-errChan:
		log.Fatalf("Error during execution: %v", err)
	case sig := <-signalChan:
		log.Printf("Received signal %v, initiating graceful shutdown", sig)
		// Give running tasks some time to complete
		gracePeriod := 5 * time.Second
		log.Printf("Allowing %v grace period for tasks to complete", gracePeriod)
		time.Sleep(gracePeriod)
	}

	// Trigger context cancellation to notify all components to shut down
	cancel()

	log.Println(" Workflow Engine shutdown complete")
}
