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
	maxCheckAttempts := cr.Spec.MaxCheckAttempts
	if maxCheckAttempts < 1 {
		maxCheckAttempts = 1
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
// by name. The CRD declares headers as a list-map, so order carries no meaning,
// but Mackerel echoes back whatever order was submitted. Sorting both sides
// keeps the planner from reporting a diff that does not exist.
func sortedHeaders(headers []mackerelv1alpha1.HeaderField) []monitor.HeaderField {
	sorted := make([]monitor.HeaderField, 0, len(headers))
	for _, h := range headers {
		sorted = append(sorted, monitor.HeaderField{Name: h.Name, Value: h.Value})
	}

	slices.SortFunc(sorted, func(a, b monitor.HeaderField) int {
		return strings.Compare(a.Name, b.Name)
	})

	return sorted
}
