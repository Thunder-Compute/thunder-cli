package main

import (
	"fmt"
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
)

func TestErrorFingerprint(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		original error
		want     string
	}{
		{
			name: "ssh session exit without status",
			msg:  "SSH session failed: wait: remote command exited without exit status or exit signal",
			want: "ssh_session_failed",
		},
		{
			name: "ssh session raw terminal mode",
			msg:  "SSH session failed: failed to set raw terminal mode: operation not supported by device",
			want: "ssh_session_failed",
		},
		{
			name: "ssh session ioctl",
			msg:  "SSH session failed: failed to set raw terminal mode: inappropriate ioctl for device",
			want: "ssh_session_failed",
		},
		{
			name: "add ssh key 500 from message",
			msg:  `failed to add SSH key: API request failed with status 500: {"error":"ssh_key_add_failed","message":"Failed to add SSH key to instance. Please try again.","code":500}`,
			want: "failed_to_add_ssh_key_500",
		},
		{
			name: "add ssh key 409 from message",
			msg:  `failed to add SSH key: API request failed with status 409: {"error":"ssh_key_duplicate","message":"An SSH key with this fingerprint already exists","code":409}`,
			want: "failed_to_add_ssh_key_409",
		},
		{
			name: "delete instance 404 from message",
			msg:  `failed to delete instance: API request failed with status 404: {"error":"instance_not_found","message":"Instance not found or not supported for deletion","code":404}`,
			want: "failed_to_delete_instance_404",
		},
		{
			name: "ssh service not available",
			msg:  "SSH service not available: TCP port check failed: dial tcp 38.128.233.54:31540: connectex: No connection could be made because the target machine actively refused it.",
			want: "ssh_service_not_available",
		},
		{
			name: "no colon separator uses full message",
			msg:  "something went wrong",
			want: "something_went_wrong",
		},
		{
			name: "arbitrary status code extracted from message",
			msg:  "failed to create snapshot: API request failed with status 422: validation error",
			want: "failed_to_create_snapshot_422",
		},
		// APIError fallback: when the error message is a clean JSON-parsed
		// message without "status NNN", extract the code from the wrapped error.
		{
			name:     "add ssh key 409 from APIError",
			msg:      "failed to add SSH key: An SSH key with this fingerprint already exists",
			original: fmt.Errorf("failed to add SSH key: %w", &api.APIError{StatusCode: 409, Message: "An SSH key with this fingerprint already exists"}),
			want:     "failed_to_add_ssh_key_409",
		},
		{
			name:     "add ssh key 500 from APIError",
			msg:      "failed to add SSH key: Failed to add SSH key to instance. Please try again.",
			original: fmt.Errorf("failed to add SSH key: %w", &api.APIError{StatusCode: 500, Message: "Failed to add SSH key to instance. Please try again."}),
			want:     "failed_to_add_ssh_key_500",
		},
		{
			name:     "delete instance 404 from APIError",
			msg:      "failed to delete instance: Instance not found or not supported for deletion",
			original: fmt.Errorf("failed to delete instance: %w", &api.APIError{StatusCode: 404, Message: "Instance not found or not supported for deletion"}),
			want:     "failed_to_delete_instance_404",
		},
		{
			name:     "non-API error with nil original",
			msg:      "SSH session failed: connection reset",
			original: nil,
			want:     "ssh_session_failed",
		},
		{
			name:     "non-API error ignores original",
			msg:      "SSH session failed: connection reset",
			original: fmt.Errorf("connection reset"),
			want:     "ssh_session_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := errorFingerprint(tt.msg, tt.original)
			if got != tt.want {
				t.Errorf("errorFingerprint(%q, ...)\n  got  %q\n  want %q", tt.msg, got, tt.want)
			}
		})
	}
}
