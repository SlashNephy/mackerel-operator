package provider

import (
	"errors"
	"testing"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	mackerel "github.com/mackerelio/mackerel-client-go"
)

func TestMergeMackerelExternalMonitorPreservesUnsupportedFields(t *testing.T) {
	responseTimeDuration := uint64(3)
	base := &mackerel.MonitorExternalHTTP{
		ID:                          "mon-1",
		Type:                        "external",
		IsMute:                      true,
		MaxCheckAttempts:            5,
		ResponseTimeDuration:        &responseTimeDuration,
		RequestBody:                 "payload",
		SkipCertificateVerification: true,
		FollowRedirect:              true,
		Headers: []mackerel.HeaderField{
			{Name: "Authorization", Value: "Bearer token"},
		},
	}
	interval := 10
	expectedStatus := 200
	warning := 1000
	critical := 2000
	certWarning := 30
	certCritical := 14
	desired := monitor.DesiredExternalMonitor{
		Name:                            "API health",
		Service:                         "my-service",
		URL:                             "https://api.example.com/healthz",
		Method:                          "GET",
		NotificationInterval:            &interval,
		ExpectedStatusCode:              &expectedStatus,
		ContainsString:                  "ok",
		ResponseTimeWarning:             &warning,
		ResponseTimeCritical:            &critical,
		CertificationExpirationWarning:  &certWarning,
		CertificationExpirationCritical: &certCritical,
	}

	got, err := mergeMackerelExternalMonitor(base, desired, "human memo")
	if err != nil {
		t.Fatalf("mergeMackerelExternalMonitor returned error: %v", err)
	}

	if got.ID != "mon-1" {
		t.Fatalf("ID = %q, want mon-1", got.ID)
	}
	if !got.IsMute {
		t.Fatal("IsMute = false, want true")
	}
	if got.MaxCheckAttempts != 5 {
		t.Fatalf("MaxCheckAttempts = %d, want 5", got.MaxCheckAttempts)
	}
	if got.ResponseTimeDuration != nil {
		t.Fatalf("ResponseTimeDuration = %v, want nil", got.ResponseTimeDuration)
	}
	if got.RequestBody != "payload" {
		t.Fatalf("RequestBody = %q, want payload", got.RequestBody)
	}
	if !got.SkipCertificateVerification {
		t.Fatal("SkipCertificateVerification = false, want true")
	}
	if !got.FollowRedirect {
		t.Fatal("FollowRedirect = false, want true")
	}
	if len(got.Headers) != 1 || got.Headers[0].Name != "Authorization" || got.Headers[0].Value != "Bearer token" {
		t.Fatalf("Headers = %#v, want preserved Authorization header", got.Headers)
	}

	if got.Name != desired.Name || got.Service != desired.Service || got.URL != desired.URL || got.Method != desired.Method {
		t.Fatalf("managed fields not applied: got=%#v desired=%#v", got, desired)
	}
	if got.Memo != "human memo" {
		t.Fatalf("Memo = %q, want human memo", got.Memo)
	}
	if got.NotificationInterval != uint64(interval) {
		t.Fatalf("NotificationInterval = %d, want %d", got.NotificationInterval, interval)
	}
}

func TestMergeMackerelExternalMonitorAppliesResponseTimeDuration(t *testing.T) {
	duration := 5
	desired := monitor.DesiredExternalMonitor{
		Name:                 "API health",
		URL:                  "https://api.example.com/healthz",
		Method:               "GET",
		ResponseTimeDuration: &duration,
	}

	got, err := mergeMackerelExternalMonitor(&mackerel.MonitorExternalHTTP{}, desired, "")
	if err != nil {
		t.Fatalf("mergeMackerelExternalMonitor returned error: %v", err)
	}
	if got.ResponseTimeDuration == nil || *got.ResponseTimeDuration != uint64(duration) {
		t.Fatalf("ResponseTimeDuration = %v, want %d", got.ResponseTimeDuration, duration)
	}
}

func TestMergeMackerelExternalMonitorRejectsNegativeUnsignedFields(t *testing.T) {
	negative := -1
	desired := monitor.DesiredExternalMonitor{
		Name:                 "API health",
		URL:                  "https://api.example.com/healthz",
		Method:               "GET",
		NotificationInterval: &negative,
	}

	_, err := mergeMackerelExternalMonitor(&mackerel.MonitorExternalHTTP{}, desired, "")
	if !errors.Is(err, errNegativeIntValue) {
		t.Fatalf("mergeMackerelExternalMonitor error = %v, want errNegativeIntValue", err)
	}
}

func TestNewMackerelProviderRejectsBlankAPIKey(t *testing.T) {
	_, err := NewMackerelProvider(" \t\n")
	if !errors.Is(err, errEmptyAPIKey) {
		t.Fatalf("NewMackerelProvider error = %v, want errEmptyAPIKey", err)
	}
}
