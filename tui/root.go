/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package tui

import (
	helpmenus "github.com/joshuawatkins04/thunder-cli-draft/tui/help-menus"
	"github.com/spf13/cobra"
)

func RenderCustomHelp(cmd *cobra.Command) {
	helpmenus.RenderRootHelp(cmd)
}
