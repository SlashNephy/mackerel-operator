# External Monitor Remaining Fields Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the six Mackerel external monitor fields missing from `ExternalMonitorSpec` — `isMute`, `followRedirect`, `skipCertificateVerification`, `maxCheckAttempts`, `requestBody`, `headers` — so the CRD covers the full external monitor API surface.

**Architecture:** The operator maps a CR through five layers: `api/v1alpha1` (CRD types) → `internal/source` (CR to desired) → `internal/planner` (desired vs actual drift) → `internal/provider` (desired to Mackerel SDK). `internal/monitor` holds the CRD-independent intermediate model plus the desired-state hash embedded in the ownership marker. Each new field is threaded through all five layers. Two normalisations in the source layer (`maxCheckAttempts` unset to `1`, headers sorted by name) keep drift detection from looping.

**Tech Stack:** Go 1.26.2, Kubebuilder / controller-runtime, `controller-gen`, `github.com/mackerelio/mackerel-client-go` v0.45.0, `github.com/stretchr/testify` v1.11.1, `mise` for the toolchain.

**Design doc:** `docs/superpowers/specs/2026-08-08-external-monitor-remaining-fields-design.md`

## Global Constraints

- Go style follows <https://google.github.io/styleguide/go/decisions.html>. Initialisms stay capitalised (`URL`, `HTTP`, `ID`).
- Pass structs by pointer, not by value, wherever a choice exists.
- Prefer empty slices over nil slices.
- Use Go 1.26 `new(literal)` to take a pointer to a literal. Do **not** add a `ptr[T](v T) *T` helper.
- New tests use table-driven subtests with `github.com/stretchr/testify` (`require` for fatal assertions, `assert` for non-fatal). Existing tests in these files use bare `t.Fatalf`; leave those alone unless a task says to rewrite one.
- Do not write tautological tests: no test that reads source files looking for patterns, and no test that compares a constant to its own copy.
- Comments in code are written in English — the repository's primary language is English.
- Log and error message strings are English.
- Commit messages follow Conventional Commits and end with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.
- All `make` invocations go through `mise exec --`, for example `mise exec -- make test`.
- Work happens on the branch `feat/external-monitor-remaining-fields`, already created.

## File Structure

| Path | Responsibility | Action |
| --- | --- | --- |
| `internal/monitor/model.go` | CRD-independent desired/actual model plus `HeaderField` | Modify |
| `internal/monitor/hash_test.go` | Pins hash backward compatibility | Modify |
| `api/v1alpha1/externalmonitor_types.go` | CRD schema and validation markers | Modify |
| `api/v1alpha1/zz_generated.deepcopy.go` | Generated deepcopy | Regenerate |
| `config/crd/bases/mackerel.starry.blue_externalmonitors.yaml` | Generated CRD | Regenerate |
| `charts/mackerel-operator/crds/mackerel.starry.blue_externalmonitors.yaml` | Chart copy of the CRD | Regenerate |
| `internal/source/externalmonitor.go` | CR to desired, plus normalisation | Modify |
| `internal/source/externalmonitor_test.go` | Source layer tests | Modify |
| `internal/planner/planner.go` | `actualMatchesDesired` drift comparison | Modify |
| `internal/planner/planner_test.go` | Planner tests | Modify |
| `internal/provider/mackerel.go` | Mackerel SDK mapping both directions | Modify |
| `internal/provider/mackerel_test.go` | Provider tests | Modify |
| `config/samples/mackerel_v1alpha1_externalmonitor.yaml` | Sample CR | Modify |
| `README.md` | Example CR | Modify |

---

### Task 1: Intermediate model gains the six fields

Adds the fields to `internal/monitor` first, because every other layer refers to these names and types. Also pins the backward-compatibility guarantee that zero-valued new fields leave the desired hash unchanged.

**Files:**
- Modify: `internal/monitor/model.go`
- Test: `internal/monitor/hash_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `monitor.HeaderField` struct with fields `Name string` and `Value string`, JSON tags `name` and `value`.
  - `monitor.DesiredExternalMonitor` gains `IsMute bool`, `FollowRedirect bool`, `SkipCertificateVerification bool`, `MaxCheckAttempts int`, `RequestBody string`, `Headers []HeaderField`, all tagged `omitempty`.
  - `monitor.ActualExternalMonitor` gains the same six fields with no JSON tags (the struct has none today).

- [ ] **Step 1: Write the failing test**

Append to `internal/monitor/hash_test.go`:

```go
func TestHashDesiredIgnoresZeroValuedNewFields(t *testing.T) {
	t.Parallel()

	base := DesiredExternalMonitor{
		Name:               "API health check",
		URL:                "https://api.example.com/healthz",
		Method:             "GET",
		ExpectedStatusCode: new(200),
	}

	withZeroValues := base
	withZeroValues.IsMute = false
	withZeroValues.FollowRedirect = false
	withZeroValues.SkipCertificateVerification = false
	withZeroValues.MaxCheckAttempts = 0
	withZeroValues.RequestBody = ""
	withZeroValues.Headers = []HeaderField{}

	baseHash, err := HashDesired(base, 12)
	require.NoError(t, err)

	zeroHash, err := HashDesired(withZeroValues, 12)
	require.NoError(t, err)

	assert.Equal(t, baseHash, zeroHash, "zero-valued new fields must not change the desired hash")
}

