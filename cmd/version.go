package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:         "version",
	Short:       "Show version information",
	Annotations: map[string]string{"skipUpdateCheck": "true"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), versionTemplate())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
