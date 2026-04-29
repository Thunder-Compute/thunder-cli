package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

var (
	mode            string
	gpuType         string
	numGPUs         int
	vcpus           int
	template        string
	snapshotAlias   string
	diskSizeGB      int
	ephemeralDiskGB int
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Thunder Compute GPU instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreate(cmd)
	},
}

func init() {
	createCmd.SetHelpFunc(wrapHelp(helpmenus.RenderCreateHelp))

	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&mode, "mode", "", "Instance mode: prototyping or production")
	createCmd.Flags().StringVar(&gpuType, "gpu", "", "GPU type (prototyping: a6000, a100, or h100; production: a100 or h100)")
	createCmd.Flags().IntVar(&numGPUs, "num-gpus", 0, "Number of GPUs: 1-8 (production), 1-2 for A100/H100 (prototyping)")
	createCmd.Flags().IntVar(&vcpus, "vcpus", 0, "CPU cores (prototyping only): options vary by GPU type and count")
	createCmd.Flags().StringVar(&template, "template", "", "OS template key or name (accepts snapshot names too; --snapshot is an alias)")
	createCmd.Flags().StringVar(&snapshotAlias, "snapshot", "", "Alias for --template; accepts a snapshot name or template key")
	createCmd.Flags().IntVar(&diskSizeGB, "primary-disk", 100, "Primary disk storage in GB (range depends on GPU config)")
	createCmd.Flags().IntVar(&diskSizeGB, "disk-size-gb", 100, "Disk storage in GB (range depends on GPU config)")
	_ = createCmd.Flags().MarkHidden("disk-size-gb")
	createCmd.Flags().IntVar(&ephemeralDiskGB, "ephemeral-disk", 0, "Ephemeral storage in GB, mounted at /ephemeral (default: 0)")
}

func createInstanceCmd(client *api.Client, req api.CreateInstanceRequest, resp **api.CreateInstanceResponse) tea.Cmd {
	return func() tea.Msg {
		r, err := client.CreateInstance(req)
		if err == nil {
			*resp = r
		}
		return tui.ProgressResultMsg{Err: err}
	}
}

func renderCreateSuccess(resp **api.CreateInstanceResponse) func() string {
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
		lines = append(lines, successTitleStyle.Render("✓ Instance created successfully!"))
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render("Instance ID:")+" "+valueStyle.Render(fmt.Sprintf("%d", (*resp).Identifier)))
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Next steps:"))
		lines = append(lines, cmdStyle.Render("  • Run 'tnr status' to monitor provisioning progress"))
		lines = append(lines, cmdStyle.Render(fmt.Sprintf("  • Run 'tnr connect %d' once the instance is RUNNING", (*resp).Identifier)))

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return "\n" + boxStyle.Render(content) + "\n\n"
	}
}

func buildCreatePresets(cmd *cobra.Command) *tui.CreatePresets {
	p := &tui.CreatePresets{}
	if cmd.Flags().Changed("mode") {
		p.Mode = &mode
	}
	if cmd.Flags().Changed("gpu") {
		p.GPUType = &gpuType
	}
	if cmd.Flags().Changed("num-gpus") {
		p.NumGPUs = &numGPUs
	}
	if cmd.Flags().Changed("vcpus") {
		p.VCPUs = &vcpus
	}
	if templateFlagChanged(cmd) {
		p.Template = &template
	}
	if cmd.Flags().Changed("primary-disk") || cmd.Flags().Changed("disk-size-gb") {
		p.DiskSizeGB = &diskSizeGB
	}
	if cmd.Flags().Changed("ephemeral-disk") {
		p.EphemeralDiskGB = &ephemeralDiskGB
	}
	return p
}

func templateFlagChanged(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("template") || cmd.Flags().Changed("snapshot")
}

// resolveTemplateAlias reconciles --template and --snapshot. They're aliases,
// so using both with different values is an error; using only --snapshot copies
// its value into the shared template var.
func resolveTemplateAlias(cmd *cobra.Command) error {
	templateSet := cmd.Flags().Changed("template")
	snapshotSet := cmd.Flags().Changed("snapshot")
	if templateSet && snapshotSet && template != snapshotAlias {
		return usageErr("--template and --snapshot are aliases; use only one")
	}
	if snapshotSet && !templateSet {
		template = snapshotAlias
	}
	return nil
}

func hasAllCreateFlags(cmd *cobra.Command) bool {
	if !cmd.Flags().Changed("mode") || !cmd.Flags().Changed("gpu") ||
		!templateFlagChanged(cmd) || !(cmd.Flags().Changed("primary-disk") || cmd.Flags().Changed("disk-size-gb")) {
		return false
	}
	m, _ := cmd.Flags().GetString("mode")
	if strings.ToLower(m) == "prototyping" {
		return cmd.Flags().Changed("vcpus")
	}
	return cmd.Flags().Changed("num-gpus")
}