func TestHashDesiredChangesWhenNewFieldsAreSet(t *testing.T) {
	t.Parallel()

	base := DesiredExternalMonitor{
		Name:   "API health check",
		URL:    "https://api.example.com/healthz",
		Method: "GET",
	}
	baseHash, err := HashDesired(base, 12)
	require.NoError(t, err)

	tests := []struct {
		name   string
		mutate func(d *DesiredExternalMonitor)
	}{
		{name: "isMute", mutate: func(d *DesiredExternalMonitor) { d.IsMute = true }},
		{name: "followRedirect", mutate: func(d *DesiredExternalMonitor) { d.FollowRedirect = true }},
		{name: "skipCertificateVerification", mutate: func(d *DesiredExternalMonitor) { d.SkipCertificateVerification = true }},
		{name: "maxCheckAttempts", mutate: func(d *DesiredExternalMonitor) { d.MaxCheckAttempts = 3 }},
		{name: "requestBody", mutate: func(d *DesiredExternalMonitor) { d.RequestBody = `{"ping":true}` }},
		{name: "headers", mutate: func(d *DesiredExternalMonitor) {
			d.Headers = []HeaderField{{Name: "Authorization", Value: "Bearer token"}}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mutated := base
			tt.mutate(&mutated)

			got, err := HashDesired(mutated, 12)
			require.NoError(t, err)
			assert.NotEqual(t, baseHash, got)
		})
	}
}
```

Update the import block at the top of `internal/monitor/hash_test.go` to:

```go
import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `mise exec -- go test ./internal/monitor/ -run 'TestHashDesired(IgnoresZeroValuedNewFields|ChangesWhenNewFieldsAreSet)' -v`

Expected: FAIL to compile, with errors like `unknown field IsMute in struct literal` and `undefined: HeaderField`.

- [ ] **Step 3: Write minimal implementation**

Replace the contents of `internal/monitor/model.go` with:

```go
package monitor

// HeaderField is an HTTP request header sent by an external monitor.
//
// This mirrors the Mackerel wire format but is declared here rather than
// reused from api/v1alpha1 so that the intermediate model stays independent
// of any single CRD version.
type HeaderField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type DesiredExternalMonitor struct {
	Name                            string        `json:"name,omitempty"`
	Service                         string        `json:"service,omitempty"`
	URL                             string        `json:"url"`
	Method                          string        `json:"method,omitempty"`
	NotificationInterval            *int          `json:"notificationInterval,omitempty"`
	ExpectedStatusCode              *int          `json:"expectedStatusCode,omitempty"`
	ContainsString                  string        `json:"containsString,omitempty"`
	ResponseTimeDuration            *int          `json:"responseTimeDuration,omitempty"`
	ResponseTimeWarning             *int          `json:"responseTimeWarning,omitempty"`
	ResponseTimeCritical            *int          `json:"responseTimeCritical,omitempty"`
	CertificationExpirationWarning  *int          `json:"certificationExpirationWarning,omitempty"`
	CertificationExpirationCritical *int          `json:"certificationExpirationCritical,omitempty"`
	IsMute                          bool          `json:"isMute,omitempty"`
	FollowRedirect                  bool          `json:"followRedirect,omitempty"`
	SkipCertificateVerification     bool          `json:"skipCertificateVerification,omitempty"`
	MaxCheckAttempts                int           `json:"maxCheckAttempts,omitempty"`
	RequestBody                     string        `json:"requestBody,omitempty"`
	Headers                         []HeaderField `json:"headers,omitempty"`
	Memo                            string        `json:"memo,omitempty"`
	Resource                        string        `json:"resource"`
	Owner                           string        `json:"owner"`
	Hash                            string        `json:"hash,omitempty"`
}

type ActualExternalMonitor struct {
	ID                              string
	Name                            string
	Service                         string
	URL                             string
	Method                          string
	NotificationInterval            *int
	ExpectedStatusCode              *int
	ContainsString                  string
	ResponseTimeDuration            *int
	ResponseTimeWarning             *int
	ResponseTimeCritical            *int
	CertificationExpirationWarning  *int
	CertificationExpirationCritical *int
	IsMute                          bool
	FollowRedirect                  bool
	SkipCertificateVerification     bool
	MaxCheckAttempts                int
	RequestBody                     string
	Headers                         []HeaderField
	Memo                            string
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `mise exec -- go test ./internal/monitor/ -v`

Expected: PASS, including `TestHashDesiredIgnoresZeroValuedNewFields` and all six `TestHashDesiredChangesWhenNewFieldsAreSet` subtests.

- [ ] **Step 5: Commit**

```bash
git add internal/monitor/model.go internal/monitor/hash_test.go
git commit -m "feat(monitor): add remaining external monitor fields to the model

Adds isMute, followRedirect, skipCertificateVerification,
maxCheckAttempts, requestBody, and headers to the intermediate model,
plus a HeaderField type kept independent of api/v1alpha1.

Every new field is tagged omitempty so that existing monitors, whose
values are all zero, keep their current desired hash and do not churn.

Refs #21

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 2: CRD schema gains the six fields

**Files:**
- Modify: `api/v1alpha1/externalmonitor_types.go`
- Regenerate: `api/v1alpha1/zz_generated.deepcopy.go`, `config/crd/bases/mackerel.starry.blue_externalmonitors.yaml`, `charts/mackerel-operator/crds/mackerel.starry.blue_externalmonitors.yaml`

**Interfaces:**
- Consumes: nothing from Task 1 — `api/v1alpha1` must not import `internal/monitor`.
- Produces:
  - `v1alpha1.HeaderField` struct with `Name string` (json `name`) and `Value string` (json `value`).
  - `v1alpha1.ExternalMonitorSpec` gains `IsMute bool`, `FollowRedirect bool`, `SkipCertificateVerification bool`, `MaxCheckAttempts int`, `RequestBody string`, `Headers []HeaderField`.

- [ ] **Step 1: Add the fields to the spec type**

