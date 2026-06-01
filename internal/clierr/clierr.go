package clierr

import "errors"

// UserFacing marks errors caused by user input, user cancellation, local
// environment state, or expected remote/service responses. These are shown to
// the user but should not be captured by the CLI catch-all Sentry handler.
type UserFacing interface {
	error
	UserFacing() bool
}

type userFacingError struct {
	err error
}

func (e *userFacingError) Error() string    { return e.err.Error() }
func (e *userFacingError) Unwrap() error    { return e.err }
func (e *userFacingError) UserFacing() bool { return true }

// New returns a stable user-facing sentinel error.
func New(message string) error {
	return Mark(errors.New(message))
}

// Mark wraps err so IsUserFacing classifies it as safe to suppress from the
// catch-all Sentry path. Nil is preserved for easier use in error helpers.
func Mark(err error) error {
	if err == nil {
		return nil
	}
	if IsUserFacing(err) {
		return err
	}
	return &userFacingError{err: err}
}

func IsUserFacing(err error) bool {
	var userErr UserFacing
	return errors.As(err, &userErr) && userErr.UserFacing()
}
