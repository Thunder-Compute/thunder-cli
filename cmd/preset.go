/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/spf13/cobra"
)

// Preset represents a saved instance configuration
type Preset struct {
	Name       string    `json:"name"`
	Mode       string    `json:"mode"`
	GPUType    string    `json:"gpu_type"`
	NumGPUs    int       `json:"num_gpus"`
	VCPUs      int       `json:"vcpus"`
	Template   string    `json:"template"`
	DiskSizeGB int       `json:"disk_size_gb"`
	CreatedAt  time.Time `json:"created_at"`
}

// PresetConfig manages the collection of presets
type PresetConfig struct {
	Presets []Preset `json:"presets"`
}

// getPresetsPath returns the path to the presets file
func getPresetsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".thunder", "presets.json"), nil
}

func LoadPresets() (*PresetConfig, error) {
	presetsPath, err := getPresetsPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(presetsPath); os.IsNotExist(err) {
		return &PresetConfig{Presets: []Preset{}}, nil
	}

	data, err := os.ReadFile(presetsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read presets file: %w", err)
	}

	var config PresetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse presets file: %w", err)
	}

	return &config, nil
}

func SavePresets(config *PresetConfig) error {
	presetsPath, err := getPresetsPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(presetsPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create presets directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal presets: %w", err)
	}

	if err := os.WriteFile(presetsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write presets file: %w", err)
	}

	return nil
}

func AddPreset(preset Preset) error {
	config, err := LoadPresets()
	if err != nil {
		return err
	}

	for _, existing := range config.Presets {
		if existing.Name == preset.Name {
			return fmt.Errorf("preset '%s' already exists", preset.Name)
		}
	}

	config.Presets = append(config.Presets, preset)
	return SavePresets(config)
}

