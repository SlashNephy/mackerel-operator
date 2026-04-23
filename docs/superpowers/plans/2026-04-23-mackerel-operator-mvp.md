# Mackerel Operator MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go/Kubebuilder Kubernetes Operator that reconciles `ExternalMonitor` CRDs into Mackerel external URL monitor configurations.

**Architecture:** Use controller-runtime for the Kubernetes control loop, keep Mackerel API access behind a provider interface, and isolate source conversion, ownership marker handling, hashing, planning, and status updates into small packages. The MVP reconciles only the `ExternalMonitor` CRD, while the `Source` boundary leaves room for future Ingress and HTTPRoute annotation sources.

**Tech Stack:** Go, Kubebuilder, controller-runtime, Mackerel Go client, Kubernetes CRDs/RBAC, Go unit tests, controller-runtime envtest.

---

## File Structure

Create or modify these files during implementation:

- `go.mod`: module `github.com/SlashNephy/mackerel-operator` and dependencies.
- `go.sum`: generated dependency checksums.
- `Makefile`: generated Kubebuilder targets plus test/build helpers.
- `PROJECT`: Kubebuilder project metadata.
- `cmd/main.go`: manager setup, flags, Mackerel provider initialization, controller registration.
- `api/v1alpha1/externalmonitor_types.go`: CRD spec/status types, markers, constants.
- `api/v1alpha1/groupversion_info.go`: API group registration.
- `api/v1alpha1/zz_generated.deepcopy.go`: generated deepcopy code.
- `internal/monitor/model.go`: internal desired/actual external monitor model.
- `internal/monitor/hash.go`: canonical desired-state hashing.
- `internal/monitor/hash_test.go`: hash tests.
- `internal/ownership/marker.go`: Mackerel memo marker parse/build/remove logic.
- `internal/ownership/marker_test.go`: marker tests.
- `internal/source/source.go`: `Source` interface.
- `internal/source/externalmonitor.go`: `ExternalMonitor` to desired monitor conversion.
- `internal/source/externalmonitor_test.go`: source conversion tests.
- `internal/planner/planner.go`: create/update/noop/delete/ownership-lost decisions.
- `internal/planner/planner_test.go`: planner tests.
- `internal/provider/provider.go`: provider interface and errors.
- `internal/provider/mackerel.go`: Mackerel API client adapter.
- `internal/status/externalmonitor.go`: condition and status helper functions.
- `internal/status/externalmonitor_test.go`: status helper tests.
- `internal/controller/externalmonitor_controller.go`: reconciler and finalizer flow.
- `internal/controller/externalmonitor_controller_test.go`: envtest/fake provider reconcile tests.
- `config/crd/bases/mackerel.starry.blue_externalmonitors.yaml`: generated CRD.
- `config/rbac/role.yaml`: generated RBAC.
- `config/manager/manager.yaml`: generated manager deployment.
- `config/samples/mackerel_v1alpha1_externalmonitor.yaml`: sample CR.
- `README.md`: minimal usage and development instructions.

## Task 1: Scaffold The Kubebuilder Project

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `Makefile`
- Create: `PROJECT`
- Create: `cmd/main.go`
- Create: `api/v1alpha1/externalmonitor_types.go`
- Create: `api/v1alpha1/groupversion_info.go`
- Create: `internal/controller/externalmonitor_controller.go`
- Create: `config/**`

- [ ] **Step 1: Initialize the Kubebuilder project**

Run:

```bash
kubebuilder init \
  --domain starry.blue \
  --repo github.com/SlashNephy/mackerel-operator \
  --owner "SlashNephy"
```

Expected: `go.mod`, `PROJECT`, `Makefile`, `cmd/main.go`, and `config/` are created.

- [ ] **Step 2: Create the API and controller scaffold**

Run:

```bash
kubebuilder create api \
  --group mackerel \
  --version v1alpha1 \
  --kind ExternalMonitor \
  --resource \
  --controller
```

When prompted, answer `y` for resource and `y` for controller.

Expected: `api/v1alpha1/externalmonitor_types.go` and `internal/controller/externalmonitor_controller.go` are created.

- [ ] **Step 3: Verify the generated project compiles before custom edits**

Run:

```bash
go test ./...
```

Expected: generated tests pass or report no test files. This command downloads Go modules when the module cache is empty.

- [ ] **Step 4: Commit the scaffold**

Run:

```bash
git add go.mod go.sum Makefile PROJECT cmd api internal config
git commit -m "chore: scaffold kubebuilder project"
```

Expected: one commit containing only generated scaffold files.

## Task 2: Define ExternalMonitor API Types

**Files:**
- Modify: `api/v1alpha1/externalmonitor_types.go`
- Modify: `config/samples/mackerel_v1alpha1_externalmonitor.yaml`
- Generate: `api/v1alpha1/zz_generated.deepcopy.go`
- Generate: `config/crd/bases/mackerel.starry.blue_externalmonitors.yaml`

- [ ] **Step 1: Replace generated spec/status with the MVP API**

Edit `api/v1alpha1/externalmonitor_types.go` so the relevant type definitions are:

```go
type ExternalMonitorSpec struct {
	Name                            string `json:"name,omitempty"`
	Service                         string `json:"service,omitempty"`
	URL                             string `json:"url"`
	Method                          string `json:"method,omitempty"`
	NotificationInterval            *int   `json:"notificationInterval,omitempty"`
	ExpectedStatusCode              *int   `json:"expectedStatusCode,omitempty"`
	ContainsString                  string `json:"containsString,omitempty"`
	ResponseTimeWarning             *int   `json:"responseTimeWarning,omitempty"`
	ResponseTimeCritical            *int   `json:"responseTimeCritical,omitempty"`
	CertificationExpirationWarning  *int   `json:"certificationExpirationWarning,omitempty"`
	CertificationExpirationCritical *int   `json:"certificationExpirationCritical,omitempty"`
	Memo                            string `json:"memo,omitempty"`
}

type ExternalMonitorStatus struct {
	MonitorID           string             `json:"monitorID,omitempty"`
	ObservedGeneration  int64              `json:"observedGeneration,omitempty"`
	LastSyncedAt        *metav1.Time       `json:"lastSyncedAt,omitempty"`
	LastAppliedHash     string             `json:"lastAppliedHash,omitempty"`
	URL                 string             `json:"url,omitempty"`
	MackerelMonitorName string             `json:"mackerelMonitorName,omitempty"`
	Conditions          []metav1.Condition `json:"conditions,omitempty"`
}
```

Ensure `metav1` is imported.

- [ ] **Step 2: Add Kubebuilder validation markers**

