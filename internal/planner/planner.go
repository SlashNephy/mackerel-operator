package planner

import (
	"reflect"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/SlashNephy/mackerel-operator/internal/ownership"
)

type Action string

const (
	ActionCreate        Action = "Create"
	ActionUpdate        Action = "Update"
	ActionNoop          Action = "Noop"
	ActionRestoreMarker Action = "RestoreMarker"
	ActionOwnershipLost Action = "OwnershipLost"
	ActionDelete        Action = "Delete"
	ActionSkipDelete    Action = "SkipDelete"
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
		reflect.DeepEqual(desired.ResponseTimeDuration, actual.ResponseTimeDuration) &&
		reflect.DeepEqual(desired.ResponseTimeWarning, actual.ResponseTimeWarning) &&
		reflect.DeepEqual(desired.ResponseTimeCritical, actual.ResponseTimeCritical) &&
		reflect.DeepEqual(desired.CertificationExpirationWarning, actual.CertificationExpirationWarning) &&
		reflect.DeepEqual(desired.CertificationExpirationCritical, actual.CertificationExpirationCritical)
}
