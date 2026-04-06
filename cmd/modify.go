package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

// modifyCmd represents the modify command
var modifyCmd = &cobra.Command{
	Use:   "modify [instance_index_or_id]",
	Short: "Modify a Thunder Compute instance configuration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runModify(cmd, args)
	},
}

func init() {
	modifyCmd.Flags().String("mode", "", "Instance mode (prototyping or production)")
	modifyCmd.Flags().String("gpu", "", "GPU type (a6000, a100, h100)")
	modifyCmd.Flags().Int("num-gpus", 0, "Number of GPUs (production mode: 1, 2, or 4)")
	modifyCmd.Flags().Int("vcpus", 0, "CPU cores (prototyping only): options vary by GPU type and count")
	modifyCmd.Flags().Int("disk-size-gb", 0, "Disk size in GB (cannot shrink, max depends on config)")

	modifyCmd.SetHelpFunc(wrapHelp(helpmenus.RenderModifyHelp))

	rootCmd.AddCommand(modifyCmd)
}

func runModify(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	// Fetch GPU specs from API
	specsMap, specsErr := client.GetSpecs()
	if specsErr != nil {
		return fmt.Errorf("failed to fetch GPU specs: %w", specsErr)
	}
	specs := utils.NewSpecStore(specsMap)

	// Fetch instances
	var instances []api.Instance
	if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
		var e error
		instances, e = client.ListInstances()
		return e
	}); err != nil {
		return fmt.Errorf("failed to fetch instances: %w", err)
	}

	if len(instances) == 0 {
		PrintWarningSimple("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
		return nil
	}

	interactive := tui.IsInteractive() && !JSONOutput

	var selectedInstance *api.Instance

	// Determine which instance to modify
	if len(args) == 0 {
		if !interactive {
			return usageErr("instance ID required in non-interactive mode")
		}
		// No argument - show interactive selector
		selectedInstance, err = tui.RunModifyInstanceSelector(client, instances)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled modification process")
				return nil
			}
			return err
		}
	} else {
		instanceIdentifier := args[0]

		// First try to find by ID, UUID, or Name
		selectedInstance = findInstance(instances, instanceIdentifier)

		// If not found and it's a number, try as array index (for backwards compatibility)
		if selectedInstance == nil {
			if index, err := strconv.Atoi(instanceIdentifier); err == nil {
				if index >= 0 && index < len(instances) {
					selectedInstance = &instances[index]
				}
			}
		}

		if selectedInstance == nil {
			return usageErr("instance '%s' not found", instanceIdentifier)
		}
	}

	// Validate instance is RUNNING
	if selectedInstance.Status != "RUNNING" {
		return usageErr("instance must be in RUNNING state to modify (current state: %s)", selectedInstance.Status)
	}

	// Build presets from flags
	modifyPresets := buildModifyPresets(cmd)

	var modifyConfig *tui.ModifyConfig
	var modifyReq api.InstanceModifyRequest

	if modifyPresets.IsEmpty() {
		if !interactive {
			return usageErr("modification flags required in non-interactive mode (--mode, --gpu, --num-gpus, --vcpus, --disk-size-gb)")
		}
		// No flags set — full interactive mode
		modifyConfig, err = tui.RunModifyInteractive(client, selectedInstance, specs)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled modification process")
				return nil
			}
			if errors.Is(err, tui.ErrNoChanges) {
				PrintWarningSimple("No changes were requested. Instance configuration unchanged.")
				return nil
			}
			return err
		}

		modifyReq, err = buildModifyRequestFromConfig(modifyConfig, selectedInstance)
		if err != nil {
			return err
		}
	} else if hasAllModifyFlags(cmd) {
		// All flags provided -> try fully non-interactive (skip confirmation)
		modifyReq, err = buildModifyRequestFromFlags(cmd, selectedInstance, specs)
		if err != nil {
			// Fall through to hybrid
			modifyConfig, err = tui.RunModifyHybrid(client, selectedInstance, specs, modifyPresets)
			if err != nil {
				if errors.Is(err, tui.ErrCancelled) {
					PrintWarningSimple("User cancelled modification process")
					return nil
				}
				if errors.Is(err, tui.ErrNoChanges) {
					PrintWarningSimple("No changes were requested. Instance configuration unchanged.")
					return nil
				}
				return err
			}
			modifyReq, err = buildModifyRequestFromConfig(modifyConfig, selectedInstance)
			if err != nil {
				return err
			}
		}
	} else {
		// Partial flags — hybrid TUI (confirmation always shown)
		modifyConfig, err = tui.RunModifyHybrid(client, selectedInstance, specs, modifyPresets)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled modification process")
				return nil
			}
			if errors.Is(err, tui.ErrNoChanges) {
				PrintWarningSimple("No changes were requested. Instance configuration unchanged.")
				return nil
			}
			return err
		}

		modifyReq, err = buildModifyRequestFromConfig(modifyConfig, selectedInstance)
		if err != nil {
			return err
		}
	}

	// Display estimated pricing for the resulting configuration
	if pricing, pricingErr := client.FetchPricing(); pricingErr == nil {
		pd := &utils.PricingData{Rates: pricing}
		// Compute resulting config: start with current values, override with modifications
		resultMode := strings.ToLower(selectedInstance.Mode)
		resultGPU := strings.ToLower(selectedInstance.GPUType)
		resultNumGPUs := 1
		if n, parseErr := strconv.Atoi(selectedInstance.NumGPUs); parseErr == nil {
			resultNumGPUs = n
		}
		resultVCPUs := 4
		if n, parseErr := strconv.Atoi(selectedInstance.CPUCores); parseErr == nil {
			resultVCPUs = n
		}
		resultDisk := selectedInstance.Storage

		if modifyReq.Mode != nil {
			resultMode = string(*modifyReq.Mode)
		}
		if modifyReq.GPUType != nil {
			resultGPU = *modifyReq.GPUType
		}
		if modifyReq.NumGPUs != nil {
			resultNumGPUs = *modifyReq.NumGPUs
		}
		if modifyReq.CPUCores != nil {
			resultVCPUs = *modifyReq.CPUCores
		}
		if modifyReq.DiskSizeGB != nil {
			resultDisk = *modifyReq.DiskSizeGB
		}
		// Get vCPUs from specs for the resulting config
		if vcpuOpts := specs.VCPUOptions(resultGPU, resultNumGPUs, resultMode); len(vcpuOpts) > 0 && modifyReq.CPUCores == nil {
			resultVCPUs = vcpuOpts[0]
		}

		included := specs.IncludedVCPUs(resultGPU, resultNumGPUs, resultMode)
		price := utils.CalculateHourlyPrice(pd, resultMode, resultGPU, resultNumGPUs, resultVCPUs, resultDisk, included)
		fmt.Printf("\nEstimated cost: %s\n", utils.FormatPrice(price))
	}

	// Make API call
	var modifyResp *api.InstanceModifyResponse

	if !interactive {
		fmt.Fprintln(os.Stderr, "Modifying instance...")
		modifyResp, err = client.ModifyInstance(selectedInstance.ID, modifyReq)
		if err != nil {
			return fmt.Errorf("failed to modify instance: %w", err)
		}
		if JSONOutput {
			printJSON(modifyResp)
		} else {
			fmt.Printf("Modified instance %s\n", selectedInstance.ID)
		}
		return nil
	}

	p := tea.NewProgram(tui.NewProgressModel("Modifying instance...",
		modifyInstanceCmd(client, selectedInstance.ID, modifyReq, &modifyResp),
		renderModifySuccess(selectedInstance.ID, &modifyResp),
	))
	finalModel, err := p.Run()
	if err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("operation", "modify_tui")
			sentry.CaptureException(err)
		})
		return fmt.Errorf("error during modification: %w", err)
	}

	result := finalModel.(tui.ProgressModel)

	if result.Cancelled() {
		PrintWarningSimple("User cancelled modification")
		return nil
	}

	if result.Err() != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("operation", "modify_instance")
			sentry.CaptureException(result.Err())
		})
		return fmt.Errorf("failed to modify instance: %w", result.Err())
	}

	// Success output is rendered in the View() method
	return nil
}

