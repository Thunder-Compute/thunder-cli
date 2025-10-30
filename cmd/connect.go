/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	tunnelPorts []string
	debugMode   bool
)

// connectCmd represents the connect command
var connectCmd = &cobra.Command{
	Use:   "connect [instance_id]",
	Short: "Establish an SSH connection to a Thunder Compute instance",
	Long: `Connect to a Thunder Compute instance via SSH with automatic setup and configuration.

This command performs setup including:
- SSH key management
- GPU virtualization configuration
- Port forwarding
- SSH config updates

After initial setup, you can reconnect using: ssh tnr-{instance_id}

If no instance ID is provided, an interactive selection menu will be displayed.`,
	Run: func(cmd *cobra.Command, args []string) {
		var instanceID string

		if len(args) > 0 {
			instanceID = args[0]
		} else {
			config, err := LoadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: not authenticated. Please run 'tnr login' first\n")
				os.Exit(1)
			}

			if config.Token == "" {
				fmt.Fprintf(os.Stderr, "Error: no authentication token found. Please run 'tnr login'\n")
				os.Exit(1)
			}

			client := api.NewClient(config.Token)
			instances, err := client.ListInstances()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: failed to list instances: %v\n", err)
				os.Exit(1)
			}

			if len(instances) == 0 {
				fmt.Println("No instances found. Create an instance first using 'tnr create'")
				os.Exit(0)
			}

			var runningInstances []api.Instance
			for _, inst := range instances {
				if inst.Status == "RUNNING" {
					runningInstances = append(runningInstances, inst)
				}
			}

			if len(runningInstances) == 0 {
				fmt.Println("No running instances found.")
				fmt.Println("\nYou have the following instances:")
				for _, inst := range instances {
					fmt.Printf("  - %s (%s) - Status: %s\n", inst.Name, inst.UUID, inst.Status)
				}
				fmt.Println("\nStart an instance using 'tnr start <instance_id>' or create a new one with 'tnr create'")
				os.Exit(0)
			}

			var instanceList []string
			instanceMap := make(map[string]api.Instance)
			for _, inst := range runningInstances {
				displayName := fmt.Sprintf("%s (%s) - %s GPU: %s", inst.Name, inst.ID, inst.NumGPUs, inst.GPUType)
				instanceList = append(instanceList, displayName)
				instanceMap[displayName] = inst
			}

			selected, err := selectInstanceTUI(instanceList)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if selected == "" {
				fmt.Println("User cancelled instance connection")
				os.Exit(0)
			}

			selectedInst := instanceMap[selected]
			instanceID = selectedInst.ID
		}

		if err := runConnect(instanceID, tunnelPorts, debugMode); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	connectCmd.Flags().MarkHidden("debug")
}

