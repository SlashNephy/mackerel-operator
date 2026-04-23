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
