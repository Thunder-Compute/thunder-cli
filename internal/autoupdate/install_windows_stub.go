//go:build !windows

package autoupdate

import (
	"context"
)

// MaybeRunWindowsUpdateHelper is a no-op on non-Windows platforms.
func MaybeRunWindowsUpdateHelper() bool {
	return false
}

// windowsInstaller is only implemented on Windows; this stub should never be
// used on other platforms.
type windowsInstaller struct{}

// Install is only implemented on Windows; this stub should never be called.
func (w windowsInstaller) Install(ctx context.Context, exe, newBinary, version string, src Source) error {
	return nil
}

// installOnWindows is only implemented on Windows; this stub should never be
// called on other platforms.
func installOnWindows(ctx context.Context, exe, newBinary, version string, src Source) error {
	return nil
}
