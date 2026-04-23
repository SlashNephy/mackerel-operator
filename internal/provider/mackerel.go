package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

var (
	errEmptyAPIKey      = errors.New("mackerel api key is empty")
	errNegativeIntValue = errors.New("negative integer cannot be converted to unsigned Mackerel field")
)

type MackerelProvider struct {
	client *mackerel.Client
}

var _ ExternalMonitorProvider = new(MackerelProvider)

func NewMackerelProvider(apiKey string) (*MackerelProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
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
	payload, err := newMackerelExternalMonitor(desired, memo)
	if err != nil {
		return nil, err
	}

	created, err := p.client.CreateMonitorContext(ctx, payload)
	if err != nil {
		return nil, mapMackerelError(err)
	}

	return externalMonitorFromMackerel(created)
}

func (p *MackerelProvider) UpdateExternalMonitor(ctx context.Context, id string, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error) {
	current, err := p.client.GetMonitorContext(ctx, id)
	if err != nil {
		return nil, mapMackerelError(err)
	}

	external, ok := current.(*mackerel.MonitorExternalHTTP)
	if !ok {
		return nil, fmt.Errorf("%w: expected external HTTP monitor, got %s", ErrInvalid, current.MonitorType())
	}

	payload, err := mergeMackerelExternalMonitor(external, desired, memo)
	if err != nil {
		return nil, err
	}

	updated, err := p.client.UpdateMonitorContext(ctx, id, payload)
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

func newMackerelExternalMonitor(desired monitor.DesiredExternalMonitor, memo string) (*mackerel.MonitorExternalHTTP, error) {
	return mergeMackerelExternalMonitor(&mackerel.MonitorExternalHTTP{
		Type: "external",
	}, desired, memo)
}

func mergeMackerelExternalMonitor(base *mackerel.MonitorExternalHTTP, desired monitor.DesiredExternalMonitor, memo string) (*mackerel.MonitorExternalHTTP, error) {
	notificationInterval, err := uint64FromIntPtr(desired.NotificationInterval)
	if err != nil {
		return nil, err
	}
	responseTimeWarning, err := float64PtrFromIntPtr(desired.ResponseTimeWarning)
	if err != nil {
		return nil, err
	}
	responseTimeCritical, err := float64PtrFromIntPtr(desired.ResponseTimeCritical)
	if err != nil {
		return nil, err
	}
	certificationExpirationWarning, err := uint64PtrFromIntPtr(desired.CertificationExpirationWarning)
	if err != nil {
		return nil, err
	}
	certificationExpirationCritical, err := uint64PtrFromIntPtr(desired.CertificationExpirationCritical)
	if err != nil {
		return nil, err
	}

	merged := *base
	merged.Type = "external"
	merged.Name = desired.Name
	merged.Memo = memo
	merged.NotificationInterval = notificationInterval
	merged.Method = desired.Method
	merged.URL = desired.URL
	merged.Service = desired.Service
	merged.ResponseTimeWarning = responseTimeWarning
	merged.ResponseTimeCritical = responseTimeCritical
	merged.ContainsString = desired.ContainsString
	merged.CertificationExpirationWarning = certificationExpirationWarning
	merged.CertificationExpirationCritical = certificationExpirationCritical
	merged.ExpectedStatusCode = desired.ExpectedStatusCode

	return &merged, nil
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
	if v > uint64(math.MaxInt) {
		return nil
	}

	i := int(v)
	return &i
}

func intPtrFromUint64Ptr(v *uint64) *int {
	if v == nil {
		return nil
	}
	if *v > uint64(math.MaxInt) {
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

func uint64FromIntPtr(v *int) (uint64, error) {
	if v == nil {
		return 0, nil
	}
	if *v < 0 {
		return 0, fmt.Errorf("%w: %d", errNegativeIntValue, *v)
	}

	return uint64(*v), nil
}

func uint64PtrFromIntPtr(v *int) (*uint64, error) {
	if v == nil {
		return nil, nil
	}
	if *v < 0 {
		return nil, fmt.Errorf("%w: %d", errNegativeIntValue, *v)
	}

	u := uint64(*v)
	return &u, nil
}

func float64PtrFromIntPtr(v *int) (*float64, error) {
	if v == nil {
		return nil, nil
	}
	if *v < 0 {
		return nil, fmt.Errorf("%w: %d", errNegativeIntValue, *v)
	}

	f := float64(*v)
	return &f, nil
}
