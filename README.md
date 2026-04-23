# mackerel-operator

`mackerel-operator` synchronizes Kubernetes `ExternalMonitor` resources with Mackerel external URL monitors.

## MVP Scope

- Manages Mackerel HTTP/HTTPS external monitors.
- Watches namespaced `ExternalMonitor` resources across the cluster.
- Reads the Mackerel API key from `MACKEREL_APIKEY`.
- Supports `--policy=upsert-only` and `--policy=sync`.
- Stores ownership metadata in the Mackerel monitor memo:

```text
<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->
```

## Example

```yaml
apiVersion: mackerel.starry.blue/v1alpha1
kind: ExternalMonitor
metadata:
  name: api-health
  namespace: default
spec:
  name: API health check
  service: my-service
  url: https://api.example.com/healthz
  method: GET
  notificationInterval: 10
  expectedStatusCode: 200
  containsString: ok
  responseTimeWarning: 3000
  responseTimeCritical: 5000
  certificationExpirationWarning: 30
  certificationExpirationCritical: 14
  memo: Managed by Kubernetes
```

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

Once GitHub Pages publishing is enabled, add the chart repository:

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
  --create-namespace \
  --set image.repository=ghcr.io/slashnephy/mackerel-operator \
  --set image.tag=0.1.0
```

The chart installs the `ExternalMonitor` CRD from `charts/mackerel-operator/crds/`.

## Publishing Helm Chart With GitHub Pages

This repository includes `.github/workflows/release-chart.yml`, which uses
`helm/chart-releaser-action` to publish `charts/mackerel-operator` as a Helm
repository on GitHub Pages.

One-time repository setup on GitHub:

1. Create and push an empty `gh-pages` branch.
2. Open repository Settings > Pages.
3. Set the publishing source to the `gh-pages` branch and the `/ (root)` folder.

After that, every push to `main` runs the chart release workflow. When
`charts/mackerel-operator/Chart.yaml` version changes, the workflow packages the
chart, creates or updates the GitHub Release, and refreshes the Pages index.

## Deletion Policy

- `upsert-only` creates and updates Mackerel monitors but does not delete them when CRDs are deleted.
- `sync` deletes only monitors whose ownership marker matches the current operator owner and source resource.
