# Helm Chart Design

## Goal

Add a hand-maintained Helm chart at `charts/mackerel-operator` so users can install the operator without applying the raw Kubebuilder Kustomize manifests directly.

## Approach

The chart will follow standard Helm layout and keep CRDs in `charts/mackerel-operator/crds/` so Helm installs them before templated resources. Runtime configuration will be exposed through `values.yaml`: image, replica count, operator policy, owner ID, hash length, Mackerel API key Secret reference, security contexts, resources, and service account settings.

## Scope

The chart includes the MVP runtime resources needed to run the controller:

- ServiceAccount
- ClusterRole and ClusterRoleBinding for `ExternalMonitor` reconciliation and leader election leases
- Deployment with the existing manager container arguments
- CRD copied from `config/crd/bases/mackerel.starry.blue_externalmonitors.yaml`

The chart does not include optional metrics auth proxy, Prometheus `ServiceMonitor`, or network policy resources. Those can be added later behind values flags when the operator distribution story needs them.

## Testing

Validation will use:

- `helm lint charts/mackerel-operator`
- `helm template mackerel-operator charts/mackerel-operator`
- `go test ./...`

If `helm` is unavailable in the local toolchain, the implementation will use `mise search helm` and add Helm to `mise.toml` only if needed.
