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

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [instance_id]",
	Short: "Delete a Thunder Compute instance",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDelete(args); err != nil {
			PrintError(err)
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

		selectedInstance, err = tui.RunDeleteInteractive(client, instances)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled delete process")
				return nil
			}
			return err
		}
		instanceID = selectedInstance.ID
	} else {
		instanceID = args[0]

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

	fmt.Println()
	successMsg, err := tui.RunDeleteProgress(client, instanceID)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w\n\nPossible reasons:\n• Instance may already be deleted\n• Server error occurred\n\nTry running 'tnr status' to check the instance state", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	if err := cleanupSSHConfig(instanceID, selectedInstance.IP); err != nil {
		PrintWarning(fmt.Sprintf("Failed to clean up SSH configuration: %v", err))
	}

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
		_ = cmd.Run() //nolint:errcheck // known_hosts cleanup failure is non-fatal
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
