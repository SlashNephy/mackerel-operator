package provider

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/SlashNephy/mackerel-operator/internal/monitor"
	mackerel "github.com/mackerelio/mackerel-client-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeMackerelExternalMonitorOwnsManagedFields(t *testing.T) {
	t.Parallel()

	// Every field the operator manages is overwritten from the desired state,
	// even when the live monitor disagrees. Only server-assigned identity
	// survives, so a change made in the Mackerel web UI is reverted.
	base := &mackerel.MonitorExternalHTTP{
		ID:                          "mon-1",
		Type:                        "external",
		IsMute:                      true,
		MaxCheckAttempts:            5,
		ResponseTimeDuration:        new(uint64(3)),
		RequestBody:                 "stale payload",
		SkipCertificateVerification: true,
		FollowRedirect:              true,
		Headers: []mackerel.HeaderField{
			{Name: "X-Stale", Value: "removed"},
		},
	}
	desired := monitor.DesiredExternalMonitor{
		Name:                            "API health",
		Service:                         "my-service",
		URL:                             "https://api.example.com/healthz",
		Method:                          "GET",
		NotificationInterval:            new(10),
		ExpectedStatusCode:              new(200),
		ContainsString:                  "ok",
		ResponseTimeWarning:             new(1000),
		ResponseTimeCritical:            new(2000),
		CertificationExpirationWarning:  new(30),
		CertificationExpirationCritical: new(14),
		IsMute:                          false,
		FollowRedirect:                  false,
		SkipCertificateVerification:     false,
		MaxCheckAttempts:                2,
		RequestBody:                     `{"ping":true}`,
		Headers: []monitor.HeaderField{
			{Name: "Authorization", Value: "Bearer token"},
		},
	}

	got, err := mergeMackerelExternalMonitor(base, desired, "human memo")
	require.NoError(t, err)

	assert.Equal(t, "mon-1", got.ID, "server-assigned identity is preserved")
	assert.Equal(t, "external", got.Type)

	assert.False(t, got.IsMute)
	assert.False(t, got.FollowRedirect)
	assert.False(t, got.SkipCertificateVerification)
	assert.Equal(t, uint64(2), got.MaxCheckAttempts)
	assert.Equal(t, `{"ping":true}`, got.RequestBody)
	assert.Equal(t, []mackerel.HeaderField{{Name: "Authorization", Value: "Bearer token"}}, got.Headers)
	require.NotNil(t, got.Dualstack)
	assert.Equal(t, mackerel.DualstackIPv4, *got.Dualstack)

	assert.Nil(t, got.ResponseTimeDuration)
	assert.Equal(t, desired.Name, got.Name)
	assert.Equal(t, desired.Service, got.Service)
	assert.Equal(t, desired.URL, got.URL)
	assert.Equal(t, desired.Method, got.Method)
	assert.Equal(t, "human memo", got.Memo)
	assert.Equal(t, uint64(10), got.NotificationInterval)
}