func runCreate(cmd *cobra.Command) error {
	if err := resolveTemplateAlias(cmd); err != nil {
		return err
	}

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

	presets := buildCreatePresets(cmd)

	var createConfig *tui.CreateConfig

	interactive := tui.IsInteractive() && !JSONOutput

	if presets.IsEmpty() {
		if !interactive {
			return usageErr("all flags required in non-interactive mode (--mode, --gpu, --template/--snapshot, --primary-disk, and --num-gpus or --vcpus)")
		}
		// No flags set — full interactive TUI
		createConfig, err = tui.RunCreateInteractive(client, specs)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled creation process")
				return nil
			}
			return err
		}
	} else if hasAllCreateFlags(cmd) {
		// All flags explicitly provided → non-interactive (skip confirmation)
		var templates []api.TemplateEntry
		var snapshots []api.Snapshot
		if fetchErr := tui.RunWithBusySpinner("Fetching templates and snapshots...", os.Stdout, func() error {
			var e error
			templates, e = client.ListTemplates()
			if e != nil {
				return e
			}
			snapshots, _ = client.ListSnapshots()
			readySnapshots := make([]api.Snapshot, 0)
			for _, s := range snapshots {
				if s.Status == "READY" {
					readySnapshots = append(readySnapshots, s)
				}
			}
			snapshots = readySnapshots
			return nil
		}); fetchErr != nil {
			return fmt.Errorf("failed to fetch templates: %w", fetchErr)
		}

		if len(templates) == 0 {
			return usageErr("no templates available")
		}

		diskSizeWasSet := cmd.Flags().Changed("primary-disk") || cmd.Flags().Changed("disk-size-gb")
		createConfig = &tui.CreateConfig{
			Mode:            mode,
			GPUType:         gpuType,
			NumGPUs:         numGPUs,
			VCPUs:           vcpus,
			Template:        template,
			DiskSizeGB:      diskSizeGB,
			EphemeralDiskGB: ephemeralDiskGB,
		}

		if valErr := validateCreateConfig(createConfig, templates, snapshots, diskSizeWasSet, specs); valErr != nil {
			// Validation failed — fall through to hybrid mode
			createConfig, err = tui.RunCreateHybrid(client, specs, presets)
			if err != nil {
				if errors.Is(err, tui.ErrCancelled) {
					PrintWarningSimple("User cancelled creation process")
					return nil
				}
				return err
			}
		} else {
			// Fully non-interactive succeeded
			if pricing, pErr := client.FetchPricing(); pErr == nil {
				pd := &utils.PricingData{Rates: pricing}
				included := specs.IncludedVCPUs(createConfig.GPUType, createConfig.NumGPUs, createConfig.Mode)
				price := utils.CalculateHourlyPrice(pd, createConfig.Mode, createConfig.GPUType, createConfig.NumGPUs, createConfig.VCPUs, createConfig.DiskSizeGB, createConfig.EphemeralDiskGB, included)
				fmt.Printf("\nEstimated cost: %s\n", utils.FormatPrice(price))
			}

			if createConfig.Mode == "prototyping" {
				fmt.Println()
				PrintWarningSimple("PROTOTYPING MODE DISCLAIMER")
				fmt.Println("Prototyping mode is optimized for development. Long running GPU processes may be interrupted.")
				fmt.Println("For production inference or batch training, use production mode.")
			}
		}
	} else {
		if !interactive {
			return usageErr("all flags required in non-interactive mode (--mode, --gpu, --template/--snapshot, --primary-disk, and --num-gpus or --vcpus)")
		}
		// Partial flags — hybrid TUI
		createConfig, err = tui.RunCreateHybrid(client, specs, presets)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled creation process")
				return nil
			}
			return err
		}
	}

	req := api.CreateInstanceRequest{
		Mode:            api.InstanceMode(createConfig.Mode),
		GPUType:         createConfig.GPUType,
		NumGPUs:         createConfig.NumGPUs,
		CPUCores:        createConfig.VCPUs,
		Template:        createConfig.Template,
		DiskSizeGB:      createConfig.DiskSizeGB,
		EphemeralDiskGB: createConfig.EphemeralDiskGB,
	}

	var resp *api.CreateInstanceResponse

	if !interactive {
		// Non-interactive: direct API call without Bubble Tea
		fmt.Fprintln(os.Stderr, "Creating instance...")
		resp, err = client.CreateInstance(req)
		if err != nil {
			return fmt.Errorf("failed to create instance: %w", err)
		}
		if JSONOutput {
			printJSON(resp)
		} else {
			fmt.Printf("Instance created: ID=%d UUID=%s\n", resp.Identifier, resp.UUID)
		}
	} else {
		progressModel := tui.NewProgressModel("Creating instance...",
			createInstanceCmd(client, req, &resp),
			renderCreateSuccess(&resp),
		)
		program := tea.NewProgram(progressModel)
		finalModel, runErr := program.Run()
		if runErr != nil {
			return fmt.Errorf("failed to render progress: %w", runErr)
		}

		result := finalModel.(tui.ProgressModel)

		if result.Cancelled() {
			PrintWarningSimple("User cancelled creation process")
			return nil
		}

		if result.Err() != nil {
			return fmt.Errorf("failed to create instance: %w", result.Err())
		}
	}

	return nil
}

