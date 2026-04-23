package monitor

import "testing"

func TestHashDesiredDefaultLength(t *testing.T) {
	desired := DesiredExternalMonitor{
		Name: "API health check",
		URL:  "https://api.example.com/healthz",
	}

	got, err := HashDesired(desired, 7)
	if err != nil {
		t.Fatalf("HashDesired returned error: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("len(hash) = %d, want 7", len(got))
	}
}

func TestHashDesiredIsStable(t *testing.T) {
	expectedStatusCode := 200
	desired := DesiredExternalMonitor{
		Name:               "API health check",
		URL:                "https://api.example.com/healthz",
		Method:             "GET",
		ExpectedStatusCode: &expectedStatusCode,
	}

	first, err := HashDesired(desired, 12)
	if err != nil {
		t.Fatalf("HashDesired first returned error: %v", err)
	}
	second, err := HashDesired(desired, 12)
	if err != nil {
		t.Fatalf("HashDesired second returned error: %v", err)
	}
	if first != second {
		t.Fatalf("hash changed: first=%q second=%q", first, second)
	}
}

func TestHashDesiredIgnoresHashField(t *testing.T) {
	expectedStatusCode := 200
	base := DesiredExternalMonitor{
		Name:               "API health check",
		URL:                "https://api.example.com/healthz",
		Method:             "GET",
		ExpectedStatusCode: &expectedStatusCode,
	}

	first, err := HashDesired(DesiredExternalMonitor{
		Name:               base.Name,
		URL:                base.URL,
		Method:             base.Method,
		ExpectedStatusCode: base.ExpectedStatusCode,
		Hash:               "aaaaaaaaaaaa",
	}, 12)
	if err != nil {
		t.Fatalf("HashDesired first returned error: %v", err)
	}

	second, err := HashDesired(DesiredExternalMonitor{
		Name:               base.Name,
		URL:                base.URL,
		Method:             base.Method,
		ExpectedStatusCode: base.ExpectedStatusCode,
		Hash:               "bbbbbbbbbbbb",
	}, 12)
	if err != nil {
		t.Fatalf("HashDesired second returned error: %v", err)
	}

	if first != second {
		t.Fatalf("hash changed when only Hash field differed: first=%q second=%q", first, second)
	}
}

func TestHashDesiredRejectsInvalidLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "negative",
			length: -1,
		},
		{
			name:   "zero",
			length: 0,
		},
		{
			name:   "too long",
			length: 65,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := HashDesired(DesiredExternalMonitor{}, tt.length)
			if err == nil {
				t.Fatalf("HashDesired length %d error = nil, want error", tt.length)
			}
		})
	}
}
