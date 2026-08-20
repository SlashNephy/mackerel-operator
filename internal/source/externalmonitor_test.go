package source

import (
	"errors"
	"testing"

	mackerelv1alpha1 "github.com/SlashNephy/mackerel-operator/api/v1alpha1"
	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultMethod = "GET"

func TestExternalMonitorSourceBuildsDesiredMonitor(t *testing.T) {
	interval := 10
	expectedStatus := 200
	responseTimeWarning := 30
	responseTimeCritical := 60
	responseTimeDuration := 5
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
			Method:                          defaultMethod,
			NotificationInterval:            &interval,
			ExpectedStatusCode:              &expectedStatus,
			ContainsString:                  containsString,
			ResponseTimeWarning:             &responseTimeWarning,
			ResponseTimeCritical:            &responseTimeCritical,
			ResponseTimeDuration:            &responseTimeDuration,
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
	if got.Method != defaultMethod {
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
	if got.ResponseTimeDuration == nil || *got.ResponseTimeDuration != responseTimeDuration {
		t.Fatalf("ResponseTimeDuration = %v, want %d", got.ResponseTimeDuration, responseTimeDuration)
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
	if got.Method != defaultMethod {
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

func TestExternalMonitorSourceMapsRemainingFields(t *testing.T) {
	t.Parallel()

	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL:                         "https://api.example.com/healthz",
			IsMute:                      true,
			FollowRedirect:              true,
			SkipCertificateVerification: true,
			MaxCheckAttempts:            3,
			RequestBody:                 `{"ping":true}`,
			Dualstack:                   "auto",
			Headers: []mackerelv1alpha1.HeaderField{
				{Name: "X-Zebra", Value: new("last")},
				{Name: "Authorization", Value: new("Bearer token")},
			},
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	require.NoError(t, err)

	assert.True(t, got.IsMute)
	assert.True(t, got.FollowRedirect)
	assert.True(t, got.SkipCertificateVerification)
	assert.Equal(t, 3, got.MaxCheckAttempts)
	assert.Equal(t, `{"ping":true}`, got.RequestBody)
	assert.Equal(t, "auto", got.Dualstack)
	assert.Equal(t, []monitor.HeaderField{
		{Name: "Authorization", Value: "Bearer token"},
		{Name: "X-Zebra", Value: "last"},
	}, got.Headers, "headers must be sorted by name so drift detection is order-insensitive")
}

func TestExternalMonitorSourceNormalisesMaxCheckAttempts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec int
		want int
	}{
		{name: "unset defaults to one", spec: 0, want: 1},
		{name: "explicit one", spec: 1, want: 1},
		{name: "explicit three", spec: 3, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cr := &mackerelv1alpha1.ExternalMonitor{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
				Spec: mackerelv1alpha1.ExternalMonitorSpec{
					URL:              "https://api.example.com/healthz",
					MaxCheckAttempts: tt.spec,
				},
			}

			src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
			got, err := src.FromExternalMonitor(cr)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.MaxCheckAttempts)
		})
	}
}

func TestExternalMonitorSourceEmitsEmptyHeadersSlice(t *testing.T) {
	t.Parallel()

	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	require.NoError(t, err)

	assert.NotNil(t, got.Headers, "prefer an empty slice over nil")
	assert.Empty(t, got.Headers)
}

// TestExternalMonitorSourceLeavesDualstackUnset pins the deliberate asymmetry
// with maxCheckAttempts: an unset dualstack is *not* widened to ipv4 here.
// Materialising the default would put the key into the desired JSON and change
// the hash of every ExternalMonitor that predates the field, forcing an Update
// on upgrade. The ipv4 semantics are applied in the planner and the provider
// instead.
func TestExternalMonitorSourceLeavesDualstackUnset(t *testing.T) {
	t.Parallel()

	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
		},
	}

	src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7}
	got, err := src.FromExternalMonitor(cr)
	require.NoError(t, err)

	assert.Empty(t, got.Dualstack)
}

