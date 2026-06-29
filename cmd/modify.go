package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

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
	Args:  wrapArgs(cobra.MaximumNArgs(1)),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runModify(cmd, args)
	},
}

func init() {
	modifyCmd.Flags().String("gpu", "", "GPU type (a6000, a100, h100)")
	modifyCmd.Flags().Int("num-gpus", 0, "Number of GPUs: 1, 2, 4, or 8")
	modifyCmd.Flags().Int("vcpus", 0, "CPU cores: options vary by GPU type and count")
	modifyCmd.Flags().Int("disk", 0, "Disk size in GB (cannot shrink, max depends on config)")
	modifyCmd.Flags().Int("disk-size-gb", 0, "Disk size in GB (cannot shrink, max depends on config)")
	_ = modifyCmd.Flags().MarkHidden("disk-size-gb")

	modifyCmd.SetHelpFunc(wrapHelp(helpmenus.RenderModifyHelp))

	rootCmd.AddCommand(modifyCmd)
}

func runModify(cmd *cobra.Command, args []string) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	var (
		specs     *utils.SpecStore
		specsErr  error
		instances []api.Instance
		instErr   error
	)
	if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			specs, specsErr = utils.FetchSpecStore(client)
		}()
		go func() {
			defer wg.Done()
			instances, instErr = client.ListInstances()
		}()
		wg.Wait()
		if specsErr != nil {
			return specsErr
		}
		if instErr != nil {
			return fmt.Errorf("failed to fetch instances: %w", instErr)
		}
		return nil
	}); err != nil {
		return err
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
			return usageErr("modification flags required in non-interactive mode (--gpu, --num-gpus, --vcpus, --disk)")
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
			if !interactive || YesFlag {
				return err
			}
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
		if vcpuOpts := specs.VCPUOptions(resultGPU, resultNumGPUs); len(vcpuOpts) > 0 && modifyReq.CPUCores == nil {
			resultVCPUs = vcpuOpts[0]
		}

		included := specs.IncludedVCPUs(resultGPU, resultNumGPUs)
		price := utils.CalculateHourlyPrice(pd, resultGPU, resultNumGPUs, resultVCPUs, resultDisk, included)
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
		if !isUserError(result.Err()) {
			sentry.WithScope(func(scope *sentry.Scope) {
				scope.SetTag("operation", "modify_instance")
				sentry.CaptureException(result.Err())
			})
		}
		return fmt.Errorf("failed to modify instance: %w", result.Err())
	}

	// Success output is rendered in the View() method
	return nil
}

func buildModifyPresets(cmd *cobra.Command) *tui.ModifyPresets {
	p := &tui.ModifyPresets{}
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
	if cmd.Flags().Changed("disk") {
		v, _ := cmd.Flags().GetInt("disk")
		p.DiskSizeGB = &v
	} else if cmd.Flags().Changed("disk-size-gb") {
		v, _ := cmd.Flags().GetInt("disk-size-gb")
		p.DiskSizeGB = &v
	}
	return p
}

func hasAllModifyFlags(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("gpu") || cmd.Flags().Changed("num-gpus") || cmd.Flags().Changed("vcpus") ||
		cmd.Flags().Changed("disk") || cmd.Flags().Changed("disk-size-gb")
}

