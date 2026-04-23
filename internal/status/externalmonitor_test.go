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
	if cr.Status.LastSyncedAt == nil {
		t.Fatal("LastSyncedAt is nil")
	}
	if cr.Status.LastAppliedHash != "deadbee" {
		t.Fatalf("LastAppliedHash = %q", cr.Status.LastAppliedHash)
	}
	if cr.Status.URL != "https://api.example.com/healthz" {
		t.Fatalf("URL = %q", cr.Status.URL)
	}
	if cr.Status.MackerelMonitorName != "API health check" {
		t.Fatalf("MackerelMonitorName = %q", cr.Status.MackerelMonitorName)
	}
	cond := findCondition(cr, ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != ReasonSynced {
		t.Fatalf("Ready condition = %#v", cond)
	}
}

func TestMarkOwnershipLost(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Generation: 5},
	}
	MarkOwnershipLost(cr, "marker missing and monitor differs")

	if cr.Status.ObservedGeneration != 5 {
		t.Fatalf("ObservedGeneration = %d", cr.Status.ObservedGeneration)
	}
	cond := findCondition(cr, ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != ReasonOwnershipLost {
		t.Fatalf("Ready condition = %#v", cond)
	}
}

func TestMarkInvalidSpec(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Generation: 7},
	}
	MarkInvalidSpec(cr, "url is required")

	if cr.Status.ObservedGeneration != 7 {
		t.Fatalf("ObservedGeneration = %d", cr.Status.ObservedGeneration)
	}
	cond := findCondition(cr, ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != ReasonInvalidSpec || cond.Message != "url is required" {
		t.Fatalf("Ready condition = %#v", cond)
	}
}

func TestMarkError(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Generation: 11},
	}
	MarkError(cr, "ProviderError", "rate limited")

	if cr.Status.ObservedGeneration != 11 {
		t.Fatalf("ObservedGeneration = %d", cr.Status.ObservedGeneration)
	}
	cond := findCondition(cr, ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != "ProviderError" || cond.Message != "rate limited" {
		t.Fatalf("Ready condition = %#v", cond)
	}
}

func TestSetConditionPreservesLastTransitionTimeWhenStatusDoesNotChange(t *testing.T) {
	originalTransition := metav1.NewTime(metav1.Now().Add(-1))
	cr := &mackerelv1alpha1.ExternalMonitor{
		Status: mackerelv1alpha1.ExternalMonitorStatus{
			Conditions: []metav1.Condition{{
				Type:               ConditionReady,
				Status:             metav1.ConditionTrue,
				Reason:             "OldReason",
				Message:            "old message",
				LastTransitionTime: originalTransition,
			}},
		},
	}

	MarkReady(cr, SyncResult{})

	cond := findCondition(cr, ConditionReady)
	if cond == nil {
		t.Fatal("Ready condition is nil")
	}
	if !cond.LastTransitionTime.Equal(&originalTransition) {
		t.Fatalf("LastTransitionTime = %s, want %s", cond.LastTransitionTime, originalTransition)
	}
	if cond.Reason != ReasonSynced {
		t.Fatalf("Reason = %q", cond.Reason)
	}
}

func TestSetConditionUpdatesLastTransitionTimeWhenStatusChanges(t *testing.T) {
	originalTransition := metav1.NewTime(metav1.Now().Add(-1))
	cr := &mackerelv1alpha1.ExternalMonitor{
		Status: mackerelv1alpha1.ExternalMonitorStatus{
			Conditions: []metav1.Condition{{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonOwnershipLost,
				LastTransitionTime: originalTransition,
			}},
		},
	}

	MarkReady(cr, SyncResult{})

	cond := findCondition(cr, ConditionReady)
	if cond == nil {
		t.Fatal("Ready condition is nil")
	}
	if !cond.LastTransitionTime.After(originalTransition.Time) {
		t.Fatalf("LastTransitionTime = %s, want after %s", cond.LastTransitionTime, originalTransition)
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
