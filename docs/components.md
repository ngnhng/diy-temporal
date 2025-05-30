## 1. User / API Client

- **CLI / API Client**

  - Entry point for users or upstream services.
  - Exposes HTTP or gRPC endpoints to:

    1. **Register** new workflow definitions.
    2. **Start** new workflow instances.

  - Marshals requests into the engine’s API layer.

---

## 2. Workflow Engine (Go)

### 2.1 API Layer (`EngineAPI`)

- **Responsibilities:**

  - **Register** workflow definitions into a durable store (NATS KV).
  - **Start** workflow instances by validating input and invoking the orchestrator.

- **Interactions:**

  - Reads/writes definitions to `KVDefs`.
  - Receives HTTP/gRPC from the CLI/API client.
  - Triggers the orchestrator to kick off instance execution.

### 2.2 Orchestration Logic (`Orchestrator`)

- **Responsibilities:**

  - Implements the core workflow semantics (task ordering, branching, retries, timeouts).
  - Drives the state machine forward: when one task completes, decide the next step.

- **Interactions:**

  - **State Manager**: asks for current state and notifies of transitions.
  - **Task Dispatcher**: sends out new tasks when needed.
  - **Result Ingestor**: consumes task results to feed back into orchestration.

### 2.3 State Manager (`StateMgr`)

- **Responsibilities:**

  - Loads and persists the per-instance state (e.g. “Task A pending,” “Task B completed”).
  - Records every state transition into an event log to enable replay/recovery.

- **Interactions:**

  - CRUD operations against `KVInstances` for the latest snapshot.
  - Appends every change as an event to `StreamEvents`.

### 2.4 Task Dispatcher (`TaskDispatcher`)

- **Responsibilities:**

  - Converts “ready” activities into concrete task messages.
  - Honors task-level metadata (timeouts, retry policies).

- **Interactions:**

  - Publishes to per-activity JetStream subjects in `StreamTasks`.
  - Ensures each task is durably enqueued for at-least-once delivery.

### 2.5 Result Ingestor (`ResultIngestor`)

- **Responsibilities:**

  - Listens for task completions or failures.
  - Delivers results back into the orchestrator loop.

- **Interactions:**

  - Subscribes to `StreamResults`.
  - Forwards each result to the orchestrator, which will then update state and potentially dispatch more tasks.

---

## 3. NATS JetStream Cluster

### 3.1 KV Store: Workflow Definitions (`KVDefs`)

- Holds canonical workflow definitions (DSL, JSON/YAML, or compiled Go metadata).
- EngineAPI writes here; orchestrator reads here on instance start (to know what tasks to schedule).

### 3.2 KV Store: Workflow Instance States (`KVInstances`)

- Key–value snapshots of each running instance’s current state.
- Enables fast lookups / restarts without replaying the full event log.

### 3.3 Stream: Workflow Events History (`StreamEvents`)

- Append-only log of every state transition and important lifecycle event.
- Used for replay, auditing, or building projections (e.g. dashboards).

### 3.4 Stream: Task Queues per Activity Type (`StreamTasks`)

- One subject (stream) per activity type or queue.
- Workers subscribe to the subjects matching the activities they can perform.
- JetStream ensures durability, ordering, and back-pressure control.

### 3.5 Stream: Task Results (`StreamResults`)

- Central sink for all activity outcomes (success or error).
- Result Ingestor subscribes here to pick up completions/failures.

> **Styling Note:** All KV and Stream resources are highlighted with the `nats` style in the diagram to emphasize their implementation in JetStream.

---

## 4. Workers (Go Clients)

- **Worker1, Worker2, … WorkerN** each host one or more _activity implementations_.
- **Responsibilities:**

  1. **Subscribe** to the relevant `StreamTasks` subject(s).
  2. **Deserialize** the task payload (which includes workflow-ID, task parameters, metadata).
  3. **Execute** the business logic.
  4. **Acknowledge** the JetStream message only upon successful completion.
  5. **Publish** the result (or error) into `StreamResults`.

- **Scalability:**

  - Multiple workers can process the same activity type, enabling horizontal scaling.
  - The engine’s retry and ordering policies, combined with NATS’s durable delivery, ensure correctness under failures.

---

## End-to-End Flow

1. **Define & Register**: User sends a definition via `EngineAPI` → stored in `KVDefs`.
2. **Start Instance**: User hits “start” → `Orchestrator` creates a new instance, writes initial state to `KVInstances` and an event to `StreamEvents`.
3. **Dispatch First Task**: `Orchestrator` asks `TaskDispatcher` to publish activity A to `StreamTasks`.
4. **Worker Executes**: A worker picks up the task, runs the logic, then writes success/failure to `StreamResults`.
5. **Result Ingestion**: `ResultIngestor` delivers the outcome to `Orchestrator`, which updates `StateMgr` and `StreamEvents`.
6. **Next Steps**: Based on updated state, more tasks are dispatched or the workflow is marked complete/failed.
