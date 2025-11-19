/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	Run: func(cmd *cobra.Command, args []string) {
		if err := runCreate(cmd); err != nil {
			PrintError(err)
			os.Exit(1)
		}
	},
}

var (
	prototypingGPUMap = map[string]string{
		"t4":   "t4",
		"a100": "a100xl",
	}

	productionGPUMap = map[string]string{
		"a100": "a100xl",
		"h100": "h100",
	}
)

func init() {
	createCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderCreateHelp(cmd)
	})

	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVar(&mode, "mode", "", "Instance mode: prototyping or production")
	createCmd.Flags().StringVar(&gpuType, "gpu", "", "GPU type (prototyping: t4 or a100, production: a100 or h100)")
	createCmd.Flags().IntVar(&numGPUs, "num-gpus", 0, "Number of GPUs (production only): 1, 2, or 4")
	createCmd.Flags().IntVar(&vcpus, "vcpus", 0, "CPU cores (prototyping only): 4, 8, 16, or 32")
	createCmd.Flags().StringVar(&template, "template", "", "OS template key or name")
	createCmd.Flags().IntVar(&diskSizeGB, "disk-size-gb", 100, "Disk storage in GB (100-1000)")
}

type createProgressModel struct {
	spinner spinner.Model
	message string

	client *api.Client
	req    api.CreateInstanceRequest

	done      bool
	err       error
	cancelled bool
	resp      *api.CreateInstanceResponse
}

type createInstanceResultMsg struct {
	resp *api.CreateInstanceResponse
	err  error
}

func newCreateProgressModel(client *api.Client, message string, req api.CreateInstanceRequest) createProgressModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8dc8ff"))

	return createProgressModel{
		spinner: s,
		message: message,
		client:  client,
		req:     req,
	}
}

func createInstanceCmd(client *api.Client, req api.CreateInstanceRequest) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.CreateInstance(req)
		return createInstanceResultMsg{
			resp: resp,
			err:  err,
		}
	}
}

func (m createProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, createInstanceCmd(m.client, m.req))
}

func (m createProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case createInstanceResultMsg:
		m.done = true
		m.err = msg.err
		if msg.err == nil {
			m.resp = msg.resp
		}
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		}

	case tea.QuitMsg:
		return m, nil
	}

	return m, nil
}

func (m createProgressModel) View() string {
	if m.done {
		if m.cancelled {
			return ""
		}

		if m.err != nil {
			return ""
		}

		headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8dc8ff")).Bold(true)
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		valueStyle := lipgloss.NewStyle().Bold(true)
		cmdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8dc8ff")).
			Padding(1, 2)

		var lines []string
		successTitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00D787")).Bold(true)
		lines = append(lines, successTitleStyle.Render("✓ Instance created successfully!"))
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render("Instance ID:")+" "+valueStyle.Render(fmt.Sprintf("%d", m.resp.Identifier)))
		if m.resp.Message != "" {
			lines = append(lines, labelStyle.Render("Message:")+" "+cmdStyle.Render(m.resp.Message))
		}
		lines = append(lines, "")
		lines = append(lines, headerStyle.Render("Next steps:"))
		lines = append(lines, cmdStyle.Render("  • Run 'tnr status' to monitor provisioning progress"))
		lines = append(lines, cmdStyle.Render(fmt.Sprintf("  • Run 'tnr connect %d' once the instance is RUNNING", m.resp.Identifier)))

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return "\n" + boxStyle.Render(content) + "\n\n"
	}

	return fmt.Sprintf("\n %s %s\n", m.spinner.View(), m.message)
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

	isInteractive := !cmd.Flags().Changed("mode")

	var createConfig *tui.CreateConfig

	if isInteractive {
		createConfig, err = tui.RunCreateInteractive(client)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled creation process")
				return nil
			}
			return err
		}
	} else {
		busy := tui.NewBusyModel("Fetching templates...")
		bp := tea.NewProgram(busy)
		busyDone := make(chan struct{})

		go func() {
			_, _ = bp.Run()
			close(busyDone)
		}()

		templates, err := client.ListTemplates()

		bp.Send(tui.BusyDoneMsg{})
		<-busyDone

		if err != nil {
			return fmt.Errorf("failed to fetch templates: %w", err)
		}

		if len(templates) == 0 {
			return fmt.Errorf("no templates available")
		}

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
			fmt.Println()
			PrintWarningSimple("PROTOTYPING MODE DISCLAIMER")
			fmt.Println("Prototyping instances are designed for development and testing.")
			fmt.Println("They may experience incompatibilities with some workloads")
			fmt.Println("for production inference or long-running tasks.")
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

	progressModel := newCreateProgressModel(client, "Creating instance...", req)
	program := tea.NewProgram(progressModel)
	finalModel, runErr := program.Run()
	if runErr != nil {
		return fmt.Errorf("failed to render progress: %w", runErr)
	}

	result, ok := finalModel.(createProgressModel)
	if !ok {
		return fmt.Errorf("unexpected result from progress renderer")
	}

	if result.cancelled {
		PrintWarningSimple("User cancelled creation process")
		return nil
	}

	if result.err != nil {
		return fmt.Errorf("failed to create instance: %w", result.err)
	}

	return nil
}

func validateCreateConfig(config *tui.CreateConfig, templates []api.Template) error {
	config.Mode = strings.ToLower(config.Mode)
	config.GPUType = strings.ToLower(config.GPUType)

	if config.Mode != "prototyping" && config.Mode != "production" {
		return fmt.Errorf("mode must be 'prototyping' or 'production'")
	}

	if config.Mode == "prototyping" {
		canonical, ok := prototypingGPUMap[config.GPUType]
		if !ok {
			return fmt.Errorf("prototyping mode supports GPU types: t4 or a100")
		}
		config.GPUType = canonical
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
		canonical, ok := productionGPUMap[config.GPUType]
		if !ok {
			return fmt.Errorf("production mode supports GPU types: a100 or h100")
		}
		config.GPUType = canonical

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
