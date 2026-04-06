package cmd

import (
	"errors"
	"fmt"
)

// ErrUsage is a sentinel error for user-input validation failures
// (wrong flags, missing args, resource not found, etc.).
// Errors wrapping ErrUsage are excluded from Sentry reporting.
var ErrUsage = errors.New("usage error")

// usageErr returns an error wrapping ErrUsage. Works like fmt.Errorf
// but automatically prepends the sentinel so isUserError catches it.
func usageErr(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrUsage}, args...)...)
}
