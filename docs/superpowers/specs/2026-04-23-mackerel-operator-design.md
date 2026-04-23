# Mackerel Operator MVP Design

## Context

This project will provide a Kubernetes Operator that synchronizes Kubernetes custom resources with Mackerel monitor configuration. The MVP focuses on Mackerel external URL monitors. The design follows the useful parts of external-dns: desired state comes from Kubernetes, actual state comes from the provider, ownership metadata prevents cross-controller conflicts, and a policy flag controls deletion behavior.

The repository is currently an initial empty project. The implementation will use Go, Kubebuilder, and controller-runtime.

## Goals

- Define Mackerel HTTP/HTTPS external monitors with a Kubernetes CRD.
- Create and update matching Mackerel external monitors.
- Support safe deletion behavior with an external-dns-like policy.
- Preserve human-edited Mackerel memo text while storing a compact ownership marker.
- Keep internal boundaries ready for future sources such as Ingress or HTTPRoute annotations.

## Non-Goals

- Do not support every Mackerel external monitor API field in the MVP.
- Do not implement Ingress, HTTPRoute, or annotation sources in the MVP.
- Do not read Kubernetes Secrets directly for the Mackerel API key in the MVP.
- Do not run real Mackerel API e2e tests as part of the initial MVP.
- Do not manage non-external Mackerel monitor types.

## API

The CRD is namespaced.

```yaml
apiVersion: mackerel.starry.blue/v1alpha1
kind: ExternalMonitor
metadata:
  name: api-health
  namespace: default
spec:
  name: "API health check"
  service: "my-service"
  url: "https://api.example.com/healthz"
  method: "GET"
  notificationInterval: 10
  expectedStatusCode: 200
  containsString: "ok"
  responseTimeWarning: 3000
  responseTimeCritical: 5000
  certificationExpirationWarning: 30
  certificationExpirationCritical: 14
  memo: "Managed by Kubernetes"
```

`spec.name` is optional. If omitted, the controller generates a stable Mackerel monitor name from the Kubernetes namespace and name.

The MVP spec fields are:

- `name`: optional Mackerel display name.
- `service`: optional Mackerel service name.
- `url`: required HTTP or HTTPS URL.
- `method`: optional request method, defaulting to `GET`; valid values are `GET`, `POST`, `PUT`, and `DELETE`.
- `notificationInterval`: optional notification resend interval in minutes.
- `expectedStatusCode`: optional status code judged as OK.
- `containsString`: optional response body substring requirement.
- `responseTimeWarning`: optional response time warning threshold in milliseconds.
- `responseTimeCritical`: optional response time critical threshold in milliseconds.
- `certificationExpirationWarning`: optional certificate expiration warning threshold in days.
- `certificationExpirationCritical`: optional certificate expiration critical threshold in days.
- `memo`: optional human-facing Mackerel monitor memo.

Additional Mackerel external monitor fields, including `headers`, `requestBody`, `followRedirect`, `skipCertificateVerification`, `responseTimeDuration`, `maxCheckAttempts`, and `isMute`, are future work.

The status fields are:

```yaml
status:
  monitorID: "2cSZzK3XfmG"
  observedGeneration: 3
  lastSyncedAt: "2026-04-23T12:00:00Z"
  lastAppliedHash: "deadbee"
  url: "https://api.example.com/healthz"
  mackerelMonitorName: "API health check"
  conditions:
    - type: Ready
      status: "True"
      reason: Synced
```

## Configuration

The controller is cluster-scoped and watches `ExternalMonitor` resources in all namespaces.

Runtime configuration:

- `MACKEREL_APIKEY`: required environment variable for Mackerel API authentication.
- `--policy=upsert-only|sync`: synchronization policy. The default is `upsert-only`.
- `--owner-id=<id>`: owner identifier used in ownership markers. The default is `default`.
- `--hash-length=<n>`: length of the shortened desired-state hash in memo markers. The default is `7`.

`upsert-only` creates and updates managed monitors but does not delete Mackerel monitors when CRDs disappear. `sync` also deletes owned Mackerel monitors when the corresponding CRD is deleted.

## Ownership Marker

Mackerel external monitor `memo` is optional but limited by the API to 2048 characters. The operator must preserve the human-facing memo and only manage a compact marker at the end of the memo.

The marker format is:

```text
<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->
```

Fields:

- `heritage=mackerel-operator`: identifies the marker.
- `resource=externalmonitor/<namespace>/<name>`: identifies the source resource.
- `owner=<owner-id>`: prevents conflicts between multiple operator deployments.
- `hash=<hash>`: shortened SHA-256 hash of the canonical desired monitor payload.

