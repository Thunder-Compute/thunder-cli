package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "no truncation needed", input: "hello", maxLen: 10, want: "hello"},
		{name: "exact length", input: "hello", maxLen: 5, want: "hello"},
		{name: "truncated with ellipsis", input: "hello world", maxLen: 8, want: "hello..."},
		{name: "maxLen 3 no ellipsis", input: "hello", maxLen: 3, want: "hel"},
		{name: "maxLen 2", input: "hello", maxLen: 2, want: "he"},
		{name: "maxLen 1", input: "hello", maxLen: 1, want: "h"},
		{name: "maxLen 4", input: "hello", maxLen: 4, want: "h..."},
		{name: "empty string", input: "", maxLen: 5, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateStr(tt.input, tt.maxLen))
		})
	}
}
