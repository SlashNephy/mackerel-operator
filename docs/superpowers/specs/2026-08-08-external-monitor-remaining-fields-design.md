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

Four behaviours drive the design and were confirmed against `api.mackerelio.com` by creating a temporary external monitor, mutating it, reading it back, and deleting it.

**Mackerel's `PUT /api/v0/monitors/{id}` is a genuine full replace.** Omitting `isMute`, `followRedirect`, `skipCertificateVerification`, or `requestBody` from the request body **resets** them to their defaults rather than leaving them unchanged. `mackerel-client-go` tags all four `omitempty`, so writing `false`/`""` emits no JSON key, and Mackerel resets the field. This means the operator can reliably clear any of these fields by writing their zero value — there is no need for an explicit "clear" mechanism.

Header values are **not** masked. Both `GET /api/v0/monitors/{id}` and `GET /api/v0/monitors` return `value` verbatim. Drift detection can therefore compare header names and values in full; no carve-out is needed.

Header order is **preserved as sent**, not normalised by Mackerel. Headers submitted as `Authorization, X-Zebra, X-Alpha` were returned in that same order. Because `listType=map` declares the list order-insensitive, both sides must be sorted by name before comparison.

**Mackerel does not normalise header name casing.** `X-Foo-Bar` is echoed back verbatim as submitted. The operator does not need to normalise header names before comparing or sending them.

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

`maxCheckAttempts` is the exception. Defaulting it to `1` makes `"maxCheckAttempts":1` appear in the marshalled desired state, changing the hash of every existing `ExternalMonitor`. Each one will take a single `Update` on its next reconcile.

**That sweep is not purely cosmetic.** On upgrade, `mergeMackerelExternalMonitor` overwrites all six fields from the CR rather than preserving live values. The consequence is:

1. Every existing `ExternalMonitor` whose hash changes due to the `maxCheckAttempts` default takes one `Update` on its next reconcile.
2. That `Update` overwrites all six fields with the CR's values (or their defaults).
3. A monitor that was **muted in the Mackerel web UI** is therefore **unmuted** on the first reconcile after upgrade, unless the CR explicitly sets `isMute: true`.

Unmuting can cause alerts to fire immediately if the monitor is in a failing state. Users must be warned and given the opportunity to set `isMute: true` in the CR before upgrading.

The sweep itself is accepted rather than worked around, because the alternatives — excluding the field from the hash, or leaving it at `0` and special-casing the comparison — trade a one-time sweep for permanent special cases.

## Ownership Semantics Change

`mergeMackerelExternalMonitor` copies the current monitor and overwrites only the fields the operator manages. Everything else survives, which is what `TestMergeMackerelExternalMonitorPreservesUnsupportedFields` pins: today the six fields in scope are deliberately left untouched so the MVP does not clobber values set elsewhere.

Bringing them under the CRD inverts that contract. The operator becomes the source of truth for all six, and a value changed in the Mackerel web UI is reverted on the next reconcile. That is the correct declarative behaviour and the point of the issue, but it is a behavioural change for anyone who has been muting an operator-managed monitor from the UI.

The existing test is therefore rewritten rather than extended: it keeps its role of pinning the preserve-versus-own boundary, but the six fields move from the preserved side to the owned side. `ID` and `Type` remain on the preserved side.

**Adoption of pre-existing monitors is narrowed.** Before this change, `findActual` could return a name-matched, marker-less monitor and `Plan` would route to `ActionRestoreMarker` as long as the core fields (name, URL, method, etc.) matched the desired state. With six new fields added to `actualMatchesDesired`, that adopt path now requires the CR to spell out the values of all six new fields correctly. A monitor created in the Mackerel UI with `maxCheckAttempts: 2`, any custom headers, or `isMute: true` will route to `ActionOwnershipLost` instead of adopting. `OwnershipLost` is the correct fail-safe: the controller surfaces a status condition rather than silently overwriting values. Users adopting such monitors must add the matching field values to the CR before the operator will adopt them.

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
