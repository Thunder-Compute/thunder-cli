package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
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

	modifyCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderModifyHelp(cmd)
	})

	rootCmd.AddCommand(modifyCmd)
}

func runModify(cmd *cobra.Command, args []string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)

	// Fetch GPU specs from API
	specsMap, specsErr := client.GetSpecs()
	if specsErr != nil {
		return fmt.Errorf("failed to fetch GPU specs: %w", specsErr)
	}
	specs := utils.NewSpecStore(specsMap)

	// Fetch instances
	busy := tui.NewBusyModel("Fetching instances...")
	bp := tea.NewProgram(busy, tea.WithOutput(os.Stdout))
	busyDone := make(chan struct{})
	go func() {
		_, _ = bp.Run()
		close(busyDone)
	}()

	instances, err := client.ListInstances()
	bp.Send(tui.BusyDoneMsg{})
	<-busyDone

	if err != nil {
		return fmt.Errorf("failed to fetch instances: %w", err)
	}

	if len(instances) == 0 {
		PrintWarningSimple("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
		return nil
	}

	var selectedInstance *api.Instance

	// Determine which instance to modify
	if len(args) == 0 {
		// No argument - show interactive selector
		selectedInstance, err = tui.RunModifyInstanceSelector(client, instances)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled modification process")
				return nil
			}
			return err
		}
	} else {
		instanceIdentifier := args[0]

		// First try to find by ID or UUID
		for i := range instances {
			if instances[i].ID == instanceIdentifier || instances[i].UUID == instanceIdentifier {
				selectedInstance = &instances[i]
				break
			}
		}

		// If not found and it's a number, try as array index (for backwards compatibility)
		if selectedInstance == nil {
			if index, err := strconv.Atoi(instanceIdentifier); err == nil {
				if index >= 0 && index < len(instances) {
					selectedInstance = &instances[index]
				}
			}
		}

		if selectedInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceIdentifier)
		}
	}

	// Validate instance is RUNNING
	if selectedInstance.Status != "RUNNING" {
		return fmt.Errorf("instance must be in RUNNING state to modify (current state: %s)", selectedInstance.Status)
	}

	// Check if interactive mode (no flags set) or flag mode
	isInteractive := !cmd.Flags().Changed("mode") &&
		!cmd.Flags().Changed("gpu") &&
		!cmd.Flags().Changed("num-gpus") &&
		!cmd.Flags().Changed("vcpus") &&
		!cmd.Flags().Changed("disk-size-gb")

	var modifyConfig *tui.ModifyConfig
	var modifyReq api.InstanceModifyRequest

	if isInteractive {
		// Run interactive mode
		modifyConfig, err = tui.RunModifyInteractive(client, selectedInstance, specs)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled modification process")
				return nil
			}
			if err.Error() == "no changes" {
				PrintWarningSimple("No changes were requested. Instance configuration unchanged.")
				return nil
			}
			return err
		}

		// Build request from interactive config
		modifyReq, err = buildModifyRequestFromConfig(modifyConfig, selectedInstance)
		if err != nil {
			return err
		}
	} else {
		// Flag mode - validate flags and build request
		modifyReq, err = buildModifyRequestFromFlags(cmd, selectedInstance, specs)
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

	// Make API call with progress spinner
	p := tea.NewProgram(newModifyProgressModel(client, selectedInstance.ID, modifyReq))
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error during modification: %w", err)
	}

	progressModel := finalModel.(modifyProgressModel)

	if progressModel.cancelled {
		PrintWarningSimple("User cancelled modification")
		return nil
	}

	if progressModel.err != nil {
		return fmt.Errorf("failed to modify instance: %w", progressModel.err)
	}

	// Success output is rendered in the View() method
	return nil
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
	req := api.InstanceModifyRequest{}
	hasChanges := false

	// Helper function to check if value is in slice
	contains := func(slice []int, val int) bool {
		for _, item := range slice {
			if item == val {
				return true
			}
		}
		return false
	}

	// Mode validation
	if cmd.Flags().Changed("mode") {
		mode, _ := cmd.Flags().GetString("mode")
		mode = strings.ToLower(mode)
		if mode != "prototyping" && mode != "production" {
			return req, fmt.Errorf("mode must be 'prototyping' or 'production'")
		}

		// If switching modes, validate dependent fields
		if mode != currentInstance.Mode {
			if mode == "production" && !cmd.Flags().Changed("num-gpus") {
				return req, fmt.Errorf("switching to production requires --num-gpus flag (1, 2, or 4)")
			}
			if mode == "prototyping" && !cmd.Flags().Changed("vcpus") {
				return req, fmt.Errorf("switching to prototyping requires --vcpus flag (options vary by GPU type)")
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
	if cmd.Flags().Changed("gpu") {
		gpuType, _ := cmd.Flags().GetString("gpu")
		gpuType = strings.ToLower(gpuType)

		normalizedGPU, ok := specs.NormalizeGPUType(gpuType, effectiveMode)
		if !ok {
			availableGPUs := specs.GPUOptionsForMode(effectiveMode)
			return req, fmt.Errorf("invalid GPU type '%s' for %s mode. Valid options: %s", gpuType, effectiveMode, strings.Join(availableGPUs, ", "))
		}

		req.GPUType = &normalizedGPU
		hasChanges = true
	}

	// VCPUs validation (prototyping only)
	if cmd.Flags().Changed("vcpus") {
		vcpus, _ := cmd.Flags().GetInt("vcpus")

		// Check mode compatibility
		if effectiveMode == "production" {
			return req, fmt.Errorf("production mode does not use --vcpus flag. Use --num-gpus instead (vCPUs auto-calculated)")
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
		if allowedVCPUs != nil && !contains(allowedVCPUs, vcpus) {
			return req, fmt.Errorf("vcpus must be one of %v for %s with %d GPU(s)", allowedVCPUs, effectiveGPU, effectiveNumGPUs)
		}

		req.CPUCores = &vcpus
		hasChanges = true
	}

	// NumGPUs validation
	if cmd.Flags().Changed("num-gpus") {
		numGPUs, _ := cmd.Flags().GetInt("num-gpus")

		effectiveGPU := currentInstance.GPUType
		if req.GPUType != nil {
			effectiveGPU = *req.GPUType
		}
		allowedGPUCounts := specs.GPUCountsForMode(effectiveGPU, effectiveMode)
		if !contains(allowedGPUCounts, numGPUs) {
			return req, fmt.Errorf("num-gpus %d is not valid for %s %s. Allowed: %v", numGPUs, effectiveGPU, effectiveMode, allowedGPUCounts)
		}

		req.NumGPUs = &numGPUs
		hasChanges = true
	}

	// Disk size validation
	if cmd.Flags().Changed("disk-size-gb") {
		diskSize, _ := cmd.Flags().GetInt("disk-size-gb")
		if diskSize < currentInstance.Storage {
			return req, fmt.Errorf("disk size cannot be smaller than current size (%d GB)", currentInstance.Storage)
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
			return req, fmt.Errorf("disk size must be between %d and %d GB", currentInstance.Storage, maxDisk)
		}
		req.DiskSizeGB = &diskSize
		hasChanges = true
	}

	if !hasChanges {
		return req, fmt.Errorf("no changes specified. Use flags to modify instance configuration")
	}

	return req, nil
}

// Progress model for modify operation
type modifyProgressModel struct {
	client     *api.Client
	instanceID string
	req        api.InstanceModifyRequest
	spinner    spinner.Model
	message    string
	done       bool
	err        error
	resp       *api.InstanceModifyResponse
	cancelled  bool
}

func newModifyProgressModel(client *api.Client, instanceID string, req api.InstanceModifyRequest) modifyProgressModel {
	theme.Init(os.Stdout)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Primary()

	return modifyProgressModel{
		client:     client,
		instanceID: instanceID,
		req:        req,
		spinner:    s,
		message:    "Modifying instance...",
	}
}

type modifyInstanceResultMsg struct {
	resp *api.InstanceModifyResponse
	err  error
}

func (m modifyProgressModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		modifyInstanceCmd(m.client, m.instanceID, m.req),
	)
}

func modifyInstanceCmd(client *api.Client, instanceID string, req api.InstanceModifyRequest) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.ModifyInstance(instanceID, req)
		return modifyInstanceResultMsg{
			resp: resp,
			err:  err,
		}
	}
}

