package provider

import (
	"context"
	"errors"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
)

var (
	ErrNotFound    = errors.New("monitor not found")
	ErrRateLimited = errors.New("mackerel rate limited")
	ErrInvalid     = errors.New("invalid mackerel monitor")
)

type ExternalMonitorProvider interface {
	GetExternalMonitor(ctx context.Context, id string) (*monitor.ActualExternalMonitor, error)
	ListExternalMonitors(ctx context.Context) ([]monitor.ActualExternalMonitor, error)
	CreateExternalMonitor(ctx context.Context, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error)
	UpdateExternalMonitor(ctx context.Context, id string, desired monitor.DesiredExternalMonitor, memo string) (*monitor.ActualExternalMonitor, error)
	DeleteExternalMonitor(ctx context.Context, id string) error
}