Add these markers to the fields in `ExternalMonitorSpec`:

```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:Pattern=`^https?://.+`
URL string `json:"url"`

// +kubebuilder:validation:Enum=GET;POST;PUT;DELETE
// +kubebuilder:default=GET
Method string `json:"method,omitempty"`

// +kubebuilder:validation:Minimum=10
NotificationInterval *int `json:"notificationInterval,omitempty"`

// +kubebuilder:validation:Minimum=100
// +kubebuilder:validation:Maximum=599
ExpectedStatusCode *int `json:"expectedStatusCode,omitempty"`

// +kubebuilder:validation:Minimum=0
ResponseTimeWarning *int `json:"responseTimeWarning,omitempty"`

// +kubebuilder:validation:Minimum=0
ResponseTimeCritical *int `json:"responseTimeCritical,omitempty"`

// +kubebuilder:validation:Minimum=0
CertificationExpirationWarning *int `json:"certificationExpirationWarning,omitempty"`

// +kubebuilder:validation:Minimum=0
CertificationExpirationCritical *int `json:"certificationExpirationCritical,omitempty"`

// +kubebuilder:validation:MaxLength=1900
Memo string `json:"memo,omitempty"`
```

Add this marker above `ExternalMonitorStatus.Conditions`:

```go
// +listType=map
// +listMapKey=type
```

- [ ] **Step 3: Add printer columns and status subresource**

Add markers above `type ExternalMonitor struct`:

```go
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="MonitorID",type=string,JSONPath=`.status.monitorID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

- [ ] **Step 4: Update the sample manifest**

Replace `config/samples/mackerel_v1alpha1_externalmonitor.yaml` with:

```yaml
apiVersion: mackerel.starry.blue/v1alpha1
kind: ExternalMonitor
metadata:
  labels:
    app.kubernetes.io/name: mackerel-operator
    app.kubernetes.io/managed-by: kustomize
  name: api-health
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

- [ ] **Step 5: Generate manifests and deepcopy code**

Run:

```bash
make generate manifests
```

Expected: generated deepcopy and CRD YAML include the spec/status fields and validation.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit the API**

Run:

```bash
git add api config
git commit -m "feat: define external monitor api"
```

Expected: one commit containing API, sample, and generated manifests.

## Task 3: Add Internal Monitor Model And Hashing

**Files:**
- Create: `internal/monitor/model.go`
- Create: `internal/monitor/hash.go`
- Create: `internal/monitor/hash_test.go`

- [ ] **Step 1: Write failing hash tests**

Create `internal/monitor/hash_test.go`:

```go
package monitor

import "testing"

func TestHashDesiredDefaultLength(t *testing.T) {
	desired := DesiredExternalMonitor{
		Name: "API health check",
		URL:  "https://api.example.com/healthz",
	}

	got, err := HashDesired(desired, 7)
	if err != nil {
		t.Fatalf("HashDesired returned error: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("len(hash) = %d, want 7", len(got))
	}
}

func TestHashDesiredIsStable(t *testing.T) {
	desired := DesiredExternalMonitor{
		Name:               "API health check",
		URL:                "https://api.example.com/healthz",
		Method:             "GET",
		ExpectedStatusCode: intPtr(200),
	}

	first, err := HashDesired(desired, 12)
	if err != nil {
		t.Fatalf("HashDesired first returned error: %v", err)
	}
	second, err := HashDesired(desired, 12)
	if err != nil {
		t.Fatalf("HashDesired second returned error: %v", err)
	}
	if first != second {
		t.Fatalf("hash changed: first=%q second=%q", first, second)
	}
}

func TestHashDesiredRejectsInvalidLength(t *testing.T) {
	_, err := HashDesired(DesiredExternalMonitor{}, 0)
	if err == nil {
		t.Fatal("HashDesired length 0 error = nil, want error")
	}
}

func intPtr(v int) *int {
	return &v
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/monitor
```

Expected: FAIL because `DesiredExternalMonitor` and `HashDesired` are undefined.

- [ ] **Step 3: Implement monitor model**

Create `internal/monitor/model.go`:

```go
package monitor

type DesiredExternalMonitor struct {
	Name                            string `json:"name,omitempty"`
	Service                         string `json:"service,omitempty"`
	URL                             string `json:"url"`
	Method                          string `json:"method,omitempty"`
	NotificationInterval            *int   `json:"notificationInterval,omitempty"`
	ExpectedStatusCode              *int   `json:"expectedStatusCode,omitempty"`
	ContainsString                  string `json:"containsString,omitempty"`
	ResponseTimeWarning             *int   `json:"responseTimeWarning,omitempty"`
	ResponseTimeCritical            *int   `json:"responseTimeCritical,omitempty"`
	CertificationExpirationWarning  *int   `json:"certificationExpirationWarning,omitempty"`
	CertificationExpirationCritical *int   `json:"certificationExpirationCritical,omitempty"`
	Memo                            string `json:"memo,omitempty"`
	Resource                        string `json:"resource"`
	Owner                           string `json:"owner"`
	Hash                            string `json:"hash,omitempty"`
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
	ResponseTimeWarning             *int
	ResponseTimeCritical            *int
	CertificationExpirationWarning  *int
	CertificationExpirationCritical *int
	Memo                            string
}
```

- [ ] **Step 4: Implement hashing**

Create `internal/monitor/hash.go`:

```go
package monitor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func HashDesired(desired DesiredExternalMonitor, length int) (string, error) {
	if length < 1 || length > sha256.Size*2 {
		return "", fmt.Errorf("hash length must be between 1 and %d: %d", sha256.Size*2, length)
	}

	desired.Hash = ""
	data, err := json.Marshal(desired)
	if err != nil {
		return "", fmt.Errorf("marshal desired monitor: %w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:length], nil
}
```

- [ ] **Step 5: Run monitor tests**

Run:

```bash
go test ./internal/monitor
```

Expected: PASS.

- [ ] **Step 6: Commit monitor model and hashing**

Run:

```bash
git add internal/monitor
git commit -m "feat: add desired monitor hashing"
```

Expected: one commit with model and hash tests.

## Task 4: Add Ownership Marker Handling

**Files:**
- Create: `internal/ownership/marker.go`
- Create: `internal/ownership/marker_test.go`

- [ ] **Step 1: Write failing ownership tests**

Create `internal/ownership/marker_test.go`:

