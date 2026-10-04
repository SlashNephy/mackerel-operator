# Agent Injector Design

## Goal

Inject [mackerel-container-agent](https://github.com/mackerelio/mackerel-container-agent) into opted-in Pods as a sidecar, replacing the unmaintained [mackerel-container-agent-sidecar-injector](https://github.com/mackerelio-labs/mackerel-container-agent-sidecar-injector) (last updated 2022-10).

## Approach

The injector is a `MutatingAdmissionPolicy` and `MutatingAdmissionPolicyBinding` (`admissionregistration.k8s.io/v1`, GA in Kubernetes 1.36) shipped in the existing Helm chart. The operator binary does not change.

This avoids a webhook server, webhook TLS certificates, and putting the operator on the Pod creation path. If the operator is down, Pods are still created and still get the sidecar.

The chart is gated behind `agentInjector.enabled` (default `false`). When enabled, the template fails unless `.Capabilities.APIVersions.Has "admissionregistration.k8s.io/v1/MutatingAdmissionPolicy"` or the Kubernetes version is 1.36 or later. Offline rendering (`helm template`, GitOps tools) only knows the version, so the version check keeps it working.

## Pod interface

| Key | Kind | Required | Meaning |
|---|---|---|---|
| `agent.mackerel.starry.blue/inject` | label | yes | `"true"` opts the Pod in. Used by the binding's `objectSelector`, so other Pods are never evaluated. |
| `agent.mackerel.starry.blue/apikey-secret-name` | annotation | yes | Secret in the Pod's namespace holding the agent API key. Without it, nothing is injected. |
| `agent.mackerel.starry.blue/apikey-secret-key` | annotation | no | Key inside that Secret. Defaults to `MACKEREL_APIKEY`. |
| `agent.mackerel.starry.blue/roles` | annotation | no | Passed as `MACKEREL_ROLES`. |
| `agent.mackerel.starry.blue/config-configmap-name` | annotation | no | ConfigMap whose `mackerel-agent.conf` is mounted at `/etc/mackerel-agent/` and passed as `MACKEREL_AGENT_CONFIG`. |

The API key Secret must be created per namespace by the user. The operator does not read or replicate Secrets, because `secretKeyRef` cannot cross namespaces and replication needs cluster-wide Secret write access.

## Injected resources

- An init container named `mackerel-container-agent` with `restartPolicy: Always` (native sidecar), so Jobs still terminate.
- A projected volume `mackerel-agent-sa-token` (service account token, `kube-root-ca.crt`, namespace) mounted at `/var/run/secrets/kubernetes.io/serviceaccount`. The ServiceAccount admission plugin runs before admission policies, so the sidecar would otherwise get no token. This also works with `automountServiceAccountToken: false`.
- Env: `MACKEREL_CONTAINER_PLATFORM=kubernetes`, kubelet host / namespace / Pod name via the downward API, `MACKEREL_KUBERNETES_KUBELET_READ_ONLY_PORT=0`, and `MACKEREL_KUBERNETES_KUBELET_INSECURE_TLS=true` (configurable). The read-only port 10255 is disabled on most clusters.

The projected token uses `expirationSeconds: 3607`, the same value the ServiceAccount admission plugin uses for `kube-api-access-*`. The agent reads the token only once at startup, so it relies on the apiserver extending tokens with this exact lifetime. Do not change it to 3600.

The `secretKeyRef` is `optional: true`. A wrong Secret name or key leaves the app container running and only the agent fails. Without it, a native sidecar that cannot resolve its env keeps the whole Pod in `CreateContainerConfigError`, unlike the original injector where the agent was a regular container.

The container gets `agentInjector.securityContext` from values. The default (UID 65532, `runAsNonRoot`, no privilege escalation, read-only root filesystem, all capabilities dropped, `RuntimeDefault` seccomp) satisfies the `restricted` Pod Security Standard. It is added as a separate JSONPatch whose value is rendered as an untyped CEL map, so any field of `SecurityContext` can be set without listing typed `Object.*` constructors. Each element is wrapped in `dyn()` because CEL rejects map literals with mixed value types. The agent creates `/var/tmp/mackerel-container-agent`, so an `emptyDir` named `mackerel-agent-tmp` is mounted at `/var/tmp` and `TMPDIR` points there for plugins; this keeps the root filesystem read-only.

Only `CREATE` is matched. Pods that already have a volume named `mackerel-agent-sa-token` or `mackerel-agent-config` are skipped, because adding a duplicate name would make the Pod invalid. A Pod that already has a `mackerel-container-agent` init container is skipped.

Mutations use `JSONPatch`. `ApplyConfiguration` rejects atomic fields such as `fieldRef`, `secretKeyRef`, and `projected.sources`. `Object.*` types are only available inside mutation expressions, not in `variables`, and `cel.bind` is not available.

## RBAC

The agent reads `nodes/pods`, `nodes/stats`, and `nodes/spec` from the kubelet. `nodes/pods` covers the kubelet's `/pods` through KubeletFineGrainedAuthz (GA in 1.36), so the original injector's `nodes/proxy` is not granted. `nodes/proxy` also allows reaching any kubelet endpoint. The chart ships a ClusterRole for this. Users bind it to their workload ServiceAccounts themselves, as with the original injector. The operator does not manage these bindings.

## Values

```yaml
agentInjector:
  enabled: false
  image:
    repository: mackerel/mackerel-container-agent
    tag: v0.13.4-plugins   # tracked by Renovate
  kubeletInsecureTLS: true
  resources:
    limits:
      memory: 128Mi
  namespaceSelector: {}    # optional extra scoping on the binding
```

## Feasibility check

A prototype policy was run on kind with Kubernetes v1.37.0.

- labelled Pod with all annotations, with `automountServiceAccountToken: false`: sidecar injected, token / CA / namespace mounted, config ConfigMap mounted, `2/2 Running`. The agent fetched Pod metadata from the kubelet on 10250 and reached the Mackerel API (with a dummy key at this point).
- `apikey-secret-key: apiKey` override: the env referenced `other/apiKey`. The Secret did not exist. Before `optional: true` the Pod stayed in `CreateContainerConfigError: secret "other" not found`; after it, the Pod was `2/2 Running` and only the agent retried.
- No label: not injected.
- Label without `apikey-secret-name`: not injected.

- With a real API key: the agent registered a host (`start the agent: host id = ...`) and posted `container.cpu.*`, `container.memory.*`, and `interface.eth0.*` metrics. Deleting the Pod retired the host (`isRetired: true`).

## Testing

- `helm lint` and `helm template` with `agentInjector.enabled=true`.
- envtest: `ENVTEST_K8S_VERSION` follows `k8s.io/api` (currently 1.37), so the envtest kube-apiserver runs admission policies. Apply the rendered policy and check the four cases above against the created Pod specs. No kubelet is needed for that.
- Running the agent itself needs kind. CI's e2e workflow installs the latest kind, which uses 1.36 or newer. `kind` is pinned in `mise.toml` for local runs.
