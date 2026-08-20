package status

import (
	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionReady = "Ready"

	ReasonSynced        = "Synced"
	ReasonOwnershipLost = "OwnershipLost"
	ReasonInvalidSpec   = "InvalidSpec"
	// ReasonSecretNotFound reports a header whose Secret or key does not exist.
	// Only a user edit can resolve it, so the reconciliation waits for the
	// Secret watch instead of retrying with backoff.
	ReasonSecretNotFound = "SecretNotFound"
	// ReasonSecretError reports a failure to read a referenced Secret, such as
	// the operator lacking the RBAC permission to do so.
	ReasonSecretError = "SecretError"
)

type SyncResult struct {
	MonitorID string
	Hash      string
	URL       string
	Name      string
	// Applied reports whether this reconciliation actually wrote to Mackerel.
	// LastSyncedAt only moves forward when it did, so a resync that finds the
	// monitor already in sync leaves the status untouched.
	Applied bool
}

func MarkReady(cr *mackerelv1alpha1.ExternalMonitor, result SyncResult) {
	cr.Status.MonitorID = result.MonitorID
	cr.Status.ObservedGeneration = cr.Generation
	if result.Applied {
		now := metav1.Now()
		cr.Status.LastSyncedAt = &now
	}
	cr.Status.LastAppliedHash = result.Hash
	cr.Status.URL = result.URL
	cr.Status.MackerelMonitorName = result.Name
	setCondition(cr, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonSynced,
		Message:            "Mackerel external monitor is synchronized",
		ObservedGeneration: cr.Generation,
	})
}

func MarkOwnershipLost(cr *mackerelv1alpha1.ExternalMonitor, message string) {
	cr.Status.ObservedGeneration = cr.Generation
	setCondition(cr, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonOwnershipLost,
		Message:            message,
		ObservedGeneration: cr.Generation,
	})
}

func MarkInvalidSpec(cr *mackerelv1alpha1.ExternalMonitor, message string) {
	cr.Status.ObservedGeneration = cr.Generation
	setCondition(cr, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonInvalidSpec,
		Message:            message,
		ObservedGeneration: cr.Generation,
	})
}

func MarkError(cr *mackerelv1alpha1.ExternalMonitor, reason, message string) {
	cr.Status.ObservedGeneration = cr.Generation
	setCondition(cr, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: cr.Generation,
	})
}

func setCondition(cr *mackerelv1alpha1.ExternalMonitor, condition metav1.Condition) {
	meta.SetStatusCondition(&cr.Status.Conditions, condition)
}
