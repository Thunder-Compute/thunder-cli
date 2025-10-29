/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package tui

import (
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/spf13/cobra"
)

func RenderCustomHelp(cmd *cobra.Command) {
	helpmenus.RenderRootHelp(cmd)
}
