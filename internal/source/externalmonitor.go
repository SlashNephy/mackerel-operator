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
	// HeaderValues holds the already resolved values of the headers declared
	// with valueFrom, keyed by header name. The resolution itself lives in the
	// caller because it needs a Kubernetes client, which this package keeps out
	// of the desired-state computation.
	//
	// A nil map means "this caller does not resolve headers at all" and drops
	// the secret-backed headers instead of failing. The deletion path relies on
	// it: deleting a monitor only needs Name, Owner and Resource, and the
	// referenced Secret may already be gone by then. A non-nil map must contain
	// every secret-backed header, otherwise the monitor would be written with a
	// partial header set.
	HeaderValues map[string]string
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

	headers, err := s.sortedHeaders(cr.Spec.Headers)
	if err != nil {
		return monitor.DesiredExternalMonitor{}, err
	}

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
		Headers:                         headers,
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
//
// Resolved secret values take part in the hash like any other value. Mackerel
// returns header values unmasked, so the short hash in the monitor memo tells a
// reader nothing they could not already read from the API, and rotating a
// Secret moves the hash instead of relying on the drift comparison alone.
func (s ExternalMonitorSource) sortedHeaders(headers []mackerelv1alpha1.HeaderField) ([]monitor.HeaderField, error) {
	sorted := make([]monitor.HeaderField, 0, len(headers))
	for _, h := range headers {
		value, ok, err := s.headerValue(&h)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		sorted = append(sorted, monitor.HeaderField{Name: h.Name, Value: value})
	}

	slices.SortStableFunc(sorted, func(a, b monitor.HeaderField) int {
		return strings.Compare(a.Name, b.Name)
	})

	return sorted, nil
}

// headerValue reports the value to send for a single header. The second result
// is false when the header has to be dropped, which only happens for a
// secret-backed header on a caller that does not resolve headers.
func (s ExternalMonitorSource) headerValue(header *mackerelv1alpha1.HeaderField) (string, bool, error) {
	if header.ValueFrom == nil {
		if header.Value == nil {
			return "", false, fmt.Errorf("header %q has neither value nor valueFrom", header.Name)
		}
		return *header.Value, true, nil
	}

	if s.HeaderValues == nil {
		return "", false, nil
	}

	value, ok := s.HeaderValues[header.Name]
	if !ok {
		return "", false, fmt.Errorf("header %q has no resolved value", header.Name)
	}
	return value, true, nil
}