func TestMergeMackerelExternalMonitorClearsHeadersWithEmptySlice(t *testing.T) {
	t.Parallel()

	// mackerel-client-go tags Headers without omitempty precisely so that an
	// empty list can be transmitted. Sending nil would leave the live headers
	// in place, so clearing every header has to produce a non-nil empty slice.
	base := &mackerel.MonitorExternalHTTP{
		Headers: []mackerel.HeaderField{{Name: "X-Stale", Value: "removed"}},
	}
	desired := monitor.DesiredExternalMonitor{
		Name:    "API health",
		URL:     "https://api.example.com/healthz",
		Method:  "GET",
		Headers: []monitor.HeaderField{},
	}

	got, err := mergeMackerelExternalMonitor(base, desired, "")
	require.NoError(t, err)

	require.NotNil(t, got.Headers)
	assert.Empty(t, got.Headers)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"headers":[]`)
}

func TestActualExternalMonitorFromMackerelReadsRemainingFields(t *testing.T) {
	t.Parallel()

	got := actualExternalMonitorFromMackerel(&mackerel.MonitorExternalHTTP{
		ID:                          "mon-1",
		Name:                        "API health",
		URL:                         "https://api.example.com/healthz",
		Method:                      "GET",
		IsMute:                      true,
		FollowRedirect:              true,
		SkipCertificateVerification: true,
		MaxCheckAttempts:            4,
		RequestBody:                 `{"ping":true}`,
		Dualstack:                   new(mackerel.DualstackIPv6),
		Headers: []mackerel.HeaderField{
			{Name: "X-Zebra", Value: "last"},
			{Name: "Authorization", Value: "Bearer token"},
		},
	})

	assert.True(t, got.IsMute)
	assert.True(t, got.FollowRedirect)
	assert.True(t, got.SkipCertificateVerification)
	assert.Equal(t, 4, got.MaxCheckAttempts)
	assert.Equal(t, `{"ping":true}`, got.RequestBody)
	assert.Equal(t, "ipv6", got.Dualstack)
	assert.Equal(t, []monitor.HeaderField{
		{Name: "X-Zebra", Value: "last"},
		{Name: "Authorization", Value: "Bearer token"},
	}, got.Headers, "the provider reads headers verbatim; the planner handles ordering")
}

func TestMergeMackerelExternalMonitorRejectsNegativeMaxCheckAttempts(t *testing.T) {
	t.Parallel()

	desired := monitor.DesiredExternalMonitor{
		Name:             "API health",
		URL:              "https://api.example.com/healthz",
		Method:           "GET",
		MaxCheckAttempts: -1,
	}

	_, err := mergeMackerelExternalMonitor(&mackerel.MonitorExternalHTTP{}, desired, "")
	require.ErrorIs(t, err, errNegativeIntValue)
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

// TestMergeMackerelExternalMonitorAlwaysSendsDualstack covers the one external
// monitor field Mackerel does *not* reset when the key is absent from a PUT.
// Because the stored value survives omission, an unset CR has to be written as
// an explicit ipv4 rather than left out: the planner reads an unset CR as ipv4,
// so omitting the key would leave a monitor set to ipv6 elsewhere unchanged and
// the same Update would be reissued on every reconcile.
func TestMergeMackerelExternalMonitorAlwaysSendsDualstack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desired string
		want    mackerel.Dualstack
	}{
		{name: "unset is written as ipv4", desired: "", want: mackerel.DualstackIPv4},
		{name: "ipv4", desired: "ipv4", want: mackerel.DualstackIPv4},
		{name: "ipv6", desired: "ipv6", want: mackerel.DualstackIPv6},
		{name: "auto", desired: "auto", want: mackerel.DualstackAuto},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := mergeMackerelExternalMonitor(
				&mackerel.MonitorExternalHTTP{Dualstack: new(mackerel.DualstackAuto)},
				monitor.DesiredExternalMonitor{
					Name:      "API health",
					URL:       "https://api.example.com/healthz",
					Method:    "GET",
					Dualstack: tt.desired,
				},
				"",
			)
			require.NoError(t, err)

			require.NotNil(t, got.Dualstack)
			assert.Equal(t, tt.want, *got.Dualstack)

			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			assert.Contains(t, string(encoded), `"dualstack":"`+string(tt.want)+`"`)
		})
	}
}

// TestActualExternalMonitorFromMackerelLeavesAbsentDualstackEmpty pins the read
// side: an absent key stays empty rather than being widened here, so the
// widening lives in exactly one place, the planner comparison.
func TestActualExternalMonitorFromMackerelLeavesAbsentDualstackEmpty(t *testing.T) {
	t.Parallel()

	got := actualExternalMonitorFromMackerel(&mackerel.MonitorExternalHTTP{
		ID:   "mon-1",
		Name: "API health",
		URL:  "https://api.example.com/healthz",
	})

	assert.Empty(t, got.Dualstack)
}
