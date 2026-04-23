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
