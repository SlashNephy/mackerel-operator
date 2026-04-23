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