In `api/v1alpha1/externalmonitor_types.go`, insert the following immediately **before** the `Memo` field inside `ExternalMonitorSpec`:

```go
	// +kubebuilder:default=false
	IsMute bool `json:"isMute,omitempty"`
	// +kubebuilder:default=false
	FollowRedirect bool `json:"followRedirect,omitempty"`
	// +kubebuilder:default=false
	SkipCertificateVerification bool `json:"skipCertificateVerification,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	MaxCheckAttempts int    `json:"maxCheckAttempts,omitempty"`
	RequestBody      string `json:"requestBody,omitempty"`
	// +listType=map
	// +listMapKey=name
	Headers []HeaderField `json:"headers,omitempty"`
```

Then append this type at the end of the file, after `ExternalMonitorList`:

```go
// HeaderField is an HTTP request header sent by the external monitor.
type HeaderField struct {
	// name is the header name. The character set is the RFC 7230 token set
	// minus the backtick, which no real header name uses and which would
	// otherwise force the pattern out of a raw string literal.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9!#$%&'*+.^_|~-]+$`
	Name string `json:"name"`
	// value is the header value sent verbatim. It is stored in plain text in
	// the cluster, so avoid credentials until Secret references are supported.
	// +kubebuilder:validation:Required
	Value string `json:"value"`
}
```

- [ ] **Step 2: Regenerate deepcopy and manifests**

Run: `mise exec -- make generate manifests`

Expected: exits 0. `git status --porcelain` then shows modifications to `api/v1alpha1/zz_generated.deepcopy.go` and `config/crd/bases/mackerel.starry.blue_externalmonitors.yaml`.

- [ ] **Step 3: Verify the generated CRD contains the new schema**

Run:

```bash
grep -n "maxCheckAttempts" -A 5 config/crd/bases/mackerel.starry.blue_externalmonitors.yaml
grep -n "headers:" -A 25 config/crd/bases/mackerel.starry.blue_externalmonitors.yaml
```

Expected: `maxCheckAttempts` shows `default: 1`, `maximum: 10`, `minimum: 1`, `type: integer`. `headers` shows `x-kubernetes-list-type: map`, `x-kubernetes-list-map-keys: [name]`, and required `name` and `value` properties with the pattern on `name`.

If `headers` does **not** show `x-kubernetes-list-type: map`, the `+listType=map` marker was placed on the wrong line — it must sit directly above the `Headers` field with no blank line between.

- [ ] **Step 4: Sync the chart CRD**

The chart keeps its own copy of the CRD. Run:

```bash
cp config/crd/bases/mackerel.starry.blue_externalmonitors.yaml \
   charts/mackerel-operator/crds/mackerel.starry.blue_externalmonitors.yaml
git diff --stat charts/mackerel-operator/crds/
```

Expected: the chart CRD shows the same additions.

If the chart file is not at that path, run `find charts -name '*externalmonitors*'` and copy to whatever path it reports. If the chart has no CRD copy at all, skip this step.

- [ ] **Step 5: Verify the whole tree still builds**

Run: `mise exec -- go build ./...`

Expected: exits 0 with no output.

- [ ] **Step 6: Commit**

```bash
git add api/v1alpha1/ config/crd/bases/ charts/
git commit -m "feat(api): expose the remaining external monitor fields in the CRD

Adds isMute, followRedirect, skipCertificateVerification,
maxCheckAttempts, requestBody, and headers to ExternalMonitorSpec.

Headers use a name/value struct list rather than a map so the shape
matches the Mackerel wire format and leaves room for a future valueFrom
field. listType=map lets the API server reject duplicate header names.

Refs #21

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 3: Source layer maps and normalises the new fields

Translates `v1alpha1.HeaderField` to `monitor.HeaderField`, coerces an unset `maxCheckAttempts` to `1`, and sorts headers by name so that the planner comparison is order-insensitive.

**Files:**
- Modify: `internal/source/externalmonitor.go`
- Test: `internal/source/externalmonitor_test.go`

**Interfaces:**
- Consumes: `monitor.DesiredExternalMonitor` and `monitor.HeaderField` from Task 1; `v1alpha1.ExternalMonitorSpec` fields and `v1alpha1.HeaderField` from Task 2.
- Produces: `ExternalMonitorSource.FromExternalMonitor` returns a desired state whose `MaxCheckAttempts` is at least `1` and whose `Headers` are sorted ascending by `Name`. Later tasks rely on that sort order.

- [ ] **Step 1: Write the failing tests**

Append to `internal/source/externalmonitor_test.go`:

```go
func TestExternalMonitorSourceMapsRemainingFields(t *testing.T) {
	t.Parallel()

	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL:                         "https://api.example.com/healthz",
			IsMute:                      true,
			FollowRedirect:              true,
			SkipCertificateVerification: true,
			MaxCheckAttempts:            3,
			RequestBody:                 `{"ping":true}`,
			Headers: []mackerelv1alpha1.HeaderField{
				{Name: "X-Zebra", Value: "last"},
				{Name: "Authorization", Value: "Bearer token"},
			},
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	require.NoError(t, err)

	assert.True(t, got.IsMute)
	assert.True(t, got.FollowRedirect)
	assert.True(t, got.SkipCertificateVerification)
	assert.Equal(t, 3, got.MaxCheckAttempts)
	assert.Equal(t, `{"ping":true}`, got.RequestBody)
	assert.Equal(t, []monitor.HeaderField{
		{Name: "Authorization", Value: "Bearer token"},
		{Name: "X-Zebra", Value: "last"},
	}, got.Headers, "headers must be sorted by name so drift detection is order-insensitive")
}

func TestExternalMonitorSourceNormalisesMaxCheckAttempts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec int
		want int
	}{
		{name: "unset defaults to one", spec: 0, want: 1},
		{name: "explicit one", spec: 1, want: 1},
		{name: "explicit three", spec: 3, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cr := &mackerelv1alpha1.ExternalMonitor{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
				Spec: mackerelv1alpha1.ExternalMonitorSpec{
					URL:              "https://api.example.com/healthz",
					MaxCheckAttempts: tt.spec,
				},
			}

			src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
			got, err := src.FromExternalMonitor(cr)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.MaxCheckAttempts)
		})
	}
}

func TestExternalMonitorSourceEmitsEmptyHeadersSlice(t *testing.T) {
	t.Parallel()

	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	require.NoError(t, err)

	assert.NotNil(t, got.Headers, "prefer an empty slice over nil")
	assert.Empty(t, got.Headers)
}
```

