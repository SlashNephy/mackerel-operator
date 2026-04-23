package source

import "github.com/SlashNephy/mackerel-operator/internal/monitor"

type Source interface {
	DesiredMonitors() ([]monitor.DesiredExternalMonitor, error)
}
