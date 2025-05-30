# Temporal-Style Workflow Engine with NATS

## Goals

- Implement POC of a lightweight, Temporal-style workflow orchestration engine built only using Go and NATS JetStream.

## Features

- **Durable Workflow Execution**: Workflows survive system restarts and failures
- **Task-based Architecture**: Clean separation between workflow orchestration and business logic
- **Fault Tolerance**: Automatic retries with configurable policies
- **Dependency Management**: Tasks execute in proper order based on dependencies
- **Horizontal Scaling**: Multiple workers can process tasks concurrently
- **Pure NATS Storage**: Uses only NATS JetStream for persistence and messaging

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Workflow       │    │  NATS JetStream  │    │  Worker         │
│  Engine         │◄──►│                  │◄──►│  (Activities)   │
│                 │    │  • Task Queue    │    │                 │
│  • State Mgmt   │    │  • Result Queue  │    │  • Execute      │
│  • Orchestration│    │  • State Store   │    │  • Report Back  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Quick Start

### Prerequisites

1. **NATS Server with JetStream**:

   ```bash
   # Using Docker
   docker run -p 4222:4222 -p 8222:8222 nats:latest -js

   # Or install locally
   # Download from https://github.com/nats-io/nats-server/releases
   nats-server -js
   ```

### Running the Example

The order processing example demonstrates a complete e-commerce workflow:
`validate_order` → `charge_payment` → `ship_order` → `send_notification`

#### Terminal 1: Start the Engine

```bash
cd examples/order_processing
go run main.go engine
```

#### Terminal 2: Start a Worker

```bash
cd examples/order_processing
WORKER_ID=worker-1 go run main.go worker
```

#### Terminal 3: Start Another Worker (Optional)

```bash
cd examples/order_processing
WORKER_ID=worker-2 go run main.go worker
```

#### Terminal 4: Run Test Client

```bash
cd examples/order_processing
go run main.go client
```

## Example Output

## Project Structure

```
|-- engine
|   `-- engine.go
|-- examples
|   `-- order_processing
|       |-- activities
|       |   `-- activities.go
|       `-- main.go
|-- nats
|   `-- nats.go
|-- sdk
|   |-- client
|   |   `-- client.go
|   `-- worker
|       `-- worker.go
`-- types
    `-- types.go
```

## Configuration

### Environment Variables

- `NATS_URL`: NATS server URL (default: `nats://localhost:4222`)
- `WORKER_ID`: Unique identifier for worker instances

### Workflow Definition

```go
workflow := &types.WorkflowDefinition{
    Name:        "my_workflow",
    Version:     "1.0.0",
    Description: "My custom workflow",
    Timeout:     10 * time.Minute,
    Activities: []types.ActivityDefinition{
        {
            Name:    "step1",
            Type:    "my_activity_type",
            Timeout: 30 * time.Second,
            RetryPolicy: types.RetryPolicy{
                MaxRetries:      3,
                InitialInterval: 1 * time.Second,
                BackoffFactor:   2.0,
                MaxInterval:     30 * time.Second,
            },
            Dependencies: []string{}, // No dependencies
        },
        // Add more activities...
    },
}
```

## Creating Custom Activities

1. **Implement the ActivityHandler interface**:

```go
type MyActivity struct{}

func (a *MyActivity) GetActivityType() string {
    return "my_activity_type"
}

func (a *MyActivity) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
    // Your business logic here
    return map[string]any{
        "result": "success",
    }, nil
}
```

2. **Register with worker**:

```go
worker.RegisterActivityHandler(&MyActivity{})
```

## Core problem & Constraints

- What is a "Temporal Workflow"?

  > It's a way to write durable, stateful, long-running business logic.

- Key features:

  - Workflows (orchestration logic), Activities (units of work), retries, persistence, visibility.

  > The orchestration logic is code, not just a static definition.

- The "Simple" Constraint:
  - Don't try to replicate all of Temporal. Focus on the core: sequential activities, basic retries, durable execution.
  - Avoid overly complex features like child workflows, signals, complex queries initially.
- The "Only NATS" Constraint:

  - Durable Execution: How do we make sure the workflow "continues" after a crash? State must be stored in NATS.
  - Queueing: How are activity tasks assigned to workers? NATS must act as the queue.
  - Messaging: How do components communicate (start workflow, activity results)? NATS pub/sub.
  - Data Store: Where is workflow state, inputs, outputs stored? NATS JetStream.

## Mapping Workflow Concepts to NATS Primitives

- Workflow Instance State:

  - Need: Store current step, inputs, intermediate results, status.

  - NATS Primitive: NATS Key-Value (KV) Store. It's perfect for storing a serialized state object per workflow instance. Key = workflow_instance_id.

- Activity Task Queueing:

  - Need: A queue for each type of activity, where workers can pick up tasks.

  - NATS Primitive: NATS JetStream Streams with WorkQueuePolicy.

  - Details: A stream named ACTIVITY_TASKS. Subjects within this stream like activity.tasks.{activity_type}. Workers subscribe to specific subjects.

- Durability: JetStream streams are durable. Messages are ACKed upon successful processing.
- Orchestration Logic & Eventing:

  - Need: The "orchestrator" needs to react to events (workflow start, activity completion) and make decisions.
  - NATS Primitive: NATS JetStream Streams for events.
  - Details: A stream named WORKFLOW_EVENTS. Subjects like:

    ```
    workflow.start.{workflow_type} (to trigger a new workflow)
    workflow.activity.completed.{instance_id}.{activity_name} (for workers to report back)
    ```

    The orchestrator will be a durable consumer on this stream.

  - Durability: Orchestrator can crash and resume processing events from where it left off.

- Messaging (Pub/Sub):
  - Core NATS pub/sub, but backed by JetStream for the critical paths (events, tasks) to ensure messages aren't lost.

## Go Components

- Workflow Definition:

  > How does the system know what activities to run and in what order?

  - A Go struct: WorkflowDefinition containing a list of ActivityDefinitions (name, retry params).

- Orchestrator:

  - Input: Messages from WORKFLOW_EVENTS stream.

  - Logic:
    - On workflow.start: Create WorkflowState in KV, dispatch first activity task to ACTIVITY_TASKS.
  - On workflow.activity.completed:
    - Load WorkflowState from KV.
    - If success: Store result, find next activity, dispatch task. Update state in KV.
    - If failure: Check retry policy.
    - If retries left: Increment attempt count in WorkflowState, re-dispatch task. Update state in KV.
    - If no retries: Mark workflow failed in WorkflowState.
    - If last activity: Mark workflow completed in WorkflowState.
  - Output: Publishes messages to ACTIVITY_TASKS stream. Updates WORKFLOW_STATES KV.
  - Key Challenge: Orchestrator logic must be idempotent. If it crashes and an event is redelivered, it shouldn't mess things up (e.g., dispatch the same activity task twice if the first one was already successfully dispatched and state updated).

- Activity Worker:

  - Input: Messages from ACTIVITY_TASKS stream (subscribes to activity.tasks.{activity_it_handles}).
  - Logic:
    - Deserialize task message.
    - Execute the actual Go function corresponding to the activity name.
    - Serialize result (output or error).
  - Output: Publishes ActivityResult message to WORKFLOW_EVENTS (subject: workflow.activity.completed...).
  - Key Challenge: Activity functions should ideally be idempotent. If a worker processes a task, publishes a result, but crashes before ACKing the task, the task will be redelivered.

- Client (Initiator):
  - Publishes a message to workflow.start.{workflow_type} with initial input.
