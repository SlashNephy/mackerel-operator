package source

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
)

var ErrExternalMonitorNil = errors.New("external monitor is nil")

type ExternalMonitorSource struct {
	OwnerID    string
	HashLength int
}

func (s ExternalMonitorSource) FromExternalMonitor(cr *mackerelv1alpha1.ExternalMonitor) (monitor.DesiredExternalMonitor, error) {
	if cr == nil {
		return monitor.DesiredExternalMonitor{}, ErrExternalMonitorNil
	}

	name := cr.Spec.Name
	if name == "" {
		name = fmt.Sprintf("%s/%s", cr.Namespace, cr.Name)
	}

	method := cr.Spec.Method
	if method == "" {
		method = "GET"
	}

	// Mackerel reports 1 for monitors created without maxCheckAttempts, so an
	// unset value has to be widened here. Leaving it at 0 would make the
	// planner see a permanent diff and rewrite the monitor on every reconcile.
	maxCheckAttempts := max(cr.Spec.MaxCheckAttempts, 1)

	desired := monitor.DesiredExternalMonitor{
		Name:                            name,
		Service:                         cr.Spec.Service,
		URL:                             cr.Spec.URL,
		Method:                          method,
		NotificationInterval:            cr.Spec.NotificationInterval,
		ExpectedStatusCode:              cr.Spec.ExpectedStatusCode,
		ContainsString:                  cr.Spec.ContainsString,
		ResponseTimeDuration:            cr.Spec.ResponseTimeDuration,
		ResponseTimeWarning:             cr.Spec.ResponseTimeWarning,
		ResponseTimeCritical:            cr.Spec.ResponseTimeCritical,
		CertificationExpirationWarning:  cr.Spec.CertificationExpirationWarning,
		CertificationExpirationCritical: cr.Spec.CertificationExpirationCritical,
		IsMute:                          cr.Spec.IsMute,
		FollowRedirect:                  cr.Spec.FollowRedirect,
		SkipCertificateVerification:     cr.Spec.SkipCertificateVerification,
		MaxCheckAttempts:                maxCheckAttempts,
		RequestBody:                     cr.Spec.RequestBody,
		Dualstack:                       cr.Spec.Dualstack,
		Headers:                         sortedHeaders(cr.Spec.Headers),
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

// sortedHeaders converts CRD headers to the intermediate model and orders them
// by name. The primary purpose is hash determinism: the resulting slice feeds
// the desired-state hash, so the order must be stable across reconciles. The
// CRD declares headers as a list-map, so the API server imposes no ordering,
// and the same logical set of headers could arrive in any order. The planner
// also sorts both sides before comparison, but the source-layer sort ensures
// the hash remains the same regardless of the order in which the user wrote
// the headers in the CR.
// The sort is stable to guard Go callers that construct headers programmatically
// with duplicate names; the CRD's listType=map makes such duplicates unreachable
// through the API server.
func sortedHeaders(headers []mackerelv1alpha1.HeaderField) []monitor.HeaderField {
	sorted := make([]monitor.HeaderField, 0, len(headers))
	for _, h := range headers {
		sorted = append(sorted, monitor.HeaderField{Name: h.Name, Value: h.Value})
	}

	slices.SortStableFunc(sorted, func(a, b monitor.HeaderField) int {
		return strings.Compare(a.Name, b.Name)
	})

	return sorted
}