func buildModifyPresets(cmd *cobra.Command) *tui.ModifyPresets {
	p := &tui.ModifyPresets{}
	if cmd.Flags().Changed("mode") {
		v, _ := cmd.Flags().GetString("mode")
		p.Mode = &v
	}
	if cmd.Flags().Changed("gpu") {
		v, _ := cmd.Flags().GetString("gpu")
		p.GPUType = &v
	}
	if cmd.Flags().Changed("num-gpus") {
		v, _ := cmd.Flags().GetInt("num-gpus")
		p.NumGPUs = &v
	}
	if cmd.Flags().Changed("vcpus") {
		v, _ := cmd.Flags().GetInt("vcpus")
		p.VCPUs = &v
	}
	if cmd.Flags().Changed("disk-size-gb") {
		v, _ := cmd.Flags().GetInt("disk-size-gb")
		p.DiskSizeGB = &v
	}
	return p
}

func hasAllModifyFlags(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("mode") || cmd.Flags().Changed("gpu") ||
		cmd.Flags().Changed("num-gpus") || cmd.Flags().Changed("vcpus") ||
		cmd.Flags().Changed("disk-size-gb")
}

func buildModifyRequestFromConfig(config *tui.ModifyConfig, currentInstance *api.Instance) (api.InstanceModifyRequest, error) {
	req := api.InstanceModifyRequest{}

	if config.ModeChanged {
		mode := api.InstanceMode(config.Mode)
		req.Mode = &mode
	}

	if config.GPUChanged {
		req.GPUType = &config.GPUType
	}

	if config.ComputeChanged {
		effectiveMode := currentInstance.Mode
		if config.ModeChanged {
			effectiveMode = config.Mode
		}

		if effectiveMode == "prototyping" {
			req.CPUCores = &config.VCPUs
		} else {
			req.NumGPUs = &config.NumGPUs
		}
	}

	if config.DiskChanged {
		req.DiskSizeGB = &config.DiskSizeGB
	}

	// Check if any changes were made
	if !config.ModeChanged && !config.GPUChanged && !config.ComputeChanged && !config.DiskChanged {
		return req, fmt.Errorf("no changes specified")
	}

	return req, nil
}

