package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// ErrUsage is a sentinel error for user-input validation failures
// (wrong flags, missing args, resource not found, etc.).
// Errors wrapping ErrUsage are excluded from Sentry reporting.
var ErrUsage = errors.New("usage error")

// usageErr returns an error wrapping ErrUsage. Works like fmt.Errorf
// but automatically prepends the sentinel so isUserError catches it.
// wrapArgs wraps a Cobra positional-args validator so that its errors
// are tagged with ErrUsage, keeping them out of Sentry.
func wrapArgs(fn cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			return fmt.Errorf("%w: %s", ErrUsage, err)
		}
		return nil
	}
}

func usageErr(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrUsage}, args...)...)
}
