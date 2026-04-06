package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/getsentry/sentry-go"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

var (
	tunnelPorts []string
	debugMode   bool
)

// mocks for testing
type connectOptions struct {
	client        api.ConnectClient
	skipTTYCheck  bool
	skipTUI       bool
	sshConnector  func(ctx context.Context, ip, keyFile string, port, maxWait int) (sshClient, error)
	sessionRunner func(ctx context.Context, cfg utils.SessionConfig) error
	configLoader  func() (*Config, error)
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

func resolveSessionRunner(opts *connectOptions) func(ctx context.Context, cfg utils.SessionConfig) error {
	if opts != nil && opts.sessionRunner != nil {
		return opts.sessionRunner
	}
	return utils.RunInteractiveSession
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
		sessionRunner: utils.RunInteractiveSession,
		configLoader:  LoadConfig,
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
		return runConnect(instanceID, tunnelPorts, debugMode)
	},
}

func init() {
	connectCmd.SetHelpFunc(wrapHelp(helpmenus.RenderConnectHelp))

	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().StringSliceVarP(&tunnelPorts, "tunnel", "t", []string{}, "Port forwarding (can specify multiple times: -t 8080 -t 3000)")
	connectCmd.Flags().BoolVar(&debugMode, "debug", false, "Show detailed timing breakdown")
	_ = connectCmd.Flags().MarkHidden("debug") //nolint:errcheck // flag hiding failure is non-fatal
}

