//go:build windows

package autoupdate

import (
	"context"
)

// unixInstaller is only implemented on Unix platforms; this stub should never be
// used on Windows.
type unixInstaller struct{}

// Install is only implemented on Unix platforms; this stub should never be called.
func (u unixInstaller) Install(ctx context.Context, exe, newBinary, version string, src Source) error {
	return nil
}
