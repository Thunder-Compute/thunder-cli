/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/joshuawatkins04/thunder-cli-draft/api"
	"github.com/joshuawatkins04/thunder-cli-draft/tui"
	"github.com/spf13/cobra"
)

var (
	mode       string
	gpuType    string
	numGPUs    int
	vcpus      int
	template   string
	diskSizeGB int
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Thunder Compute GPU instance",
	Long: `Create a new Thunder Compute GPU instance with customizable configuration.

The command supports two modes:
- Interactive: Run without flags to use the step-by-step wizard
- Non-interactive: Provide all required flags for automated creation

Instance Modes:
1. Prototyping (default, lowest cost):
   - GPU: T4 or A100
   - GPU Count: Exactly 1 GPU
   - vCPUs: Choose 4, 8, 16, or 32 vCPUs (8GB RAM per vCPU)
   - Use Case: Development and testing

2. Production (highest stability):
   - GPU: A100 or H100
   - GPU Count: 1, 2, or 4 GPUs
   - vCPUs: Fixed at 18 per GPU (144GB RAM per GPU)
   - Use Case: Long-running production workloads

Examples:
  # Interactive mode
  tnr create

  # Non-interactive prototyping instance
  tnr create --mode prototyping --gpu t4 --vcpus 8 --template ubuntu-22.04 --disk-size-gb 100

  # Non-interactive production instance
  tnr create --mode production --num-gpus 2 --template pytorch --disk-size-gb 500`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreate(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&mode, "mode", "", "Instance mode: prototyping or production")
	createCmd.Flags().StringVar(&gpuType, "gpu", "", "GPU type: t4, a100, or h100")
	createCmd.Flags().IntVar(&numGPUs, "num-gpus", 0, "Number of GPUs (production only): 1, 2, or 4")
	createCmd.Flags().IntVar(&vcpus, "vcpus", 0, "CPU cores (prototyping only): 4, 8, 16, or 32")
	createCmd.Flags().StringVar(&template, "template", "", "OS template key or name")
	createCmd.Flags().IntVar(&diskSizeGB, "disk-size-gb", 100, "Disk storage in GB (100-1000)")
}

func runCreate(cmd *cobra.Command) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token)

	fmt.Println("Fetching available templates...")
	templates, err := client.ListTemplates()
	if err != nil {
		return fmt.Errorf("failed to fetch templates: %w", err)
	}

	if len(templates) == 0 {
		return fmt.Errorf("no templates available")
	}

	isInteractive := !cmd.Flags().Changed("mode")

	var createConfig *tui.CreateConfig

	if isInteractive {
		fmt.Println("Starting interactive instance creation wizard...")
		createConfig, err = tui.RunCreateInteractive(templates)
		if err != nil {
			return err
		}
	} else {
		createConfig = &tui.CreateConfig{
			Mode:       mode,
			GPUType:    gpuType,
			NumGPUs:    numGPUs,
			VCPUs:      vcpus,
			Template:   template,
			DiskSizeGB: diskSizeGB,
		}

		if err := validateCreateConfig(createConfig, templates); err != nil {
			return err
		}

		if createConfig.Mode == "prototyping" {
			fmt.Println("\nPROTOTYPING MODE DISCLAIMER")
			fmt.Println("Prototyping instances are designed for development and testing.")
			fmt.Println("They may experience occasional interruptions and are not recommended")
			fmt.Println("for production workloads or long-running tasks.")
		}
	}

	req := api.CreateInstanceRequest{
		Mode:       createConfig.Mode,
		GPUType:    createConfig.GPUType,
		NumGPUs:    createConfig.NumGPUs,
		CPUCores:   createConfig.VCPUs,
		Template:   createConfig.Template,
		DiskSizeGB: createConfig.DiskSizeGB,
	}

	fmt.Println("Creating instance...")
	resp, err := client.CreateInstance(req)
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	fmt.Println("\n✓ Instance created successfully!")
	fmt.Printf("\nInstance ID: %s\n", resp.UUID)
	if resp.Message != "" {
		fmt.Printf("Message: %s\n", resp.Message)
	}
	fmt.Println("\nNext steps:")
	fmt.Println("  • Run 'tnr status' to monitor provisioning progress")
	fmt.Printf("  • Run 'tnr connect %s' once the instance is RUNNING\n", resp.UUID)

	return nil
}

func validateCreateConfig(config *tui.CreateConfig, templates []api.Template) error {
	if config.Mode != "prototyping" && config.Mode != "production" {
		return fmt.Errorf("mode must be 'prototyping' or 'production'")
	}

	config.GPUType = strings.ToLower(config.GPUType)
	if config.GPUType != "t4" && config.GPUType != "a100" && config.GPUType != "h100" {
		return fmt.Errorf("gpu type must be 't4', 'a100', or 'h100'")
	}

	if config.Mode == "prototyping" {
		config.NumGPUs = 1

		if config.VCPUs == 0 {
			return fmt.Errorf("prototyping mode requires --vcpus flag (4, 8, 16, or 32)")
		}

		validVCPUs := []int{4, 8, 16, 32}
		valid := false
		for _, v := range validVCPUs {
			if config.VCPUs == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("vcpus must be one of: 4, 8, 16, or 32")
		}
	} else {
		if config.GPUType != "a100" && config.GPUType != "h100" {
			return fmt.Errorf("production mode only supports 'a100' or 'h100' GPU types")
		}

		if config.NumGPUs == 0 {
			return fmt.Errorf("production mode requires --num-gpus flag (1, 2, or 4)")
		}

		validGPUs := []int{1, 2, 4}
		valid := false
		for _, v := range validGPUs {
			if config.NumGPUs == v {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("num-gpus must be one of: 1, 2, or 4")
		}

		config.VCPUs = 18 * config.NumGPUs
	}

	if config.DiskSizeGB < 100 || config.DiskSizeGB > 1000 {
		return fmt.Errorf("disk size must be between 100 and 1000 GB")
	}

	if config.Template == "" {
		return fmt.Errorf("template is required (use --template flag)")
	}

	templateFound := false
	for _, t := range templates {
		if t.Key == config.Template || strings.EqualFold(t.DisplayName, config.Template) {
			config.Template = t.Key
			templateFound = true
			break
		}
	}
	if !templateFound {
		return fmt.Errorf("template '%s' not found. Run 'tnr templates' to list available templates", config.Template)
	}

	return nil
}
