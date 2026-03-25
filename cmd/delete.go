package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete [instance_id]",
	Short: "Delete a Thunder Compute instance",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDelete(args)
	},
}

func init() {
	deleteCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderDeleteHelp(cmd)
	})

	rootCmd.AddCommand(deleteCmd)
}

func runDelete(args []string) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	interactive := tui.IsInteractive() && !JSONOutput

	var instanceID string
	var selectedInstance *api.Instance

	if len(args) == 0 {
		if !interactive {
			return fmt.Errorf("instance ID required in non-interactive mode")
		}
		var instances []api.Instance
		if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
			var e error
			instances, e = client.ListInstances()
			return e
		}); err != nil {
			return fmt.Errorf("failed to fetch instances: %w", err)
		}

		if len(instances) == 0 {
			PrintWarningSimple("No instances found. Use 'tnr create' to create a Thunder Compute instance.")
			return nil
		}

		selectedInstance, err = tui.RunDeleteInteractive(client, instances)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled delete process")
				return nil
			}
			return err
		}
		instanceID = selectedInstance.ID
	} else {
		instanceID = args[0]

		var instances []api.Instance
		if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
			var e error
			instances, e = client.ListInstances()
			return e
		}); err != nil {
			return fmt.Errorf("failed to fetch instances: %w", err)
		}

		selectedInstance = findInstance(instances, instanceID)

		if selectedInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceID)
		}
	}

	if selectedInstance.Status == "DELETING" {
		return fmt.Errorf("instance '%s' is already being deleted", instanceID)
	}

	if !interactive {
		if !YesFlag {
			return fmt.Errorf("use --yes to confirm deletion in non-interactive mode")
		}
		// Non-interactive: direct API call
		fmt.Fprintln(os.Stderr, "Deleting instance...")
		resp, deleteErr := client.DeleteInstance(instanceID)
		if deleteErr != nil {
			return fmt.Errorf("failed to delete instance: %w", deleteErr)
		}
		if JSONOutput {
			printJSON(resp)
		} else {
			fmt.Printf("Deleted instance %s\n", instanceID)
		}

		if err := cleanupSSHConfig(instanceID, selectedInstance.GetIP()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clean up SSH configuration: %v\n", err)
		}
		return nil
	}

	successMsg, err := tui.RunDeleteProgress(client, instanceID)
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w\n\nPossible reasons:\n• Instance may already be deleted\n• Server error occurred\n\nTry running 'tnr status' to check the instance state", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	if err := cleanupSSHConfig(instanceID, selectedInstance.GetIP()); err != nil {
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

	return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
