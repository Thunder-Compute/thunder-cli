package autoupdate

import (
	"context"
	"runtime"
)

// installer abstracts platform-specific installation logic.
type installer interface {
	Install(ctx context.Context, exe, newBinary, version string, src Source) error
}

// detectInstaller returns the appropriate installer implementation for the current platform.
func detectInstaller() installer {
	if runtime.GOOS == "windows" {
		return windowsInstaller{}
	}
	return unixInstaller{}
}
