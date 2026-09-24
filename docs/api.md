# Carabiner Core API

The Carabiner Core API defines the foundational data model for representing
CI/CD systems, their organizational structure, and execution events. Types are
defined as Protocol Buffers in the `carabiner.core.v1` package.

## Go Import

```go
import core "github.com/carabiner-dev/core/api/carabiner/core/v1"
```

## Objects

Objects represent the static entities that make up a CI/CD system. They are
defined in [`objects.proto`](../proto/carabiner/core/v1/objects.proto).

### System

A System abstracts a platform capable of handling one or more stages of the
SDLC (e.g. a CI/CD system like GitHub Actions or GitLab CI).

| Field | Type   | Description                                      | Validation                          |
|-------|--------|--------------------------------------------------|-------------------------------------|
| ID    | string | Unique identifier for the system.                | UUID                                |
| Name  | string | Human-readable name of the system.               |                                     |
| Type  | string | Type of the system platform.                     | One of: `github`, `gitlab`          |

### Namespace

A Namespace abstracts an organizational unit inside of a System (e.g. a GitHub
organization or GitLab group).

| Field  | Type   | Description                                      | Validation                                    |
|--------|--------|--------------------------------------------------|-----------------------------------------------|
| ID     | string | Unique identifier for the namespace.             | UUID                                          |
| system | System | The system this namespace belongs to.            |                                               |
| name   | string | Name of the namespace.                           | Pattern `^[-_a-zA-Z0-9]$`, max 200 characters |

### Repository

A Repository represents a code repository within a Namespace.

| Field     | Type      | Description                                      | Validation                                    |
|-----------|-----------|--------------------------------------------------|-----------------------------------------------|
| ID        | string    | Unique identifier for the repository.            | UUID                                          |
| name      | string    | Name of the repository.                          | Pattern `^[-_a-zA-Z0-9]$`, max 200 characters |
| namespace | Namespace | The namespace this repository belongs to.        |                                               |

### Pipeline

A Pipeline abstracts a collection of steps within a Repository (e.g. a GitHub
Actions workflow or GitLab CI pipeline).

| Field      | Type       | Description                                      | Validation                                    |
|------------|------------|--------------------------------------------------|-----------------------------------------------|
| ID         | string     | Unique identifier for the pipeline.              | UUID                                          |
| name       | string     | Name of the pipeline.                            | Pattern `^[-_a-zA-Z0-9]$`, max 200 characters |
| repository | Repository | The repository this pipeline belongs to.         |                                               |

### Step

A Step is a single CI/CD operation that is part of a Pipeline (e.g. a job in a
GitHub Actions workflow).

| Field    | Type     | Description                                      | Validation                                    |
|----------|----------|--------------------------------------------------|-----------------------------------------------|
| ID       | string   | Unique identifier for the step.                  | UUID                                          |
| name     | string   | Name of the step.                                | Pattern `^[-_a-zA-Z0-9]$`, max 200 characters |
| pipeline | Pipeline | The pipeline this step belongs to.               |                                               |

### Object Hierarchy

The objects form a hierarchy that models a CI/CD environment:

```
System
  └── Namespace
        └── Repository
              └── Pipeline
                    └── Step
```

## Events

Events capture execution data from CI/CD runs. They are defined in
[`events.proto`](../proto/carabiner/core/v1/events.proto).

### EventType

An enum describing the lifecycle state of an execution event.

| Value          | Number | Name       | Description                              |
|----------------|--------|------------|------------------------------------------|
| ETYPE_UNKONWN  | 0      |            | Default/unknown event type.              |
| ETYPE_STARTED  | 1      | started    | The execution has started.               |
| ETYPE_FINISHED | 2      | finished   | The execution has finished.              |
| ETYPE_QUEUED   | 3      | queued     | The execution has been queued.           |

### PipelineRun

Captures the data of a single pipeline execution.

| Field     | Type                       | Description                                      | Validation                                    |
|-----------|----------------------------|--------------------------------------------------|-----------------------------------------------|
| type      | EventType                  | The lifecycle state of the run.                  |                                               |
| pipeline  | Pipeline                   | The pipeline being executed.                     |                                               |
| name      | string                     | Name of the pipeline run.                        | Pattern `^[-_a-zA-Z0-9]$`, max 200 characters |
| timestamp | google.protobuf.Timestamp  | When the event occurred.                         |                                               |
| payload   | google.protobuf.Struct     | Arbitrary additional data about the run.         |                                               |

### StepRun

Captures the data of a single step execution within a pipeline run.

| Field        | Type                       | Description                                      |
|--------------|----------------------------|--------------------------------------------------|
| type         | EventType                  | The lifecycle state of the run.                  |
| step         | Step                       | The step being executed.                         |
| pipeline_run | PipelineRun                | The parent pipeline run.                         |
| timestamp    | google.protobuf.Timestamp  | When the event occurred.                         |
| payload      | google.protobuf.Struct     | Arbitrary additional data about the run.         |

## Interfaces

The Go package also defines two interfaces in
[`interfaces.go`](../api/carabiner/core/v1/interfaces.go) for categorizing
the protobuf types:

- **`Object`** -- implemented by static entity types (`System`, `Namespace`,
  `Repository`, `Pipeline`, `Step`). Requires a `Kind() string` method.
- **`Event`** -- implemented by execution event types (`PipelineRun`,
  `StepRun`). Requires a `Kind() string` method.
