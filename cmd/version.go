package cmd

import "github.com/spf13/cobra"

var versionCmd = &cobra.Command{
	Use:         "version",
	Short:       "Show version information",
	Annotations: map[string]string{"skipUpdateCheck": "true"},
	Args:        wrapArgs(cobra.NoArgs),
	Run: func(cmd *cobra.Command, args []string) {
		printVersion(cmd)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
