package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	termx "github.com/charmbracelet/x/term"
	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/internal/sshkeys"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

var (
	tunnelPorts        []string
	debugMode          bool
	connectSSHKeyName  string
)

// mocks for testing
type connectOptions struct {
	client       api.ConnectClient
	skipTTYCheck bool
	skipTUI      bool
	sshKeyName   string
	sshConnector func(ctx context.Context, ip, keyFile string, port, maxWait int) (sshClient, error)
	execCommand  func(name string, args ...string) *exec.Cmd
	configLoader func() (*Config, error)
}

type sshClient interface {
	Close() error
}

func resolveConnectClient(opts *connectOptions, token, baseURL string) api.ConnectClient {
	if opts != nil && opts.client != nil {
		return opts.client
	}
	return api.NewClient(token, baseURL)
}

func resolveExecCommand(opts *connectOptions) func(name string, args ...string) *exec.Cmd {
	if opts != nil && opts.execCommand != nil {
		return opts.execCommand
	}
	return exec.Command
}

func resolveConfigLoader(opts *connectOptions) func() (*Config, error) {
	if opts != nil && opts.configLoader != nil {
		return opts.configLoader
	}
	return LoadConfig
}

func defaultConnectOptions(token, baseURL string) *connectOptions {
	return &connectOptions{
		client:       api.NewClient(token, baseURL),
		skipTTYCheck: false,
		skipTUI:      false,
		sshConnector: func(ctx context.Context, ip, keyFile string, port, maxWait int) (sshClient, error) {
			return utils.RobustSSHConnectCtx(ctx, ip, keyFile, port, maxWait)
		},
		execCommand:  exec.Command,
		configLoader: LoadConfig,
	}
}

var connectCmd = &cobra.Command{
	Use:   "connect [instance_id]",
	Short: "Establish an SSH connection to a Thunder Compute instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		var instanceID string
		if len(args) > 0 {
			instanceID = args[0]
		}
		return runConnect(instanceID, tunnelPorts, debugMode, connectSSHKeyName)
	},
}

func init() {
	connectCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderConnectHelp(cmd)
	})

	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().StringSliceVarP(&tunnelPorts, "tunnel", "t", []string{}, "Port forwarding (can specify multiple times: -t 8080 -t 3000)")
	connectCmd.Flags().StringVar(&connectSSHKeyName, "ssh-key", "", "Name of a saved SSH key to use (see 'tnr ssh-keys list')")
	connectCmd.Flags().BoolVar(&debugMode, "debug", false, "Show detailed timing breakdown")
	_ = connectCmd.Flags().MarkHidden("debug") //nolint:errcheck // flag hiding failure is non-fatal
}

func runConnect(instanceID string, tunnelPortsStr []string, debug bool, sshKeyName string) error {
	return runConnectWithOptions(instanceID, tunnelPortsStr, debug, &connectOptions{sshKeyName: sshKeyName})
}