func validateCreateConfig(config *tui.CreateConfig, templates []api.TemplateEntry, snapshots []api.Snapshot, diskSizeWasSet bool, specs *utils.SpecStore) error {
	config.Mode = strings.ToLower(config.Mode)
	config.GPUType = strings.ToLower(config.GPUType)

	if config.Mode != "prototyping" && config.Mode != "production" {
		return usageErr("mode must be 'prototyping' or 'production'")
	}

	// Normalize GPU type
	canonical, ok := specs.NormalizeGPUType(config.GPUType, config.Mode)
	if !ok {
		availableGPUs := specs.GPUOptionsForMode(config.Mode)
		return usageErr("%s mode supports GPU types: %s", config.Mode, strings.Join(availableGPUs, ", "))
	}
	config.GPUType = canonical

	// Validate GPU count
	if config.NumGPUs == 0 {
		config.NumGPUs = 1
	}

	allowedVCPUs := specs.VCPUOptions(config.GPUType, config.NumGPUs, config.Mode)
	if allowedVCPUs == nil {
		allowedCounts := specs.GPUCountsForMode(config.GPUType, config.Mode)
		return usageErr("GPU count %d is not valid for %s %s. Allowed: %v", config.NumGPUs, config.GPUType, config.Mode, allowedCounts)
	}

	if config.Mode == "prototyping" {
		if config.VCPUs == 0 {
			return usageErr("prototyping mode requires --vcpus flag (options for %s with %d GPU(s): %v)", config.GPUType, config.NumGPUs, allowedVCPUs)
		}

		if !slices.Contains(allowedVCPUs, config.VCPUs) {
			return usageErr("vcpus must be one of %v for %s with %d GPU(s)", allowedVCPUs, config.GPUType, config.NumGPUs)
		}
	} else {
		// Production: vCPUs are auto-set from the spec (first/only option)
		config.VCPUs = allowedVCPUs[0]
	}

	if config.Template == "" {
		return usageErr("template is required (use --template or --snapshot flag)")
	}

	// Check if template is actually a snapshot
	var selectedSnapshot *api.Snapshot
	templateFound := false

	// First check templates
	for _, t := range templates {
		if t.Key == config.Template || strings.EqualFold(t.Template.DisplayName, config.Template) {
			config.Template = t.Key
			templateFound = true
			break
		}
	}

	// If not found in templates, check snapshots
	if !templateFound {
		for _, s := range snapshots {
			if s.Name == config.Template {
				selectedSnapshot = &s
				templateFound = true
				break
			}
		}
	}

	if !templateFound {
		return usageErr("template or snapshot '%s' not found. Run 'tnr templates' to list available templates and 'tnr snapshots' for snapshots", config.Template)
	}

	// Default disk size to the snapshot's size when unspecified.
	if selectedSnapshot != nil && !diskSizeWasSet {
		config.DiskSizeGB = selectedSnapshot.MinimumDiskSizeGB
	}

	// Validate disk size. With a snapshot the range becomes
	// [max(minSpec, snapshot), max(maxSpec, snapshot)]: disk must be at least the
	// snapshot's size, and snapshots larger than maxSpec restore as-is.
	minStorage, maxStorage := specs.StorageRange(config.GPUType, config.NumGPUs, config.Mode)
	if selectedSnapshot != nil {
		if selectedSnapshot.MinimumDiskSizeGB > minStorage {
			minStorage = selectedSnapshot.MinimumDiskSizeGB
		}
		if selectedSnapshot.MinimumDiskSizeGB > maxStorage {
			maxStorage = selectedSnapshot.MinimumDiskSizeGB
		}
	}
	if config.DiskSizeGB < minStorage || config.DiskSizeGB > maxStorage {
		return usageErr("disk size must be between %d and %d GB", minStorage, maxStorage)
	}

	return nil
}
