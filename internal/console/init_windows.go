//go:build windows
// +build windows

package console

import (
	"os"

	"golang.org/x/sys/windows"
)

func Init() {
	// Enable UTF-8 for emoji support
	_ = windows.SetConsoleOutputCP(65001)
	_ = windows.SetConsoleCP(65001)

	// Enable ANSI escape sequences (VT mode)
	const ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	handle := windows.Handle(os.Stdout.Fd())

	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err == nil {
		mode |= ENABLE_VIRTUAL_TERMINAL_PROCESSING
		_ = windows.SetConsoleMode(handle, mode)
	}
}