func TestExternalMonitorSourceResolvesHeaderValues(t *testing.T) {
	t.Parallel()

	secretHeader := mackerelv1alpha1.HeaderField{
		Name: "Authorization",
		ValueFrom: &mackerelv1alpha1.HeaderValueSource{
			SecretKeyRef: &mackerelv1alpha1.SecretKeySelector{Name: "api-credentials", Key: "token"},
		},
	}

	tests := []struct {
		name         string
		headers      []mackerelv1alpha1.HeaderField
		headerValues map[string]string
		want         []monitor.HeaderField
		wantErr      bool
	}{
		{
			name:         "resolved value replaces the reference",
			headers:      []mackerelv1alpha1.HeaderField{secretHeader},
			headerValues: map[string]string{"Authorization": "Bearer token"},
			want:         []monitor.HeaderField{{Name: "Authorization", Value: "Bearer token"}},
		},
		{
			name: "resolved and inline values are sorted together",
			headers: []mackerelv1alpha1.HeaderField{
				{Name: "X-Zebra", Value: new("last")},
				secretHeader,
			},
			headerValues: map[string]string{"Authorization": "Bearer token"},
			want: []monitor.HeaderField{
				{Name: "Authorization", Value: "Bearer token"},
				{Name: "X-Zebra", Value: "last"},
			},
		},
		{
			name:         "an empty secret value is sent as an empty header",
			headers:      []mackerelv1alpha1.HeaderField{secretHeader},
			headerValues: map[string]string{"Authorization": ""},
			want:         []monitor.HeaderField{{Name: "Authorization", Value: ""}},
		},
		{
			name:         "a caller that resolves headers must resolve all of them",
			headers:      []mackerelv1alpha1.HeaderField{secretHeader},
			headerValues: map[string]string{},
			wantErr:      true,
		},
		{
			name: "a caller that does not resolve headers drops them",
			headers: []mackerelv1alpha1.HeaderField{
				{Name: "X-Zebra", Value: new("last")},
				secretHeader,
			},
			headerValues: nil,
			want:         []monitor.HeaderField{{Name: "X-Zebra", Value: "last"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cr := &mackerelv1alpha1.ExternalMonitor{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
				Spec: mackerelv1alpha1.ExternalMonitorSpec{
					URL:     "https://api.example.com/healthz",
					Headers: tt.headers,
				},
			}

			src := ExternalMonitorSource{OwnerID: "prod", HashLength: 7, HeaderValues: tt.headerValues}
			got, err := src.FromExternalMonitor(cr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Headers)
		})
	}
}

func TestExternalMonitorSourceHashesResolvedHeaderValues(t *testing.T) {
	t.Parallel()

	cr := &mackerelv1alpha1.ExternalMonitor{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "api-health"},
		Spec: mackerelv1alpha1.ExternalMonitorSpec{
			URL: "https://api.example.com/healthz",
			Headers: []mackerelv1alpha1.HeaderField{{
				Name: "Authorization",
				ValueFrom: &mackerelv1alpha1.HeaderValueSource{
					SecretKeyRef: &mackerelv1alpha1.SecretKeySelector{Name: "api-credentials", Key: "token"},
				},
			}},
		},
	}

	before, err := ExternalMonitorSource{
		OwnerID:      "prod",
		HashLength:   7,
		HeaderValues: map[string]string{"Authorization": "Bearer token"},
	}.FromExternalMonitor(cr)
	require.NoError(t, err)

	after, err := ExternalMonitorSource{
		OwnerID:      "prod",
		HashLength:   7,
		HeaderValues: map[string]string{"Authorization": "Bearer rotated"},
	}.FromExternalMonitor(cr)
	require.NoError(t, err)

	assert.NotEqual(t, before.Hash, after.Hash, "rotating a referenced Secret must move the desired hash")
}