func (m modifyProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancelled = true
			return m, tea.Quit
		}

	case modifyInstanceResultMsg:
		m.done = true
		m.err = msg.err
		m.resp = msg.resp
		return m, tea.Quit

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m modifyProgressModel) View() string {
	if m.done {
		if m.cancelled {
			return ""
		}

		if m.err != nil {
			return ""
		}

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
		lines = append(lines, labelStyle.Render("Instance ID:")+"   "+valueStyle.Render(m.resp.Identifier))
		lines = append(lines, labelStyle.Render("Instance Name:")+" "+valueStyle.Render(m.resp.InstanceName))

		if m.resp.Mode != nil {
			lines = append(lines, labelStyle.Render("New Mode:")+"      "+valueStyle.Render(*m.resp.Mode))
		}
		if m.resp.GPUType != nil {
			lines = append(lines, labelStyle.Render("New GPU:")+"       "+valueStyle.Render(*m.resp.GPUType))
		}
		if m.resp.NumGPUs != nil {
			lines = append(lines, labelStyle.Render("New GPUs:")+"      "+valueStyle.Render(fmt.Sprintf("%d", *m.resp.NumGPUs)))
		}

		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Next steps:"))
		lines = append(lines, cmdStyle.Render("  • Instance is restarting with new configuration"))
		lines = append(lines, cmdStyle.Render("  • Run 'tnr status' to monitor progress"))
		lines = append(lines, cmdStyle.Render(fmt.Sprintf("  • Run 'tnr connect %s' once RUNNING", m.instanceID)))

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return "\n" + boxStyle.Render(content) + "\n\n"
	}

	return fmt.Sprintf("\n   %s %s\n\n", m.spinner.View(), m.message)
}
