package cmd

import "strings"

// getErrorType categorizes errors for better Sentry grouping
func getErrorType(err error) string {
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "unauthorized"):
		return "auth_error"

	case strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "forbidden"):
		return "permission_error"

	case strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "dial") ||
		strings.Contains(errStr, "no route to host"):
		return "network_error"

	case strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "404"):
		return "not_found"

	case strings.Contains(errStr, "instance"):
		return "instance_error"

	case strings.Contains(errStr, "ssh"):
		return "ssh_error"

	case strings.Contains(errStr, "scp") ||
		strings.Contains(errStr, "file transfer"):
		return "file_transfer_error"

	case strings.Contains(errStr, "api") ||
		strings.Contains(errStr, "status") ||
		strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504"):
		return "api_error"

	case strings.Contains(errStr, "config") ||
		strings.Contains(errStr, "configuration"):
		return "config_error"

	case strings.Contains(errStr, "token") ||
		strings.Contains(errStr, "credential"):
		return "credential_error"

	case strings.Contains(errStr, "json") ||
		strings.Contains(errStr, "unmarshal") ||
		strings.Contains(errStr, "parse"):
		return "parsing_error"

	case strings.Contains(errStr, "snapshot"):
		return "snapshot_error"

	default:
		return "unknown_error"
	}
}
