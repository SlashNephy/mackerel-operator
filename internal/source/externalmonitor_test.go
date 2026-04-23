package source

import (
	"testing"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExternalMonitorSourceBuildsDesiredMonitor(t *testing.T) {
	interval := 10
	expectedStatus := 200
	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "api-health",
		},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			Name:                 "API health check",
			Service:              "my-service",
			URL:                  "https://api.example.com/healthz",
			Method:               "GET",
			NotificationInterval: &interval,
			ExpectedStatusCode:   &expectedStatus,
			Memo:                 "human memo",
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