Update the import block at the top of `internal/source/externalmonitor_test.go` to:

```go
import (
	"errors"
	"testing"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `mise exec -- go test ./internal/source/ -run 'TestExternalMonitorSource(MapsRemainingFields|NormalisesMaxCheckAttempts|EmitsEmptyHeadersSlice)' -v`

Expected: FAIL. `MapsRemainingFields` and `NormalisesMaxCheckAttempts` fail on assertions (zero values, and `0` instead of `1`); `EmitsEmptyHeadersSlice` fails with `Expected value not to be nil`.

- [ ] **Step 3: Write the implementation**

In `internal/source/externalmonitor.go`, add the import of `slices` and `strings`, so the block reads:

```go
import (
	"errors"
	"fmt"
	"slices"
	"strings"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
)
```

Inside `FromExternalMonitor`, after the existing `method` defaulting block, insert:

```go
	// Mackerel reports 1 for monitors created without maxCheckAttempts, so an
	// unset value has to be widened here. Leaving it at 0 would make the
	// planner see a permanent diff and rewrite the monitor on every reconcile.
	maxCheckAttempts := cr.Spec.MaxCheckAttempts
	if maxCheckAttempts < 1 {
		maxCheckAttempts = 1
	}
```

Then add the six fields to the `monitor.DesiredExternalMonitor` literal, immediately before `Memo`:

```go
		IsMute:                          cr.Spec.IsMute,
		FollowRedirect:                  cr.Spec.FollowRedirect,
		SkipCertificateVerification:     cr.Spec.SkipCertificateVerification,
		MaxCheckAttempts:                maxCheckAttempts,
		RequestBody:                     cr.Spec.RequestBody,
		Headers:                         sortedHeaders(cr.Spec.Headers),
```

Finally append this function to the end of the file:

```go
// sortedHeaders converts CRD headers to the intermediate model and orders them
// by name. The CRD declares headers as a list-map, so order carries no meaning,
// but Mackerel echoes back whatever order was submitted. Sorting both sides
// keeps the planner from reporting a diff that does not exist.
func sortedHeaders(headers []mackerelv1alpha1.HeaderField) []monitor.HeaderField {
	sorted := make([]monitor.HeaderField, 0, len(headers))
	for _, h := range headers {
		sorted = append(sorted, monitor.HeaderField{Name: h.Name, Value: h.Value})
	}

	slices.SortFunc(sorted, func(a, b monitor.HeaderField) int {
		return strings.Compare(a.Name, b.Name)
	})

	return sorted
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `mise exec -- go test ./internal/source/ -v`

Expected: PASS, all tests in the package including the four pre-existing ones.

- [ ] **Step 5: Commit**

```bash
git add internal/source/externalmonitor.go internal/source/externalmonitor_test.go
git commit -m "feat(source): map and normalise the remaining external monitor fields

Widens an unset maxCheckAttempts to 1 and sorts headers by name. Both
normalisations exist so that actualMatchesDesired does not report a
permanent diff: Mackerel always reports maxCheckAttempts as at least 1,
and it echoes headers back in submission order rather than sorting them.

Refs #21

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 4: Planner detects drift on the new fields

**Files:**
- Modify: `internal/planner/planner.go`
- Test: `internal/planner/planner_test.go`

**Interfaces:**
- Consumes: `monitor.DesiredExternalMonitor`, `monitor.ActualExternalMonitor`, `monitor.HeaderField` from Task 1. Relies on Task 3's guarantee that desired headers arrive sorted by name.
- Produces: `actualMatchesDesired` returns false when any of the six fields differ. No exported surface changes.

- [ ] **Step 1: Write the failing test**

Append to `internal/planner/planner_test.go`:

```go
func TestPlanDetectsDriftOnRemainingFields(t *testing.T) {
	t.Parallel()

	const (
		owner    = "prod"
		resource = "externalmonitor/default/api"
		hash     = "deadbee"
	)

	newDesired := func() monitor.DesiredExternalMonitor {
		return monitor.DesiredExternalMonitor{
			Name:             "api",
			URL:              "https://example.com",
			MaxCheckAttempts: 1,
			Headers:          []monitor.HeaderField{{Name: "Authorization", Value: "Bearer token"}},
			Owner:            owner,
			Resource:         resource,
			Hash:             hash,
		}
	}

	newActual := func() monitor.ActualExternalMonitor {
		return monitor.ActualExternalMonitor{
			ID:               "mon-1",
			Name:             "api",
			URL:              "https://example.com",
			MaxCheckAttempts: 1,
			Headers:          []monitor.HeaderField{{Name: "Authorization", Value: "Bearer token"}},
			Memo:             ownership.BuildMarker(ownership.Marker{Resource: resource, Owner: owner, Hash: hash}),
		}
	}

	tests := []struct {
		name   string
		mutate func(a *monitor.ActualExternalMonitor)
		want   Action
	}{
		{name: "in sync", mutate: func(_ *monitor.ActualExternalMonitor) {}, want: ActionNoop},
		{name: "isMute drifted", mutate: func(a *monitor.ActualExternalMonitor) { a.IsMute = true }, want: ActionUpdate},
		{name: "followRedirect drifted", mutate: func(a *monitor.ActualExternalMonitor) { a.FollowRedirect = true }, want: ActionUpdate},
		{name: "skipCertificateVerification drifted", mutate: func(a *monitor.ActualExternalMonitor) { a.SkipCertificateVerification = true }, want: ActionUpdate},
		{name: "maxCheckAttempts drifted", mutate: func(a *monitor.ActualExternalMonitor) { a.MaxCheckAttempts = 5 }, want: ActionUpdate},
		{name: "requestBody drifted", mutate: func(a *monitor.ActualExternalMonitor) { a.RequestBody = "changed" }, want: ActionUpdate},
		{
			name: "header value drifted",
			mutate: func(a *monitor.ActualExternalMonitor) {
				a.Headers = []monitor.HeaderField{{Name: "Authorization", Value: "Bearer rotated"}}
			},
			want: ActionUpdate,
		},
		{
			name: "header removed",
			mutate: func(a *monitor.ActualExternalMonitor) {
				a.Headers = []monitor.HeaderField{}
			},
			want: ActionUpdate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual := newActual()
			tt.mutate(&actual)

			decision := Plan(PlanInput{Desired: newDesired(), Actual: &actual})
			assert.Equal(t, tt.want, decision.Action, decision.Reason)
		})
	}
}

