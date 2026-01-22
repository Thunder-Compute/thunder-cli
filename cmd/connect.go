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
	"github.com/spf13/cobra"

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
	client       api.ConnectClient
	skipTTYCheck bool
	skipTUI      bool
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
	Run: func(cmd *cobra.Command, args []string) {
		var instanceID string
		if len(args) > 0 {
			instanceID = args[0]
		}

		if err := runConnect(instanceID, tunnelPorts, debugMode); err != nil {
			PrintError(err)
			os.Exit(1)
		}
	},
}

func init() {
	connectCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderConnectHelp(cmd)
	})

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
			if instances[i].ID == instanceID || instances[i].UUID == instanceID || instances[i].Name == instanceID {
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

		if foundInstance.IP == "" {
			return fmt.Errorf("instance '%s' has no IP address", instanceID)
		}

		instanceID = foundInstance.ID
	}

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

	hashChan := make(chan string, 1)
	hashErrChan := make(chan error, 1)

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

	// Fetch binary hash in background
	go func() {
		hash, err := client.GetLatestBinaryHashCtx(ctx)
		if err != nil {
			hashErrChan <- err
			return
		}
		hashChan <- hash
	}()

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
		if instances[i].ID == instanceID || instances[i].UUID == instanceID || instances[i].Name == instanceID {
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

	if instance.IP == "" {
		err := fmt.Errorf("instance %s has no IP address", instanceID)
		shutdownTUI()
		return err
	}

	port := instance.Port
	if port == 0 {
		port = 22
	}

	gpuCount := 1
	if instance.NumGPUs != "" {
		if count, err := strconv.Atoi(instance.NumGPUs); err == nil {
			gpuCount = count
		}
	}

	phaseTimings["instance_validation"] = time.Since(phase2Start)
	tui.SendPhaseUpdate(p, 1, tui.PhaseCompleted, fmt.Sprintf("Found: %s (%s)", instance.Name, instance.IP), phaseTimings["instance_validation"])

	phase3Start := time.Now()
	tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Checking SSH keys...", 0)

	keyFile := utils.GetKeyFile(instance.UUID)
	newKeyCreated := false
	if checkCancelled() {
		return nil
	}
	if !utils.KeyExists(instance.UUID) {
		tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Generating new SSH key...", 0)
		keyResp, err := client.AddSSHKeyCtx(ctx, instanceID)
		if checkCancelled() {
			return nil
		}
		if err != nil {
			shutdownTUI()
			return fmt.Errorf("failed to add SSH key: %w", err)
		}

		if err := utils.SavePrivateKey(instance.UUID, keyResp.Key); err != nil {
			shutdownTUI()
			return fmt.Errorf("failed to save private key: %w", err)
		}
		newKeyCreated = true
	}

	phaseTimings["ssh_key_management"] = time.Since(phase3Start)
	tui.SendPhaseComplete(p, 2, phaseTimings["ssh_key_management"])

	phase4Start := time.Now()
	tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Waiting for SSH service on %s:%d...", instance.IP, port), 0)

	if checkCancelled() {
		return nil
	}
	if err := utils.WaitForTCPPort(ctx, instance.IP, port, 120*time.Second); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		shutdownTUI()
		return fmt.Errorf("SSH service not available: %w", err)
	}

	if checkCancelled() {
		return nil
	}

	tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Connecting to %s:%d...", instance.IP, port), 0)

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

	// Use different connection strategies for new keys vs reconnections
	if newKeyCreated {
		// New key: expect auth failures while key propagates, use longer timeout
		sshClient, err = utils.RobustSSHConnectWithProgress(ctx, instance.IP, keyFile, port, 120, progressCallback)
	} else {
		// Reconnecting: enable persistent auth failure detection (detects deleted ~/.ssh quickly)
		sshConnectOpts := &utils.SSHConnectOptions{
			DetectPersistentAuthFailure: true,
		}
		sshClient, err = utils.RobustSSHConnectWithOptions(ctx, instance.IP, keyFile, port, 60, progressCallback, sshConnectOpts)
	}
	if checkCancelled() {
		return nil
	}

	// Handle persistent auth failure (likely deleted ~/.ssh on instance) or other auth errors
	needsKeyRegeneration := err != nil && !newKeyCreated && (errors.Is(err, utils.ErrPersistentAuthFailure) || utils.IsAuthError(err) || utils.IsKeyParseError(err))
	if needsKeyRegeneration {
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
			shutdownTUI()
			return fmt.Errorf("failed to generate new SSH key: %w", keyErr)
		}

		if saveErr := utils.SavePrivateKey(instance.UUID, keyResp.Key); saveErr != nil {
			shutdownTUI()
			return fmt.Errorf("failed to save new private key: %w", saveErr)
		}

		keyFile = utils.GetKeyFile(instance.UUID)

		tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Retrying connection with new key to %s:%d...", instance.IP, port), 0)

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
		sshClient, err = utils.RobustSSHConnectWithProgress(ctx, instance.IP, keyFile, port, 120, retryCallback)
		if checkCancelled() {
			return nil
		}
		if err != nil {
			shutdownTUI()
			return fmt.Errorf("failed to establish SSH connection after key regeneration: %w", err)
		}
	} else if err != nil {
		shutdownTUI()
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}

	phaseTimings["ssh_connection"] = time.Since(phase4Start)
	tui.SendPhaseComplete(p, 3, phaseTimings["ssh_connection"])

	phase5Start := time.Now()
	tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Setting up instance...", 0)

	if checkCancelled() {
		return nil
	}

	// Always ensure token is set first (idempotent, fast)
	_ = utils.EnsureToken(sshClient, config.Token)

	// Get binary hash (already fetched in background)
	var binaryHash string
	select {
	case hash := <-hashChan:
		binaryHash = hash
	case <-hashErrChan:
		binaryHash = ""
	case <-ctx.Done():
		if checkCancelled() {
			return nil
		}
	case <-time.After(2 * time.Second):
		binaryHash = ""
	}

	// For production mode, check active sessions first (like VSCode extension) to skip operations if needed
	var activeSessions int
	var existingConfig *utils.ThunderConfig
	var existingHash string
	var canEarlyReturn bool

	if instance.Mode == "production" {
		var checkErr error
		activeSessions, checkErr = utils.CheckActiveSessions(sshClient)
		if checkErr != nil {
			activeSessions = 0
		}

		if activeSessions > 1 {
			// Token already set above, just mark complete
			phaseTimings["instance_setup"] = time.Since(phase5Start)
			tui.SendPhaseComplete(p, 4, phaseTimings["instance_setup"])
			canEarlyReturn = true
		} else {
			// No active sessions - match VSCode extension: skip config/hash check, run cleanup (idempotent)
			if err := utils.RemoveThunderVirtualization(sshClient, config.Token); err != nil {
				shutdownTUI()
				return fmt.Errorf("failed to remove Thunder virtualization: %w", err)
			}
			phaseTimings["instance_setup"] = time.Since(phase5Start)
			tui.SendPhaseComplete(p, 4, phaseTimings["instance_setup"])
			canEarlyReturn = true

		}
	}

	// For prototyping mode, do full config/hash read in parallel
	if instance.Mode != "production" || !canEarlyReturn {
		// Clean up ld.so.preload early if binary is missing to prevent stderr pollution
		_ = utils.CleanupLdSoPreloadIfBinaryMissing(sshClient)

		type configResult struct {
			config *utils.ThunderConfig
			err    error
		}
		type instanceHashResult struct {
			hash string
			err  error
		}

		configChan := make(chan configResult, 1)
		instanceHashChan := make(chan instanceHashResult, 1)

		go func() {
			config, err := utils.GetThunderConfig(sshClient)
			configChan <- configResult{config: config, err: err}
		}()

		expectedHash := utils.NormalizeHash(binaryHash)
		isValidHash := expectedHash != "" && len(expectedHash) == 32 && utils.IsHexString(expectedHash)
		hashAlgorithm := utils.DetectHashAlgorithm(expectedHash)

		if isValidHash {
			go func() {
				hash, err := utils.GetInstanceBinaryHash(sshClient, hashAlgorithm)
				instanceHashChan <- instanceHashResult{hash: hash, err: err}
			}()
		} else {
			instanceHashChan <- instanceHashResult{hash: "", err: nil}
		}

		configRes := <-configChan
		hashRes := <-instanceHashChan

		if configRes.err == nil {
			existingConfig = configRes.config
		}

		if hashRes.err == nil {
			existingHash = hashRes.hash
		}

	}

	ranConfigurator := false

	// Early return if GPU config and hash match (token already set above)
	if !canEarlyReturn {
		if instance.Mode == "prototyping" && existingConfig != nil && existingConfig.DeviceID != "" {
			expectedHash := utils.NormalizeHash(binaryHash)
			isValidHash := expectedHash != "" && len(expectedHash) == 32 && utils.IsHexString(expectedHash)
			gpuTypeMatches := strings.EqualFold(existingConfig.GPUType, instance.GPUType)
			gpuCountMatches := existingConfig.GPUCount == gpuCount
			hashMatches := isValidHash && existingHash != "" && existingHash == expectedHash

			if gpuTypeMatches && gpuCountMatches && hashMatches {
				phaseTimings["instance_setup"] = time.Since(phase5Start)
				tui.SendPhaseComplete(p, 4, phaseTimings["instance_setup"])
				canEarlyReturn = true
				ranConfigurator = true
			}
		}
	}

	// Skip active sessions check if GPU config matches (ConfigureThunderVirtualization handles binary update)
	skipActiveSessionsCheck := canEarlyReturn
	if !canEarlyReturn && instance.Mode == "prototyping" && existingConfig != nil && existingConfig.DeviceID != "" {
		gpuTypeMatches := strings.EqualFold(existingConfig.GPUType, instance.GPUType)
		gpuCountMatches := existingConfig.GPUCount == gpuCount
		if gpuTypeMatches && gpuCountMatches {
			skipActiveSessionsCheck = true
		}
	}

	// For prototyping mode, handle active sessions check (token already set above)
	if instance.Mode == "prototyping" && !canEarlyReturn {
		if !skipActiveSessionsCheck {
			var checkErr error
			activeSessions, checkErr = utils.CheckActiveSessions(sshClient)
			if checkErr != nil {
				activeSessions = 0
			}
		} else {
			activeSessions = 0
		}
	} else if instance.Mode == "prototyping" {
		activeSessions = 0
	}

	if !canEarlyReturn {
		switch instance.Mode {
		case "production":
			tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Production mode detected, disabling Thunder virtualization...", 0)
			if err := utils.RemoveThunderVirtualization(sshClient, config.Token); err != nil {
				shutdownTUI()
				return fmt.Errorf("failed to remove Thunder virtualization: %w", err)
			}
		default:
			var deviceID string
			if existingConfig != nil && existingConfig.DeviceID != "" {
				deviceID = existingConfig.DeviceID
			} else {
				if newID, err := client.GetNextDeviceID(); err == nil {
					deviceID = newID
				}
			}

			switch {
			case activeSessions > 1:
				tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, fmt.Sprintf("Detected %d active SSH sessions, skipping binary update", activeSessions), 0)
			case deviceID == "":
				tui.SendPhaseUpdate(p, 4, tui.PhaseWarning, "Unable to determine device ID, skipping environment setup", 0)
			default:
				tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Updating Thunder binary and config if needed...", 0)
				if err := utils.ConfigureThunderVirtualization(sshClient, instanceID, deviceID, instance.GPUType, gpuCount, config.Token, binaryHash, existingConfig); err != nil {
					shutdownTUI()
					return fmt.Errorf("failed to configure Thunder virtualization: %w", err)
				}
				ranConfigurator = true
			}
		}
	}

	if checkCancelled() {
		return nil
	}

	if instance.Mode == "prototyping" && !ranConfigurator && binaryHash != "" {
		tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Checking Thunder binary version...", 0)
		expectedHash := utils.NormalizeHash(binaryHash)
		hashAlgo := utils.DetectHashAlgorithm(expectedHash)

		existingHash, hashErr := utils.GetInstanceBinaryHash(sshClient, hashAlgo)
		existingHashNormalized := utils.NormalizeHash(existingHash)

		if hashErr == nil && existingHashNormalized != "" && existingHashNormalized != expectedHash {
			tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Binary outdated, updating in background...", 0)
			deviceID := ""
			if existingConfig != nil && existingConfig.DeviceID != "" {
				deviceID = existingConfig.DeviceID
			}
			if deviceID != "" {
				_ = utils.TriggerBackgroundSetup(sshClient, instanceID, deviceID, instance.GPUType, gpuCount, config.Token)
			}
		}
	}

	if checkCancelled() {
		return nil
	}

	phaseTimings["instance_setup"] = time.Since(phase5Start)
	tui.SendPhaseComplete(p, 4, phaseTimings["instance_setup"])

	// Update SSH config for easy reconnection via `ssh tnr-{instance_id}`
	templatePorts := utils.GetTemplateOpenPorts(instance.Template)
	_ = utils.UpdateSSHConfig(instanceID, instance.IP, port, instance.UUID, tunnelPorts, templatePorts)

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

	sshArgs = append(sshArgs, fmt.Sprintf("ubuntu@%s", instance.IP))

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
