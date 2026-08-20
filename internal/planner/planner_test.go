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

// TestPlanOwnershipLostWhenMarkerMissingAndOnlyNewFieldDiffers pins the
// adoption boundary introduced when the six new fields were brought under
// operator control. Before this branch, MaxCheckAttempts was ignored in
// actualMatchesDesired, so a marker-less monitor with a non-default value
// would adopt cleanly. Now it routes to OwnershipLost, which is the correct
// fail-safe: the operator cannot silently overwrite a value set in the UI.
func TestPlanOwnershipLostWhenMarkerMissingAndOnlyNewFieldDiffers(t *testing.T) {
	actual := monitor.ActualExternalMonitor{
		ID:               "mon-1",
		Name:             "api",
		URL:              "https://example.com",
		MaxCheckAttempts: 2, // differs from desired's 1; everything else matches
		Memo:             "human memo",
	}
	decision := Plan(PlanInput{
		Desired: monitor.DesiredExternalMonitor{
			Name:             "api",
			URL:              "https://example.com",
			MaxCheckAttempts: 1,
			Owner:            "prod",
			Resource:         "externalmonitor/default/api",
			Hash:             "deadbee",
		},
		Actual: &actual,
	})
	assert.Equal(t, ActionOwnershipLost, decision.Action, decision.Reason)
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
		{name: "dualstack drifted", mutate: func(a *monitor.ActualExternalMonitor) { a.Dualstack = "ipv6" }, want: ActionUpdate},
		// Mackerel omits dualstack from a monitor that never set it, so an
		// explicit ipv4 and an absent value describe the same monitor.
		{name: "dualstack reported as explicit ipv4", mutate: func(a *monitor.ActualExternalMonitor) { a.Dualstack = "ipv4" }, want: ActionNoop},
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

// TestPlanNoopWhenDualstackMatchesAcrossRepresentations covers the pair the
// normalisation exists for. Mackerel returns no dualstack key for a monitor
// that never set one, and the CR leaves the field empty when it is omitted, so
// the two sides can disagree textually while describing the same monitor.
// Without widening both to ipv4 the planner would issue the same Update on
// every reconcile.
func TestPlanNoopWhenDualstackMatchesAcrossRepresentations(t *testing.T) {
	t.Parallel()

	const (
		owner    = "prod"
		resource = "externalmonitor/default/api"
		hash     = "deadbee"
	)

	tests := []struct {
		name    string
		desired string
		actual  string
		want    Action
	}{
		{name: "both unset", desired: "", actual: "", want: ActionNoop},
		{name: "desired unset, actual explicit ipv4", desired: "", actual: "ipv4", want: ActionNoop},
		{name: "desired explicit ipv4, actual unset", desired: "ipv4", actual: "", want: ActionNoop},
		{name: "desired ipv6, actual ipv6", desired: "ipv6", actual: "ipv6", want: ActionNoop},
		{name: "desired unset, actual ipv6", desired: "", actual: "ipv6", want: ActionUpdate},
		{name: "desired auto, actual unset", desired: "auto", actual: "", want: ActionUpdate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			decision := Plan(PlanInput{
				Desired: monitor.DesiredExternalMonitor{
					Name:             "api",
					URL:              "https://example.com",
					MaxCheckAttempts: 1,
					Dualstack:        tt.desired,
					Owner:            owner,
					Resource:         resource,
					Hash:             hash,
				},
				Actual: &monitor.ActualExternalMonitor{
					ID:               "mon-1",
					Name:             "api",
					URL:              "https://example.com",
					MaxCheckAttempts: 1,
					Dualstack:        tt.actual,
					Memo:             ownership.BuildMarker(ownership.Marker{Resource: resource, Owner: owner, Hash: hash}),
				},
			})

			assert.Equal(t, tt.want, decision.Action, decision.Reason)
		})
	}
}