func buildModifyRequestFromConfig(config *tui.ModifyConfig, currentInstance *api.Instance) (api.InstanceModifyRequest, error) {
	req := api.InstanceModifyRequest{}

	if config.GPUChanged {
		req.GPUType = &config.GPUType
	}

	if config.ComputeChanged {
		currentNumGPUs, _ := strconv.Atoi(currentInstance.NumGPUs)
		currentVCPUs, _ := strconv.Atoi(currentInstance.CPUCores)

		if config.NumGPUs > 0 && config.NumGPUs != currentNumGPUs {
			req.NumGPUs = &config.NumGPUs
		}
		if config.VCPUs > 0 && config.VCPUs != currentVCPUs {
			req.CPUCores = &config.VCPUs
		}
	}

	if config.DiskChanged {
		req.DiskSizeGB = &config.DiskSizeGB
	}

	// Check if any changes were made
	if req.GPUType == nil && req.CPUCores == nil && req.NumGPUs == nil && req.DiskSizeGB == nil {
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

	// GPU type validation
	if presets.GPUType != nil {
		gpuType := strings.ToLower(*presets.GPUType)

		normalizedGPU, ok := specs.NormalizeGPUType(gpuType)
		if !ok {
			availableGPUs := specs.GPUOptions()
			return req, usageErr("invalid GPU type '%s'. Valid options: %s", gpuType, strings.Join(availableGPUs, ", "))
		}

		req.GPUType = &normalizedGPU
		hasChanges = true
	}

	// VCPUs validation (development only)
	if presets.VCPUs != nil {
		vcpus := *presets.VCPUs

		// Determine effective GPU type for validation
		effectiveGPU := currentInstance.GPUType
		if req.GPUType != nil {
			effectiveGPU = *req.GPUType
		}
		effectiveNumGPUs := 1
		if presets.NumGPUs != nil {
			effectiveNumGPUs = *presets.NumGPUs
		} else if currentInstance.NumGPUs != "" {
			if n, err := fmt.Sscanf(currentInstance.NumGPUs, "%d", &effectiveNumGPUs); n != 1 || err != nil {
				effectiveNumGPUs = 1
			}
		}

		allowedVCPUs := specs.VCPUOptions(effectiveGPU, effectiveNumGPUs)
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
		allowedGPUCounts := specs.GPUCounts(effectiveGPU)
		if !slices.Contains(allowedGPUCounts, numGPUs) {
			return req, usageErr("num-gpus %d is not valid for %s. Allowed: %v", numGPUs, effectiveGPU, allowedGPUCounts)
		}

		req.NumGPUs = &numGPUs
		hasChanges = true
	}

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
	targetVCPUOptions := specs.VCPUOptions(effectiveGPU, effectiveNumGPUs)
	if len(targetVCPUOptions) == 0 {
		return req, usageErr("unsupported configuration: %s with %d GPU(s)", effectiveGPU, effectiveNumGPUs)
	}
	if !specs.IsSpecAvailable(effectiveGPU, effectiveNumGPUs) {
		return req, usageErr("GPU configuration %s x%d is currently unavailable", effectiveGPU, effectiveNumGPUs)
	}
	if len(targetVCPUOptions) == 1 {
		targetVCPUs := targetVCPUOptions[0]
		currentVCPUs, _ := strconv.Atoi(currentInstance.CPUCores)
		if targetVCPUs != currentVCPUs {
			req.CPUCores = &targetVCPUs
			hasChanges = true
		}
	} else {
		targetVCPUs := 0
		if req.CPUCores != nil {
			targetVCPUs = *req.CPUCores
		} else {
			targetVCPUs, _ = strconv.Atoi(currentInstance.CPUCores)
		}
		if !slices.Contains(targetVCPUOptions, targetVCPUs) {
			return req, usageErr("vcpus must be one of %v for %s with %d GPU(s)", targetVCPUOptions, effectiveGPU, effectiveNumGPUs)
		}
	}

	// Disk size validation
	if presets.DiskSizeGB != nil {
		diskSize := *presets.DiskSizeGB
		if diskSize < currentInstance.Storage {
			return req, usageErr("disk size cannot be smaller than current size (%d GB)", currentInstance.Storage)
		}

		minDisk, maxDisk := specs.StorageRange(effectiveGPU, effectiveNumGPUs)
		minAllowedDisk := max(currentInstance.Storage, minDisk)
		if diskSize < minAllowedDisk || diskSize > maxDisk {
			return req, usageErr("disk size must be between %d and %d GB", minAllowedDisk, maxDisk)
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