func DeletePreset(name string) error {
	config, err := LoadPresets()
	if err != nil {
		return err
	}

	var found bool
	newPresets := make([]Preset, 0, len(config.Presets))
	for _, preset := range config.Presets {
		if preset.Name != name {
			newPresets = append(newPresets, preset)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("preset '%s' not found", name)
	}

	config.Presets = newPresets
	return SavePresets(config)
}

func GetPreset(name string) (*Preset, error) {
	config, err := LoadPresets()
	if err != nil {
		return nil, err
	}

	for _, preset := range config.Presets {
		if preset.Name == name {
			return &preset, nil
		}
	}

	return nil, fmt.Errorf("preset '%s' not found", name)
}

func ListPresets() ([]Preset, error) {
	config, err := LoadPresets()
	if err != nil {
		return nil, err
	}

	return config.Presets, nil
}

func GeneratePresetName(config *tui.CreateConfig) string {
	var parts []string

	parts = append(parts, strings.ToLower(config.Mode))

	gpuType := strings.ToLower(config.GPUType)
	if strings.Contains(gpuType, "a100") {
		parts = append(parts, "a100")
	} else if strings.Contains(gpuType, "h100") {
		parts = append(parts, "h100")
	} else if strings.Contains(gpuType, "t4") {
		parts = append(parts, "t4")
	}

	if config.Mode == "prototyping" {
		parts = append(parts, fmt.Sprintf("%dvcpu", config.VCPUs))
	} else {
		parts = append(parts, fmt.Sprintf("%dgpu", config.NumGPUs))
	}

	template := strings.ToLower(config.Template)
	template = strings.Split(template, "-")[0]
	parts = append(parts, template)

	timestamp := time.Now().Format("20060102")
	parts = append(parts, timestamp)

	return strings.Join(parts, "-")
}

func PresetExists(name string) bool {
	preset, err := GetPreset(name)
	return err == nil && preset != nil
}

var presetCmd = &cobra.Command{
	Use:   "preset",
	Short: "Manage instance configuration presets",
	Long:  `Manage saved instance configurations that can be reused for creating instances.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var presetSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save an instance configuration as a preset",
	Long:  `Create a new preset by configuring an instance. Run the interactive wizard to set up your configuration, then save it with a name.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPresetSave(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var presetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved presets",
	Long:  `Display all saved instance configuration presets in a table format.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPresetList(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var presetDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a preset by name",
	Long:  `Remove a saved preset from your configuration.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPresetDelete(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var presetShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Display preset details",
	Long:  `Show detailed information about a specific preset.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPresetShow(args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(presetCmd)
	presetCmd.AddCommand(presetSaveCmd)
	presetCmd.AddCommand(presetListCmd)
	presetCmd.AddCommand(presetDeleteCmd)
	presetCmd.AddCommand(presetShowCmd)
}

func runPresetSave() error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token)

	preset, err := tui.RunPresetSaveInteractive(client)
	if err != nil {
		return err
	}

	newPreset := Preset{
		Name:       preset.Name,
		Mode:       preset.Config.Mode,
		GPUType:    preset.Config.GPUType,
		NumGPUs:    preset.Config.NumGPUs,
		VCPUs:      preset.Config.VCPUs,
		Template:   preset.Config.Template,
		DiskSizeGB: preset.Config.DiskSizeGB,
		CreatedAt:  time.Now(),
	}

	if err := AddPreset(newPreset); err != nil {
		return fmt.Errorf("failed to save preset: %w", err)
	}

	fmt.Printf("✓ Preset '%s' saved successfully!\n", preset.Name)
	return nil
}

func runPresetList() error {
	presets, err := ListPresets()
	if err != nil {
		return err
	}

	if len(presets) == 0 {
		fmt.Println("No presets saved. Use 'tnr preset save' to create one.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tMODE\tGPU\tCOMPUTE\tTEMPLATE\tDISK\tCREATED")
	fmt.Fprintln(w, "----\t----\t---\t-------\t--------\t----\t-------")

	for _, preset := range presets {
		var compute string
		if preset.Mode == "prototyping" {
			compute = fmt.Sprintf("%d vCPUs", preset.VCPUs)
		} else {
			compute = fmt.Sprintf("%d GPUs", preset.NumGPUs)
		}

		gpu := preset.GPUType
		if strings.Contains(gpu, "a100") {
			gpu = "A100"
		} else if strings.Contains(gpu, "h100") {
			gpu = "H100"
		} else if strings.Contains(gpu, "t4") {
			gpu = "T4"
		}

		template := strings.Split(preset.Template, "-")[0]
		created := preset.CreatedAt.Format("2006-01-02")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d GB\t%s\n",
			preset.Name,
			strings.Title(preset.Mode),
			gpu,
			compute,
			template,
			preset.DiskSizeGB,
			created,
		)
	}

	return w.Flush()
}

func runPresetDelete(name string) error {
	preset, err := GetPreset(name)
	if err != nil {
		return err
	}

	fmt.Printf("\nPreset: %s\n", preset.Name)
	fmt.Printf("Mode: %s\n", strings.Title(preset.Mode))
	fmt.Printf("GPU: %s\n", preset.GPUType)
	if preset.Mode == "prototyping" {
		fmt.Printf("vCPUs: %d\n", preset.VCPUs)
	} else {
		fmt.Printf("GPUs: %d\n", preset.NumGPUs)
	}
	fmt.Printf("Template: %s\n", preset.Template)
	fmt.Printf("Disk Size: %d GB\n", preset.DiskSizeGB)
	fmt.Println()

	model := tui.NewPresetDeleteModel(preset.Name)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run confirmation dialog: %w", err)
	}

	result := finalModel.(tui.PresetDeleteModel)
	if !result.Confirmed() {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	if err := DeletePreset(name); err != nil {
		return err
	}

	fmt.Printf("✓ Preset '%s' deleted successfully\n", name)
	return nil
}

func runPresetShow(name string) error {
	preset, err := GetPreset(name)
	if err != nil {
		return err
	}

	fmt.Printf("\nPreset: %s\n", preset.Name)
	fmt.Printf("Created: %s\n", preset.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Mode:       %s\n", strings.Title(preset.Mode))
	fmt.Printf("  GPU Type:   %s\n", strings.ToUpper(preset.GPUType))

	if preset.Mode == "prototyping" {
		fmt.Printf("  vCPUs:      %d\n", preset.VCPUs)
		fmt.Printf("  RAM:        %d GB\n", preset.VCPUs*8)
	} else {
		fmt.Printf("  GPUs:       %d\n", preset.NumGPUs)
		fmt.Printf("  vCPUs:      %d\n", preset.VCPUs)
		fmt.Printf("  RAM:        %d GB\n", preset.NumGPUs*144)
	}

	fmt.Printf("  Template:   %s\n", preset.Template)
	fmt.Printf("  Disk Size:  %d GB\n", preset.DiskSizeGB)
	fmt.Println()

	return nil
}
