package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/internal/sshkeys"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

var (
	mode             string
	gpuType          string
	numGPUs          int
	vcpus            int
	template         string
	diskSizeGB       int
	createSSHKeyName string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Thunder Compute GPU instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreate(cmd)
	},
}

func init() {
	createCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderCreateHelp(cmd)
	})

	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&mode, "mode", "", "Instance mode: prototyping or production")
	createCmd.Flags().StringVar(&gpuType, "gpu", "", "GPU type (prototyping: a6000, a100, or h100; production: a100 or h100)")
	createCmd.Flags().IntVar(&numGPUs, "num-gpus", 0, "Number of GPUs: 1-8 (production), 1-2 for A100/H100 (prototyping)")
	createCmd.Flags().IntVar(&vcpus, "vcpus", 0, "CPU cores (prototyping only): options vary by GPU type and count")
	createCmd.Flags().StringVar(&template, "template", "", "OS template key or name")
	createCmd.Flags().IntVar(&diskSizeGB, "disk-size-gb", 100, "Disk storage in GB (range depends on GPU config)")
	createCmd.Flags().StringVar(&createSSHKeyName, "ssh-key", "", "[Optional] Name of an external SSH key to attach (see 'tnr ssh-keys --help')")
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

func runCreate(cmd *cobra.Command) error {
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

	isInteractive := !cmd.Flags().Changed("mode")

	var createConfig *tui.CreateConfig

	if isInteractive {
		createConfig, err = tui.RunCreateInteractive(client, specs)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled creation process")
				return nil
			}
			return err
		}
	} else {
		var templates []api.TemplateEntry
		var snapshots []api.Snapshot
		if err := tui.RunWithBusySpinner("Fetching templates and snapshots...", os.Stdout, func() error {
			var e error
			templates, e = client.ListTemplates()
			if e != nil {
				return e
			}
			snapshots, _ = client.ListSnapshots()
			// Filter for READY snapshots only
			readySnapshots := make([]api.Snapshot, 0)
			for _, s := range snapshots {
				if s.Status == "READY" {
					readySnapshots = append(readySnapshots, s)
				}
			}
			snapshots = readySnapshots
			return nil
		}); err != nil {
			return fmt.Errorf("failed to fetch templates: %w", err)
		}

		if len(templates) == 0 {
			return fmt.Errorf("no templates available")
		}

		// Check if disk size was explicitly set by the user
		diskSizeWasSet := cmd.Flags().Changed("disk-size-gb")

		createConfig = &tui.CreateConfig{
			Mode:       mode,
			GPUType:    gpuType,
			NumGPUs:    numGPUs,
			VCPUs:      vcpus,
			Template:   template,
			DiskSizeGB: diskSizeGB,
		}

		if err := validateCreateConfig(createConfig, templates, snapshots, diskSizeWasSet, specs); err != nil {
			return err
		}

		// Display estimated pricing
		if pricing, err := client.FetchPricing(); err == nil {
			pd := &utils.PricingData{Rates: pricing}
			included := specs.IncludedVCPUs(createConfig.GPUType, createConfig.NumGPUs, createConfig.Mode)
			price := utils.CalculateHourlyPrice(pd, createConfig.Mode, createConfig.GPUType, createConfig.NumGPUs, createConfig.VCPUs, createConfig.DiskSizeGB, included)
			fmt.Printf("\nEstimated cost: %s\n", utils.FormatPrice(price))
		}

		if createConfig.Mode == "prototyping" {
			fmt.Println()
			PrintWarningSimple("PROTOTYPING MODE DISCLAIMER")
			fmt.Println("Prototyping instances are designed for development and testing.")
			fmt.Println("They may experience incompatibilities with some workloads")
			fmt.Println("for production inference or long-running tasks.")
		}
	}

	// Resolve SSH key if --ssh-key flag was provided
	var resolvedPublicKey string
	var privateKeyPath string
	if createSSHKeyName != "" {
		keys, err := client.ListSSHKeys()
		if err != nil {
			return fmt.Errorf("failed to fetch SSH keys: %w", err)
		}

		var matchedKey *api.SSHKey
		for i := range keys {
			if strings.EqualFold(keys[i].Name, createSSHKeyName) {
				matchedKey = &keys[i]
				break
			}
		}

		if matchedKey == nil {
			return fmt.Errorf("SSH key '%s' not found. Run 'tnr ssh-keys list' to see available keys", createSSHKeyName)
		}

		// Verify local private key exists so user can connect later
		privateKeyPath, err = sshkeys.FindPrivateKeyForPublicKey(matchedKey.PublicKey)
		if err != nil {
			return fmt.Errorf("failed to find local private key for '%s': %w", createSSHKeyName, err)
		}

		resolvedPublicKey = matchedKey.PublicKey
	}

	req := api.CreateInstanceRequest{
		Mode:       api.InstanceMode(createConfig.Mode),
		GPUType:    createConfig.GPUType,
		NumGPUs:    createConfig.NumGPUs,
		CPUCores:   createConfig.VCPUs,
		Template:   createConfig.Template,
		DiskSizeGB: createConfig.DiskSizeGB,
		PublicKey:  resolvedPublicKey,
	}

	var resp *api.CreateInstanceResponse
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

	// Symlink user's private key so `tnr connect` finds it automatically
	if privateKeyPath != "" && resp != nil {
		keyFile := utils.GetKeyFile(resp.UUID)
		_ = os.MkdirAll(filepath.Dir(keyFile), 0o700)
		_ = os.Remove(keyFile)
		if err := os.Symlink(privateKeyPath, keyFile); err != nil {
			PrintWarningSimple(fmt.Sprintf("Could not link SSH key for auto-connect: %v", err))
		}
	}

	return nil
}