```go
package ownership

import "testing"

func TestBuildMarker(t *testing.T) {
	got := BuildMarker(Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := "<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	if got != want {
		t.Fatalf("BuildMarker() = %q, want %q", got, want)
	}
}

func TestParseMarker(t *testing.T) {
	memo := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	got, ok := ParseMarker(memo)
	if !ok {
		t.Fatal("ParseMarker ok = false, want true")
	}
	if got.Resource != "externalmonitor/default/api-health" || got.Owner != "prod" || got.Hash != "deadbee" {
		t.Fatalf("ParseMarker() = %#v", got)
	}
}

func TestApplyMarkerPreservesHumanMemo(t *testing.T) {
	got := ApplyMarker("human memo", Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "deadbee",
	})
	want := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	if got != want {
		t.Fatalf("ApplyMarker() = %q, want %q", got, want)
	}
}

func TestApplyMarkerReplacesExistingMarker(t *testing.T) {
	memo := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=oldhash -->"
	got := ApplyMarker(memo, Marker{
		Resource: "externalmonitor/default/api-health",
		Owner:    "prod",
		Hash:     "newhash",
	})
	if got != "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=newhash -->" {
		t.Fatalf("ApplyMarker() = %q", got)
	}
}

func TestRemoveMarker(t *testing.T) {
	memo := "human memo\n<!-- heritage=mackerel-operator,resource=externalmonitor/default/api-health,owner=prod,hash=deadbee -->"
	got := RemoveMarker(memo)
	if got != "human memo" {
		t.Fatalf("RemoveMarker() = %q, want human memo", got)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/ownership
```

Expected: FAIL because ownership functions are undefined.

- [ ] **Step 3: Implement marker handling**

Create `internal/ownership/marker.go`:

```go
package ownership

import (
	"fmt"
	"regexp"
	"strings"
)

const Heritage = "mackerel-operator"

var markerPattern = regexp.MustCompile(`<!--\s*heritage=mackerel-operator,resource=([^,\s]+),owner=([^,\s]+),hash=([^,\s]+)\s*-->`)

type Marker struct {
	Resource string
	Owner    string
	Hash     string
}

func BuildMarker(marker Marker) string {
	return fmt.Sprintf("<!-- heritage=%s,resource=%s,owner=%s,hash=%s -->", Heritage, marker.Resource, marker.Owner, marker.Hash)
}

func ParseMarker(memo string) (Marker, bool) {
	matches := markerPattern.FindStringSubmatch(memo)
	if matches == nil {
		return Marker{}, false
	}
	return Marker{
		Resource: matches[1],
		Owner:    matches[2],
		Hash:     matches[3],
	}, true
}

func ApplyMarker(memo string, marker Marker) string {
	base := RemoveMarker(memo)
	if strings.TrimSpace(base) == "" {
		return BuildMarker(marker)
	}
	return strings.TrimRight(base, "\n") + "\n" + BuildMarker(marker)
}

func RemoveMarker(memo string) string {
	without := markerPattern.ReplaceAllString(memo, "")
	return strings.TrimRight(strings.TrimSpace(without), "\n")
}
```

- [ ] **Step 4: Run ownership tests**

Run:

```bash
go test ./internal/ownership
```

Expected: PASS.

- [ ] **Step 5: Commit ownership marker code**

Run:

```bash
git add internal/ownership
git commit -m "feat: add mackerel ownership markers"
```

Expected: one commit with marker implementation and tests.

## Task 5: Add ExternalMonitor Source Conversion

**Files:**
- Create: `internal/source/source.go`
- Create: `internal/source/externalmonitor.go`
- Create: `internal/source/externalmonitor_test.go`

- [ ] **Step 1: Write failing source conversion tests**

Create `internal/source/externalmonitor_test.go`:

```go
package source

import (
	"testing"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExternalMonitorSourceBuildsDesiredMonitor(t *testing.T) {
	interval := 10
	expectedStatus := 200
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-health",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			Name:                 "API health check",
			Service:              "my-service",
			URL:                  "https://api.example.com/healthz",
			Method:               "GET",
			NotificationInterval: &interval,
			ExpectedStatusCode:   &expectedStatus,
			Memo:                 "human memo",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	if err != nil {
		t.Fatalf("FromExternalMonitor returned error: %v", err)
	}
	if got.Name != "API health check" {
		t.Fatalf("Name = %q", got.Name)
	}
	if got.Resource != "externalmonitor/default/api-health" {
		t.Fatalf("Resource = %q", got.Resource)
	}
	if got.Owner != "prod" {
		t.Fatalf("Owner = %q", got.Owner)
	}
	if len(got.Hash) != 7 {
		t.Fatalf("Hash length = %d, want 7", len(got.Hash))
	}
}

func TestExternalMonitorSourceDefaultsNameAndMethod(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-health",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	if err != nil {
		t.Fatalf("FromExternalMonitor returned error: %v", err)
	}
	if got.Name != "default/api-health" {
		t.Fatalf("Name = %q, want default/api-health", got.Name)
	}
	if got.Method != "GET" {
		t.Fatalf("Method = %q, want GET", got.Method)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/source
```

Expected: FAIL because `ExternalMonitorSource` is undefined.

- [ ] **Step 3: Implement source interface**

Create `internal/source/source.go`:

```go
package source

import "github.com/SlashNephy/mackerel-operator/internal/monitor"

type Source interface {
	DesiredMonitors() ([]monitor.DesiredExternalMonitor, error)
}
```

- [ ] **Step 4: Implement ExternalMonitor conversion**

Create `internal/source/externalmonitor.go`:

```go
package source

import (
	"fmt"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
)

type ExternalMonitorSource struct {
	OwnerID    string
	HashLength int
}

func (s ExternalMonitorSource) FromExternalMonitor(cr *mackerelv1alpha1.ExternalMonitor) (monitor.DesiredExternalMonitor, error) {
	name := cr.Spec.Name
	if name == "" {
		name = fmt.Sprintf("%s/%s", cr.Namespace, cr.Name)
	}

	method := cr.Spec.Method
	if method == "" {
		method = "GET"
	}

	desired := monitor.DesiredExternalMonitor{
		Name:                            name,
		Service:                         cr.Spec.Service,
		URL:                             cr.Spec.URL,
		Method:                          method,
		NotificationInterval:            cr.Spec.NotificationInterval,
		ExpectedStatusCode:              cr.Spec.ExpectedStatusCode,
		ContainsString:                  cr.Spec.ContainsString,
		ResponseTimeWarning:             cr.Spec.ResponseTimeWarning,
		ResponseTimeCritical:            cr.Spec.ResponseTimeCritical,
		CertificationExpirationWarning:  cr.Spec.CertificationExpirationWarning,
		CertificationExpirationCritical: cr.Spec.CertificationExpirationCritical,
		Memo:                            cr.Spec.Memo,
		Resource:                        fmt.Sprintf("externalmonitor/%s/%s", cr.Namespace, cr.Name),
		Owner:                           s.OwnerID,
	}

	hash, err := monitor.HashDesired(desired, s.HashLength)
	if err != nil {
		return monitor.DesiredExternalMonitor{}, err
	}
	desired.Hash = hash
	return desired, nil
}
```

