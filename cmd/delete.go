/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuawatkins04/thunder-cli-draft/api"
	"github.com/joshuawatkins04/thunder-cli-draft/tui"
	helpmenus "github.com/joshuawatkins04/thunder-cli-draft/tui/help-menus"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [instance_id]",
	Short: "Delete a Thunder Compute instance",
	Long: `Permanently delete a Thunder Compute instance.

This command will:
• Delete the instance from Thunder Compute servers
• Clean up SSH configuration (~/.ssh/config)
• Remove the instance from known hosts (~/.ssh/known_hosts)

WARNING: This action is IRREVERSIBLE!
All data on the instance will be permanently lost.

Examples:
  # Interactive mode - select from a list
  tnr delete

  # Direct deletion with instance ID
  tnr delete 0`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDelete(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	deleteCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderDeleteHelp(cmd)
	})

	rootCmd.AddCommand(deleteCmd)
}

type deleteSpinnerModel struct {
	spinner  spinner.Model
	message  string
	quitting bool
}

func newDeleteSpinnerModel(message string) deleteSpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff"))
	return deleteSpinnerModel{
		spinner: s,
		message: message,
	}
}

func (m deleteSpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m deleteSpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.QuitMsg:
		m.quitting = true
		return m, nil
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m deleteSpinnerModel) View() string {
	if m.quitting {
		return ""
	}
	return fmt.Sprintf("\n %s %s\n\n", m.spinner.View(), m.message)
}

func runDelete(args []string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token)

	var instanceID string
	var selectedInstance *api.Instance

	if len(args) == 0 {
		selectedInstance, err = tui.RunDeleteInteractive(client)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				fmt.Println("User cancelled delete process")
				return nil
			}
			if strings.Contains(err.Error(), "no instances available to delete") {
				fmt.Println("No instances found. Create an instance first using 'tnr create'")
				return nil
			}
			return err
		}
		instanceID = selectedInstance.ID
	} else {
		instanceID = args[0]

		instances, err := client.ListInstances()
		if err != nil {
			return fmt.Errorf("failed to fetch instances: %w", err)
		}

		for i := range instances {
			if instances[i].ID == instanceID || instances[i].UUID == instanceID {
				selectedInstance = &instances[i]
				break
			}
		}

		if selectedInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceID)
		}
	}

	if selectedInstance.Status == "DELETING" {
		return fmt.Errorf("instance '%s' is already being deleted", instanceID)
	}

	if selectedInstance.Status == "STARTING" {
		fmt.Printf("\nWarning: Instance '%s' is currently STARTING.\n", instanceID)
		fmt.Println("Deletion may fail. It's recommended to wait until the instance is RUNNING.")
		fmt.Println("\nAttempting deletion anyway...")
	}

	p := tea.NewProgram(newDeleteSpinnerModel(fmt.Sprintf("Deleting instance %s...", instanceID)))
	go func() {
		p.Run()
	}()

	_, err = client.DeleteInstance(instanceID)
	p.Quit()

	if err != nil {
		return fmt.Errorf("failed to delete instance: %w\n\nPossible reasons:\n• Instance may be in STARTING state (wait for it to fully start first)\n• Instance may already be deleted\n• Server error occurred\n\nTry running 'tnr status' to check the instance state", err)
	}

	if err := cleanupSSHConfig(instanceID, selectedInstance.IP); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to clean up SSH configuration: %v\n", err)
	}

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0391ff")).Bold(true)
	fmt.Println(successStyle.Render(fmt.Sprintf("\n✓ Successfully deleted Thunder Compute instance %s", instanceID)))

	return nil
}

func cleanupSSHConfig(instanceID, ipAddress string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	sshConfigPath := filepath.Join(homeDir, ".ssh", "config")

	if err := removeSSHHostEntry(sshConfigPath, instanceID); err != nil {
		return fmt.Errorf("failed to clean SSH config: %w", err)
	}

	if ipAddress != "" {
		cmd := exec.Command("ssh-keygen", "-R", ipAddress)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Run()
	}

	return nil
}

func removeSSHHostEntry(configPath, instanceID string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	hostName := fmt.Sprintf("tnr-%s", instanceID)
	inTargetHost := false
	skipUntilNextHost := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "Host ") {
			if trimmedLine == fmt.Sprintf("Host %s", hostName) {
				inTargetHost = true
				skipUntilNextHost = true
				continue
			} else {
				inTargetHost = false
				skipUntilNextHost = false
			}
		}

		if skipUntilNextHost && inTargetHost {
			if strings.HasPrefix(trimmedLine, "Host ") {
				skipUntilNextHost = false
				inTargetHost = false
				lines = append(lines, line)
			}
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}
