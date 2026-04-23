package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var errEmptyAPIKey = errors.New("mackerel api key is empty")

type MackerelProvider struct {
	client *mackerel.Client
}

var _ ExternalMonitorProvider = new(MackerelProvider)

func NewMackerelProvider(apiKey string) (*MackerelProvider, error) {
	if apiKey == "" {
		return nil, errEmptyAPIKey
	}

	return &MackerelProvider{client: mackerel.NewClient(apiKey)}, nil
}

func (p *MackerelProvider) GetExternalMonitor(ctx context.Context, id string) (*monitor.ActualExternalMonitor, error) {
	actual, err := p.client.GetMonitorContext(ctx, id)
	if err != nil {
		return nil, mapMackerelError(err)
	}

	return externalMonitorFromMackerel(actual)
}

func (p *MackerelProvider) ListExternalMonitors(ctx context.Context) ([]monitor.ActualExternalMonitor, error) {
	monitors, err := p.client.FindMonitorsContext(ctx)
	if err != nil {
		return nil, mapMackerelError(err)
	}

	actuals := make([]monitor.ActualExternalMonitor, 0, len(monitors))
	for _, m := range monitors {
		external, ok := m.(*mackerel.MonitorExternalHTTP)
		if !ok {
			continue
		}
		actuals = append(actuals, actualExternalMonitorFromMackerel(external))
	}

	return actuals, nil
}

func (p *MackerelProvider) CreateExternalMonitor(ctx context.Context, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	created, err := p.client.CreateMonitorContext(ctx, mackerelExternalMonitorFromDesired(desired, memo))
	if err != nil {
		return nil, mapMackerelError(err)
	}

	return externalMonitorFromMackerel(created)
}

func (p *MackerelProvider) UpdateExternalMonitor(ctx context.Context, id string, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	updated, err := p.client.UpdateMonitorContext(ctx, id, mackerelExternalMonitorFromDesired(desired, memo))
	if err != nil {
		return nil, mapMackerelError(err)
	}

	return externalMonitorFromMackerel(updated)
}

func (p *MackerelProvider) DeleteExternalMonitor(ctx context.Context, id string) error {
	if _, err := p.client.DeleteMonitorContext(ctx, id); err != nil {
		return mapMackerelError(err)
	}

	return nil
}

func externalMonitorFromMackerel(m mackerel.Monitor) (*monitor.ActualExternalMonitor, error) {
	external, ok := m.(*mackerel.MonitorExternalHTTP)
	if !ok {
		return nil, fmt.Errorf("%w: expected external HTTP monitor, got %s", ErrInvalid, m.MonitorType())
	}

	actual := actualExternalMonitorFromMackerel(external)
	return &actual, nil
}

func actualExternalMonitorFromMackerel(m *mackerel.MonitorExternalHTTP) monitor.ActualExternalMonitor {
	return monitor.ActualExternalMonitor{
		ID:                              m.ID,
		Name:                            m.Name,
		Service:                         m.Service,
		URL:                             m.URL,
		Method:                          m.Method,
		NotificationInterval:            intPtrFromUint64(m.NotificationInterval),
		ExpectedStatusCode:              m.ExpectedStatusCode,
		ContainsString:                  m.ContainsString,
		ResponseTimeWarning:             intPtrFromFloat64(m.ResponseTimeWarning),
		ResponseTimeCritical:            intPtrFromFloat64(m.ResponseTimeCritical),
		CertificationExpirationWarning:  intPtrFromUint64Ptr(m.CertificationExpirationWarning),
		CertificationExpirationCritical: intPtrFromUint64Ptr(m.CertificationExpirationCritical),
		Memo:                            m.Memo,
	}
}

func mackerelExternalMonitorFromDesired(desired monitor.DesiredExternalMonitor, memo string) *mackerel.MonitorExternalHTTP {
	return &mackerel.MonitorExternalHTTP{
		Type:                            "external",
		Name:                            desired.Name,
		Memo:                            memo,
		NotificationInterval:            uint64FromIntPtr(desired.NotificationInterval),
		Method:                          desired.Method,
		URL:                             desired.URL,
		Service:                         desired.Service,
		ResponseTimeWarning:             float64PtrFromIntPtr(desired.ResponseTimeWarning),
		ResponseTimeCritical:            float64PtrFromIntPtr(desired.ResponseTimeCritical),
		ContainsString:                  desired.ContainsString,
		CertificationExpirationWarning:  uint64PtrFromIntPtr(desired.CertificationExpirationWarning),
		CertificationExpirationCritical: uint64PtrFromIntPtr(desired.CertificationExpirationCritical),
		ExpectedStatusCode:              desired.ExpectedStatusCode,
	}
}

func mapMackerelError(err error) error {
	var apiErr *mackerel.APIError
	if !errors.As(err, &apiErr) {
		return err
	}

	switch apiErr.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: %w", ErrRateLimited, err)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	default:
		return err
	}
}

func intPtrFromUint64(v uint64) *int {
	if v == 0 {
		return nil
	}

	i := int(v)
	return &i
}

func intPtrFromUint64Ptr(v *uint64) *int {
	if v == nil {
		return nil
	}

	i := int(*v)
	return &i
}

func intPtrFromFloat64(v *float64) *int {
	if v == nil {
		return nil
	}

	i := int(*v)
	return &i
}

func uint64FromIntPtr(v *int) uint64 {
	if v == nil {
		return 0
	}

	return uint64(*v)
}

func uint64PtrFromIntPtr(v *int) *uint64 {
	if v == nil {
		return nil
	}

	u := uint64(*v)
	return &u
}

func float64PtrFromIntPtr(v *int) *float64 {
	if v == nil {
		return nil
	}

	f := float64(*v)
	return &f
}
