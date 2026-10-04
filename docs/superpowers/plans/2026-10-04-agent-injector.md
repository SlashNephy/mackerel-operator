# Agent Injector Implementation Plan

Spec: `docs/superpowers/specs/2026-10-04-agent-injector-design.md`

## Tasks

1. **Chart values.** Add `agentInjector` (`enabled`, `image.repository`, `image.tag`, `kubeletInsecureTLS`, `resources`, `namespaceSelector`) to `charts/mackerel-operator/values.yaml`. Renovate's helm-values manager picks up `image.repository` + `image.tag`.
2. **Chart template.** Add `templates/agent-injector.yaml`, rendered only when `agentInjector.enabled`:
   - `fail` unless `.Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/MutatingAdmissionPolicy"`.
   - `MutatingAdmissionPolicy` with the verified spike policy, values substituted as CEL string literals via `toJson`.
   - `MutatingAdmissionPolicyBinding` with the `inject` label `objectSelector` and optional `namespaceSelector`.
   - `ClusterRole` granting `get` on `nodes/proxy`, `nodes/stats`, `nodes/spec` for users to bind.
3. **envtest.** New package `test/agentinjector`: render the template with `helm template`, apply it to an envtest apiserver (1.37), create Pods, and assert on the stored spec. Table cases:
   - all annotations, `automountServiceAccountToken: false`
   - `apikey-secret-key` override
   - no label → untouched
   - label without `apikey-secret-name` → untouched
   - Pod that already has the sidecar → not injected twice
   - existing init containers / volumes → appended, not replaced
   Plus: `helm template` without the API version fails.
4. **CI.** `make test` now needs `helm`. Install the tools pinned in `mise.toml` in `test.yml` via `jdx/mise-action`.
5. **Docs.** README section for the injector: requirements (Kubernetes 1.36+), Pod label/annotations, RBAC binding example.
6. **Verify.** `make lint`, `make test`, `helm lint`, and the kind cluster run with the chart-rendered policy and a real key.