- [ ] **Step 5: Run source tests**

Run:

```bash
go test ./internal/source
```

Expected: PASS.

- [ ] **Step 6: Commit source conversion**

Run:

```bash
git add internal/source
git commit -m "feat: convert externalmonitor resources"
```

Expected: one commit with source interface and conversion tests.

## Task 6: Add Planner Decisions

**Files:**
- Create: `internal/planner/planner.go`
- Create: `internal/planner/planner_test.go`

- [ ] **Step 1: Write failing planner tests**

Create `internal/planner/planner_test.go`:

```go
package planner

import (
	"testing"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
)

func TestPlanCreateWhenActualMissing(t *testing.T) {
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{Name: "api", URL: "https://example.com", Owner: "prod", Resource: "externalmonitor/default/api", Hash: "deadbee"},
	})
	if decision.Action != ActionCreate {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionCreate)
	}
}

func TestPlanNoopWhenHashMatches(t *testing.T) {
	actual := monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Name: "api",
		URL:  "https://example.com",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/api", Owner: "prod", Hash: "deadbee"}),
	}
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{Name: "api", URL: "https://example.com", Owner: "prod", Resource: "externalmonitor/default/api", Hash: "deadbee"},
		Actual:  &actual,
	})
	if decision.Action != ActionNoop {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionNoop)
	}
}

func TestPlanUpdateWhenOwnedHashDiffers(t *testing.T) {
	actual := monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Name: "api",
		URL:  "https://example.com",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/api", Owner: "prod", Hash: "oldhash"}),
	}
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{Name: "api", URL: "https://example.com", Owner: "prod", Resource: "externalmonitor/default/api", Hash: "deadbee"},
		Actual:  &actual,
	})
	if decision.Action != ActionUpdate {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionUpdate)
	}
}

func TestPlanRestoreMarkerWhenMissingButActualMatches(t *testing.T) {
	actual := monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Name: "api",
		URL:  "https://example.com",
		Memo: "human memo",
	}
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{Name: "api", URL: "https://example.com", Owner: "prod", Resource: "externalmonitor/default/api", Hash: "deadbee"},
		Actual:  &actual,
	})
	if decision.Action != ActionRestoreMarker {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionRestoreMarker)
	}
}

func TestPlanOwnershipLostWhenMissingMarkerAndActualDiffers(t *testing.T) {
	actual := monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Name: "api",
		URL:  "https://changed.example.com",
		Memo: "human memo",
	}
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{Name: "api", URL: "https://example.com", Owner: "prod", Resource: "externalmonitor/default/api", Hash: "deadbee"},
		Actual:  &actual,
	})
	if decision.Action != ActionOwnershipLost {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionOwnershipLost)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/planner
```

Expected: FAIL because planner types are undefined.

- [ ] **Step 3: Implement planner**

Create `internal/planner/planner.go`:

```go
package planner

import (
	"reflect"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
)

type Action string

const (
	ActionCreate         Action = "Create"
	ActionUpdate         Action = "Update"
	ActionNoop           Action = "Noop"
	ActionRestoreMarker  Action = "RestoreMarker"
	ActionOwnershipLost  Action = "OwnershipLost"
	ActionDelete         Action = "Delete"
	ActionSkipDelete     Action = "SkipDelete"
)

type PlanInput struct {
	Desired monitor.DesiredExternalMonitor
	Actual  *monitor.ActualExternalMonitor
}

type Decision struct {
	Action Action
	Reason string
}

func Plan(input PlanInput) Decision {
	if input.Actual == nil {
		return Decision{Action: ActionCreate, Reason: "actual monitor is missing"}
	}

	marker, ok := ownership.ParseMarker(input.Actual.Memo)
	if !ok {
		if actualMatchesDesired(input.Desired, *input.Actual) {
			return Decision{Action: ActionRestoreMarker, Reason: "ownership marker is missing but monitor matches desired state"}
		}
		return Decision{Action: ActionOwnershipLost, Reason: "ownership marker is missing and monitor differs from desired state"}
	}

	if marker.Owner != input.Desired.Owner || marker.Resource != input.Desired.Resource {
		return Decision{Action: ActionOwnershipLost, Reason: "ownership marker belongs to another owner or resource"}
	}

	if marker.Hash == input.Desired.Hash && actualMatchesDesired(input.Desired, *input.Actual) {
		return Decision{Action: ActionNoop, Reason: "actual monitor matches desired state"}
	}

	return Decision{Action: ActionUpdate, Reason: "owned monitor differs from desired state"}
}

func PlanDelete(actual *monitor.ActualExternalMonitor, owner, resource, policy string) Decision {
	if actual == nil {
		return Decision{Action: ActionSkipDelete, Reason: "actual monitor is missing"}
	}
	if policy != "sync" {
		return Decision{Action: ActionSkipDelete, Reason: "policy is not sync"}
	}
	marker, ok := ownership.ParseMarker(actual.Memo)
	if !ok || marker.Owner != owner || marker.Resource != resource {
		return Decision{Action: ActionSkipDelete, Reason: "monitor is not owned by this controller"}
	}
	return Decision{Action: ActionDelete, Reason: "policy is sync and monitor is owned"}
}

func actualMatchesDesired(desired monitor.DesiredExternalMonitor, actual monitor.ActualExternalMonitor) bool {
	return desired.Name == actual.Name &&
		desired.Service == actual.Service &&
		desired.URL == actual.URL &&
		desired.Method == actual.Method &&
		reflect.DeepEqual(desired.NotificationInterval, actual.NotificationInterval) &&
		reflect.DeepEqual(desired.ExpectedStatusCode, actual.ExpectedStatusCode) &&
		desired.ContainsString == actual.ContainsString &&
		reflect.DeepEqual(desired.ResponseTimeWarning, actual.ResponseTimeWarning) &&
		reflect.DeepEqual(desired.ResponseTimeCritical, actual.ResponseTimeCritical) &&
		reflect.DeepEqual(desired.CertificationExpirationWarning, actual.CertificationExpirationWarning) &&
		reflect.DeepEqual(desired.CertificationExpirationCritical, actual.CertificationExpirationCritical)
}
```

- [ ] **Step 4: Run planner tests**