func buildModifyRequestFromFlags(cmd *cobra.Command, currentInstance *api.Instance, specs *utils.SpecStore) (api.InstanceModifyRequest, error) {
	presets := buildModifyPresets(cmd)
	return validateAndBuildModifyRequest(presets, currentInstance, specs)
}

// validateAndBuildModifyRequest validates modify presets against the current instance
// and spec store, returning a ready-to-send API request or an error.
func validateAndBuildModifyRequest(presets *tui.ModifyPresets, currentInstance *api.Instance, specs *utils.SpecStore) (api.InstanceModifyRequest, error) {
	req := api.InstanceModifyRequest{}
	hasChanges := false

	// Mode validation
	if presets.Mode != nil {
		mode := strings.ToLower(*presets.Mode)
		if mode != "prototyping" && mode != "production" {
			return req, usageErr("mode must be 'prototyping' or 'production'")
		}

		// If switching modes, validate dependent fields
		if mode != currentInstance.Mode {
			if mode == "production" && presets.NumGPUs == nil {
				return req, usageErr("switching to production requires --num-gpus flag (1, 2, or 4)")
			}
			if mode == "prototyping" && presets.VCPUs == nil {
				return req, usageErr("switching to prototyping requires --vcpus flag (options vary by GPU type)")
			}
		}
		instanceMode := api.InstanceMode(mode)
		req.Mode = &instanceMode
		hasChanges = true
	}

	// Determine effective mode for GPU and compute validation
	effectiveMode := currentInstance.Mode
	if req.Mode != nil {
		effectiveMode = string(*req.Mode)
	}

	// GPU type validation
	if presets.GPUType != nil {
		gpuType := strings.ToLower(*presets.GPUType)

		normalizedGPU, ok := specs.NormalizeGPUType(gpuType, effectiveMode)
		if !ok {
			availableGPUs := specs.GPUOptionsForMode(effectiveMode)
			return req, usageErr("invalid GPU type '%s' for %s mode. Valid options: %s", gpuType, effectiveMode, strings.Join(availableGPUs, ", "))
		}

		req.GPUType = &normalizedGPU
		hasChanges = true
	}

	// VCPUs validation (prototyping only)
	if presets.VCPUs != nil {
		vcpus := *presets.VCPUs

		// Check mode compatibility
		if effectiveMode == "production" {
			return req, usageErr("production mode does not use --vcpus flag. Use --num-gpus instead (vCPUs auto-calculated)")
		}

		// Determine effective GPU type for validation
		effectiveGPU := currentInstance.GPUType
		if req.GPUType != nil {
			effectiveGPU = *req.GPUType
		}
		effectiveNumGPUs := 1
		if req.NumGPUs != nil {
			effectiveNumGPUs = *req.NumGPUs
		} else if currentInstance.NumGPUs != "" {
			if n, err := fmt.Sscanf(currentInstance.NumGPUs, "%d", &effectiveNumGPUs); n != 1 || err != nil {
				effectiveNumGPUs = 1
			}
		}

		allowedVCPUs := specs.VCPUOptions(effectiveGPU, effectiveNumGPUs, effectiveMode)
		if allowedVCPUs != nil && !slices.Contains(allowedVCPUs, vcpus) {
			return req, usageErr("vcpus must be one of %v for %s with %d GPU(s)", allowedVCPUs, effectiveGPU, effectiveNumGPUs)
		}

		req.CPUCores = &vcpus
		hasChanges = true
	}

	// NumGPUs validation
	if presets.NumGPUs != nil {
		numGPUs := *presets.NumGPUs

		effectiveGPU := currentInstance.GPUType
		if req.GPUType != nil {
			effectiveGPU = *req.GPUType
		}
		allowedGPUCounts := specs.GPUCountsForMode(effectiveGPU, effectiveMode)
		if !slices.Contains(allowedGPUCounts, numGPUs) {
			return req, usageErr("num-gpus %d is not valid for %s %s. Allowed: %v", numGPUs, effectiveGPU, effectiveMode, allowedGPUCounts)
		}

		req.NumGPUs = &numGPUs
		hasChanges = true
	}

	// Disk size validation
	if presets.DiskSizeGB != nil {
		diskSize := *presets.DiskSizeGB
		if diskSize < currentInstance.Storage {
			return req, usageErr("disk size cannot be smaller than current size (%d GB)", currentInstance.Storage)
		}

		// Determine effective GPU type and count for storage range lookup
		effectiveGPU := currentInstance.GPUType
		if req.GPUType != nil {
			effectiveGPU = *req.GPUType
		}
		effectiveNumGPUs := 1
		if req.NumGPUs != nil {
			effectiveNumGPUs = *req.NumGPUs
		} else if currentInstance.NumGPUs != "" {
			if n, parseErr := fmt.Sscanf(currentInstance.NumGPUs, "%d", &effectiveNumGPUs); n != 1 || parseErr != nil {
				effectiveNumGPUs = 1
			}
		}
		_, maxDisk := specs.StorageRange(effectiveGPU, effectiveNumGPUs, effectiveMode)
		if diskSize > maxDisk {
			return req, usageErr("disk size must be between %d and %d GB", currentInstance.Storage, maxDisk)
		}
		req.DiskSizeGB = &diskSize
		hasChanges = true
	}

	if !hasChanges {
		return req, usageErr("no changes specified. Use flags to modify instance configuration")
	}

	return req, nil
}

