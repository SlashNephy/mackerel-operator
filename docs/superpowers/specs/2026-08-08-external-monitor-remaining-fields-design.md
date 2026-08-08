# External Monitor Remaining Fields Design

Closes #21.

## Goal

`ExternalMonitorSpec` currently exposes only the subset of external monitor fields chosen for the MVP. Six fields supported by the Mackerel external monitor API are unreachable from the CRD, so users who need them must fall back to managing those monitors outside the operator. This design adds all six so the CRD covers the full external monitor surface.

## Scope

The gap was determined by diffing `ExternalMonitorSpec` against `mackerel.MonitorExternalHTTP` in `mackerel-client-go` v0.45.0, which itself covers every parameter documented at <https://mackerel.io/api-docs/entry/monitors>.

| Field | Type | Default | Validation |
| --- | --- | --- | --- |
| `isMute` | `bool` | `false` | — |
| `followRedirect` | `bool` | `false` | — |
| `skipCertificateVerification` | `bool` | `false` | — |
| `maxCheckAttempts` | `int` | `1` | `Minimum=1`, `Maximum=10` |
| `requestBody` | `string` | — | — |
| `headers` | `[]HeaderField` | — | `listType=map`, `listMapKey=name` |

Out of scope: resolving header values from a `Secret` (`valueFrom.secretKeyRef`). That requires Secret RBAC, a watch and mapping so Secret rotation retriggers reconciliation, and a decision about whether resolved values participate in the desired hash. It is tracked as a separate issue.

Also out of scope: raising the `memo` `MaxLength` from 1900. The Mackerel limit is 2048, and the 1900 ceiling reserves room for the ownership marker appended to the memo. Changing it needs its own analysis of the marker budget.

## API Shape

`HeaderField` is a new type in `api/v1alpha1`:

```go
// HeaderField is an HTTP request header sent by the external monitor.
type HeaderField struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9!#$%&'*+.^_|~-]+$`
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}
```

Headers are modelled as a list of `{name, value}` structs rather than `map[string]string`, matching both the Mackerel wire format and the Kubernetes convention used by `corev1.HTTPHeader` and Gateway API. The struct also leaves room to add `valueFrom` alongside `value` when Secret references land, without replacing the type.

`Name` is restricted to the RFC 7230 token character set minus the backtick. Dropping the backtick keeps the pattern writable as a Go raw string literal, and no header name in practice contains one. Invalid names are then rejected by `kubectl apply` instead of surfacing as a 400 from the Mackerel API mid-reconcile.

The three booleans and `maxCheckAttempts` use plain types with `kubebuilder:default` rather than pointers. Mackerel's monitor update is a full replace, so it has no concept of "unset" distinct from "default"; modelling that distinction in the CRD would create a state the backend cannot represent.

## Field Semantics Verified Against the Live API

Two behaviours drive the design and were confirmed against `api.mackerelio.com` by creating a temporary external monitor, reading it back, and deleting it.

Header values are **not** masked. Both `GET /api/v0/monitors/{id}` and `GET /api/v0/monitors` return `value` verbatim. Drift detection can therefore compare header names and values in full; no carve-out is needed.

Header order is **preserved as sent**, not normalised by Mackerel. Headers submitted as `Authorization, X-Zebra, X-Alpha` were returned in that same order. Because `listType=map` declares the list order-insensitive, both sides must be sorted by name before comparison.

`requestBody`, `isMute`, `followRedirect`, `skipCertificateVerification`, and `maxCheckAttempts` all appear in read responses, so each is usable for drift detection.

## Layer Changes

The existing five-layer pipeline is followed as-is.

- `api/v1alpha1/externalmonitor_types.go` — add the six fields and the `HeaderField` type.
- `internal/monitor/model.go` — add the fields to both `DesiredExternalMonitor` and `ActualExternalMonitor`, plus a `HeaderField` type local to the package. `internal/monitor` does not import `api/v1alpha1` today, and keeping that direction of dependency absent means the intermediate model stays independent of the CRD version. `internal/source` translates between the two. The desired struct keeps `omitempty` on every new field so hashes stay stable (see Backward Compatibility).
- `internal/source/externalmonitor.go` — CR to desired conversion. Normalises `maxCheckAttempts` and sorts headers by name.
- `internal/planner/planner.go` — extend `actualMatchesDesired` with the six fields. Headers compare with `slices.Equal` on the already-sorted slices.
- `internal/provider/mackerel.go` — bidirectional mapping in `mergeMackerelExternalMonitor` and `actualExternalMonitorFromMackerel`. `maxCheckAttempts` converts through the existing `uint64` helpers. Headers convert to `[]mackerel.HeaderField`, using an empty slice rather than nil so that removing every header is transmitted explicitly.

## Normalisation

Mackerel replaces the whole monitor on update and returns a normalised representation. Where the desired and actual representations of the same state differ, `actualMatchesDesired` returns false forever and every reconcile issues a write. Two normalisations prevent that:

- `maxCheckAttempts`: the source layer coerces an unset value to `1`, because Mackerel always reports `1` for monitors created without it.
- `headers`: both desired and actual are sorted by name before comparison, because Mackerel echoes the submitted order.

## Backward Compatibility

`HashDesired` marshals `DesiredExternalMonitor` to JSON, so a new field tagged `omitempty` and left at its zero value does not change the hash. The three booleans, `requestBody`, and `headers` are therefore invisible to existing monitors. This holds for headers whether the source layer emits nil or an empty slice, since `omitempty` drops both; the repository convention of preferring empty slices over nil is kept.

`maxCheckAttempts` is the exception. Defaulting it to `1` makes `"maxCheckAttempts":1` appear in the marshalled desired state, changing the hash of every existing `ExternalMonitor`. Each one will take a single `Update` on its next reconcile. The write is idempotent and rewrites the same values plus the new marker hash, so the churn is bounded and self-healing. This is accepted rather than worked around, because the alternatives — excluding the field from the hash, or leaving it at `0` and special-casing the comparison — trade a one-time sweep for permanent special cases.

## Testing

Table-driven tests with testify, per the existing convention in each package.

- `internal/source`: a CR with every field populated maps to the expected desired state; `maxCheckAttempts` unset normalises to `1`; headers are sorted by name.
- `internal/planner`: each of the six fields independently produces an `Update` decision when it differs; headers differing only in order do **not** produce a diff; headers differing in value do.
- `internal/provider`: round-trip mapping through `mergeMackerelExternalMonitor` and `actualExternalMonitorFromMackerel` preserves every field; clearing all headers sends an empty slice rather than nil.
- `internal/monitor`: hashing a desired state whose new fields are zero-valued yields the same hash as before the fields existed, pinning the backward-compatibility guarantee.

## Documentation

- `config/samples/mackerel_v1alpha1_externalmonitor.yaml` — extend the sample with the new fields.
- `README.md` — extend the spec field table.
- `make manifests` regenerates `config/crd/bases/` and the chart CRD under `charts/`.

## Verification

- `make lint test`
- Apply a CR exercising all six fields against a live Mackerel organisation, then read the monitor back through the API to confirm the values landed, and confirm a second reconcile reports `Noop` rather than looping on `Update`.
