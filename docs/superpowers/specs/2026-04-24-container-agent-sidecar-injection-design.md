# Container Agent Sidecar Injection Design

## Goal

Add a future extension to `mackerel-operator` that injects `mackerel-container-agent` as a sidecar into opted-in workloads.

The first implementation should:

- use workload annotations as the public entry point
- support `Deployment`, `StatefulSet`, and `DaemonSet`
- inject only the minimum configuration required to send basic metrics
- preserve human edits outside operator-managed fields

The design should also keep a clear migration path toward a dedicated CRD in the future.

## Non-Goals

- mutating admission webhook in the first version
- support for `Job` and `CronJob` in the first version
- full container-agent configuration surface in the first version
- automatic discovery of workloads without explicit opt-in

## Recommended Approach

Use workload annotations as the first public API, but normalize them into an internal model before rendering any pod template changes.

This gives the project a lightweight initial UX while preserving a clean path to a future CRD-based API.

## Alternatives Considered

### 1. Annotation-Based Injection

Watch workloads directly and reconcile pod templates based on opt-in annotations.

Pros:

- lowest operational complexity
- easy adoption for users
- no extra CRD required

Cons:

- annotation keys can become noisy as options grow
- validation is weaker than a CRD

### 2. Dedicated CRD

Introduce a new resource such as `ContainerAgentInjection` that references a target workload.

Pros:

- strong validation surface
- better long-term extensibility
- clearer ownership of injection policy

Cons:

- heavier initial UX
- more code and docs needed for the first release

### 3. Mutating Admission Webhook

Inject the sidecar at admission time.

Pros:

- Kubernetes-native injection flow
- no need to patch existing workloads repeatedly

Cons:

- much higher operational complexity
- requires certificate and webhook lifecycle management
- unnecessary for the initial scope

## Decision

Start with alternative 1 and design internals so that alternative 2 can be added later without rewriting reconcile logic.

## Scope

### Supported workload kinds

Initial implementation targets:

- `Deployment`
- `StatefulSet`
- `DaemonSet`

The API shape should remain compatible with future expansion to other pod-template workloads.

### Public API

Initial opt-in is controlled by pod template annotations.

Required initial annotation:

- `mackerel.starry.blue/inject-container-agent: "true"`

Initial optional annotations:

- `mackerel.starry.blue/container-agent-api-key-secret-name`
- `mackerel.starry.blue/container-agent-api-key-secret-key`

Reserved for future expansion:

- `mackerel.starry.blue/container-agent-image`
- `mackerel.starry.blue/container-agent-config-secret-name`

Operator defaults should be used when optional annotations are omitted.

## Internal Architecture

To preserve future extensibility, split the implementation into the following layers.

### Source

Reads user intent from Kubernetes objects.

Initial source:

- workload pod template annotations

Future source:

- dedicated CRD such as `ContainerAgentInjection`

### Resolver

Normalizes source input into a stable internal model.

Example responsibilities:

- determine whether injection is enabled
- resolve default image and secret references
- validate required inputs

### Renderer

Builds the operator-managed portion of the target `PodTemplateSpec`.

Example responsibilities:

- render the sidecar container
- render env references
- render operator-managed volumes and mounts
- render any operator-owned metadata markers

### Applier

Applies the rendered state to a specific workload kind.

Example responsibilities:

- read and write `Deployment`, `StatefulSet`, and `DaemonSet` pod templates
- patch only managed fields
- avoid touching unrelated user-managed fields

## Internal Model

Introduce an internal model that does not depend on annotations directly.

Example shape:

- `TargetRef`
  - kind
  - namespace
  - name
- `ContainerAgentConfig`
  - enabled
  - image
  - apiKeySecretRef
  - optionalConfigSecretRef
- `Ownership`
  - managed container names
  - managed volume names
  - managed annotation keys

This model is the compatibility layer between the initial annotation API and a future CRD API.

## Reconcile Semantics

The operator should not treat the entire workload as owned.
It should manage only the fields required for container-agent injection.

Managed resources should use stable names.

Initial managed names:

- container: `mackerel-container-agent`
- volumes: reserved operator-managed names defined by the renderer
- annotations: `mackerel.starry.blue/container-agent-*`

### When injection is enabled

- ensure the sidecar container exists
- ensure required env references exist
- ensure required volumes and mounts exist
- update only operator-managed fragments

### When injection is disabled or opt-in annotation is removed

- remove only operator-managed fragments
- keep all unrelated user-managed containers, volumes, env, and annotations intact

### Conflict handling

If a container with the managed name already exists, treat it as the operator-managed container and reconcile required fields onto it.

If required inputs cannot be resolved, the operator should surface a clear failure through conditions or events instead of partially mutating the workload.

## Human Edit Safety

Human edits must be preserved outside explicitly managed fields.

The reconciler must not overwrite:

- unrelated containers
- unrelated env vars
- unrelated volumes
- unrelated annotations

This follows the same design principle already used in `ExternalMonitor`: short ownership boundaries and targeted reconciliation instead of broad replacement.

## Initial Injected Configuration

The first version should inject only what is required for basic metric delivery.

Initial requirements:

- one `mackerel-container-agent` sidecar
- API key from a referenced Secret
- minimum required env configuration
- minimum required volume and mount configuration if needed by the image

Detailed config injection from a user-provided Secret is deferred to a later phase.

## Testing Strategy

### Unit tests

- annotation to internal model resolution
- internal model to rendered pod template fragments
- application to each supported workload kind
- removal of managed fragments when opt-in is removed
- preservation of user-managed fields

### Integration tests

- workload with opt-in annotation receives sidecar injection
- workload without opt-in annotation is not mutated
- annotation removal removes only operator-managed fragments
- invalid secret reference produces a failure signal without corrupting the workload

### End-to-end tests

- sample workloads in a real cluster receive the sidecar
- rollout remains valid after injection

## Phased Implementation

### Phase 1

- watch `Deployment`, `StatefulSet`, and `DaemonSet`
- implement annotation source
- implement resolver, renderer, and applier boundaries
- inject sidecar with API key Secret reference
- surface failures through conditions or events

### Phase 2

- allow image override
- allow additional config Secret
- improve status reporting

### Phase 3

- add a dedicated CRD as an additional source
- define precedence rules between annotation and CRD input
- keep renderer and applier reusable

## Open Design Choices Deferred Intentionally

The following items should remain out of the first implementation plan:

- exact CRD schema for the future injection API
- exact config file format passed to `mackerel-container-agent`
- support policy for short-lived workloads such as `Job` and `CronJob`
- admission webhook adoption

## Summary

The first implementation should provide annotation-based sidecar injection for `Deployment`, `StatefulSet`, and `DaemonSet`, while internally separating source resolution, rendering, and application.

This keeps the first release small and practical, and it gives the project a clean path to a future CRD-driven injection API.