func runConnect(instanceID string, tunnelPortsStr []string, debug bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	startTime := time.Now()
	phaseTimings := make(map[string]time.Duration)

	var tunnelPorts []int
	for _, portStr := range tunnelPortsStr {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port: %s", portStr)
		}
		tunnelPorts = append(tunnelPorts, port)
	}

	flowModel := tui.NewConnectFlowModel(instanceID)
	p := tea.NewProgram(
		flowModel,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	tuiDone := make(chan error, 1)
	cancelCtx, cancel := context.WithCancel(context.Background())
	var cancelledMu sync.Mutex
	cancelled := false
	printedCancel := false

	go func() {
		finalModel, err := p.Run()
		if err != nil {
			tuiDone <- err
		}
		close(tuiDone)
		if fm, ok := finalModel.(tui.ConnectFlowModel); ok && fm.Cancelled() {
			cancel()
			cancelledMu.Lock()
			cancelled = true
			cancelledMu.Unlock()
		}
	}()

	time.Sleep(50 * time.Millisecond)

	checkCancelled := func() bool {
		cancelledMu.Lock()
		isCancelled := cancelled
		alreadyPrinted := printedCancel
		if isCancelled && !alreadyPrinted {
			fmt.Println("User cancelled instance connection")
			printedCancel = true
			cancelledMu.Unlock()
			return true
		}
		cancelledMu.Unlock()
		select {
		case <-cancelCtx.Done():
			cancelledMu.Lock()
			if cancelled && !printedCancel {
				fmt.Println("User cancelled instance connection")
				printedCancel = true
			}
			cancelledMu.Unlock()
			return true
		default:
			return false
		}
	}

	if checkCancelled() {
		return nil
	}

	phase1Start := time.Now()
	tui.SendPhaseUpdate(p, 0, tui.PhaseInProgress, "Checking prerequisites...", 0)

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
	if err := utils.AcquireLock(instanceID); err != nil {
		tui.SendConnectError(p, err)
		<-tuiDone
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer utils.ReleaseLock(instanceID)

	phaseTimings["pre_connection"] = time.Since(phase1Start)
	tui.SendPhaseComplete(p, 0, phaseTimings["pre_connection"])

	phase2Start := time.Now()
	tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Validating instance...", 0)

	config, err := LoadConfig()
	if err != nil {
		tui.SendConnectError(p, err)
		<-tuiDone
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		err := fmt.Errorf("no authentication token found")
		tui.SendConnectError(p, err)
		<-tuiDone
		return err
	}

	if checkCancelled() {
		return nil
	}
	client := api.NewClient(config.Token)

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
	instances, err := client.ListInstancesWithIPUpdateCtx(cancelCtx)
	if checkCancelled() {
		return nil
	}
	if err != nil {
		tui.SendConnectError(p, err)
		<-tuiDone
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
		tui.SendConnectError(p, err)
		<-tuiDone
		return err
	}

	if instance.Status != "RUNNING" {
		err := fmt.Errorf("instance %s is not running (status: %s)", instanceID, instance.Status)
		tui.SendConnectError(p, err)
		<-tuiDone
		return err
	}

	if instance.IP == "" {
		err := fmt.Errorf("instance %s has no IP address", instanceID)
		tui.SendConnectError(p, err)
		<-tuiDone
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
			tui.SendConnectError(p, err)
			<-tuiDone
			return fmt.Errorf("failed to add SSH key: %w", err)
		}

		if err := utils.SavePrivateKey(instance.UUID, keyResp.Key); err != nil {
			tui.SendConnectError(p, err)
			<-tuiDone
			return fmt.Errorf("failed to save private key: %w", err)
		}
	}

	phaseTimings["ssh_key_management"] = time.Since(phase3Start)
	tui.SendPhaseComplete(p, 2, phaseTimings["ssh_key_management"])

	phase4Start := time.Now()
	tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Connecting to %s:%d...", instance.IP, port), 0)

	// Establish SSH connection with 2-minute timeout and retry logic
	if checkCancelled() {
		return nil
	}
	sshClient, err := utils.RobustSSHConnectCtx(cancelCtx, instance.IP, keyFile, port, 120)
	if checkCancelled() {
		return nil
	}
	if err != nil {
		tui.SendConnectError(p, err)
		<-tuiDone
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	defer sshClient.Close()

	phaseTimings["ssh_connection"] = time.Since(phase4Start)
	tui.SendPhaseComplete(p, 3, phaseTimings["ssh_connection"])

	phase5Start := time.Now()
	tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Setting up instance...", 0)

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

	// Update SSH config for easy reconnection via `ssh tnr-{instance_id}`
	templatePorts := utils.GetTemplateOpenPorts(instance.Template)
	_ = utils.UpdateSSHConfig(instanceID, instance.IP, port, instance.UUID, tunnelPorts, templatePorts)

	phaseTimings["instance_setup"] = time.Since(phase5Start)
	tui.SendPhaseComplete(p, 4, phaseTimings["instance_setup"])

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

	if debug {
		fmt.Println("\n=== Timing Breakdown ===")
		totalTime := time.Since(startTime)
		for phase, duration := range phaseTimings {
			percentage := float64(duration) / float64(totalTime) * 100
			fmt.Printf("%-25s: %10s (%5.1f%%)\n", phase, duration.Round(time.Millisecond), percentage)
		}
		fmt.Printf("%-25s: %10s\n", "Total", totalTime.Round(time.Millisecond))
		fmt.Println("========================")
		fmt.Println()
	}

	sshClient.Close()

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
	if runtime.GOOS != "windows" {
		homeDir, _ := os.UserHomeDir()
		controlPath := fmt.Sprintf("%s/.thunder/thunder-control-%%h-%%p-%%r", homeDir)
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

	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	err = sshCmd.Run()

	fmt.Println("\n⚡ Exiting Thunder instance ⚡")

	// Handle SSH exit codes (130 = Ctrl+C, 255 = connection closed)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			if exitCode != 0 && exitCode != 130 && exitCode != 255 {
				return fmt.Errorf("SSH session failed with exit code %d", exitCode)
			}
		}
	}

	return nil
}

func checkWindowsOpenSSH() error {
	cmd := exec.Command("ssh", "-V")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("OpenSSH not found. Please install OpenSSH on Windows")
	}
	return nil
}

func selectInstanceTUI(instanceList []string) (string, error) {
	return tui.RunConnect(instanceList)
}