func modifyInstanceCmd(client *api.Client, instanceID string, req api.InstanceModifyRequest, resp **api.InstanceModifyResponse) tea.Cmd {
	return func() tea.Msg {
		r, err := client.ModifyInstance(instanceID, req)
		if err == nil {
			*resp = r
		}
		return tui.ProgressResultMsg{Err: err}
	}
}

func renderModifySuccess(instanceID string, resp **api.InstanceModifyResponse) func() string {
	return func() string {
		headerStyle := theme.Primary().Bold(true)
		labelStyle := theme.Neutral()
		valueStyle := lipgloss.NewStyle().Bold(true)
		cmdStyle := theme.Neutral()
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(1, 2)

		var lines []string
		successTitleStyle := theme.Success()
		lines = append(lines, successTitleStyle.Render("✓ Instance modified successfully!"))
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render("Instance ID:")+"   "+valueStyle.Render((*resp).Identifier))
		lines = append(lines, labelStyle.Render("Instance Name:")+" "+valueStyle.Render((*resp).InstanceName))

		if (*resp).Mode != nil {
			lines = append(lines, labelStyle.Render("New Mode:")+"      "+valueStyle.Render(*(*resp).Mode))
		}
		if (*resp).GPUType != nil {
			lines = append(lines, labelStyle.Render("New GPU:")+"       "+valueStyle.Render(*(*resp).GPUType))
		}
		if (*resp).NumGPUs != nil {
			lines = append(lines, labelStyle.Render("New GPUs:")+"      "+valueStyle.Render(fmt.Sprintf("%d", *(*resp).NumGPUs)))
		}

		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Next steps:"))
		lines = append(lines, cmdStyle.Render("  • Instance is restarting with new configuration"))
		lines = append(lines, cmdStyle.Render("  • Run 'tnr status' to monitor progress"))
		lines = append(lines, cmdStyle.Render(fmt.Sprintf("  • Run 'tnr connect %s' once RUNNING", instanceID)))

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return "\n" + boxStyle.Render(content) + "\n\n"
	}
}
