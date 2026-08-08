package planner

import (
	"testing"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
	"github.com/stretchr/testify/assert"
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

func TestPlanDeleteActualNilSkips(t *testing.T) {
	decision := PlanDelete(nil, "prod", "externalmonitor/default/api", "sync")
	if decision.Action != ActionSkipDelete {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionSkipDelete)
	}
}

func TestPlanDeleteMissingMarkerSkips(t *testing.T) {
	actual := &monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Memo: "human memo",
	}
	decision := PlanDelete(actual, "prod", "externalmonitor/default/api", "sync")
	if decision.Action != ActionSkipDelete {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionSkipDelete)
	}
}

func TestPlanDeleteWrongOwnerSkips(t *testing.T) {
	actual := &monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/api", Owner: "staging", Hash: "deadbee"}),
	}
	decision := PlanDelete(actual, "prod", "externalmonitor/default/api", "sync")
	if decision.Action != ActionSkipDelete {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionSkipDelete)
	}
}

func TestPlanDeleteWrongResourceSkips(t *testing.T) {
	actual := &monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/other", Owner: "prod", Hash: "deadbee"}),
	}
	decision := PlanDelete(actual, "prod", "externalmonitor/default/api", "sync")
	if decision.Action != ActionSkipDelete {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionSkipDelete)
	}
}

func TestPlanDeleteUnknownPolicySkips(t *testing.T) {
	actual := &monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/api", Owner: "prod", Hash: "deadbee"}),
	}
	decision := PlanDelete(actual, "prod", "externalmonitor/default/api", "create-only")
	if decision.Action != ActionSkipDelete {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionSkipDelete)
	}
}

func TestPlanOwnershipLostWhenMarkerOwnerDiffers(t *testing.T) {
	actual := monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Name: "api",
		URL:  "https://example.com",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/api", Owner: "staging", Hash: "deadbee"}),
	}
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{Name: "api", URL: "https://example.com", Owner: "prod", Resource: "externalmonitor/default/api", Hash: "deadbee"},
		Actual:  &actual,
	})
	if decision.Action != ActionOwnershipLost {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionOwnershipLost)
	}
}

func TestPlanOwnershipLostWhenMarkerResourceDiffers(t *testing.T) {
	actual := monitor.ActualExternalMonitor{
		ID:   "mon-1",
		Name: "api",
		URL:  "https://example.com",
		Memo: ownership.BuildMarker(ownership.Marker{Resource: "externalmonitor/default/other", Owner: "prod", Hash: "deadbee"}),
	}
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{Name: "api", URL: "https://example.com", Owner: "prod", Resource: "externalmonitor/default/api", Hash: "deadbee"},
		Actual:  &actual,
	})
	if decision.Action != ActionOwnershipLost {
		t.Fatalf("Action = %s, want %s", decision.Action, ActionOwnershipLost)
	}
}

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
