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
	desired := DesiredExternalMonitor{
		Name:               "API health check",
		URL:                "https://api.example.com/healthz",
		Method:             "GET",
		ExpectedStatusCode: intPtr(200),
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

func TestHashDesiredRejectsInvalidLength(t *testing.T) {
	_, err := HashDesired(DesiredExternalMonitor{}, 0)
	if err == nil {
		t.Fatal("HashDesired length 0 error = nil, want error")
	}
}

func intPtr(v int) *int {
	return &v
}
