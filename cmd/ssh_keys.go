package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
)

// ── ssh-keys (parent) ───────────────────────────────────────────────────────

var sshKeysCmd = &cobra.Command{
	Use:     "ssh-keys",
	Aliases: []string{"ssh-key", "keys"},
	Short:   "Add external keys to Thunder Compute instances",
	Long:    "Manage saved SSH public keys for your organization.",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	sshKeysCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSSHKeysHelp(cmd)
	})
	rootCmd.AddCommand(sshKeysCmd)

	// ── list ────────────────────────────────────────────────────────────

	sshKeysListCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSSHKeysListHelp(cmd)
	})
	sshKeysCmd.AddCommand(sshKeysListCmd)

	// ── add ─────────────────────────────────────────────────────────────

	sshKeysAddCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSSHKeysAddHelp(cmd)
	})
	sshKeysAddCmd.Flags().StringVar(&sshKeyAddName, "name", "", "Name for the SSH key")
	sshKeysAddCmd.Flags().StringVar(&sshKeyAddKeyFile, "key-file", "", "Path to SSH public key file")
	sshKeysAddCmd.Flags().StringVar(&sshKeyAddKey, "key", "", "SSH public key string")
	sshKeysCmd.AddCommand(sshKeysAddCmd)

	// ── delete ──────────────────────────────────────────────────────────

	sshKeysDeleteCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSSHKeysDeleteHelp(cmd)
	})
	sshKeysCmd.AddCommand(sshKeysDeleteCmd)
}

// ── ssh-keys list ───────────────────────────────────────────────────────────

var sshKeysListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all SSH keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSSHKeysList()
	},
}

func runSSHKeysList() error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	var keys api.SSHKeyListResponse
	if err := tui.RunWithBusySpinner("Fetching SSH keys...", os.Stdout, func() error {
		var e error
		keys, e = client.ListSSHKeys()
		return e
	}); err != nil {
		return fmt.Errorf("failed to fetch SSH keys: %w", err)
	}

	if len(keys) == 0 {
		fmt.Println(tui.WarningStyle().Render("⚠ No SSH keys found. Add one with 'tnr ssh-keys add'."))
		return nil
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt < keys[j].CreatedAt
	})

	renderSSHKeysTable(keys)

	return nil
}

func renderSSHKeysTable(keys api.SSHKeyListResponse) {
	tui.InitCommonStyles(os.Stdout)

	headerStyle := tui.PrimaryTitleStyle().Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)

	colWidths := map[string]int{
		"Name":        20,
		"Fingerprint": 52, // SHA256 fingerprints are fixed at 50 chars (7 prefix + 43 base64)
		"Key Type":    16,
		"Created":     22,
	}

	var b strings.Builder

	headers := []string{"Name", "Fingerprint", "Key Type", "Created"}
	headerRow := make([]string, len(headers))
	for i, h := range headers {
		headerRow[i] = headerStyle.Width(colWidths[h]).Render(h)
	}
	b.WriteString(strings.Join(headerRow, ""))
	b.WriteString("\n")

	separatorRow := make([]string, len(headers))
	for i, h := range headers {
		separatorRow[i] = strings.Repeat("─", colWidths[h]+2)
	}
	b.WriteString(strings.Join(separatorRow, ""))
	b.WriteString("\n")

	const pad = 2 // cellStyle Padding(0,1) = 1 left + 1 right
	for _, key := range keys {
		name := truncate(key.Name, colWidths["Name"]-pad)
		fingerprint := truncate(key.Fingerprint, colWidths["Fingerprint"]-pad)
		keyType := truncate(key.KeyType, colWidths["Key Type"]-pad)

		createdTime := time.Unix(key.CreatedAt, 0)
		created := truncate(createdTime.Format("2006-01-02 15:04:05"), colWidths["Created"]-pad)

		row := []string{
			cellStyle.Width(colWidths["Name"]).Render(name),
			cellStyle.Width(colWidths["Fingerprint"]).Render(fingerprint),
			cellStyle.Width(colWidths["Key Type"]).Render(keyType),
			cellStyle.Width(colWidths["Created"]).Render(created),
		}
		b.WriteString(strings.Join(row, ""))
		b.WriteString("\n")
	}

	fmt.Print(b.String())
}

// ── ssh-keys add ────────────────────────────────────────────────────────────

var (
	sshKeyAddName    string
	sshKeyAddKeyFile string
	sshKeyAddKey     string
)

var sshKeysAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add an SSH key",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSSHKeysAdd(cmd)
	},
}

func runSSHKeysAdd(cmd *cobra.Command) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	isNonInteractive := cmd.Flags().Changed("name")

	if isNonInteractive {
		return runSSHKeysAddNonInteractive(client, cmd)
	}

	return runSSHKeysAddInteractive(client)
}

