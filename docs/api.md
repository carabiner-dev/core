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
| labels | map<string, string> | The namespace's own labels. Its repositories inherit them. | See [Labels](#labels), max 64 |
| effective_labels | map<string, string> | The labels selectors match; equal to `labels`. Output only. | |

### Repository

A Repository represents a code repository within a Namespace.

| Field     | Type      | Description                                      | Validation                                    |
|-----------|-----------|--------------------------------------------------|-----------------------------------------------|
| ID        | string    | Unique identifier for the repository.            | UUID                                          |
| name      | string    | Name of the repository.                          | Pattern `^[-_a-zA-Z0-9]$`, max 200 characters |
| namespace | Namespace | The namespace this repository belongs to.        |                                               |
| labels    | map<string, string> | The repository's own labels.           | See [Labels](#labels), max 64                 |
| effective_labels | map<string, string> | The labels selectors match: its own with its namespace's laid over them. Output only. | |
| inherited_label_keys | repeated string | The keys of `effective_labels` that come from the namespace, sorted. Output only. | |

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

## Labels

Labels are key/value pairs on resources that rules select resources by. They
are the platform's own, unrelated to labels in a source system. They are
defined in [`labels.proto`](../proto/carabiner/core/v1/labels.proto), with the
fields that carry them on the objects above.

### Syntax

- A **key** is an optional prefix and a name: `tier`, `example.com/team`. The
  name is 1 to 63 characters of `[A-Za-z0-9_.-]`, starting and ending with a
  letter or digit. The prefix is a lowercase DNS subdomain of at most 253
  characters. Keys are case-sensitive.
- A **value** is empty, which makes the label a tag, or follows the rules of
  a name.
- A resource carries at most 64 labels of its own.
- The `carabiner.dev/` prefix and its subdomains are **reserved** for labels
  the platform sets itself. Users can select on them but not set them.

### Inheritance

A repository inherits its namespace's labels. Its **effective** labels are its
own with the namespace's laid over them, and on a key both set the namespace
wins: a key set on a namespace is locked on its repositories, and a
repository's own value under such a key is shadowed.

### LabelSelector

A LabelSelector selects resources by their effective labels. All of its
requirements must hold. A selector with no requirements is invalid and
matches nothing.

| Field             | Type                      | Description                                                    |
|-------------------|---------------------------|----------------------------------------------------------------|
| match_labels      | map<string, string>       | Each key is set with exactly this value (an empty value matches a tag). |
| match_expressions | repeated LabelRequirement | Further requirements.                                          |

A LabelRequirement has a `key`, an `operator` and `values`:

| Operator                        | Holds when                                   |
|---------------------------------|----------------------------------------------|
| `LABEL_OPERATOR_IN`             | The key is set and its value is one of `values`. |
| `LABEL_OPERATOR_NOT_IN`         | The key is not set, or its value is none of `values`. |
| `LABEL_OPERATOR_EXISTS`         | The key is set, with any value.              |
| `LABEL_OPERATOR_DOES_NOT_EXIST` | The key is not set.                          |

### Go helpers

The Go package is the one implementation of the syntax and of matching, so
every service agrees:

```go
err := core.ValidateLabels(labels)          // syntax and the limit
reserved := core.IsReservedLabelKey(key)    // carabiner.dev/… keys
effective := core.EffectiveLabels(own, inherited)
shadowed := core.ShadowedLabelKeys(own, inherited)

err = selector.Validate()                   // can it be evaluated?
ok := selector.Matches(effective)           // nil and empty match nothing
```

`ValidateLabels` and `Validate` wrap `core.ErrInvalidLabel` and
`core.ErrInvalidSelector`.

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