Run:

```bash
go test ./internal/planner
```

Expected: PASS.

- [ ] **Step 5: Add deletion policy tests**

Append to `internal/planner/planner_test.go`:

```go
func TestPlanDeleteSyncOwned(t *testing.T) {
	actual := &monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/api", Owner: "prod", Hash: "deadbee"}),
	}
	decision := PlanDelete(actual, "prod", "externalmonitor/default/api", "sync")
	if decision.Action != ActionDelete {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionDelete)
	}
}

func TestPlanDeleteUpsertOnlySkips(t *testing.T) {
	actual := &monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/api", Owner: "prod", Hash: "deadbee"}),
	}
	decision := PlanDelete(actual, "prod", "externalmonitor/default/api", "upsert-only")
	if decision.Action != ActionSkipDelete {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionSkipDelete)
	}
}
```

- [ ] **Step 6: Run planner tests again**

Run:

```bash
go test ./internal/planner
```

Expected: PASS.

- [ ] **Step 7: Commit planner**

Run:

```bash
git add internal/planner
git commit -m "feat: plan external monitor changes"
```

Expected: one commit with planner implementation and tests.

## Task 7: Add Provider Interface And Mackerel Adapter

**Files:**
- Create: `internal/provider/provider.go`
- Create: `internal/provider/mackerel.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add provider interface**

Create `internal/provider/provider.go`:

```go
package provider

import (
	"context"
	"errors"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
)

var (
	ErrNotFound    = errors.New("monitor not found")
	ErrRateLimited = errors.New("mackerel rate limited")
	ErrInvalid     = errors.New("invalid mackerel monitor")
)

type ExternalMonitorProvider interface {
	GetExternalMonitor(ctx context.Context, id string) (*monitor.ActualExternalMonitor, error)
	ListExternalMonitors(ctx context.Context) ([]monitor.ActualExternalMonitor, error)
	CreateExternalMonitor(ctx context.Context, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error)
	UpdateExternalMonitor(ctx context.Context, id string, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error)
	DeleteExternalMonitor(ctx context.Context, id string) error
}
```

- [ ] **Step 2: Add Mackerel client dependency**

Run:

```bash
go get github.com/mackerelio/mackerel-client-go@latest
```

Expected: `go.mod` and `go.sum` include `github.com/mackerelio/mackerel-client-go`.

- [ ] **Step 3: Implement provider adapter**

Create `internal/provider/mackerel.go`:

```go
package provider

import (
	"context"
	"errors"
	"fmt"

	mackerel "github.com/mackerelio/mackerel-client-go"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
)

type MackerelProvider struct {
	client *mackerel.Client
}

func NewMackerelProvider(apiKey string) (*MackerelProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("MACKEREL_APIKEY is required")
	}
	return &MackerelProvider{client: mackerel.NewClient(apiKey)}, nil
}

func (p *MackerelProvider) GetExternalMonitor(ctx context.Context, id string) (*monitor.ActualExternalMonitor, error) {
	mon, err := p.client.GetMonitorContext(ctx, id)
	if err != nil {
		return nil, mapMackerelError(err)
	}
	external, ok := mon.(*mackerel.MonitorExternalHTTP)
	if !ok {
		return nil, fmt.Errorf("%w: monitor %s is not external", ErrInvalid, id)
	}
	actual := fromMackerelExternal(external)
	return &actual, nil
}

func (p *MackerelProvider) ListExternalMonitors(ctx context.Context) ([]monitor.ActualExternalMonitor, error) {
	monitors, err := p.client.FindMonitorsContext(ctx)
	if err != nil {
		return nil, mapMackerelError(err)
	}
	var out []monitor.ActualExternalMonitor
	for _, mon := range monitors {
		external, ok := mon.(*mackerel.MonitorExternalHTTP)
		if !ok {
			continue
		}
		out = append(out, fromMackerelExternal(external))
	}
	return out, nil
}

func (p *MackerelProvider) CreateExternalMonitor(ctx context.Context, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	payload := toMackerelExternal("", desired, memo)
	created, err := p.client.CreateMonitorContext(ctx, payload)
	if err != nil {
		return nil, mapMackerelError(err)
	}
	external, ok := created.(*mackerel.MonitorExternalHTTP)
	if !ok {
		return nil, fmt.Errorf("%w: created monitor is not external", ErrInvalid)
	}
	actual := fromMackerelExternal(external)
	return &actual, nil
}

func (p *MackerelProvider) UpdateExternalMonitor(ctx context.Context, id string, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	current, err := p.GetExternalMonitor(ctx, id)
	if err != nil {
		return nil, err
	}
	payload := toMackerelExternal(current.ID, desired, memo)
	updated, err := p.client.UpdateMonitorContext(ctx, id, payload)
	if err != nil {
		return nil, mapMackerelError(err)
	}
	external, ok := updated.(*mackerel.MonitorExternalHTTP)
	if !ok {
		return nil, fmt.Errorf("%w: updated monitor is not external", ErrInvalid)
	}
	actual := fromMackerelExternal(external)
	return &actual, nil
}

func (p *MackerelProvider) DeleteExternalMonitor(ctx context.Context, id string) error {
	_, err := p.client.DeleteMonitorContext(ctx, id)
	return mapMackerelError(err)
}

func fromMackerelExternal(mon *mackerel.MonitorExternalHTTP) monitor.ActualExternalMonitor {
	return monitor.ActualExternalMonitor{
		ID:                              mon.ID,
		Name:                            mon.Name,
		Service:                         mon.Service,
		URL:                             mon.URL,
		Method:                          mon.Method,
		NotificationInterval:            uint64ToIntPtr(mon.NotificationInterval),
		ExpectedStatusCode:              mon.ExpectedStatusCode,
		ContainsString:                  mon.ContainsString,
		ResponseTimeWarning:             float64ToIntPtr(mon.ResponseTimeWarning),
		ResponseTimeCritical:            float64ToIntPtr(mon.ResponseTimeCritical),
		CertificationExpirationWarning:  uint64PtrToIntPtr(mon.CertificationExpirationWarning),
		CertificationExpirationCritical: uint64PtrToIntPtr(mon.CertificationExpirationCritical),
		Memo:                            mon.Memo,
	}
}