func validateCreateConfig(config *tui.CreateConfig, templates []api.TemplateEntry, snapshots []api.Snapshot, diskSizeWasSet bool, specs *utils.SpecStore) error {
	config.Mode = strings.ToLower(config.Mode)
	config.GPUType = strings.ToLower(config.GPUType)

	if config.Mode != "prototyping" && config.Mode != "production" {
		return fmt.Errorf("mode must be 'prototyping' or 'production'")
	}

	// Normalize GPU type
	canonical, ok := specs.NormalizeGPUType(config.GPUType, config.Mode)
	if !ok {
		availableGPUs := specs.GPUOptionsForMode(config.Mode)
		return fmt.Errorf("%s mode supports GPU types: %s", config.Mode, strings.Join(availableGPUs, ", "))
	}
	config.GPUType = canonical

	// Validate GPU count
	if config.NumGPUs == 0 {
		config.NumGPUs = 1
	}

	allowedVCPUs := specs.VCPUOptions(config.GPUType, config.NumGPUs, config.Mode)
	if allowedVCPUs == nil {
		allowedCounts := specs.GPUCountsForMode(config.GPUType, config.Mode)
		return fmt.Errorf("GPU count %d is not valid for %s %s. Allowed: %v", config.NumGPUs, config.GPUType, config.Mode, allowedCounts)
	}

	if config.Mode == "prototyping" {
		if config.VCPUs == 0 {
			return fmt.Errorf("prototyping mode requires --vcpus flag (options for %s with %d GPU(s): %v)", config.GPUType, config.NumGPUs, allowedVCPUs)
		}

		valid := false
		for _, v := range allowedVCPUs {
			if config.VCPUs == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("vcpus must be one of %v for %s with %d GPU(s)", allowedVCPUs, config.GPUType, config.NumGPUs)
		}
	} else {
		// Production: vCPUs are auto-set from the spec (first/only option)
		config.VCPUs = allowedVCPUs[0]
	}

	if config.Template == "" {
		return fmt.Errorf("template is required (use --template flag)")
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
		return fmt.Errorf("template '%s' not found. Run 'tnr templates' to list available templates", config.Template)
	}

	// If a snapshot was selected, set default disk size or validate minimum
	if selectedSnapshot != nil {
		if !diskSizeWasSet {
			config.DiskSizeGB = selectedSnapshot.MinimumDiskSizeGB
		} else {
			if config.DiskSizeGB < selectedSnapshot.MinimumDiskSizeGB {
				return fmt.Errorf("disk size must be at least %d GB for snapshot '%s'", selectedSnapshot.MinimumDiskSizeGB, selectedSnapshot.Name)
			}
		}
	}

	// Validate disk size against spec storage range
	minStorage, maxStorage := specs.StorageRange(config.GPUType, config.NumGPUs, config.Mode)
	if config.DiskSizeGB < minStorage || config.DiskSizeGB > maxStorage {
		return fmt.Errorf("disk size must be between %d and %d GB", minStorage, maxStorage)
	}

	return nil
}