// runConnectWithOptions accepts options for testing. If opts is nil, default options are used.
func runConnectWithOptions(instanceID string, tunnelPortsStr []string, debug bool, opts *connectOptions) error {
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "starting connection",
		Data: map[string]interface{}{
			"instance_id": instanceID,
			"has_tunnels": len(tunnelPortsStr) > 0,
		},
		Level: sentry.LevelInfo,
	})

	configLoader := resolveConfigLoader(opts)
	config, err := configLoader()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	skipTTYCheck := opts != nil && opts.skipTTYCheck
	if !skipTTYCheck && !termx.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("error running connect TUI: not a TTY")
	}

	client := resolveConnectClient(opts, config.Token, config.APIURL)

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "fetching instances",
		Level:    sentry.LevelInfo,
	})

	busy := tui.NewBusyModel("Fetching instances...")
	bp := tea.NewProgram(busy, tea.WithOutput(os.Stdout))
	busyDone := make(chan struct{})
	go func() { _, _ = bp.Run(); close(busyDone) }()

	instances, err := client.ListInstances()
	bp.Send(tui.BusyDoneMsg{})
	<-busyDone
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}
	if len(instances) == 0 {
		PrintWarningSimple("No instances found. Create an instance first using 'tnr create'")
		return nil
	}

	if instanceID == "" {
		instanceID, err = tui.RunConnectSelectWithInstances(instances)
		if err != nil {
			if _, ok := err.(*tui.CancellationError); ok {
				PrintWarningSimple("User cancelled instance connection")
				return nil
			}
			if err.Error() == "no running instances" {
				PrintWarningSimple("No running instances found.")
				return nil
			}
			return err
		}
	} else {
		var foundInstance *api.Instance
		for i := range instances {
			if instances[i].ID == instanceID || instances[i].Uuid == instanceID || instances[i].Name == instanceID {
				foundInstance = &instances[i]
				break
			}
		}

		if foundInstance == nil {
			return fmt.Errorf("instance '%s' not found", instanceID)
		}

		if foundInstance.Status != "RUNNING" {
			return fmt.Errorf("instance '%s' is not running (status: %s)", instanceID, foundInstance.Status)
		}

		if foundInstance.GetIP() == "" {
			return fmt.Errorf("instance '%s' has no IP address", instanceID)
		}

		instanceID = foundInstance.ID
	}

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "instance selected",
		Data: map[string]interface{}{
			"instance_id": instanceID,
		},
		Level: sentry.LevelInfo,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	phaseTimings := make(map[string]time.Duration)

	var tunnelPorts []int
	for _, portStr := range tunnelPortsStr {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port: %s", portStr)
		}
		tunnelPorts = append(tunnelPorts, port)
	}

	tui.InitCommonStyles(os.Stdout)

	flowModel := tui.NewConnectFlowModel(instanceID)
	p := tea.NewProgram(
		flowModel,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	tuiDone := make(chan error, 1)
	var wasCancelled bool

	go func() {
		finalModel, err := p.Run()
		if fm, ok := finalModel.(tui.ConnectFlowModel); ok && fm.Cancelled() {
			wasCancelled = true
		}
		if err != nil {
			tuiDone <- err
		}
		close(tuiDone)
	}()

	time.Sleep(50 * time.Millisecond)

	shutdownTUI := func() {
		stop()
		// Don't block on tuiDone if context is already cancelled
		select {
		case <-tuiDone:
		case <-ctx.Done():
			// Context cancelled, don't wait for TUI
		case <-time.After(100 * time.Millisecond):
			// Short timeout to avoid hanging
		}
	}

	checkCancelled := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			// Also check if TUI was cancelled
			if wasCancelled {
				stop() // Cancel context when TUI is cancelled
				return true
			}
			return false
		}
	}

	if checkCancelled() {
		return nil
	}

	phase1Start := time.Now()
	tui.SendPhaseUpdate(p, 0, tui.PhaseInProgress, "Fetching instances...", 0)

	if runtime.GOOS == "windows" {
		if err := checkWindowsOpenSSH(); err != nil {
			return err
		}
	}

	if checkCancelled() {
		return nil
	}

	phaseTimings["pre_connection"] = time.Since(phase1Start)
	tui.SendPhaseComplete(p, 0, phaseTimings["pre_connection"])

	phase2Start := time.Now()
	tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Validating instance...", 0)

	if checkCancelled() {
		return nil
	}

	if checkCancelled() {
		return nil
	}
	instances, err = client.ListInstancesWithIPUpdateCtx(ctx)
	if checkCancelled() {
		return nil
	}
	if err != nil {
		shutdownTUI()
		return fmt.Errorf("failed to list instances: %w", err)
	}

	var instance *api.Instance
	for i := range instances {
		if instances[i].ID == instanceID || instances[i].Uuid == instanceID || instances[i].Name == instanceID {
			instance = &instances[i]
			break
		}
	}

	if instance == nil {
		err := fmt.Errorf("instance %s not found", instanceID)
		shutdownTUI()
		return err
	}

	if instance.Status != "RUNNING" {
		err := fmt.Errorf("instance %s is not running (status: %s)", instanceID, instance.Status)
		shutdownTUI()
		return err
	}

	if instance.GetIP() == "" {
		err := fmt.Errorf("instance %s has no IP address", instanceID)
		shutdownTUI()
		return err
	}

	port := instance.Port
	if port == 0 {
		port = 22
	}

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "instance validated",
		Data: map[string]interface{}{
			"instance_id":   instanceID,
			"instance_name": instance.Name,
			"instance_ip":   instance.GetIP(),
			"instance_port": port,
			"instance_mode": instance.Mode,
		},
		Level: sentry.LevelInfo,
	})

	phaseTimings["instance_validation"] = time.Since(phase2Start)
	tui.SendPhaseUpdate(p, 1, tui.PhaseCompleted, fmt.Sprintf("Found: %s (%s)", instance.Name, instance.GetIP()), phaseTimings["instance_validation"])

	phase3Start := time.Now()
	tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Checking SSH keys...", 0)

	sshKeyName := ""
	if opts != nil {
		sshKeyName = opts.sshKeyName
	}

	keyFile := utils.GetKeyFile(instance.Uuid)
	newKeyCreated := false
	usingSavedKey := sshKeyName != ""

	if checkCancelled() {
		return nil
	}

	if usingSavedKey {
		// Saved key path: look up key by name, find local private key, push public key to instance
		tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Looking up saved SSH key...", 0)

		keys, err := client.ListSSHKeys()
		if err != nil {
			shutdownTUI()
			return fmt.Errorf("failed to fetch SSH keys: %w", err)
		}

		var matchedKey *api.SSHKey
		for i := range keys {
			if strings.EqualFold(keys[i].Name, sshKeyName) {
				matchedKey = &keys[i]
				break
			}
		}

		if matchedKey == nil {
			shutdownTUI()
			return fmt.Errorf("SSH key '%s' not found. Run 'tnr ssh-keys list' to see available keys", sshKeyName)
		}

		tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Finding local private key...", 0)

		localPrivateKeyPath, err := sshkeys.FindPrivateKeyForPublicKey(matchedKey.PublicKey)
		if err != nil {
			shutdownTUI()
			return fmt.Errorf("failed to find local private key for '%s': %w", sshKeyName, err)
		}

		tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Pushing key to instance...", 0)

		if _, err := client.AddSSHKeyToInstanceWithPublicKey(instance.Uuid, matchedKey.PublicKey); err != nil {
			shutdownTUI()
			return fmt.Errorf("failed to add key to instance: %w", err)
		}

		keyFile = localPrivateKeyPath
		newKeyCreated = true // Use longer timeout for key propagation

		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "saved SSH key attached to instance",
			Data: map[string]interface{}{
				"key_name":  sshKeyName,
				"key_file":  localPrivateKeyPath,
			},
			Level: sentry.LevelInfo,
		})
	} else {
		// Default path: auto-generate key if none exists locally
		keyExists := utils.KeyExists(instance.Uuid)

		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "checking SSH keys",
			Data: map[string]interface{}{
				"key_exists": keyExists,
				"key_file":   keyFile,
			},
			Level: sentry.LevelInfo,
		})

		if !keyExists {
			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "connect",
				Message:  "generating new SSH key",
				Data: map[string]interface{}{
					"instance_id": instanceID,
				},
				Level: sentry.LevelInfo,
			})

			tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Generating new SSH key...", 0)
			keyResp, err := client.AddSSHKeyCtx(ctx, instanceID)
			if checkCancelled() {
				return nil
			}
			if err != nil {
				sentry.AddBreadcrumb(&sentry.Breadcrumb{
					Category: "connect",
					Message:  "SSH key generation failed",
					Data: map[string]interface{}{
						"error": err.Error(),
					},
					Level: sentry.LevelError,
				})
				shutdownTUI()
				return fmt.Errorf("failed to add SSH key: %w", err)
			}

			if keyResp.Key != nil {
				if err := utils.SavePrivateKey(instance.Uuid, *keyResp.Key); err != nil {
					sentry.AddBreadcrumb(&sentry.Breadcrumb{
						Category: "connect",
						Message:  "SSH key save failed",
						Data: map[string]interface{}{
							"error": err.Error(),
						},
						Level: sentry.LevelError,
					})
					shutdownTUI()
					return fmt.Errorf("failed to save private key: %w", err)
				}
			}
			newKeyCreated = true
			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "connect",
				Message:  "SSH key created successfully",
				Level:    sentry.LevelInfo,
			})
		}
	}

	phaseTimings["ssh_key_management"] = time.Since(phase3Start)
	tui.SendPhaseComplete(p, 2, phaseTimings["ssh_key_management"])

	phase4Start := time.Now()
	tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Waiting for SSH service on %s:%d...", instance.GetIP(), port), 0)

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "waiting for SSH port",
		Data: map[string]interface{}{
			"ip":   instance.GetIP(),
			"port": port,
		},
		Level: sentry.LevelInfo,
	})

	if checkCancelled() {
		return nil
	}
	if err := utils.WaitForTCPPort(ctx, instance.GetIP(), port, 120*time.Second); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "SSH port not available",
			Data: map[string]interface{}{
				"ip":    instance.GetIP(),
				"port":  port,
				"error": err.Error(),
			},
			Level: sentry.LevelError,
		})
		shutdownTUI()
		return fmt.Errorf("SSH service not available: %w", err)
	}

	if checkCancelled() {
		return nil
	}

	tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Connecting to %s:%d...", instance.GetIP(), port), 0)

	var sshClient *utils.SSHClient
	progressCallback := func(info utils.SSHRetryInfo) {
		switch info.Status {
		case utils.SSHStatusDialing:
			tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Establishing SSH connection...", 0)
		case utils.SSHStatusHandshake:
			if newKeyCreated {
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Setting up SSH, this can take a minute...", 0)
			} else {
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Retrying SSH connection...", 0)
			}
		case utils.SSHStatusAuth:
			if newKeyCreated {
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Waiting for key to propagate...", 0)
			} else {
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Authentication failed, retrying...", 0)
			}
		case utils.SSHStatusSuccess:
			tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "SSH connection established", 0)
		}
	}

	if checkCancelled() {
		return nil
	}

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "establishing SSH connection",
		Data: map[string]interface{}{
			"ip":              instance.GetIP(),
			"port":            port,
			"new_key_created": newKeyCreated,
		},
		Level: sentry.LevelInfo,
	})

	// Use different connection strategies for new keys vs reconnections
	if newKeyCreated {
		// New key: expect auth failures while key propagates, use longer timeout
		sshClient, err = utils.RobustSSHConnectWithProgress(ctx, instance.GetIP(), keyFile, port, 120, progressCallback)
	} else {
		// Reconnecting: enable persistent auth failure detection (detects deleted ~/.ssh quickly)
		sshConnectOpts := &utils.SSHConnectOptions{
			DetectPersistentAuthFailure: true,
		}
		sshClient, err = utils.RobustSSHConnectWithOptions(ctx, instance.GetIP(), keyFile, port, 60, progressCallback, sshConnectOpts)
	}
	if checkCancelled() {
		return nil
	}

	// Handle persistent auth failure (likely deleted ~/.ssh on instance) or other auth errors
	// When using a saved key, don't auto-regenerate on auth failure
	if err != nil && usingSavedKey {
		shutdownTUI()
		return fmt.Errorf("SSH authentication failed with saved key '%s'. Verify the private key in ~/.ssh/ is correct", sshKeyName)
	}

	needsKeyRegeneration := err != nil && !newKeyCreated && (errors.Is(err, utils.ErrPersistentAuthFailure) || utils.IsAuthError(err) || utils.IsKeyParseError(err))
	if needsKeyRegeneration {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "SSH auth failed, regenerating key",
			Data: map[string]interface{}{
				"error":                   err.Error(),
				"is_persistent_auth_fail": errors.Is(err, utils.ErrPersistentAuthFailure),
				"is_auth_error":           utils.IsAuthError(err),
				"is_key_parse_error":      utils.IsKeyParseError(err),
			},
			Level: sentry.LevelWarning,
		})

		if errors.Is(err, utils.ErrPersistentAuthFailure) {
			tui.SendPhaseUpdate(p, 3, tui.PhaseWarning, "SSH keys on instance appear to be missing. Reconfiguring access...", 0)
		} else {
			tui.SendPhaseUpdate(p, 3, tui.PhaseWarning, "SSH key not found on instance. This typically occurs when your node crashes due to OOM, low disk space, or other reasons.", 0)
		}

		keyResp, keyErr := client.AddSSHKeyCtx(ctx, instanceID)
		if checkCancelled() {
			return nil
		}
		if keyErr != nil {
			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "connect",
				Message:  "key regeneration failed",
				Data: map[string]interface{}{
					"error": keyErr.Error(),
				},
				Level: sentry.LevelError,
			})
			shutdownTUI()
			return fmt.Errorf("failed to generate new SSH key: %w", keyErr)
		}

		if keyResp.Key != nil {
			if saveErr := utils.SavePrivateKey(instance.Uuid, *keyResp.Key); saveErr != nil {
				sentry.AddBreadcrumb(&sentry.Breadcrumb{
					Category: "connect",
					Message:  "key save failed after regeneration",
					Data: map[string]interface{}{
						"error": saveErr.Error(),
					},
					Level: sentry.LevelError,
				})
				shutdownTUI()
				return fmt.Errorf("failed to save new private key: %w", saveErr)
			}
		}

		keyFile = utils.GetKeyFile(instance.Uuid)
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "key regenerated, retrying connection",
			Level:    sentry.LevelInfo,
		})

		tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Retrying connection with new key to %s:%d...", instance.GetIP(), port), 0)

		retryCallback := func(info utils.SSHRetryInfo) {
			switch info.Status {
			case utils.SSHStatusDialing:
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Establishing SSH connection...", 0)
			case utils.SSHStatusHandshake, utils.SSHStatusAuth:
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Waiting for new key to propagate, this can take a minute...", 0)
			case utils.SSHStatusSuccess:
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "SSH connection established", 0)
			}
		}

		if checkCancelled() {
			return nil
		}
		sshClient, err = utils.RobustSSHConnectWithProgress(ctx, instance.GetIP(), keyFile, port, 120, retryCallback)
		if checkCancelled() {
			return nil
		}
		if err != nil {
			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "connect",
				Message:  "SSH connection failed after key regeneration",
				Data: map[string]interface{}{
					"error": err.Error(),
				},
				Level: sentry.LevelError,
			})
			shutdownTUI()
			return fmt.Errorf("failed to establish SSH connection after key regeneration: %w", err)
		}
	} else if err != nil {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "SSH connection failed",
			Data: map[string]interface{}{
				"error":         err.Error(),
				"error_type":    string(utils.ClassifySSHError(err)),
				"is_auth_error": utils.IsAuthError(err),
			},
			Level: sentry.LevelError,
		})
		shutdownTUI()
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "SSH connection established",
		Data: map[string]interface{}{
			"duration_ms": time.Since(phase4Start).Milliseconds(),
		},
		Level: sentry.LevelInfo,
	})

	phaseTimings["ssh_connection"] = time.Since(phase4Start)
	tui.SendPhaseComplete(p, 3, phaseTimings["ssh_connection"])

	phase5Start := time.Now()
	tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Setting up instance...", 0)

	if checkCancelled() {
		return nil
	}

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "setting up token",
		Data: map[string]interface{}{
			"mode": instance.Mode,
		},
		Level: sentry.LevelInfo,
	})

	// Set up token on the instance (binary is now managed by the instance itself)
	if instance.Mode == "production" {
		tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Production mode detected, setting up token...", 0)
		if err := utils.RemoveThunderVirtualization(sshClient, config.Token); err != nil {
			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "connect",
				Message:  "token setup failed (production)",
				Data: map[string]interface{}{
					"error": err.Error(),
				},
				Level: sentry.LevelError,
			})
			shutdownTUI()
			return fmt.Errorf("failed to set up token: %w", err)
		}
	} else {
		tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Setting up token...", 0)
		if err := utils.SetupToken(sshClient, config.Token); err != nil {
			sentry.AddBreadcrumb(&sentry.Breadcrumb{
				Category: "connect",
				Message:  "token setup failed (prototyping)",
				Data: map[string]interface{}{
					"error": err.Error(),
				},
				Level: sentry.LevelError,
			})
			shutdownTUI()
			return fmt.Errorf("failed to set up token: %w", err)
		}
	}

	if checkCancelled() {
		return nil
	}

	phaseTimings["instance_setup"] = time.Since(phase5Start)
	tui.SendPhaseComplete(p, 4, phaseTimings["instance_setup"])

	// Update SSH config for easy reconnection via `ssh tnr-{instance_id}`
	templatePorts := utils.GetTemplateOpenPorts(instance.Template)
	_ = utils.UpdateSSHConfig(instanceID, instance.GetIP(), port, instance.Uuid, tunnelPorts, templatePorts)

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "connection setup complete",
		Data: map[string]interface{}{
			"instance_id":   instanceID,
			"tunnel_count":  len(tunnelPorts),
			"total_time_ms": time.Since(phase1Start).Milliseconds(),
		},
		Level: sentry.LevelInfo,
	})

	tui.SendConnectComplete(p)

	if checkCancelled() {
		return nil
	}

	select {
	case err := <-tuiDone:
		if err != nil {
			if checkCancelled() {
				return nil
			}
			return fmt.Errorf("TUI error: %w", err)
		}
	default:
		if err := <-tuiDone; err != nil {
			if checkCancelled() {
				return nil
			}
			return fmt.Errorf("TUI error: %w", err)
		}
	}

	if checkCancelled() {
		return nil
	}

	if sshClient != nil {
		sshClient.Close()
	}

	sshArgs := []string{
		"-q",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", keyFile,
		"-p", fmt.Sprintf("%d", port),
		"-t",
	}

	allPorts := make(map[int]bool)
	for _, p := range tunnelPorts {
		allPorts[p] = true
	}
	for _, p := range templatePorts {
		allPorts[p] = true
	}

	for port := range allPorts {
		sshArgs = append(sshArgs, "-L", fmt.Sprintf("%d:localhost:%d", port, port))
	}

	sshArgs = append(sshArgs, fmt.Sprintf("ubuntu@%s", instance.GetIP()))

	execCmd := resolveExecCommand(opts)
	sshCmd := execCmd("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	err = sshCmd.Run()
	// Handle SSH exit codes (130 = Ctrl+C, 255 = connection closed)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			if exitCode != 0 && exitCode != 130 && exitCode != 255 {
				return fmt.Errorf("SSH session failed with exit code %d", exitCode)
			}
		}
	}

	if wasCancelled {
		PrintWarningSimple("User cancelled instance connection")
		return nil
	}

	return nil
}

