package types

import "testing"

func TestSnapshotReadiness(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		normalized string
		ready      bool
	}{
		{name: "ready", status: "READY", normalized: "READY", ready: true},
		{name: "ready is case insensitive", status: " ready ", normalized: "READY", ready: true},
		{name: "creating", status: "CREATING", normalized: "CREATING", ready: false},
		{name: "failed", status: "FAILED", normalized: "FAILED", ready: false},
		{name: "empty is unknown", status: "", normalized: "UNKNOWN", ready: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := Snapshot{Status: tt.status}
			if got := snapshot.NormalizedStatus(); got != tt.normalized {
				t.Fatalf("NormalizedStatus() = %q, want %q", got, tt.normalized)
			}
			if got := snapshot.IsReady(); got != tt.ready {
				t.Fatalf("IsReady() = %t, want %t", got, tt.ready)
			}
		})
	}
}