func runSSHKeysAddNonInteractive(client *api.Client, cmd *cobra.Command) error {
	if sshKeyAddName == "" {
		return fmt.Errorf("--name is required")
	}

	var publicKey string

	if sshKeyAddKeyFile != "" && sshKeyAddKey != "" {
		return fmt.Errorf("provide either --key-file or --key, not both")
	}

	if sshKeyAddKeyFile != "" {
		data, err := os.ReadFile(sshKeyAddKeyFile)
		if err != nil {
			return fmt.Errorf("failed to read key file: %w", err)
		}
		publicKey = strings.TrimSpace(string(data))
	} else if sshKeyAddKey != "" {
		publicKey = strings.TrimSpace(sshKeyAddKey)
	} else {
		return fmt.Errorf("provide --key-file or --key with the public key")
	}

	var resp *api.SSHKeyAddResponse
	if err := tui.RunWithBusySpinner("Adding SSH key...", os.Stdout, func() error {
		var e error
		resp, e = client.AddSSHKeyToOrg(sshKeyAddName, publicKey)
		return e
	}); err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("operation", "ssh_key_add")
			sentry.CaptureException(err)
		})
		return fmt.Errorf("failed to add SSH key: %w", err)
	}

	PrintSuccessSimple(fmt.Sprintf("SSH key '%s' added (fingerprint: %s)", resp.Key.Name, resp.Key.Fingerprint))
	return nil
}

func runSSHKeysAddInteractive(client *api.Client) error {
	addConfig, err := tui.RunSSHKeyAddInteractive(client)
	if err != nil {
		if errors.Is(err, tui.ErrCancelled) {
			PrintWarningSimple("User cancelled add process")
			return nil
		}
		return err
	}

	var resp *api.SSHKeyAddResponse
	if err := tui.RunWithBusySpinner("Adding SSH key...", os.Stdout, func() error {
		var e error
		resp, e = client.AddSSHKeyToOrg(addConfig.Name, addConfig.PublicKey)
		return e
	}); err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("operation", "ssh_key_add")
			sentry.CaptureException(err)
		})
		return fmt.Errorf("failed to add SSH key: %w", err)
	}

	PrintSuccessSimple(fmt.Sprintf("SSH key '%s' added (fingerprint: %s)", resp.Key.Name, resp.Key.Fingerprint))
	return nil
}

// ── ssh-keys delete ─────────────────────────────────────────────────────────

var sshKeysDeleteCmd = &cobra.Command{
	Use:   "delete [key_name_or_id]",
	Short: "Delete an SSH key",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSSHKeysDelete(args)
	},
}

func runSSHKeysDelete(args []string) error {
	client, err := getAuthenticatedClient()
	if err != nil {
		return err
	}

	var keyID string
	var selectedKey *api.SSHKey

	if len(args) == 0 {
		// Interactive mode
		var keys api.SSHKeyListResponse
		if err := tui.RunWithBusySpinner("Fetching SSH keys...", os.Stdout, func() error {
			var e error
			keys, e = client.ListSSHKeys()
			return e
		}); err != nil {
			return fmt.Errorf("failed to fetch SSH keys: %w", err)
		}

		if len(keys) == 0 {
			PrintWarningSimple("No SSH keys found.")
			return nil
		}

		sort.Slice(keys, func(i, j int) bool {
			return keys[i].CreatedAt < keys[j].CreatedAt
		})

		selectedKey, err = tui.RunSSHKeyDeleteInteractive(client, keys)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled delete process")
				return nil
			}
			return err
		}
		keyID = selectedKey.ID
	} else {
		// Non-interactive mode
		keyNameOrID := args[0]

		var keys api.SSHKeyListResponse
		if err := tui.RunWithBusySpinner("Validating SSH key...", os.Stdout, func() error {
			var e error
			keys, e = client.ListSSHKeys()
			return e
		}); err != nil {
			return fmt.Errorf("failed to fetch SSH keys: %w", err)
		}

		for i := range keys {
			if strings.EqualFold(keys[i].Name, keyNameOrID) || keys[i].ID == keyNameOrID {
				selectedKey = &keys[i]
				break
			}
		}

		if selectedKey == nil {
			return fmt.Errorf("SSH key '%s' not found", keyNameOrID)
		}

		keyID = selectedKey.ID

		fmt.Println()
		fmt.Printf("About to delete SSH key: %s\n", selectedKey.Name)
		fmt.Printf("Fingerprint: %s\n", selectedKey.Fingerprint)
		fmt.Printf("Key Type: %s\n", selectedKey.KeyType)
		fmt.Println()
		fmt.Print("Are you sure you want to delete this key? (yes/no): ")

		var confirmation string
		fmt.Scanln(&confirmation)

		if confirmation != "yes" && confirmation != "y" {
			PrintWarningSimple("Deletion cancelled")
			return nil
		}
	}

	// Run deletion with progress
	successMsg, err := tui.RunSSHKeyDeleteProgress(client, keyID, selectedKey.Name)
	if err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("operation", "ssh_key_delete")
			sentry.CaptureException(err)
		})
		return fmt.Errorf("failed to delete SSH key: %w", err)
	}

	if successMsg != "" {
		PrintSuccessSimple(successMsg)
	}

	return nil
}
