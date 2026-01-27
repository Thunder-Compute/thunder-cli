package cmd

import (
	"errors"
	"time"

	"github.com/Thunder-Compute/thunder-cli/internal/version"
	"github.com/Thunder-Compute/thunder-cli/sentry"
	"github.com/Thunder-Compute/thunder-cli/tui"
	sentrygo "github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
)

// WrapCommandWithSentry wraps a cobra.Command's Run function
// to automatically capture panics to Sentry
func WrapCommandWithSentry(cmd *cobra.Command) {
	if cmd.Run == nil {
		return
	}

	originalRun := cmd.Run
	cmd.Run = func(c *cobra.Command, args []string) {
		defer sentry.CapturePanic(&sentry.EventOptions{
			Tags: sentry.NewTags().
				Set("command", cmd.Name()).
				Set("version", version.BuildVersion),
		})

		originalRun(c, args)
	}
}

// CaptureCommandError captures errors from command execution
// It intelligently filters out user cancellations and categorizes errors
func CaptureCommandError(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}

	// Don't capture user cancellations
	var cancellationErr *tui.CancellationError
	if errors.As(err, &cancellationErr) {
		return
	}

	// Capture the error with context
	eventID := sentry.CaptureError(err, &sentry.EventOptions{
		Tags: sentry.NewTags().
			Set("command", cmd.Name()).
			Set("version", version.BuildVersion).
			Set("error_type", getErrorType(err)),
		Extra: sentry.NewExtra().
			Set("args", cmd.Flags().Args()),
		Level: ptr(getLogLevelForError(err)),
	})

	if eventID != nil {
		// Flush to ensure the event is sent before the process exits
		// (os.Exit skips deferred functions like sentry.Shutdown)
		sentry.Flush(2 * time.Second)
		// fmt.Printf("\nError report sent to Sentry (event: %s, command: %s, type: %s)\n", *eventID, cmd.Name(), getErrorType(err))
	}
	// else {
	// 	fmt.Printf("\nSentry not initialized (DSN not set in build) - error not reported\n")
	// }
}

// ptr is a helper to create a pointer to a value
func ptr[T any](v T) *T {
	return &v
}

// getLogLevelForError determines the appropriate Sentry level for an error
func getLogLevelForError(err error) sentrygo.Level {
	errStr := err.Error()

	// Authentication and critical errors
	if containsAny(errStr, []string{"authentication", "401", "403", "unauthorized"}) {
		return sentrygo.LevelError
	}

	// Network and timeout errors (less critical)
	if containsAny(errStr, []string{"timeout", "connection refused", "no route to host"}) {
		return sentrygo.LevelWarning
	}

	// Not found errors (informational)
	if containsAny(errStr, []string{"not found", "404"}) {
		return sentrygo.LevelWarning
	}

	// Server errors (critical)
	if containsAny(errStr, []string{"500", "502", "503", "504"}) {
		return sentrygo.LevelError
	}

	// Default to error level
	return sentrygo.LevelError
}

// containsAny checks if the string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains is a simple substring check (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-sensitive check for now
	// Could be enhanced with strings.ToLower for case-insensitive
	return len(s) >= len(substr) && indexSubstring(s, substr) >= 0
}

func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
