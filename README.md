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
  headers:
    - name: X-Request-Source
      value: mackerel-operator
  memo: Check the connection to the API.
```

Header values are stored in plain text in the cluster and are returned unmasked
by the Mackerel API. Avoid putting credentials in `headers` until Secret
references are supported.

### Upgrading from a version without `isMute`, `followRedirect`, `skipCertificateVerification`, `maxCheckAttempts`, `requestBody`, or `headers`

These six fields are now fully managed by the operator. **Before upgrading**, check every `ExternalMonitor` in the cluster and set each of these fields in the CR to reflect the value you rely on, whether that value was set in the Mackerel web UI, via the API, or by another tool. After the upgrade, the operator becomes the sole source of truth for these fields; any value set outside the CR is replaced on the next reconcile.

The most disruptive case is `isMute`. A monitor that was muted in the Mackerel web UI will be **unmuted automatically** on the first reconcile after the upgrade unless you add `isMute: true` to the CR before upgrading. Unmuting can cause alerts to fire immediately if the monitor is in a failing state.

The other change to be aware of is monitor adoption. Before this release, the operator would adopt a name-matched, marker-less Mackerel monitor (one created in the UI before the operator was set up) even if that monitor had a non-default `maxCheckAttempts`, custom headers, or any of the other new fields. After the upgrade, the operator requires the CR to spell out those field values exactly. If the monitor's current values do not match the CR's defaults (for example `maxCheckAttempts` is 2 in Mackerel but the CR omits the field, so the operator defaults it to 1), the adoption path will report `OwnershipLost` instead of adopting the monitor. Resolve this by setting the field explicitly in the CR to match what Mackerel currently holds, applying the CR, and then letting the operator adopt and manage the monitor normally.

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
  --set image.tag=0.1.2
```

The chart installs the `ExternalMonitor` CRD from `charts/mackerel-operator/crds/`.
The release workflow publishes `ghcr.io/slashnephy/mackerel-operator:<chart version>`
and `ghcr.io/slashnephy/mackerel-operator:latest` to GHCR.

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
