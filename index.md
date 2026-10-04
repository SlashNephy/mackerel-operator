# mackerel-operator

`mackerel-operator` manages Mackerel resources from Kubernetes. It ships as a
Helm chart with two independent components.

## Features

### ExternalMonitor controller

- Synchronizes namespaced `ExternalMonitor` resources across the cluster with
  Mackerel HTTP/HTTPS external monitors.
- Manages every external monitor setting, including request headers, request
  body, check attempts, redirects, certificate checks, mute state, and the IP
  version (`dualstack`).
- Reads header values from Secrets in the same namespace and re-syncs when a
  Secret rotates.
- Adopts an existing monitor with the same name when its settings match the CR.
- Reports the result through the `Ready` condition (`Synced`, `InvalidSpec`,
  `OwnershipLost`, `SecretNotFound`, `SecretError`) and shows the monitor ID in
  `kubectl get externalmonitors`.
- Supports `--policy=upsert-only` and `--policy=sync`. See
  [Deletion Policy](#deletion-policy).
- Reads the Mackerel API key from `MACKEREL_APIKEY`.
- Stores ownership metadata in the Mackerel monitor memo:

```text
<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->
```

### mackerel-container-agent injector

- Injects mackerel-container-agent into labeled Pods as a native sidecar via a
  `MutatingAdmissionPolicy`. See
  [Injecting mackerel-container-agent](#injecting-mackerel-container-agent).

## Requirements

| Component | Kubernetes | Why |
|---|---|---|
| ExternalMonitor controller | 1.25 or later | The CRD validates specs with CEL rules (`x-kubernetes-validations`). Kubernetes 1.24 drops these rules silently, so invalid specs are accepted. |
| mackerel-container-agent injector | 1.36 or later | It is a `MutatingAdmissionPolicy`. |

The injector is disabled by default (`agentInjector.enabled: false`), so the
chart installs on 1.25 or later as long as it stays disabled.

## Example

```yaml
apiVersion: mackerel.starry.blue/v1alpha1
kind: ExternalMonitor
metadata:
  name: api-health
  namespace: app

spec:
  name: API health check
  service: my-service
  url: https://api.example.com/healthz
  method: GET
  notificationInterval: 10
  expectedStatusCode: 200
  containsString: ok
  responseTimeDuration: 5
  responseTimeWarning: 3000
  responseTimeCritical: 5000
  certificationExpirationWarning: 30
  certificationExpirationCritical: 14
  isMute: false
  followRedirect: true
  skipCertificateVerification: false
  maxCheckAttempts: 3
  dualstack: ipv4
  headers:
    - name: X-Request-Source
      value: mackerel-operator
  memo: Check the connection to the API.
```

`requestBody` is omitted above because the example uses `method: GET`. It applies
to monitors that send a payload:

```yaml
spec:
  url: https://api.example.com/graphql
  method: POST
  headers:
    - name: Content-Type
      value: application/json
  requestBody: '{"query":"{ health }"}'
```

`dualstack` selects the IP version used to reach the URL: `ipv4` (no IPv6
retry), `ipv6` (no IPv4 retry), or `auto` (IPv6 first, then IPv4). Omitting the
field behaves as `ipv4`, which is what Mackerel applies to a monitor that never
set it.

### How `headers` handles being unset

`headers` is the one field where "omitted" and "empty" mean different things.

- **Omitted.** The operator does not manage headers at all. It neither writes
  nor reconciles them, so whatever Mackerel holds stays. This matters because
  Mackerel adds `Cache-Control: no-cache` to every external monitor it creates,
  which stops a CDN or reverse proxy from answering the check with a cached
  `200`.
- **`headers: []`.** Remove every header, including that default.
- **A non-empty list.** The operator owns the headers and replaces whatever is
  live with exactly the list in the CR.

Every other field in the spec resets to its default when omitted. `headers` is
the exception because the default it would otherwise destroy exists to keep the
monitor honest.

### Header values from a Secret

A header value written as `value` is stored in plain text in the CR and in etcd,
and the Mackerel API returns it unmasked. Read credentials from a Secret
instead:

```yaml
spec:
  headers:
    - name: Authorization
      valueFrom:
        secretKeyRef:
          name: api-credentials
          key: token
```

Exactly one of `value` and `valueFrom` must be set on a header; the API server
rejects a header that sets both or neither.

The Secret must live in the same namespace as the `ExternalMonitor`. The
operator watches the referenced Secrets, so rotating one triggers a
reconciliation that pushes the new value to Mackerel. While a referenced Secret
or key is missing, the monitor is left untouched and the CR reports
`Ready=False` with reason `SecretNotFound`, rather than being written with a
partial header set.

Note that this protects the value at rest in the cluster only. Mackerel stores
and returns the resolved value like any other header value, so anyone with
access to the Mackerel API can still read it.

The operator therefore needs `get`, `list` and `watch` on Secrets. Because it
watches `ExternalMonitor` resources across the whole cluster, that permission is
granted cluster wide. Only the metadata of Secrets is cached; the values are
read on demand and are not held in the operator's memory.

### Upgrading from a version without `isMute`, `followRedirect`, `skipCertificateVerification`, `maxCheckAttempts`, `requestBody`, or `headers`

These six fields are now fully managed by the operator. **Before upgrading**, check every `ExternalMonitor` in the cluster and set each of these fields in the CR to reflect the value you rely on, whether that value was set in the Mackerel web UI, via the API, or by another tool. After the upgrade, the operator becomes the sole source of truth for these fields; any value set outside the CR is replaced on the next reconcile.

The most disruptive case is `isMute`. A monitor that was muted in the Mackerel web UI will be **unmuted automatically** on the first reconcile after the upgrade unless you add `isMute: true` to the CR before upgrading. Unmuting can cause alerts to fire immediately if the monitor is in a failing state.

The other change to be aware of is monitor adoption. Before this release, the operator would adopt a name-matched, marker-less Mackerel monitor (one created in the UI before the operator was set up) even if that monitor had a non-default `maxCheckAttempts`, custom headers, or any of the other new fields. After the upgrade, the operator requires the CR to spell out those field values exactly. If the monitor's current values do not match the CR's defaults (for example `maxCheckAttempts` is 2 in Mackerel but the CR omits the field, so the operator defaults it to 1), the adoption path will report `OwnershipLost` instead of adopting the monitor. Resolve this by setting the field explicitly in the CR to match what Mackerel currently holds, applying the CR, and then letting the operator adopt and manage the monitor normally.

### Upgrading from a version without `dualstack`

`dualstack` is now managed by the operator as well, and it behaves differently from the six fields above: Mackerel **keeps** the stored value when the key is absent from a request, so earlier versions of the operator never touched it. From this release the operator writes the field on every create and update, sending `ipv4` when the CR omits it.

**Before upgrading**, add `dualstack` to any `ExternalMonitor` whose monitor was switched to IPv6 or auto in the Mackerel web UI. Otherwise the next reconcile resets that monitor to IPv4, and a URL that only resolves over IPv6 starts failing its check.

Adding the field does not change the desired-state hash of an `ExternalMonitor` that omits it, so unlike the `maxCheckAttempts` default this upgrade does not by itself cause an Update on every resource. Monitor adoption follows the same rule as the other fields: a name-matched, marker-less monitor set to IPv6 is only adopted if the CR spells out `dualstack: ipv6`.

### Restoring `Cache-Control: no-cache` after 0.2.0

Version 0.2.0 sent an explicit empty header list for any `ExternalMonitor` that
did not declare `headers`, which removed the `Cache-Control: no-cache` that
Mackerel had added when the monitor was created. Upgrading past 0.2.0 stops the
deletion, but it does not put the header back: Mackerel only injects it when the
monitor is created, and the operator no longer touches headers a CR does not
declare.

If a monitor lost the header while running 0.2.0, restore it explicitly:

```yaml
spec:
  headers:
    - name: Cache-Control
      value: no-cache
```

To check which monitors are affected, look for external monitors with no headers
in the Mackerel web UI or via `GET /api/v0/monitors`.

## Development

```bash
mise exec -- make generate manifests
mise exec -- go test ./...
```

## Running Locally

```bash
export MACKEREL_APIKEY=...
mise exec -- make install
mise exec -- go run ./cmd/main.go --policy=upsert-only --owner-id=default --hash-length=7
```

## Installing With Helm

Add the chart repository:

```bash
helm repo add mackerel-operator https://slashnephy.github.io/mackerel-operator
helm repo update
```

Create a Secret that contains the Mackerel API key:

```bash
kubectl create namespace mackerel-operator-system
kubectl create secret generic mackerel-api-key \
  --namespace mackerel-operator-system \
  --from-literal=apiKey=...
```

Install the chart:

```bash
helm install mackerel-operator mackerel-operator/mackerel-operator \
  --namespace mackerel-operator-system \
  --create-namespace
```

The chart installs the `ExternalMonitor` CRD from `charts/mackerel-operator/crds/`.
The image tag defaults to the chart's `appVersion`. Set `policy`, `ownerID`, and
`hashLength` in the values to change the corresponding operator flags.
The release workflow publishes `ghcr.io/slashnephy/mackerel-operator:<chart version>`
and `ghcr.io/slashnephy/mackerel-operator:latest` to GHCR.

## Injecting mackerel-container-agent

The chart can inject [mackerel-container-agent](https://github.com/mackerelio/mackerel-container-agent)
into Pods as a native sidecar. It is a `MutatingAdmissionPolicy`, so the
operator itself is not involved and Pod creation does not depend on it.
It requires Kubernetes 1.36 or later; installing with it enabled on an older
cluster fails. Offline rendering such as `helm template` checks `--kube-version`
instead.

```bash
helm upgrade --install mackerel-operator mackerel-operator/mackerel-operator \
  --namespace mackerel-operator-system \
  --set agentInjector.enabled=true
```

The agent reads metrics from the kubelet, so bind the chart's ClusterRole
(`mackerel-operator-agent` for a release named `mackerel-operator`) to the
ServiceAccount of each workload. The role grants `get` on `nodes/pods`,
`nodes/stats`, and `nodes/spec` for every node, and every container in the Pod
shares the ServiceAccount token. Bind it only to ServiceAccounts used by Pods
that run the agent.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-app-mackerel-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: mackerel-operator-agent
subjects:
  - kind: ServiceAccount
    name: my-app
    namespace: app
```

Put the agent's API key in a Secret in the Pod's namespace, then label and
annotate the Pod template:

```yaml
spec:
  template:
    metadata:
      labels:
        agent.mackerel.starry.blue/inject: "true"
      annotations:
        agent.mackerel.starry.blue/apikey-secret-name: mackerel-agent-api-key
        agent.mackerel.starry.blue/roles: "my-service:app"
```

| Key | Kind | Required | Meaning |
|---|---|---|---|
| `agent.mackerel.starry.blue/inject` | label | yes | `"true"` opts the Pod in. |
| `agent.mackerel.starry.blue/apikey-secret-name` | annotation | yes | Secret holding the API key. Nothing is injected without it. |
| `agent.mackerel.starry.blue/apikey-secret-key` | annotation | no | Key in that Secret. Defaults to `MACKEREL_APIKEY`. |
| `agent.mackerel.starry.blue/roles` | annotation | no | Passed as `MACKEREL_ROLES`. |
| `agent.mackerel.starry.blue/config-configmap-name` | annotation | no | ConfigMap whose `mackerel-agent.conf` is used as the agent config. |

The annotation is required, but the Secret it names does not have to exist:
if the Secret or key is missing, the app container still starts and only the
agent fails. Only newly created Pods are
injected, so roll out existing workloads after enabling it.

The injected container runs as UID 65532 with a securityContext that satisfies
the `restricted` Pod Security Standard, so it can be used in namespaces that
enforce it, and with a read-only root filesystem. `/var/tmp` is an injected
`emptyDir` (`TMPDIR` points there for plugins). Override
`agentInjector.securityContext` to change it.

## Publishing Helm Chart With GitHub Pages

This repository includes `.github/workflows/release-chart.yml`, which uses
`helm/chart-releaser-action` to publish `charts/mackerel-operator` as a Helm
repository on GitHub Pages and also copies `README.md` to the published
`index.md`.

One-time repository setup on GitHub:

1. Create and push an empty `gh-pages` branch.
2. Open repository Settings > Pages.
3. Set the publishing source to the `gh-pages` branch and the `/ (root)` folder.

After that, every push to `main` runs the chart release workflow. When
`charts/mackerel-operator/Chart.yaml` version changes, the workflow packages the
chart, creates or updates the GitHub Release, refreshes the Pages index, and
updates the top page by syncing `README.md` to `index.md`.

## Deletion Policy

- `upsert-only` creates and updates Mackerel monitors but does not delete them when CRDs are deleted.
- `sync` deletes only monitors whose ownership marker matches the current operator owner and source resource.
