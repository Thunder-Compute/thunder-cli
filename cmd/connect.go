package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/utils"
	tea "github.com/charmbracelet/bubbletea"
	termx "github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
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

// connectCmd represents the connect command
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

// runConnectWithOptions is the internal implementation that accepts options for testing.
// If opts is nil, default options are used.
func runConnectWithOptions(instanceID string, tunnelPortsStr []string, debug bool, opts *connectOptions) error {
	var config *Config
	var err error

	configLoader := LoadConfig
	if opts != nil && opts.configLoader != nil {
		configLoader = opts.configLoader
	}

	config, err = configLoader()
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

	var client api.ConnectClient
	if opts != nil && opts.client != nil {
		client = opts.client
	} else {
		client = api.NewClient(config.Token, config.APIURL)
	}

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
	cancelCtx, cancel := context.WithCancel(context.Background())
	var wasCancelled bool

	go func() {
		finalModel, err := p.Run()
		if err != nil {
			tuiDone <- err
		}
		close(tuiDone)
		if fm, ok := finalModel.(tui.ConnectFlowModel); ok && fm.Cancelled() {
			wasCancelled = true
			cancel()
		}
	}()

	time.Sleep(50 * time.Millisecond)

	shutdownTUI := func() {
		cancel()
		stop()
		<-tuiDone
	}

	checkCancelled := func() bool {
		select {
		case <-cancelCtx.Done():
			return true
		case <-ctx.Done():
			cancel()
			return true
		default:
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

	// Fetch binary hash in background for potential virtualization setup
	go func() {
		hash, err := client.GetLatestBinaryHashCtx(cancelCtx)
		if err != nil {
			hashErrChan <- err
			return
		}
		hashChan <- hash
	}()

	if checkCancelled() {
		return nil
	}
	instances, err = client.ListInstancesWithIPUpdateCtx(cancelCtx)
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
		keyResp, err := client.AddSSHKeyCtx(cancelCtx, instanceID)
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

	controlPath := ""
	if runtime.GOOS != "windows" {
		if homeDir, err := os.UserHomeDir(); err == nil {
			controlPath = fmt.Sprintf("%s/.thunder/thunder-control-%%h-%%p-%%r", homeDir)
		}
	}

	phase4Start := time.Now()
	tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Connecting to %s:%d...", instance.IP, port), 0)

	needProvisioning := true
	var sshClient *utils.SSHClient

	if controlPath != "" && !newKeyCreated {
		setupComplete, checkErr := checkSetupCompleteViaSSH(cancelCtx, instance.IP, keyFile, port, controlPath)
		if checkErr == nil && setupComplete {
			needProvisioning = false
			tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Instance already configured, using ControlMaster connection", 0)
		} else if checkErr == nil && !setupComplete {
			_ = shutdownControlMaster(instance.IP, keyFile, port, controlPath)
		}
	}

	if !needProvisioning {
		go func() {
			select {
			case <-hashChan:
			case <-hashErrChan:
			case <-cancelCtx.Done():
			}
		}()
	}

	if needProvisioning {
		progressCallback := func(info utils.SSHRetryInfo) {
			switch info.Status {
			case utils.SSHStatusDialing:
				tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Waiting for instance to be ready...", 0)
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
		sshClient, err = utils.RobustSSHConnectWithProgress(cancelCtx, instance.IP, keyFile, port, 120, progressCallback)
		if checkCancelled() {
			return nil
		}

		if err != nil && (utils.IsAuthError(err) || utils.IsKeyParseError(err)) && !newKeyCreated {
			tui.SendPhaseUpdate(p, 3, tui.PhaseWarning, "SSH key not found on instance. This typically occurs when your node crashes due to OOM, low disk space, or other reasons.", 0)

			keyResp, keyErr := client.AddSSHKeyCtx(cancelCtx, instanceID)
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

			tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Retrying connection with new key to %s:%d...", instance.IP, port), 0)

			retryCallback := func(info utils.SSHRetryInfo) {
				switch info.Status {
				case utils.SSHStatusDialing:
					tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Waiting for instance to be ready...", 0)
				case utils.SSHStatusHandshake, utils.SSHStatusAuth:
					tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "Waiting for new key to propagate, this can take a minute...", 0)
				case utils.SSHStatusSuccess:
					tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, "SSH connection established", 0)
				}
			}

			if checkCancelled() {
				return nil
			}
			sshClient, err = utils.RobustSSHConnectWithProgress(cancelCtx, instance.IP, keyFile, port, 120, retryCallback)
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
	}

	phaseTimings["ssh_connection"] = time.Since(phase4Start)
	tui.SendPhaseComplete(p, 3, phaseTimings["ssh_connection"])

	if needProvisioning {
		phase5Start := time.Now()
		tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Setting up instance...", 0)

		setupComplete := utils.IsInstanceSetupComplete(sshClient)
		if !setupComplete {
			// Inject API token into instance environment
			tokenCmd := fmt.Sprintf("sed -i '/export TNR_API_TOKEN/d' /home/ubuntu/.bashrc && echo 'export TNR_API_TOKEN=%s' >> /home/ubuntu/.bashrc", config.Token)
			_, _ = utils.ExecuteSSHCommand(sshClient, tokenCmd)

			// Check if there are multiple active sessions (skip setup if others are connected)
			activeSessions, err := utils.CheckActiveSessions(sshClient)
			if err != nil {
				activeSessions = 0
			}

			// Get binary hash with timeout (may have been fetched in background)
			var binaryHash string
			select {
			case hash := <-hashChan:
				binaryHash = hash
			case <-hashErrChan:
				binaryHash = ""
			case <-cancelCtx.Done():
				if checkCancelled() {
					return nil
				}
			case <-time.After(10 * time.Second):
				binaryHash = ""
			}

			if checkCancelled() {
				return nil
			}

			// Only configure virtualization if we're the only/first session
			if activeSessions <= 1 {
				gpuCount := 1
				if instance.NumGPUs != "" {
					if count, err := strconv.Atoi(instance.NumGPUs); err == nil {
						gpuCount = count
					}
				}

				switch instance.Mode {
				case "prototyping":
					var deviceID string
					existingConfig, _ := utils.GetThunderConfig(sshClient)
					if existingConfig != nil && existingConfig.DeviceID != "" {
						deviceID = existingConfig.DeviceID
					} else if newID, err := client.GetNextDeviceID(); err == nil {
						deviceID = newID
					}

					if deviceID != "" {
						_ = utils.ConfigureThunderVirtualization(sshClient, instanceID, deviceID, instance.GPUType, gpuCount, config.Token, binaryHash)
					}
				case "production":
					_ = utils.RemoveThunderVirtualization(sshClient, config.Token)
				}
			}

			_ = utils.MarkInstanceSetupComplete(sshClient)

			phaseTimings["instance_setup"] = time.Since(phase5Start)
			tui.SendPhaseComplete(p, 4, phaseTimings["instance_setup"])
		} else {
			tui.SendPhaseSkipped(p, 4, "Instance already configured")
		}
	} else {
		tui.SendPhaseSkipped(p, 4, "")
	}

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

	// Build SSH command with port forwarding and connection multiplexing
	sshArgs := []string{
		"-q",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", keyFile,
		"-p", fmt.Sprintf("%d", port),
		"-t",
	}

	// Use ControlMaster for connection multiplexing (not supported on Windows)
	if controlPath != "" {
		sshArgs = append(sshArgs,
			"-o", "ControlMaster=auto",
			"-o", fmt.Sprintf("ControlPath=%s", controlPath),
			"-o", "ControlPersist=5m",
		)
	}

	// Merge user-specified tunnel ports with template open ports
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

	execCmd := exec.Command
	if opts != nil && opts.execCommand != nil {
		execCmd = opts.execCommand
	}

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

func checkSetupCompleteViaSSH(ctx context.Context, ip, keyFile string, port int, controlPath string) (bool, error) {
	if controlPath == "" {
		return false, fmt.Errorf("control path not configured")
	}

	args := []string{
		"-q",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "ControlMaster=auto",
		"-o", fmt.Sprintf("ControlPath=%s", controlPath),
		"-o", "ControlPersist=5m",
		"-i", keyFile,
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("ubuntu@%s", ip),
		fmt.Sprintf("test -f %s && echo yes || echo no", utils.ThunderSetupMarker),
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(output)) == "yes", nil
}

func shutdownControlMaster(ip, keyFile string, port int, controlPath string) error {
	if controlPath == "" {
		return nil
	}

	args := []string{
		"-S", controlPath,
		"-O", "exit",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", keyFile,
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("ubuntu@%s", ip),
	}

	cmd := exec.Command("ssh", args...)
	return cmd.Run()
}
