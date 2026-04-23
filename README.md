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

## Deletion Policy

- `upsert-only` creates and updates Mackerel monitors but does not delete them when CRDs are deleted.
- `sync` deletes only monitors whose ownership marker matches the current operator owner and source resource.
