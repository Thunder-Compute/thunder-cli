/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"github.com/Thunder-Compute/thunder-cli/cmd"
	"github.com/Thunder-Compute/thunder-cli/internal/autoupdate"
	"github.com/Thunder-Compute/thunder-cli/internal/console"
)

func main() {
	// On Windows, this allows the same binary to act as an elevated helper
	// process for staging updates when triggered via UAC. On other platforms
	// this is a no-op.
	if autoupdate.MaybeRunWindowsUpdateHelper() {
		return
	}

	console.Init()
	cmd.Execute()
}