The hash defaults to 7 hex characters and can be changed with `--hash-length`. The hash is an optimization and diagnostic aid, not the only safety check. Updates and deletes must also check `heritage`, `resource`, and `owner`, and compare actual monitor fields where needed.

If a user edits the Mackerel memo, the operator preserves all text outside the marker. If the marker is removed, the operator uses `status.monitorID` to fetch the monitor. It restores the marker only when the actual monitor still effectively matches the desired state. If the actual monitor differs, the operator treats the monitor as ownership-lost and does not update or delete it automatically.

## Components

- `ExternalMonitorReconciler`: watches CRD events and reconciles one `ExternalMonitor` at a time.
- `Source`: interface that returns desired monitor objects. The MVP implements only `ExternalMonitorSource`.
- `MackerelProvider`: thin wrapper around the Mackerel API client for external monitor list, get, create, update, and delete operations.
- `Ownership`: parses, builds, removes, and validates memo markers.
- `Planner`: lightweight functions that compare desired and actual state and return create, update, delete, noop, or ownership-lost decisions.
- `Hasher`: canonicalizes desired monitor payloads, computes SHA-256, and shortens the hash.
- `Status`: helper functions for conditions, observed generation, last sync time, monitor ID, URL, monitor name, and last applied hash.

This is intentionally a middle ground rather than a full external-dns clone. The reconciler stays CRD-oriented for the MVP, but the source and provider boundaries leave room for future annotation-based sources.

## Reconcile Flow

1. Fetch the `ExternalMonitor`.
2. Build the desired Mackerel external monitor from the spec.
3. Compute the desired-state hash.
4. If `status.monitorID` is set, fetch that Mackerel monitor.
5. If the monitor exists and has a matching ownership marker, update it when desired and actual differ.
6. If the monitor exists but the marker is missing, restore the marker only if actual state effectively matches desired state.
7. If the monitor differs and the marker is missing, set an `OwnershipLost` condition and stop without updating.
8. If `status.monitorID` is empty or the monitor no longer exists, search for an existing monitor with a matching marker.
9. If no owned monitor exists, create a new Mackerel external monitor with the human memo plus marker.
10. On success, update status.

Deletion uses a finalizer. With `upsert-only`, the finalizer does not delete the Mackerel monitor. With `sync`, the finalizer deletes only a monitor whose marker has matching `heritage`, `resource`, and `owner`.

## Error Handling

Mackerel API errors are reflected in conditions and Kubernetes events.

- Transient network errors, rate limits, and 5xx errors requeue.
- Validation errors, missing Mackerel services, and other user-fixable 4xx errors set a not-ready condition and avoid aggressive retry loops.
- Missing or invalid `MACKEREL_APIKEY` prevents startup or sets a clear controller initialization error.
- Ownership conflicts never cause destructive updates. The controller records a condition and leaves the monitor untouched.

Mackerel update requests require complete monitor fields. Before every update, the provider must fetch actual state and merge it with the desired MVP-managed fields. Fields outside the MVP must be copied from actual state into the update request so the operator does not clear them accidentally.

## Validation

CRD validation covers:

- `spec.url` must use `http` or `https`.
- `spec.method` must be one of `GET`, `POST`, `PUT`, or `DELETE`.
- Numeric thresholds must be non-negative.
- `notificationInterval`, if set, must be at least 10 minutes to match Mackerel API requirements.
- Response time thresholds require `service` where Mackerel requires it.
- Certificate expiration thresholds apply only to HTTPS URLs.
- `spec.memo` plus ownership marker must fit within Mackerel's 2048-character memo limit.

## Testing

The MVP test suite prioritizes deterministic unit tests and fake provider tests.

Required tests:

- Ownership marker parse, build, remove, and human memo preservation.
- Default 7-character hash and configurable hash length.
- Conversion from `ExternalMonitorSpec` to Mackerel external monitor payload.
- Planner decisions for create, update, noop, delete, and ownership-lost cases.
- `upsert-only` versus `sync` deletion behavior.
- Status condition updates.
- CRD validation or equivalent validation behavior.

Controller-runtime envtest covers the main reconcile paths with a fake Mackerel provider. Real Mackerel API e2e tests are deferred.

## Future Work

- Add Ingress annotation source.
- Add HTTPRoute annotation source.
- Add more Mackerel external monitor fields.
- Support direct Secret reference for API key configuration.
- Add import or adoption workflow for pre-existing Mackerel external monitors.
- Add dry-run mode.
- Add richer metrics and controller observability.