func runConnect(instanceID string, tunnelPortsStr []string, debug bool) error {
	return runConnectWithOptions(instanceID, tunnelPortsStr, debug, nil)
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
		return usageErr("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return usageErr("no authentication token found. Please run 'tnr login'")
	}

	skipTTYCheck := opts != nil && opts.skipTTYCheck
	interactive := (skipTTYCheck || tui.IsInteractive()) && !JSONOutput

	client := resolveConnectClient(opts, config.Token, config.APIURL)

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "fetching instances",
		Level:    sentry.LevelInfo,
	})

	var instances []api.Instance
	if err := tui.RunWithBusySpinner("Fetching instances...", os.Stdout, func() error {
		var e error
		instances, e = client.ListInstances()
		return e
	}); err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}
	if len(instances) == 0 {
		PrintWarningSimple("No instances found. Create an instance first using 'tnr create'")
		return nil
	}

	if instanceID == "" {
		if !interactive {
			return usageErr("instance ID required in non-interactive mode")
		}
		instanceID, err = tui.RunConnectSelectWithInstances(instances)
		if err != nil {
			if errors.Is(err, tui.ErrCancelled) {
				PrintWarningSimple("User cancelled instance connection")
				return nil
			}
			if errors.Is(err, tui.ErrNoRunningInstances) {
				PrintWarningSimple("No running instances found.")
				return nil
			}
			return err
		}
	} else {
		foundInstance := findInstance(instances, instanceID)

		if foundInstance == nil {
			return usageErr("instance '%s' not found", instanceID)
		}

		if foundInstance.Status != "RUNNING" {
			return usageErr("instance '%s' is not running (status: %s)", instanceID, foundInstance.Status)
		}

		if foundInstance.GetIP() == "" {
			return usageErr("instance '%s' has no IP address", instanceID)
		}

		instanceID = foundInstance.ID
	}

	instance := findInstance(instances, instanceID)

	port := instance.Port
	if port == 0 {
		port = 22
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
			return usageErr("invalid port: %s", portStr)
		}
		tunnelPorts = append(tunnelPorts, port)
	}

	// Non-interactive progress logging to stderr
	logProgress := func(msg string) {
		if !interactive {
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}
	}

	var p *tea.Program
	var tuiDone chan error
	var wasCancelled bool

	if interactive {
		tui.InitCommonStyles(os.Stdout)

		flowModel := tui.NewConnectFlowModel(instanceID)
		p = tea.NewProgram(
			flowModel,
			tea.WithContext(ctx),
			tea.WithOutput(os.Stdout),
		)

		tuiDone = make(chan error, 1)

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
	}

	shutdownTUI := func() {
		if p != nil {
			stop()
			tui.ShutdownProgram(p, tuiDone, os.Stdout)
		}
	}

	checkCancelled := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			if wasCancelled {
				stop()
				return true
			}
			return false
		}
	}

	if checkCancelled() {
		return nil
	}

	phase3Start := time.Now()
	logProgress("Checking SSH keys...")
	tui.SendPhaseUpdate(p, 0, tui.PhaseInProgress, "Checking SSH keys...", 0)

	keyFile := utils.GetKeyFile(instance.UUID)
	newKeyCreated := false
	keyExists := utils.KeyExists(instance.UUID)

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "checking SSH keys",
		Data: map[string]interface{}{
			"key_exists": keyExists,
			"key_file":   keyFile,
		},
		Level: sentry.LevelInfo,
	})

	if checkCancelled() {
		return nil
	}
	if !keyExists {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "generating new SSH key",
			Data: map[string]interface{}{
				"instance_id": instanceID,
			},
			Level: sentry.LevelInfo,
		})

		tui.SendPhaseUpdate(p, 0, tui.PhaseInProgress, "Generating new SSH key...", 0)
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
			if err := utils.SavePrivateKey(instance.UUID, *keyResp.Key); err != nil {
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

	phaseTimings["ssh_key_management"] = time.Since(phase3Start)
	tui.SendPhaseComplete(p, 0, phaseTimings["ssh_key_management"])

	// JSON mode: output connection info without starting SSH session
	if JSONOutput {
		shutdownTUI()
		type connectInfo struct {
			InstanceID string `json:"instance_id"`
			UUID       string `json:"uuid"`
			Name       string `json:"name"`
			IP         string `json:"ip"`
			Port       int    `json:"port"`
			KeyFile    string `json:"key_file"`
			SSHCommand string `json:"ssh_command"`
		}
		sshCmd := fmt.Sprintf("ssh -i %s root@%s -p %d", keyFile, instance.GetIP(), port)
		printJSON(connectInfo{
			InstanceID: instanceID,
			UUID:       instance.UUID,
			Name:       instance.Name,
			IP:         instance.GetIP(),
			Port:       port,
			KeyFile:    keyFile,
			SSHCommand: sshCmd,
		})
		return nil
	}

	phase4Start := time.Now()
	logProgress(fmt.Sprintf("Waiting for SSH service on %s:%d...", instance.GetIP(), port))
	tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, fmt.Sprintf("Waiting for SSH service on %s:%d...", instance.GetIP(), port), 0)

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

	logProgress(fmt.Sprintf("Connecting to %s:%d...", instance.GetIP(), port))
	tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, fmt.Sprintf("Connecting to %s:%d...", instance.GetIP(), port), 0)

	var sshClient *utils.SSHClient
	progressCallback := func(info utils.SSHRetryInfo) {
		switch info.Status {
		case utils.SSHStatusDialing:
			tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Establishing SSH connection...", 0)
		case utils.SSHStatusHandshake:
			if newKeyCreated {
				tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Setting up SSH, this can take a minute...", 0)
			} else {
				tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Retrying SSH connection...", 0)
			}
		case utils.SSHStatusAuth:
			if newKeyCreated {
				tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Waiting for key to propagate...", 0)
			} else {
				tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Authentication failed, retrying...", 0)
			}
		case utils.SSHStatusSuccess:
			tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "SSH connection established", 0)
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
			tui.SendPhaseUpdate(p, 1, tui.PhaseWarning, "SSH keys on instance appear to be missing. Reconfiguring access...", 0)
		} else {
			tui.SendPhaseUpdate(p, 1, tui.PhaseWarning, "SSH key not found on instance. This typically occurs when your node crashes due to OOM, low disk space, or other reasons.", 0)
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
			if saveErr := utils.SavePrivateKey(instance.UUID, *keyResp.Key); saveErr != nil {
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

		keyFile = utils.GetKeyFile(instance.UUID)
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "key regenerated, retrying connection",
			Level:    sentry.LevelInfo,
		})

		tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, fmt.Sprintf("Retrying connection with new key to %s:%d...", instance.GetIP(), port), 0)

		retryCallback := func(info utils.SSHRetryInfo) {
			switch info.Status {
			case utils.SSHStatusDialing:
				tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Establishing SSH connection...", 0)
			case utils.SSHStatusHandshake, utils.SSHStatusAuth:
				tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Waiting for new key to propagate, this can take a minute...", 0)
			case utils.SSHStatusSuccess:
				tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "SSH connection established", 0)
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
	tui.SendPhaseComplete(p, 1, phaseTimings["ssh_connection"])

	phase5Start := time.Now()
	logProgress("Setting up instance...")
	tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Setting up instance...", 0)

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

	tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Setting up token...", 0)
	if err := utils.SetupToken(sshClient, config.Token); err != nil {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "token setup failed",
			Data: map[string]interface{}{
				"error": err.Error(),
			},
			Level: sentry.LevelError,
		})
		shutdownTUI()
		return fmt.Errorf("failed to set up token: %w", err)
	}

	if checkCancelled() {
		return nil
	}

	phaseTimings["instance_setup"] = time.Since(phase5Start)
	tui.SendPhaseComplete(p, 2, phaseTimings["instance_setup"])

	// Update SSH config for easy reconnection via `ssh tnr-{instance_id}`
	templatePorts := utils.GetTemplateOpenPorts(instance.Template)
	if sshConfigErr := utils.UpdateSSHConfig(instanceID, instance.GetIP(), port, instance.UUID, tunnelPorts, templatePorts); sshConfigErr != nil {
		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category: "connect",
			Message:  "SSH config update failed",
			Data: map[string]interface{}{
				"error":       sshConfigErr.Error(),
				"instance_id": instanceID,
			},
			Level: sentry.LevelWarning,
		})
	}

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category: "connect",
		Message:  "connection setup complete",
		Data: map[string]interface{}{
			"instance_id":   instanceID,
			"tunnel_count":  len(tunnelPorts),
			"total_time_ms": time.Since(phase3Start).Milliseconds(),
		},
		Level: sentry.LevelInfo,
	})

	tui.SendConnectComplete(p)

	if checkCancelled() {
		return nil
	}

	if interactive {
		select {
		case err := <-tuiDone:
			if err != nil {
				if checkCancelled() {
					return nil
				}
				shutdownTUI()
				return fmt.Errorf("TUI error: %w", err)
			}
		default:
			if err := <-tuiDone; err != nil {
				if checkCancelled() {
					return nil
				}
				shutdownTUI()
				return fmt.Errorf("TUI error: %w", err)
			}
		}

		if checkCancelled() {
			return nil
		}
	}

	allPorts := make(map[int]bool)
	for _, p := range tunnelPorts {
		allPorts[p] = true
	}
	for _, p := range templatePorts {
		allPorts[p] = true
	}
	var portList []int
	for p := range allPorts {
		portList = append(portList, p)
	}

	sessionCfg := utils.SessionConfig{
		Client: sshClient,
		Ports:  portList,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	runner := resolveSessionRunner(opts)
	err = runner(ctx, sessionCfg)

	if sshClient != nil {
		sshClient.Close()
	}

	// Remote shell exit codes are not connect errors.
	if err != nil {
		var exitErr *ssh.ExitError
		if !errors.As(err, &exitErr) {
			return fmt.Errorf("SSH session failed: %w", err)
		}
	}

	if wasCancelled {
		PrintWarningSimple("User cancelled instance connection")
		return nil
	}

	return nil
}