func TestPlanIgnoresHeaderOrder(t *testing.T) {
	t.Parallel()

	const (
		owner    = "prod"
		resource = "externalmonitor/default/api"
		hash     = "deadbee"
	)

	// The source layer sorts desired headers by name; Mackerel echoes back the
	// order they were originally submitted in. A monitor that is genuinely in
	// sync must not be rewritten just because those orders differ.
	desired := monitor.DesiredExternalMonitor{
		Name:             "api",
		URL:              "https://example.com",
		MaxCheckAttempts: 1,
		Headers: []monitor.HeaderField{
			{Name: "Authorization", Value: "Bearer token"},
			{Name: "X-Zebra", Value: "last"},
		},
		Owner:    owner,
		Resource: resource,
		Hash:     hash,
	}
	actual := monitor.ActualExternalMonitor{
		ID:               "mon-1",
		Name:             "api",
		URL:              "https://example.com",
		MaxCheckAttempts: 1,
		Headers: []monitor.HeaderField{
			{Name: "X-Zebra", Value: "last"},
			{Name: "Authorization", Value: "Bearer token"},
		},
		Memo: ownership.BuildMarker(ownership.Marker{Resource: resource, Owner: owner, Hash: hash}),
	}

	decision := Plan(PlanInput{Desired: desired, Actual: &actual})
	assert.Equal(t, ActionNoop, decision.Action, decision.Reason)
}
```

Update the import block at the top of `internal/planner/planner_test.go` to:

```go
import (
	"testing"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `mise exec -- go test ./internal/planner/ -run 'TestPlan(DetectsDriftOnRemainingFields|IgnoresHeaderOrder)' -v`

Expected: FAIL. Every drift subtest reports `Noop` where `Update` was wanted, because `actualMatchesDesired` does not yet look at these fields. `TestPlanIgnoresHeaderOrder` passes at this point — that is expected, and it becomes a real guard once Step 3 lands.

- [ ] **Step 3: Write the implementation**

In `internal/planner/planner.go`, change the import block to:

```go
import (
	"reflect"
	"slices"
	"strings"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
)
```

Replace `actualMatchesDesired` with:

```go
func actualMatchesDesired(desired monitor.DesiredExternalMonitor, actual monitor.ActualExternalMonitor) bool {
	return desired.Name == actual.Name &&
		desired.Service == actual.Service &&
		desired.URL == actual.URL &&
		desired.Method == actual.Method &&
		reflect.DeepEqual(desired.NotificationInterval, actual.NotificationInterval) &&
		reflect.DeepEqual(desired.ExpectedStatusCode, actual.ExpectedStatusCode) &&
		desired.ContainsString == actual.ContainsString &&
		reflect.DeepEqual(desired.ResponseTimeDuration, actual.ResponseTimeDuration) &&
		reflect.DeepEqual(desired.ResponseTimeWarning, actual.ResponseTimeWarning) &&
		reflect.DeepEqual(desired.ResponseTimeCritical, actual.ResponseTimeCritical) &&
		reflect.DeepEqual(desired.CertificationExpirationWarning, actual.CertificationExpirationWarning) &&
		reflect.DeepEqual(desired.CertificationExpirationCritical, actual.CertificationExpirationCritical) &&
		desired.IsMute == actual.IsMute &&
		desired.FollowRedirect == actual.FollowRedirect &&
		desired.SkipCertificateVerification == actual.SkipCertificateVerification &&
		desired.MaxCheckAttempts == actual.MaxCheckAttempts &&
		desired.RequestBody == actual.RequestBody &&
		headersMatch(desired.Headers, actual.Headers)
}

// headersMatch compares headers as an unordered set keyed by name. Mackerel
// preserves the submission order rather than normalising it, so comparing the
// slices positionally would report drift for a monitor that is in sync.
func headersMatch(desired, actual []monitor.HeaderField) bool {
	if len(desired) != len(actual) {
		return false
	}

	byName := func(a, b monitor.HeaderField) int {
		return strings.Compare(a.Name, b.Name)
	}

	sortedDesired := slices.Clone(desired)
	slices.SortFunc(sortedDesired, byName)
	sortedActual := slices.Clone(actual)
	slices.SortFunc(sortedActual, byName)

	return slices.Equal(sortedDesired, sortedActual)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `mise exec -- go test ./internal/planner/ -v`

Expected: PASS, all tests in the package including the pre-existing ones.

- [ ] **Step 5: Commit**

```bash
git add internal/planner/planner.go internal/planner/planner_test.go
git commit -m "feat(planner): detect drift on the remaining external monitor fields

Headers compare as an unordered set keyed by name. Mackerel preserves
the order headers were submitted in rather than normalising it, so a
positional comparison would report drift on a monitor that is in sync
and rewrite it on every reconcile.

Refs #21

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 5: Provider maps the new fields to and from the Mackerel SDK

This is the task that inverts the preserve-versus-own contract described in the design doc, so it rewrites an existing test rather than only adding new ones.

**Files:**
- Modify: `internal/provider/mackerel.go`
- Test: `internal/provider/mackerel_test.go`

**Interfaces:**
- Consumes: `monitor.DesiredExternalMonitor`, `monitor.ActualExternalMonitor`, `monitor.HeaderField` from Task 1.
- Produces: `mergeMackerelExternalMonitor` writes all six fields onto the payload; `actualExternalMonitorFromMackerel` reads all six back. No exported surface changes.

- [ ] **Step 1: Rewrite the preserve test and add the round-trip test**

In `internal/provider/mackerel_test.go`, replace the whole of `TestMergeMackerelExternalMonitorPreservesUnsupportedFields` (lines 11 through 83, ending at the closing brace before `TestMergeMackerelExternalMonitorAppliesResponseTimeDuration`) with:

```go
func TestMergeMackerelExternalMonitorOwnsManagedFields(t *testing.T) {
	t.Parallel()

	// Every field the operator manages is overwritten from the desired state,
	// even when the live monitor disagrees. Only server-assigned identity
	// survives, so a change made in the Mackerel web UI is reverted.
	base := &mackerel.MonitorExternalHTTP{
		ID:                          "mon-1",
		Type:                        "external",
		IsMute:                      true,
		MaxCheckAttempts:            5,
		ResponseTimeDuration:        new(uint64(3)),
		RequestBody:                 "stale payload",
		SkipCertificateVerification: true,
		FollowRedirect:              true,
		Headers: []mackerel.HeaderField{
			{Name: "X-Stale", Value: "removed"},
		},
	}
	desired := monitor.DesiredExternalMonitor{
		Name:                            "API health",
		Service:                         "my-service",
		URL:                             "https://api.example.com/healthz",
		Method:                          "GET",
		NotificationInterval:            new(10),
		ExpectedStatusCode:              new(200),
		ContainsString:                  "ok",
		ResponseTimeWarning:             new(1000),
		ResponseTimeCritical:            new(2000),
		CertificationExpirationWarning:  new(30),
		CertificationExpirationCritical: new(14),
		IsMute:                          false,
		FollowRedirect:                  false,
		SkipCertificateVerification:     false,
		MaxCheckAttempts:                2,
		RequestBody:                     `{"ping":true}`,
		Headers: []monitor.HeaderField{
			{Name: "Authorization", Value: "Bearer token"},
		},
	}

	got, err := mergeMackerelExternalMonitor(base, desired, "human memo")
	require.NoError(t, err)

	assert.Equal(t, "mon-1", got.ID, "server-assigned identity is preserved")
	assert.Equal(t, "external", got.Type)

	assert.False(t, got.IsMute)
	assert.False(t, got.FollowRedirect)
	assert.False(t, got.SkipCertificateVerification)
	assert.Equal(t, uint64(2), got.MaxCheckAttempts)
	assert.Equal(t, `{"ping":true}`, got.RequestBody)
	assert.Equal(t, []mackerel.HeaderField{{Name: "Authorization", Value: "Bearer token"}}, got.Headers)

	assert.Nil(t, got.ResponseTimeDuration)
	assert.Equal(t, desired.Name, got.Name)
	assert.Equal(t, desired.Service, got.Service)
	assert.Equal(t, desired.URL, got.URL)
	assert.Equal(t, desired.Method, got.Method)
	assert.Equal(t, "human memo", got.Memo)
	assert.Equal(t, uint64(10), got.NotificationInterval)
}

func TestMergeMackerelExternalMonitorClearsHeadersWithEmptySlice(t *testing.T) {
	t.Parallel()

	// mackerel-client-go tags Headers without omitempty precisely so that an
	// empty list can be transmitted. Sending nil would leave the live headers
	// in place, so clearing every header has to produce a non-nil empty slice.
	base := &mackerel.MonitorExternalHTTP{
		Headers: []mackerel.HeaderField{{Name: "X-Stale", Value: "removed"}},
	}
	desired := monitor.DesiredExternalMonitor{
		Name:    "API health",
		URL:     "https://api.example.com/healthz",
		Method:  "GET",
		Headers: []monitor.HeaderField{},
	}

	got, err := mergeMackerelExternalMonitor(base, desired, "")
	require.NoError(t, err)

	require.NotNil(t, got.Headers)
	assert.Empty(t, got.Headers)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"headers":[]`)
}

