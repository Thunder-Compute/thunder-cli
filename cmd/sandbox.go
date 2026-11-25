package cmd

import (
	"fmt"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/spf13/cobra"
)

func init() {
	var request api.StartSandboxRequest
	request.Lifetime.EnforceTTL = true
	request.NetworkPolicy.InternetAccess = "open"

	create := &cobra.Command{
		Use:   "create",
		Short: "Start a GPU sandbox from a ready Thunder image",
		Args:  wrapArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			input := request
			input.ImageID = strings.TrimSpace(input.ImageID)
			input.Spec.GPUType = strings.ToUpper(strings.TrimSpace(input.Spec.GPUType))
			if input.ImageID == "" || input.Spec.GPUType == "" {
				return usageErr("--image-id and --gpu are required")
			}
			if input.Lifetime.MaxTTLSeconds < 1 || input.Spec.CPUCount < 1 || input.Spec.MemoryGiB < 1 || input.Spec.StorageGiB < 1 {
				return usageErr("--timeout, --vcpus, --memory and --storage must be positive")
			}
			client, err := getAuthenticatedClient()
			if err != nil {
				return err
			}
			var response api.StartSandboxResponse
			start := func() error {
				var startErr error
				response, startErr = client.StartSandbox(cmd.Context(), input)
				return startErr
			}
			if tui.IsInteractive() && !JSONOutput {
				err = tui.RunProgress("Starting sandbox...", start, nil)
			} else {
				err = start()
			}
			if err != nil {
				return err
			}
			if JSONOutput {
				printJSON(response)
				return nil
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Sandbox start accepted: %s\n", response.ID)
			return err
		},
	}
	create.Flags().StringVar(&request.ImageID, "image-id", "", "ID of a ready Thunder container image (required)")
	create.Flags().StringVar(&request.Spec.GPUType, "gpu", "", "GPU type, for example a6000 (required)")
	create.Flags().StringVar(&request.Name, "name", "", "Sandbox name for display in the Thunder console")
	create.Flags().StringVar(&request.WorkspaceID, "workspace-id", "", "Thunder workspace ID (defaults to main)")
	create.Flags().BoolVar(&request.TimingSensitive, "timing-sensitive", false, "Pass the node's physical GPU through for precise GPU timing")
	create.Flags().IntVar(&request.Lifetime.MaxTTLSeconds, "timeout", 900, "Seconds before the ready sandbox expires")
	create.Flags().IntVar(&request.Spec.CPUCount, "vcpus", 4, "Number of vCPUs")
	create.Flags().IntVar(&request.Spec.MemoryGiB, "memory", 32, "Memory in GiB")
	create.Flags().IntVar(&request.Spec.StorageGiB, "storage", 50, "Storage in GiB")
	request.Spec.GPUCount = 1

	sandbox := &cobra.Command{Use: "sandbox", Short: "Start short-lived Thunder GPU sandboxes"}
	sandbox.SetHelpFunc(wrapHelp(printDefaultHelp))
	sandbox.AddCommand(create)
	rootCmd.AddCommand(sandbox)
}
