package clierr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Thunder-Compute/thunder-cli/internal/clierr"
)

func TestNewUserFacingError(t *testing.T) {
	err := clierr.New("bad input")
	if err.Error() != "bad input" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "bad input")
	}
	if !clierr.IsUserFacing(err) {
		t.Fatal("New error should be user-facing")
	}
	if !errors.Is(err, err) {
		t.Fatal("New should return a stable comparable sentinel")
	}
}

func TestWrappedUserFacingError(t *testing.T) {
	sentinel := clierr.New("bad input")
	err := fmt.Errorf("wrapped: %w", sentinel)

	if !errors.Is(err, sentinel) {
		t.Fatal("wrapped error should match sentinel")
	}
	if !clierr.IsUserFacing(err) {
		t.Fatal("wrapped sentinel should remain user-facing")
	}
}

func TestMark(t *testing.T) {
	base := errors.New("local environment problem")
	err := clierr.Mark(base)

	if !errors.Is(err, base) {
		t.Fatal("marked error should wrap the original")
	}
	if !clierr.IsUserFacing(err) {
		t.Fatal("marked error should be user-facing")
	}
	if clierr.Mark(nil) != nil {
		t.Fatal("Mark(nil) should return nil")
	}
	if clierr.Mark(err) != err {
		t.Fatal("Mark should preserve already-user-facing errors")
	}
}

func TestIsUserFacingFalse(t *testing.T) {
	if clierr.IsUserFacing(nil) {
		t.Fatal("nil should not be user-facing")
	}
	if clierr.IsUserFacing(errors.New("internal failure")) {
		t.Fatal("plain errors should not be user-facing")
	}
}