func toMackerelExternal(id string, desired monitor.DesiredExternalMonitor, memo string) *mackerel.MonitorExternalHTTP {
	return &mackerel.MonitorExternalHTTP{
		ID:                              id,
		Name:                            desired.Name,
		Memo:                            memo,
		Type:                            "external",
		Method:                          desired.Method,
		URL:                             desired.URL,
		Service:                         desired.Service,
		NotificationInterval:            intPtrToUint64(desired.NotificationInterval),
		ResponseTimeWarning:             intPtrToFloat64(desired.ResponseTimeWarning),
		ResponseTimeCritical:            intPtrToFloat64(desired.ResponseTimeCritical),
		ContainsString:                  desired.ContainsString,
		ExpectedStatusCode:              desired.ExpectedStatusCode,
		CertificationExpirationWarning:  intPtrToUint64Ptr(desired.CertificationExpirationWarning),
		CertificationExpirationCritical: intPtrToUint64Ptr(desired.CertificationExpirationCritical),
	}
}

func intPtrToUint64(v *int) uint64 {
	if v == nil {
		return 0
	}
	return uint64(*v)
}

func intPtrToUint64Ptr(v *int) *uint64 {
	if v == nil {
		return nil
	}
	out := uint64(*v)
	return &out
}

func intPtrToFloat64(v *int) *float64 {
	if v == nil {
		return nil
	}
	out := float64(*v)
	return &out
}

func uint64ToIntPtr(v uint64) *int {
	if v == 0 {
		return nil
	}
	out := int(v)
	return &out
}

func float64ToIntPtr(v *float64) *int {
	if v == nil {
		return nil
	}
	out := int(*v)
	return &out
}

func uint64PtrToIntPtr(v *uint64) *int {
	if v == nil {
		return nil
	}
	out := int(*v)
	return &out
}

func mapMackerelError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *mackerel.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 404:
			return ErrNotFound
		case 429:
			return fmt.Errorf("%w: %v", ErrRateLimited, err)
		case 400:
			return fmt.Errorf("%w: %v", ErrInvalid, err)
		default:
			return err
		}
	}
	return err
}
```

- [ ] **Step 4: Compile provider code**

Run:

```bash
go test ./internal/provider
```

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit provider**

Run:

```bash
git add go.mod go.sum internal/provider
git commit -m "feat: add mackerel provider adapter"
```

Expected: one commit with provider interface and adapter.

## Task 8: Add Status Helpers

**Files:**
- Create: `internal/status/externalmonitor.go`
- Create: `internal/status/externalmonitor_test.go`

- [ ] **Step 1: Write failing status tests**

Create `internal/status/externalmonitor_test.go`:

```go
package status

import (
	"testing"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMarkReady(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
	}
	MarkReady(cr, SyncResult{
		MonitorID: "mon-1",
		Hash:      "deadbee",
		URL:       "https://api.example.com/healthz",
		Name:      "API health check",
	})

	if cr.Status.MonitorID != "mon-1" {
		t.Fatalf("MonitorID = %q", cr.Status.MonitorID)
	}
	if cr.Status.ObservedGeneration != 3 {
		t.Fatalf("ObservedGeneration = %d", cr.Status.ObservedGeneration)
	}
	cond := findCondition(cr, "Ready")
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "Synced" {
		t.Fatalf("Ready condition = %#v", cond)
	}
}

func TestMarkOwnershipLost(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{}
	MarkOwnershipLost(cr, "marker missing and monitor differs")

	cond := findCondition(cr, "Ready")
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != "OwnershipLost" {
		t.Fatalf("Ready condition = %#v", cond)
	}
}

func findCondition(cr *mackerelv1alpha1.ExternalMonitor, conditionType string) *metav1.Condition {
	for i := range cr.Status.Conditions {
		if cr.Status.Conditions[i].Type == conditionType {
			return &cr.Status.Conditions[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./internal/status
```

Expected: FAIL because status helpers are undefined.

- [ ] **Step 3: Implement status helpers**

Create `internal/status/externalmonitor.go`:

```go
package status

import (
	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SyncResult struct {
	MonitorID string
	Hash      string
	URL       string
	Name      string
}

func MarkReady(cr *mackerelv1alpha1.ExternalMonitor, result SyncResult) {
	now := metav1.Now()
	cr.Status.MonitorID = result.MonitorID
	cr.Status.ObservedGeneration = cr.Generation
	cr.Status.LastSyncedAt = &now
	cr.Status.LastAppliedHash = result.Hash
	cr.Status.URL = result.URL
	cr.Status.MackerelMonitorName = result.Name
	setCondition(cr, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Synced",
		Message:            "Mackerel external monitor is synchronized",
		ObservedGeneration: cr.Generation,
	})
}

func MarkOwnershipLost(cr *mackerelv1alpha1.ExternalMonitor, message string) {
	setCondition(cr, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "OwnershipLost",
		Message:            message,
		ObservedGeneration: cr.Generation,
	})
}

func MarkInvalidSpec(cr *mackerelv1alpha1.ExternalMonitor, message string) {
	setCondition(cr, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "InvalidSpec",
		Message:            message,
		ObservedGeneration: cr.Generation,
	})
}

func MarkError(cr *mackerelv1alpha1.ExternalMonitor, reason, message string) {
	setCondition(cr, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cr.Generation,
	})
}

func setCondition(cr *mackerelv1alpha1.ExternalMonitor, condition metav1.Condition) {
	condition.LastTransitionTime = metav1.Now()
	for i := range cr.Status.Conditions {
		if cr.Status.Conditions[i].Type == condition.Type {
			cr.Status.Conditions[i] = condition
			return
		}
	}
	cr.Status.Conditions = append(cr.Status.Conditions, condition)
}
```

- [ ] **Step 4: Run status tests**

Run:

```bash
go test ./internal/status
```

Expected: PASS.

- [ ] **Step 5: Commit status helpers**

Run:

```bash
git add internal/status
git commit -m "feat: add external monitor status helpers"
```

Expected: one commit with status helper implementation and tests.

## Task 9: Implement Reconciler With Fake-Provider Tests

**Files:**
- Modify: `internal/controller/externalmonitor_controller.go`
- Create: `internal/controller/externalmonitor_controller_test.go`

- [ ] **Step 1: Write fake-provider reconcile tests**

Create `internal/controller/externalmonitor_controller_test.go`:

```go
package controller

import (
	"context"
	"testing"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/provider"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileCreatesMonitor(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := mackerelv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
		},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr).WithStatusSubresource(cr).Build()
	fakeProvider := &fakeExternalProvider{}
	reconciler := &ExternalMonitorReconciler{
		Client:     k8sClient,
		Scheme:     scheme,
		Provider:   fakeProvider,
		OwnerID:    "prod",
		Policy:     "upsert-only",
		HashLength: 7,
	}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "api"}})
	if err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
	if len(fakeProvider.created) != 1 {
		t.Fatalf("created monitors = %d, want 1", len(fakeProvider.created))
	}
	var updated mackerelv1alpha1.ExternalMonitor
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "api"}, &updated); err != nil {
		t.Fatalf("Get updated CR: %v", err)
	}
	if updated.Status.MonitorID == "" {
		t.Fatal("Status.MonitorID is empty")
	}
}

