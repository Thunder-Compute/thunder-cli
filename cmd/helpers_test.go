package cmd

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/stretchr/testify/assert"
)

func sampleInstances() []api.Instance {
	ip1 := "10.0.0.1"
	ip2 := "10.0.0.2"
	return []api.Instance{
		{ID: "1", UUID: "uuid-aaa", Name: "alpha", IP: &ip1, Status: "RUNNING"},
		{ID: "2", UUID: "uuid-bbb", Name: "beta", IP: &ip2, Status: "STOPPED"},
	}
}

func TestFindInstance(t *testing.T) {
	instances := sampleInstances()

	tests := []struct {
		name       string
		identifier string
		wantID     string
		wantNil    bool
	}{
		{name: "by ID", identifier: "1", wantID: "1"},
		{name: "by UUID", identifier: "uuid-bbb", wantID: "2"},
		{name: "by name", identifier: "alpha", wantID: "1"},
		{name: "not found", identifier: "nonexistent", wantNil: true},
		{name: "empty identifier", identifier: "", wantNil: true},
		{name: "partial ID no match", identifier: "uu", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findInstance(instances, tt.identifier)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.wantID, result.ID)
			}
		})
	}
}

func TestFindInstance_EmptyList(t *testing.T) {
	result := findInstance([]api.Instance{}, "anything")
	assert.Nil(t, result)
}

func TestFindInstance_ReturnsPointerToOriginal(t *testing.T) {
	instances := sampleInstances()
	result := findInstance(instances, "1")
	assert.NotNil(t, result)
	// Verify it's a pointer into the original slice, not a copy
	assert.Equal(t, &instances[0], result)
}

func TestFindInstance_PrefersFirstMatch(t *testing.T) {
	// If ID of one instance matches name of another, ID match wins (it's found first)
	instances := []api.Instance{
		{ID: "alpha", UUID: "uuid-1", Name: "first"},
		{ID: "2", UUID: "uuid-2", Name: "alpha"},
	}
	result := findInstance(instances, "alpha")
	assert.NotNil(t, result)
	assert.Equal(t, "alpha", result.ID)
}
