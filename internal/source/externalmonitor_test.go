package source

import (
	"errors"
	"testing"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExternalMonitorSourceBuildsDesiredMonitor(t *testing.T) {
	interval := 10
	expectedStatus := 200
	responseTimeWarning := 30
	responseTimeCritical := 60
	certificationWarning := 15
	certificationCritical := 5
	containsString := "ok"
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-health",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			Name:                            "API health check",
			Service:                         "my-service",
			URL:                             "https://api.example.com/healthz",
			Method:                          "GET",
			NotificationInterval:            &interval,
			ExpectedStatusCode:              &expectedStatus,
			ContainsString:                  containsString,
			ResponseTimeWarning:             &responseTimeWarning,
			ResponseTimeCritical:            &responseTimeCritical,
			CertificationExpirationWarning:  &certificationWarning,
			CertificationExpirationCritical: &certificationCritical,
			Memo:                            "human memo",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	if err != nil {
		t.Fatalf("FromExternalMonitor returned error: %v", err)
	}
	if got.Name != "API health check" {
		t.Fatalf("Name = %q", got.Name)
	}
	if got.Service != "my-service" {
		t.Fatalf("Service = %q", got.Service)
	}
	if got.URL != "https://api.example.com/healthz" {
		t.Fatalf("URL = %q", got.URL)
	}
	if got.Method != "GET" {
		t.Fatalf("Method = %q", got.Method)
	}
	if got.NotificationInterval == nil || *got.NotificationInterval != interval {
		t.Fatalf("NotificationInterval = %v, want %d", got.NotificationInterval, interval)
	}
	if got.ExpectedStatusCode == nil || *got.ExpectedStatusCode != expectedStatus {
		t.Fatalf("ExpectedStatusCode = %v, want %d", got.ExpectedStatusCode, expectedStatus)
	}
	if got.ContainsString != containsString {
		t.Fatalf("ContainsString = %q, want %q", got.ContainsString, containsString)
	}
	if got.ResponseTimeWarning == nil || *got.ResponseTimeWarning != responseTimeWarning {
		t.Fatalf("ResponseTimeWarning = %v, want %d", got.ResponseTimeWarning, responseTimeWarning)
	}
	if got.ResponseTimeCritical == nil || *got.ResponseTimeCritical != responseTimeCritical {
		t.Fatalf("ResponseTimeCritical = %v, want %d", got.ResponseTimeCritical, responseTimeCritical)
	}
	if got.CertificationExpirationWarning == nil || *got.CertificationExpirationWarning != certificationWarning {
		t.Fatalf("CertificationExpirationWarning = %v, want %d", got.CertificationExpirationWarning, certificationWarning)
	}
	if got.CertificationExpirationCritical == nil || *got.CertificationExpirationCritical != certificationCritical {
		t.Fatalf("CertificationExpirationCritical = %v, want %d", got.CertificationExpirationCritical, certificationCritical)
	}
	if got.Memo != "human memo" {
		t.Fatalf("Memo = %q", got.Memo)
	}
	if got.Resource != "externalmonitor/default/api-health" {
		t.Fatalf("Resource = %q", got.Resource)
	}
	if got.Owner != "prod" {
		t.Fatalf("Owner = %q", got.Owner)
	}
	if len(got.Hash) != 7 {
		t.Fatalf("Hash length = %d, want 7", len(got.Hash))
	}
}

func TestExternalMonitorSourceRejectsNilCR(t *testing.T) {
	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}

	got, err := src.FromExternalMonitor(nil)
	if err == nil {
		t.Fatal("FromExternalMonitor returned nil error, want error")
	}
	if !errors.Is(err, ErrExternalMonitorNil) {
		t.Fatalf("FromExternalMonitor error = %v, want ErrExternalMonitorNil", err)
	}
	if got.Name != "" || got.Resource != "" || got.Owner != "" {
		t.Fatalf("FromExternalMonitor result = %+v, want zero value", got)
	}
}

func TestExternalMonitorSourceDefaultsNameAndMethod(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-health",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	if err != nil {
		t.Fatalf("FromExternalMonitor returned error: %v", err)
	}
	if got.Name != "default/api-health" {
		t.Fatalf("Name = %q, want default/api-health", got.Name)
	}
	if got.Method != "GET" {
		t.Fatalf("Method = %q, want GET", got.Method)
	}
}

func TestExternalMonitorSourceRejectsInvalidHashLength(t *testing.T) {
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-health",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 0}
	_, err := src.FromExternalMonitor(cr)
	if err == nil {
		t.Fatal("FromExternalMonitor returned nil error, want error")
	}
}