func TestActualExternalMonitorFromMackerelReadsRemainingFields(t *testing.T) {
	t.Parallel()

	got := actualExternalMonitorFromMackerel(&mackerel.MonitorExternalHTTP{
		ID:                          "mon-1",
		Name:                        "API health",
		URL:                         "https://api.example.com/healthz",
		Method:                      "GET",
		IsMute:                      true,
		FollowRedirect:              true,
		SkipCertificateVerification: true,
		MaxCheckAttempts:            4,
		RequestBody:                 `{"ping":true}`,
		Headers: []mackerel.HeaderField{
			{Name: "X-Zebra", Value: "last"},
			{Name: "Authorization", Value: "Bearer token"},
		},
	})

	assert.True(t, got.IsMute)
	assert.True(t, got.FollowRedirect)
	assert.True(t, got.SkipCertificateVerification)
	assert.Equal(t, 4, got.MaxCheckAttempts)
	assert.Equal(t, `{"ping":true}`, got.RequestBody)
	assert.Equal(t, []monitor.HeaderField{
		{Name: "X-Zebra", Value: "last"},
		{Name: "Authorization", Value: "Bearer token"},
	}, got.Headers, "the provider reads headers verbatim; the planner handles ordering")
}

func TestMergeMackerelExternalMonitorRejectsNegativeMaxCheckAttempts(t *testing.T) {
	t.Parallel()

	desired := monitor.DesiredExternalMonitor{
		Name:             "API health",
		URL:              "https://api.example.com/healthz",
		Method:           "GET",
		MaxCheckAttempts: -1,
	}

	_, err := mergeMackerelExternalMonitor(&mackerel.MonitorExternalHTTP{}, desired, "")
	require.ErrorIs(t, err, errNegativeIntValue)
}
```

Update the import block at the top of `internal/provider/mackerel_test.go` to:

```go
import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `mise exec -- go test ./internal/provider/ -v`