func checkWindowsOpenSSH() error {
	if _, err := exec.LookPath("ssh"); err == nil {
		return nil
	}

	fmt.Println("OpenSSH client not found. Attempting to install...")

	// Try auto-install via PowerShell (requires admin privileges)
	installCmd := exec.Command("powershell", "-Command",
		"Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0")
	installOutput, installErr := installCmd.CombinedOutput()

	if installErr == nil {
		if _, err := exec.LookPath("ssh"); err == nil {
			fmt.Println("OpenSSH client installed successfully!")
			return nil
		}
		// ssh still not in PATH after install - likely needs terminal restart
		fmt.Println("OpenSSH installation completed. Please restart your terminal and try again.")
		return fmt.Errorf("OpenSSH installed but not yet available. Please restart your terminal")
	}

	errDetails := ""
	if len(installOutput) > 0 {
		errDetails = string(installOutput)
	}

	return fmt.Errorf(`OpenSSH client not found and automatic installation failed.

To install OpenSSH manually, choose one of these options:

Option 1: Run PowerShell as Administrator and execute:
  Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0

Option 2: Install via Windows Settings:
  1. Open Settings > Apps > Optional Features
  2. Click "Add a feature"
  3. Search for "OpenSSH Client" and install it

Option 3: Install via winget:
  winget install Microsoft.OpenSSH.Client

After installation, restart your terminal and try again.

%s`, errDetails)
}
