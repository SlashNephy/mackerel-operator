package source

import (
	"errors"
	"fmt"

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