Expected: FAIL to compile with `unknown field IsMute in struct literal of type monitor.DesiredExternalMonitor` if Task 1 has not landed; otherwise FAIL on assertions — `got.IsMute` is still `true`, `got.MaxCheckAttempts` is still `5`, and `got.Headers` still holds `X-Stale`.

- [ ] **Step 3: Write the implementation**

In `internal/provider/mackerel.go`, add the six fields to `actualExternalMonitorFromMackerel`, immediately before `Memo`:

```go
		IsMute:                          m.IsMute,
		FollowRedirect:                  m.FollowRedirect,
		SkipCertificateVerification:     m.SkipCertificateVerification,
		MaxCheckAttempts:                intFromUint64(m.MaxCheckAttempts),
		RequestBody:                     m.RequestBody,
		Headers:                         headerFieldsFromMackerel(m.Headers),
```

In `mergeMackerelExternalMonitor`, add this conversion alongside the existing ones, after the `certificationExpirationCritical` block:

```go
	maxCheckAttempts, err := uint64FromIntPtr(&desired.MaxCheckAttempts)
	if err != nil {
		return nil, err
	}
```

and add these assignments after `merged.ExpectedStatusCode = desired.ExpectedStatusCode`:

```go
	merged.IsMute = desired.IsMute
	merged.FollowRedirect = desired.FollowRedirect
	merged.SkipCertificateVerification = desired.SkipCertificateVerification
	merged.MaxCheckAttempts = maxCheckAttempts
	merged.RequestBody = desired.RequestBody
	merged.Headers = headerFieldsToMackerel(desired.Headers)
```

Append these three helpers to the end of `internal/provider/mackerel.go`:

```go
func intFromUint64(v uint64) int {
	if v > uint64(math.MaxInt) {
		return 0
	}

	return int(v)
}

func headerFieldsFromMackerel(headers []mackerel.HeaderField) []monitor.HeaderField {
	converted := make([]monitor.HeaderField, 0, len(headers))
	for _, h := range headers {
		converted = append(converted, monitor.HeaderField{Name: h.Name, Value: h.Value})
	}

	return converted
}

// headerFieldsToMackerel always returns a non-nil slice. mackerel-client-go
// omits the omitempty tag on Headers so that an empty list can be sent to
// remove every header; returning nil would silently leave the live headers in
// place instead.
func headerFieldsToMackerel(headers []monitor.HeaderField) []mackerel.HeaderField {
	converted := make([]mackerel.HeaderField, 0, len(headers))
	for _, h := range headers {
		converted = append(converted, mackerel.HeaderField{Name: h.Name, Value: h.Value})
	}

	return converted
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `mise exec -- go test ./internal/provider/ -v`

Expected: PASS, all tests in the package.

- [ ] **Step 5: Run the whole unit suite**

Run: `mise exec -- go test ./internal/... ./api/...`

Expected: PASS across every package. If `internal/controller` fails, its envtest binaries are missing — run `mise exec -- make setup-envtest` first.

- [ ] **Step 6: Commit**

```bash
git add internal/provider/mackerel.go internal/provider/mackerel_test.go
git commit -m "feat(provider): map the remaining external monitor fields

The operator now owns isMute, followRedirect,
skipCertificateVerification, maxCheckAttempts, requestBody, and headers
rather than preserving whatever the live monitor holds. A value changed
in the Mackerel web UI is reverted on the next reconcile, which is the
declarative behaviour the CRD implies.

Headers are always sent as a non-nil slice. mackerel-client-go leaves
omitempty off that field so an empty list can clear every header;
sending nil would leave the live headers untouched.

Refs #21

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 6: Documentation and samples

**Files:**
- Modify: `config/samples/mackerel_v1alpha1_externalmonitor.yaml`
- Modify: `README.md`

**Interfaces:**
- Consumes: the CRD field names from Task 2.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Extend the sample CR**

In `config/samples/mackerel_v1alpha1_externalmonitor.yaml`, insert the following between the `certificationExpirationCritical: 14` line and the `memo:` line:

```yaml
  isMute: false
  followRedirect: true
  skipCertificateVerification: false
  maxCheckAttempts: 3
  requestBody: ""
  headers:
    - name: X-Request-Source
      value: mackerel-operator
```

- [ ] **Step 2: Extend the README example**

In `README.md`, apply the same insertion to the YAML block under `## Example`, between `certificationExpirationCritical: 14` and `memo:`.

Then add this paragraph directly below the closing ``` of that block:

```markdown
Header values are stored in plain text in the cluster and are returned unmasked
by the Mackerel API. Avoid putting credentials in `headers` until Secret
references are supported.
```

- [ ] **Step 3: Verify the sample is accepted by the generated CRD schema**

Run:

```bash
mise exec -- make manifests
git diff --exit-code config/crd/bases/ && echo "CRD unchanged as expected"
```

Expected: `CRD unchanged as expected`. A non-empty diff means Task 2's regeneration was not committed — commit it before continuing.

- [ ] **Step 4: Commit**

```bash
git add config/samples/mackerel_v1alpha1_externalmonitor.yaml README.md
git commit -m "docs: document the remaining external monitor fields

Refs #21

Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>"
```

---

### Task 7: Full verification and pull request

**Files:**
- No source changes. Produces evidence for the PR body.

**Interfaces:**
- Consumes: everything from Tasks 1 through 6.
- Produces: a pull request and a follow-up issue.

- [ ] **Step 1: Run lint and the full test suite**

Run:

```bash
mise exec -- make lint
mise exec -- make test
```

Expected: both exit 0. Capture the output verbatim — it goes in the PR body.

If golangci-lint reports a finding you cannot resolve, **stop and ask the user how to proceed**. Do not silence it with a `//nolint` comment and do not edit the linter configuration.

- [ ] **Step 2: Verify against the live Mackerel API**

This step needs a Mackerel API key with monitor read and write scope, supplied by the user. Do not hardcode a key into any file, and do not commit one.

Create a monitor through the operator's own mapping path by applying a CR to a cluster running the operator. If no cluster is available, ask the user whether to verify by driving `internal/provider` directly from a short throwaway program instead, and follow their answer.

Whichever path is used, confirm three things and capture the raw API responses:

1. All six fields land in Mackerel with the values from the CR.
2. Reading the monitor back returns those values.
3. A second reconcile of the unchanged CR produces `Noop`, not `Update`. This is the check that catches a normalisation mistake — if `maxCheckAttempts` or header ordering were handled wrongly, the second pass rewrites the monitor.

Delete any monitor created for verification, and confirm the deletion by listing monitors afterwards.

- [ ] **Step 3: Open the follow-up issue for Secret-backed header values**

````bash
gh issue create \
  --title "Support Secret references for external monitor header values" \
  --label enhancement \
  --body 'ExternalMonitor gained a `headers` field in #21, but values are written in plain text in the CR and stored unencrypted in etcd. Header values are commonly credentials such as `Authorization: Bearer ...`.

Add a `valueFrom.secretKeyRef` alternative to `value` on `HeaderField`, mirroring the `corev1.EnvVarSource` shape:

```yaml
headers:
  - name: Authorization
    valueFrom:
      secretKeyRef:
        name: api-credentials
        key: token
```

Work involved:

- Extend `HeaderField` in `api/v1alpha1` with `valueFrom`, and add a CEL validation rule so exactly one of `value` and `valueFrom` is set.
- Grant the controller RBAC to read Secrets, and decide whether that is namespace-scoped or cluster-wide.
- Watch referenced Secrets and map changes back to the owning `ExternalMonitor`, so rotating a Secret retriggers reconciliation.
- Decide whether resolved values participate in the desired hash. Including them makes rotation detectable but writes a hash derived from secret material into the monitor memo; excluding them means rotation is only picked up by the drift comparison.
- Decide the failure mode when a referenced Secret or key is missing: block the reconcile with a `Ready=False` condition rather than sending a partial header set.

The Mackerel API returns header values unmasked on read, which the drift comparison in `internal/planner` already relies on.'
````

Expected: prints the URL of the new issue. Record it for the PR body.

- [ ] **Step 4: Push the branch and open the pull request**

```bash
git push -u origin feat/external-monitor-remaining-fields
```

Then open a PR whose body covers:

- What was added, as the six-field table from the design doc.
- The ownership semantics change, stated plainly: these fields are now reverted on reconcile if changed in the Mackerel web UI.
- The one-time `Update` sweep caused by `maxCheckAttempts` defaulting to `1`.
- Verbatim `make lint` and `make test` output from Step 1.
- Verbatim API request and response evidence from Step 2, with any API key redacted.
- `Close #21` and a link to the follow-up issue from Step 3.

Assign the user with `--assignee SlashNephy`.

If any item from Step 2 could not be verified, open the PR as a draft with `--draft` and say which item is outstanding.

---

## Notes for the implementer

**Why `new(200)` and not a local variable.** Go 1.26 lets `new` take a literal, so `new(200)` is an `*int` pointing at 200. The repository forbids adding a `ptr` helper. If a task shows `new(uint64(3))`, the conversion is what fixes the type — `new(3)` would give `*int`.

**Why `maxCheckAttempts` uses `uint64FromIntPtr(&desired.MaxCheckAttempts)`.** The existing helper takes a pointer so that nil maps to zero. `MaxCheckAttempts` is a plain `int`, so taking its address reuses the negative-value guard without a new helper. A negative value cannot reach here through the CRD, whose minimum is 1, but the provider is also reachable from tests and future callers.

**If a test in `internal/controller` starts failing.** That package uses envtest and needs `KUBEBUILDER_ASSETS`. `mise exec -- make test` sets it; a bare `go test ./...` does not.