type fakeExternalProvider struct {
	monitors map[string]monitor.ActualExternalMonitor
	created  []monitor.DesiredExternalMonitor
	deleted  []string
}

func (f *fakeExternalProvider) GetExternalMonitor(ctx context.Context, id string) (*monitor.ActualExternalMonitor, error) {
	if f.monitors == nil {
		return nil, provider.ErrNotFound
	}
	mon, ok := f.monitors[id]
	if !ok {
		return nil, provider.ErrNotFound
	}
	return &mon, nil
}

func (f *fakeExternalProvider) ListExternalMonitors(ctx context.Context) ([]monitor.ActualExternalMonitor, error) {
	out := make([]monitor.ActualExternalMonitor, 0, len(f.monitors))
	for _, mon := range f.monitors {
		out = append(out, mon)
	}
	return out, nil
}

func (f *fakeExternalProvider) CreateExternalMonitor(ctx context.Context, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	f.created = append(f.created, desired)
	actual := monitor.ActualExternalMonitor{
		ID:     "mon-1",
		Name:   desired.Name,
		URL:    desired.URL,
		Method: desired.Method,
		Memo:   memo,
	}
	if f.monitors == nil {
		f.monitors = map[string]monitor.ActualExternalMonitor{}
	}
	f.monitors[actual.ID] = actual
	return &actual, nil
}

func (f *fakeExternalProvider) UpdateExternalMonitor(ctx context.Context, id string, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	actual := monitor.ActualExternalMonitor{
		ID:     id,
		Name:   desired.Name,
		URL:    desired.URL,
		Method: desired.Method,
		Memo:   memo,
	}
	f.monitors[id] = actual
	return &actual, nil
}

func (f *fakeExternalProvider) DeleteExternalMonitor(ctx context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	delete(f.monitors, id)
	return nil
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test ./internal/controller
```

Expected: FAIL because `ExternalMonitorReconciler` does not have `Provider`, `OwnerID`, `Policy`, and `HashLength` fields or reconciliation logic.

- [ ] **Step 3: Implement reconciler fields and constants**

Modify `internal/controller/externalmonitor_controller.go` to add imports and fields:

```go
const externalMonitorFinalizer = "externalmonitor.mackerel.starry.blue/finalizer"

type ExternalMonitorReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Provider   provider.ExternalMonitorProvider
	OwnerID    string
	Policy     string
	HashLength int
}
```

Ensure imports include:

```go
import (
	"context"
	"errors"
	"fmt"
	"time"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
	"github.com/SlashNephy/mackerel-operator/internal/planner"
	"github.com/SlashNephy/mackerel-operator/internal/provider"
	"github.com/SlashNephy/mackerel-operator/internal/source"
	monitorstatus "github.com/SlashNephy/mackerel-operator/internal/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)
```

- [ ] **Step 4: Implement reconcile create/update flow**

Replace the generated `Reconcile` body with:

```go
func (r *ExternalMonitorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var cr mackerelv1alpha1.ExternalMonitor
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	src := source.ExternalMonitorSource{OwnerID: r.OwnerID, HashLength: r.HashLength}
	desired, err := src.FromExternalMonitor(&cr)
	if err != nil {
		monitorstatus.MarkInvalidSpec(&cr, err.Error())
		_ = r.Status().Update(ctx, &cr)
		return ctrl.Result{}, nil
	}

	if cr.DeletionTimestamp != nil {
		return r.reconcileDelete(ctx, &cr, desired)
	}

	if controllerutil.AddFinalizer(&cr, externalMonitorFinalizer) {
		if err := r.Update(ctx, &cr); err != nil {
			return ctrl.Result{}, err
		}
	}

	actual, err := r.findActual(ctx, &cr, desired)
	if err != nil {
		if errors.Is(err, provider.ErrRateLimited) {
			return ctrl.Result{RequeueAfter: defaultRequeueAfter}, err
		}
		monitorstatus.MarkError(&cr, "ProviderError", err.Error())
		_ = r.Status().Update(ctx, &cr)
		return ctrl.Result{}, err
	}

	decision := planner.Plan(planner.PlanInput{Desired: desired, Actual: actual})
	log.Info("planned external monitor reconciliation", "action", decision.Action, "reason", decision.Reason)

	var synced *monitor.ActualExternalMonitor
	switch decision.Action {
	case planner.ActionCreate:
		memo := ownership.ApplyMarker(desired.Memo, ownership.Marker{Resource: desired.Resource, Owner: desired.Owner, Hash: desired.Hash})
		synced, err = r.Provider.CreateExternalMonitor(ctx, desired, memo)
	case planner.ActionUpdate, planner.ActionRestoreMarker:
		memo := ownership.ApplyMarker(actual.Memo, ownership.Marker{Resource: desired.Resource, Owner: desired.Owner, Hash: desired.Hash})
		synced, err = r.Provider.UpdateExternalMonitor(ctx, actual.ID, desired, memo)
	case planner.ActionNoop:
		synced = actual
	case planner.ActionOwnershipLost:
		monitorstatus.MarkOwnershipLost(&cr, decision.Reason)
		return ctrl.Result{}, r.Status().Update(ctx, &cr)
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported planner action: %s", decision.Action)
	}
	if err != nil {
		monitorstatus.MarkError(&cr, "ProviderError", err.Error())
		_ = r.Status().Update(ctx, &cr)
		return ctrl.Result{}, err
	}

	monitorstatus.MarkReady(&cr, monitorstatus.SyncResult{
		MonitorID: synced.ID,
		Hash:      desired.Hash,
		URL:       synced.URL,
		Name:      synced.Name,
	})
	return ctrl.Result{}, r.Status().Update(ctx, &cr)
}
```

Add near the top:

```go
var defaultRequeueAfter = time.Minute
```

Ensure `time` is imported.

- [ ] **Step 5: Implement helper methods**

Add below `Reconcile`:

```go
func (r *ExternalMonitorReconciler) findActual(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor, desired monitor.DesiredExternalMonitor) (*monitor.ActualExternalMonitor, error) {
	if cr.Status.MonitorID != "" {
		actual, err := r.Provider.GetExternalMonitor(ctx, cr.Status.MonitorID)
		if err == nil {
			return actual, nil
		}
		if !errors.Is(err, provider.ErrNotFound) {
			return nil, err
		}
	}

	monitors, err := r.Provider.ListExternalMonitors(ctx)
	if err != nil {
		return nil, err
	}
	for i := range monitors {
		marker, ok := ownership.ParseMarker(monitors[i].Memo)
		if !ok {
			continue
		}
		if marker.Owner == desired.Owner && marker.Resource == desired.Resource {
			return &monitors[i], nil
		}
	}
	return nil, nil
}

func (r *ExternalMonitorReconciler) reconcileDelete(ctx context.Context, cr *mackerelv1alpha1.ExternalMonitor, desired monitor.DesiredExternalMonitor) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(cr, externalMonitorFinalizer) {
		return ctrl.Result{}, nil
	}

	actual, err := r.findActual(ctx, cr, desired)
	if err != nil {
		return ctrl.Result{}, err
	}
	decision := planner.PlanDelete(actual, desired.Owner, desired.Resource, r.Policy)
	if decision.Action == planner.ActionDelete {
		if err := r.Provider.DeleteExternalMonitor(ctx, actual.ID); err != nil {
			return ctrl.Result{}, err
		}
	}

	controllerutil.RemoveFinalizer(cr, externalMonitorFinalizer)
	return ctrl.Result{}, r.Update(ctx, cr)
}
```

- [ ] **Step 6: Fix generated RBAC markers**

Ensure the RBAC markers above the reconciler include status and finalizers:

```go
// +kubebuilder:rbac:groups=mackerel.starry.blue,resources=externalmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mackerel.starry.blue,resources=externalmonitors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mackerel.starry.blue,resources=externalmonitors/finalizers,verbs=update
```

- [ ] **Step 7: Run controller tests**

Run:

```bash
go test ./internal/controller
```

Expected: PASS.

- [ ] **Step 8: Run all tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 9: Commit reconciler**

Run:

```bash
git add internal/controller
git commit -m "feat: reconcile external monitors"
```

Expected: one commit with reconciler logic and tests.

## Task 10: Wire Runtime Flags And Manager Setup

**Files:**
- Modify: `cmd/main.go`
- Modify: `config/manager/manager.yaml`

- [ ] **Step 1: Add runtime flags and provider initialization**

Modify `cmd/main.go` to define flags:

```go
var policy string
var ownerID string
var hashLength int

