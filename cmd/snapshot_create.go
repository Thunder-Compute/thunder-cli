package cmd

import (
	"fmt"
	"os"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	snapshotInstanceID string
	snapshotName       string
)

var snapshotCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a snapshot from an instance",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSnapshotCreate(cmd); err != nil {
			PrintError(err)
			os.Exit(1)
		}
	},
}

func init() {
	snapshotCreateCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSnapshotCreateHelp(cmd)
	})

	snapshotCmd.AddCommand(snapshotCreateCmd)

	snapshotCreateCmd.Flags().StringVar(&snapshotInstanceID, "instance-id", "", "Instance ID or UUID to snapshot")
	snapshotCreateCmd.Flags().StringVar(&snapshotName, "name", "", "Name for the snapshot")
}

type snapshotCreateProgressModel struct {
	spinner spinner.Model
	message string

	client *api.Client
	req    api.CreateSnapshotRequest

	done      bool
	err       error
	cancelled bool
	resp      *api.CreateSnapshotResponse
}

type createSnapshotResultMsg struct {
	resp *api.CreateSnapshotResponse
	err  error
}

func newSnapshotCreateProgressModel(client *api.Client, message string, req api.CreateSnapshotRequest) snapshotCreateProgressModel {
	theme.Init(os.Stdout)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.Primary()

	return snapshotCreateProgressModel{
		spinner: s,
		message: message,
		client:  client,
		req:     req,
	}
}

func createSnapshotCmd(client *api.Client, req api.CreateSnapshotRequest) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.CreateSnapshot(req)
		return createSnapshotResultMsg{
			resp: resp,
			err:  err,
		}
	}
}

func (m snapshotCreateProgressModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, createSnapshotCmd(m.client, m.req))
}

func (m snapshotCreateProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case createSnapshotResultMsg:
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

func (m snapshotCreateProgressModel) View() string {
	if m.done {
		if m.cancelled {
			return ""
		}

		if m.err != nil {
			return ""
		}

		labelStyle := theme.Neutral()
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(theme.PrimaryColor)).
			Padding(1, 2)

		var lines []string
		successTitleStyle := theme.Success()
		lines = append(lines, successTitleStyle.Render("✓ Snapshot created successfully!"))
		lines = append(lines, "")
		if m.resp.Message != "" {
			lines = append(lines, labelStyle.Render("Message: ")+m.resp.Message)
		}

		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		result := "\n" + boxStyle.Render(content) + "\n\n"

		// Add beta notice
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.WarningColor))
		betaNotice := "ℹ Snapshots are currently in beta. Please share feedback with us on Discord (https://discord.gg/nwuETS9jJK) or by emailing support@thundercompute.com"
		result += warningStyle.Render(betaNotice) + "\n\n"

		return result
	}

	return fmt.Sprintf("\n %s %s\n", m.spinner.View(), m.message)
}

func runSnapshotCreate(cmd *cobra.Command) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)

	isInteractive := !cmd.Flags().Changed("instance-id")

	var instanceID, name string

	if isInteractive {
		// Run interactive flow
		createConfig, err := tui.RunSnapshotCreateInteractive(client)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled snapshot creation")
				return nil
			}
			return err
		}
		instanceID = createConfig.InstanceID
		name = createConfig.Name
	} else {
		// Non-interactive mode: validate flags
		if snapshotInstanceID == "" {
			return fmt.Errorf("--instance-id is required")
		}
		if snapshotName == "" {
			return fmt.Errorf("--name is required")
		}
		instanceID = snapshotInstanceID
		name = snapshotName

		// Validate instance exists and is in RUNNING state
		busy := tui.NewBusyModel("Validating instance...")
		bp := tea.NewProgram(busy)
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

		var foundInstance *api.Instance
		for i := range instances {
			if instances[i].ID == instanceID || instances[i].UUID == instanceID {
				foundInstance = &instances[i]
				break
			}
		}

		if foundInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceID)
		}

		if foundInstance.Status != "RUNNING" {
			return fmt.Errorf("instance must be in RUNNING state to create snapshot (current state: %s)", foundInstance.Status)
		}

		// Use UUID for the API call
		instanceID = foundInstance.UUID
	}

	req := api.CreateSnapshotRequest{
		InstanceId: instanceID,
		Name:       name,
	}

	progressModel := newSnapshotCreateProgressModel(client, "Creating snapshot...", req)
	program := tea.NewProgram(progressModel)
	finalModel, runErr := program.Run()
	if runErr != nil {
		return fmt.Errorf("failed to render progress: %w", runErr)
	}

	result, ok := finalModel.(snapshotCreateProgressModel)
	if !ok {
		return fmt.Errorf("unexpected result from progress renderer")
	}

	if result.cancelled {
		PrintWarningSimple("User cancelled snapshot creation")
		return nil
	}

	if result.err != nil {
		return fmt.Errorf("failed to create snapshot: %w", result.err)
	}

	return nil
}
