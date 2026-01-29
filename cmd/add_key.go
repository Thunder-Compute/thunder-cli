package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/sentry"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var addKeyCmd = &cobra.Command{
	Use:   "add-key [instance_id]",
	Short: "Add your SSH public key to a Thunder Compute instance",
	Long: `Add an existing SSH public key to an instance's authorized_keys.

By default, uses ~/.ssh/id_rsa.pub. You can specify a different key file path.
This allows you to SSH into the instance using your existing key pair.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAddKey(args)
	},
}

func init() {
	addKeyCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderAddKeyHelp(cmd)
	})

	rootCmd.AddCommand(addKeyCmd)
}

func runAddKey(args []string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	client := api.NewClient(config.Token, config.APIURL)

	var instanceID string
	var selectedInstance *api.Instance

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

	// Filter for running instances only
	runningInstances := make([]api.Instance, 0)
	for _, inst := range instances {
		if inst.Status == "RUNNING" {
			runningInstances = append(runningInstances, inst)
		}
	}

	if len(runningInstances) == 0 {
		PrintWarningSimple("No running instances found. SSH keys can only be added to running instances.")
		return nil
	}

	if len(args) == 0 {
		// Interactive instance selection
		selectedInstance, err = tui.RunAddKeyInteractive(client, runningInstances)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("Cancelled")
				return nil
			}
			return err
		}
		instanceID = selectedInstance.ID
	} else {
		instanceID = args[0]

		// Find the instance
		for i := range instances {
			if instances[i].ID == instanceID || instances[i].UUID == instanceID {
				selectedInstance = &instances[i]
				break
			}
		}

		if selectedInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceID)
		}

		if selectedInstance.Status != "RUNNING" {
			return fmt.Errorf("instance '%s' is not running (status: %s). SSH keys can only be added to running instances", instanceID, selectedInstance.Status)
		}

		// For non-interactive mode, still run the key selection flow
		selectedInstance, err = tui.RunAddKeyInteractive(client, []api.Instance{*selectedInstance})
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("Cancelled")
				return nil
			}
			return err
		}
	}

	PrintSuccessSimple(fmt.Sprintf("SSH key added to instance %s (%s)", selectedInstance.ID, selectedInstance.IP))
	if selectedInstance.Port != 0 {
		PrintWarningSimple(fmt.Sprintf("You can now connect using: ssh -p %d ubuntu@%s", selectedInstance.Port, selectedInstance.IP))
	} else {
		PrintWarningSimple("You can now connect using: ssh ubuntu@" + selectedInstance.IP)
	}
	return nil
}

// getDefaultKeyPath returns the default SSH public key path
func getDefaultKeyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".ssh", "id_rsa.pub")
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// readPublicKey reads and validates a public key file
func readPublicKey(path string) (string, error) {
	expandedPath := expandPath(path)

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("key file is empty")
	}

	// Basic validation - check if it looks like an SSH public key
	validPrefixes := []string{
		"ssh-rsa ",
		"ssh-ed25519 ",
		"ssh-dss ",
		"ecdsa-sha2-nistp256 ",
		"ecdsa-sha2-nistp384 ",
		"ecdsa-sha2-nistp521 ",
		"sk-ssh-ed25519@openssh.com ",
		"sk-ecdsa-sha2-nistp256@openssh.com ",
	}

	valid := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(key, prefix) {
			valid = true
			break
		}
	}

	if !valid {
		return "", fmt.Errorf("file does not appear to be a valid SSH public key")
	}

	return key, nil
}

// getPathSuggestions returns file path suggestions for autocomplete
func getPathSuggestions(input string) []string {
	expandedInput := expandPath(input)

	// If input is empty, start from home directory
	if input == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		expandedInput = homeDir
	}

	// Determine the directory to scan
	dir := expandedInput
	prefix := ""

	stat, err := os.Stat(expandedInput)
	if err != nil || !stat.IsDir() {
		// Not a directory, use parent directory
		dir = filepath.Dir(expandedInput)
		prefix = filepath.Base(expandedInput)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless user is explicitly looking for them
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		// Filter by prefix
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}

		fullPath := filepath.Join(dir, name)

		// Convert back to use ~ if in home directory
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" && strings.HasPrefix(fullPath, homeDir) {
			fullPath = "~" + fullPath[len(homeDir):]
		}

		// Add trailing slash for directories
		if entry.IsDir() {
			fullPath += "/"
		}

		suggestions = append(suggestions, fullPath)
	}

	// Sort suggestions
	sort.Strings(suggestions)

	// Limit to reasonable number
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	return suggestions
}

// keyFileExists checks if a key file exists
func keyFileExists(path string) bool {
	expandedPath := expandPath(path)
	_, err := os.Stat(expandedPath)
	return err == nil
}

// addSSHKeyToInstance adds the SSH key to the instance via API
func addSSHKeyToInstance(client *api.Client, instanceID, publicKey string) error {
	sentry.AddBreadcrumb("add-key", "adding public key to instance", map[string]interface{}{
		"instance_id": instanceID,
	}, sentry.LevelInfo)

	req := &api.AddSSHKeyRequest{
		PublicKey: &publicKey,
	}

	resp, err := client.AddSSHKey(instanceID, req)
	if err != nil {
		sentry.AddBreadcrumb("add-key", "failed to add key", map[string]interface{}{
			"error": err.Error(),
		}, sentry.LevelError)
		return err
	}

	if !resp.Success {
		return fmt.Errorf("server reported failure: %s", resp.Message)
	}

	sentry.AddBreadcrumb("add-key", "key added successfully", nil, sentry.LevelInfo)
	return nil
}