flag.StringVar(&policy, "policy", "upsert-only", "sync policy: upsert-only or sync")
flag.StringVar(&ownerID, "owner-id", "default", "owner identifier for Mackerel monitor ownership markers")
flag.IntVar(&hashLength, "hash-length", 7, "short hash length for Mackerel monitor ownership markers")
```

After `flag.Parse()`, validate:

```go
if policy != "upsert-only" && policy != "sync" {
	setupLog.Error(fmt.Errorf("invalid policy %q", policy), "policy must be upsert-only or sync")
	os.Exit(1)
}
if hashLength < 1 || hashLength > 64 {
	setupLog.Error(fmt.Errorf("invalid hash length %d", hashLength), "hash length must be between 1 and 64")
	os.Exit(1)
}

mackerelProvider, err := provider.NewMackerelProvider(os.Getenv("MACKEREL_APIKEY"))
if err != nil {
	setupLog.Error(err, "unable to initialize Mackerel provider")
	os.Exit(1)
}
```

Ensure imports include `fmt`, `os`, and `github.com/SlashNephy/mackerel-operator/internal/provider`.

- [ ] **Step 2: Pass runtime configuration to the reconciler**

Update controller setup in `cmd/main.go`:

```go
if err = (&controller.ExternalMonitorReconciler{
	Client:     mgr.GetClient(),
	Scheme:     mgr.GetScheme(),
	Provider:   mackerelProvider,
	OwnerID:    ownerID,
	Policy:     policy,
	HashLength: hashLength,
}).SetupWithManager(mgr); err != nil {
	setupLog.Error(err, "unable to create controller", "controller", "ExternalMonitor")
	os.Exit(1)
}
```

- [ ] **Step 3: Add env placeholder and args to manager manifest**

Patch `config/manager/manager.yaml` container spec:

```yaml
        args:
        - --leader-elect
        - --policy=upsert-only
        - --owner-id=default
        - --hash-length=7
        env:
        - name: MACKEREL_APIKEY
          valueFrom:
            secretKeyRef:
              name: mackerel-api-key
              key: apiKey
```

- [ ] **Step 4: Run tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit runtime wiring**

Run:

```bash
git add cmd/main.go config/manager/manager.yaml
git commit -m "feat: wire mackerel operator runtime options"
```

Expected: one commit with flags and manager manifest updates.

## Task 11: Add README Usage Documentation

**Files:**
- Create or Modify: `README.md`

- [ ] **Step 1: Write README**

Create or replace `README.md`:

````markdown
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
make generate manifests
go test ./...
```

## Running Locally

```bash
export MACKEREL_APIKEY=...
make install
go run ./cmd/main.go --policy=upsert-only --owner-id=default --hash-length=7
```

## Deletion Policy

- `upsert-only` creates and updates Mackerel monitors but does not delete them when CRDs are deleted.
- `sync` deletes only monitors whose ownership marker matches the current operator owner and source resource.
````

- [ ] **Step 2: Verify markdown fence nesting**

Run:

```bash
sed -n '1,220p' README.md
```

Expected: Markdown code fences are balanced.

- [ ] **Step 3: Run tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit README**

Run:

```bash
git add README.md
git commit -m "docs: add mackerel operator usage"
```

Expected: one commit with README documentation.

## Task 12: Final Verification

**Files:**
- Read: all files changed by previous tasks.

- [ ] **Step 1: Regenerate generated files**

Run:

```bash
make generate manifests
```

Expected: command succeeds and either leaves no diff or only expected generated output.

- [ ] **Step 2: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Build the manager**

Run:

```bash
go build ./cmd/main.go
```

Expected: PASS and a `main` binary is produced in the repository root.

- [ ] **Step 4: Remove local build artifact**

Run:

```bash
rm ./main
```

Expected: local build artifact is removed.

- [ ] **Step 5: Inspect git diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: only intentional files are changed. After all planned commits, the worktree should be clean except pre-existing untracked files such as `.codex` and `AGENTS.md`.

- [ ] **Step 6: Record final result**

Prepare a short summary for the user:

```text
Implemented the Mackerel Operator MVP: ExternalMonitor CRD, Mackerel provider, ownership marker handling, planner, reconciler, runtime flags, manifests, tests, and README.
Verification: make generate manifests, go test ./..., go build ./cmd/main.go.
```
