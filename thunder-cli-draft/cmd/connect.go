/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joshuawatkins04/thunder-cli-draft/api"
	"github.com/joshuawatkins04/thunder-cli-draft/tui"
	"github.com/joshuawatkins04/thunder-cli-draft/utils"
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

This command performs sophisticated setup including:
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
				fmt.Println("Connection cancelled.")
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
	rootCmd.AddCommand(connectCmd)
	connectCmd.Flags().StringSliceVarP(&tunnelPorts, "tunnel", "t", []string{}, "Port forwarding (can specify multiple times: -t 8080 -t 3000)")
	connectCmd.Flags().BoolVar(&debugMode, "debug", false, "Show detailed timing breakdown")
	connectCmd.Flags().MarkHidden("debug")
}

func runConnect(instanceID string, tunnelPortsStr []string, debug bool) error {
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

	// Initialize TUI
	flowModel := tui.NewConnectFlowModel(instanceID)
	p := tea.NewProgram(flowModel)

	// Run TUI in background
	go func() {
		if _, err := p.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		}
	}()

	// Give TUI time to initialize
	time.Sleep(50 * time.Millisecond)

	// Phase 1: Pre-Connection Setup
	phase1Start := time.Now()
	tui.SendPhaseUpdate(p, 0, tui.PhaseInProgress, "Checking prerequisites...", 0)

	hashChan := make(chan string, 1)
	hashErrChan := make(chan error, 1)

	if runtime.GOOS == "windows" {
		if err := checkWindowsOpenSSH(); err != nil {
			return err
		}
	}

	if err := utils.AcquireLock(instanceID); err != nil {
		tui.SendConnectError(p, err)
		time.Sleep(100 * time.Millisecond)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer utils.ReleaseLock(instanceID)

	phaseTimings["pre_connection"] = time.Since(phase1Start)
	tui.SendPhaseComplete(p, 0, phaseTimings["pre_connection"])

	// Phase 2: Instance Validation
	phase2Start := time.Now()
	tui.SendPhaseUpdate(p, 1, tui.PhaseInProgress, "Validating instance...", 0)

	config, err := LoadConfig()
	if err != nil {
		tui.SendConnectError(p, err)
		time.Sleep(100 * time.Millisecond)
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		err := fmt.Errorf("no authentication token found")
		tui.SendConnectError(p, err)
		time.Sleep(100 * time.Millisecond)
		return err
	}

	client := api.NewClient(config.Token)

	go func() {
		hash, err := client.GetLatestBinaryHash()
		if err != nil {
			hashErrChan <- err
			return
		}
		hashChan <- hash
	}()

	instances, err := client.ListInstancesWithIPUpdate()
	if err != nil {
		tui.SendConnectError(p, err)
		time.Sleep(100 * time.Millisecond)
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
		time.Sleep(100 * time.Millisecond)
		return err
	}

	if instance.Status != "RUNNING" {
		err := fmt.Errorf("instance %s is not running (status: %s)", instanceID, instance.Status)
		tui.SendConnectError(p, err)
		time.Sleep(100 * time.Millisecond)
		return err
	}

	if instance.IP == "" {
		err := fmt.Errorf("instance %s has no IP address", instanceID)
		tui.SendConnectError(p, err)
		time.Sleep(100 * time.Millisecond)
		return err
	}

	port := instance.Port
	if port == 0 {
		port = 22
	}

	phaseTimings["instance_validation"] = time.Since(phase2Start)
	tui.SendPhaseUpdate(p, 1, tui.PhaseCompleted, fmt.Sprintf("Found: %s (%s)", instance.Name, instance.IP), phaseTimings["instance_validation"])

	// Phase 3: SSH Key Management
	phase3Start := time.Now()
	tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Checking SSH keys...", 0)

	keyFile := utils.GetKeyFile(instance.UUID)
	if !utils.KeyExists(instance.UUID) {
		tui.SendPhaseUpdate(p, 2, tui.PhaseInProgress, "Generating new SSH key...", 0)
		keyResp, err := client.AddSSHKey(instanceID)
		if err != nil {
			tui.SendConnectError(p, err)
			time.Sleep(100 * time.Millisecond)
			return fmt.Errorf("failed to add SSH key: %w", err)
		}

		if err := utils.SavePrivateKey(instance.UUID, keyResp.Key); err != nil {
			tui.SendConnectError(p, err)
			time.Sleep(100 * time.Millisecond)
			return fmt.Errorf("failed to save private key: %w", err)
		}
	}

	phaseTimings["ssh_key_management"] = time.Since(phase3Start)
	tui.SendPhaseComplete(p, 2, phaseTimings["ssh_key_management"])

	// Phase 4: Robust SSH Connection
	phase4Start := time.Now()
	tui.SendPhaseUpdate(p, 3, tui.PhaseInProgress, fmt.Sprintf("Connecting to %s:%d...", instance.IP, port), 0)

	sshClient, err := utils.RobustSSHConnect(instance.IP, keyFile, port, 120)
	if err != nil {
		tui.SendConnectError(p, err)
		time.Sleep(100 * time.Millisecond)
		return fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	defer sshClient.Close()

	phaseTimings["ssh_connection"] = time.Since(phase4Start)
	tui.SendPhaseComplete(p, 3, phaseTimings["ssh_connection"])

	// Phase 5: Environment Setup
	phase5Start := time.Now()
	tui.SendPhaseUpdate(p, 4, tui.PhaseInProgress, "Configuring environment...", 0)

	tokenCmd := fmt.Sprintf("sed -i '/export TNR_API_TOKEN/d' /home/ubuntu/.bashrc && echo 'export TNR_API_TOKEN=%s' >> /home/ubuntu/.bashrc", config.Token)
	if _, err := utils.ExecuteSSHCommand(sshClient, tokenCmd); err != nil {
		tui.SendPhaseUpdate(p, 4, tui.PhaseWarning, "Token injection warning", time.Since(phase5Start))
	} else {
		phaseTimings["environment_setup"] = time.Since(phase5Start)
		tui.SendPhaseComplete(p, 4, phaseTimings["environment_setup"])
	}

	// Phase 6: Thunder Virtualization Configuration
	phase6Start := time.Now()
	tui.SendPhaseUpdate(p, 5, tui.PhaseInProgress, "Configuring GPU virtualization...", 0)

	activeSessions, err := utils.CheckActiveSessions(sshClient)
	if err != nil {
		fmt.Printf("Warning: failed to check active sessions: %v\n", err)
		activeSessions = 0
	}

	var binaryHash string
	select {
	case hash := <-hashChan:
		binaryHash = hash
	case <-hashErrChan:
		binaryHash = ""
	case <-time.After(10 * time.Second):
		binaryHash = ""
	}

	if activeSessions <= 1 {
		gpuCount := 1
		if instance.NumGPUs != "" {
			if count, err := strconv.Atoi(instance.NumGPUs); err == nil {
				gpuCount = count
			}
		}

		if instance.Mode == "prototyping" {
			deviceID := ""
			existingConfig, _ := utils.GetThunderConfig(sshClient)
			if existingConfig != nil && existingConfig.DeviceID != "" {
				deviceID = existingConfig.DeviceID
			} else {
				deviceID, err = client.GetNextDeviceID()
				if err != nil {
					fmt.Printf("Warning: failed to get device ID: %v\n", err)
				}
			}

			if deviceID != "" {
				if err := utils.ConfigureThunderVirtualization(sshClient, instanceID, deviceID, instance.GPUType, gpuCount, config.Token, binaryHash); err != nil {
					tui.SendPhaseUpdate(p, 5, tui.PhaseWarning, "Virtualization warning", time.Since(phase6Start))
				} else {
					tui.SendPhaseUpdate(p, 5, tui.PhaseCompleted, "Prototyping mode configured", time.Since(phase6Start))
				}
			}
		} else if instance.Mode == "production" {
			if err := utils.RemoveThunderVirtualization(sshClient, config.Token); err != nil {
				tui.SendPhaseUpdate(p, 5, tui.PhaseWarning, "Cleanup warning", time.Since(phase6Start))
			} else {
				tui.SendPhaseUpdate(p, 5, tui.PhaseCompleted, "Production mode configured", time.Since(phase6Start))
			}
		}
	} else {
		tui.SendPhaseUpdate(p, 5, tui.PhaseSkipped, fmt.Sprintf("Skipped (%d active sessions)", activeSessions), time.Since(phase6Start))
	}

	phaseTimings["thunder_config"] = time.Since(phase6Start)

	// Phase 7: SSH Config Management
	phase7Start := time.Now()
	tui.SendPhaseUpdate(p, 6, tui.PhaseInProgress, "Updating SSH config...", 0)

	templatePorts := utils.GetTemplateOpenPorts(instance.Template)
	if err := utils.UpdateSSHConfig(instanceID, instance.IP, port, instance.UUID, tunnelPorts, templatePorts); err != nil {
		tui.SendPhaseUpdate(p, 6, tui.PhaseWarning, "SSH config warning", time.Since(phase7Start))
	} else {
		tui.SendPhaseUpdate(p, 6, tui.PhaseCompleted, fmt.Sprintf("Use 'ssh tnr-%s' to reconnect", instanceID), time.Since(phase7Start))
	}

	phaseTimings["ssh_config"] = time.Since(phase7Start)

	// Phase 8: Interactive SSH Session
	phase8Start := time.Now()
	tui.SendPhaseUpdate(p, 7, tui.PhaseInProgress, "Preparing SSH session...", 0)

	sshClient.Close()

	sshArgs := []string{
		"-q",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "IdentitiesOnly=yes",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", keyFile,
		"-p", fmt.Sprintf("%d", port),
		"-t",
	}

	if runtime.GOOS != "windows" {
		homeDir, _ := os.UserHomeDir()
		controlPath := fmt.Sprintf("%s/.thunder/thunder-control-%%h-%%p-%%r", homeDir)
		sshArgs = append(sshArgs,
			"-o", "ControlMaster=auto",
			"-o", fmt.Sprintf("ControlPath=%s", controlPath),
			"-o", "ControlPersist=5m",
		)
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

	phaseTimings["ssh_command_prep"] = time.Since(phase8Start)
	tui.SendPhaseComplete(p, 7, phaseTimings["ssh_command_prep"])

	// Complete the TUI
	tui.SendConnectComplete(p)
	time.Sleep(500 * time.Millisecond) // Let user see the completed state

	// Show debug timing if requested
	if debug {
		fmt.Println("\n=== Timing Breakdown ===")
		totalTime := time.Since(startTime)
		for phase, duration := range phaseTimings {
			percentage := float64(duration) / float64(totalTime) * 100
			fmt.Printf("%-25s: %10s (%5.1f%%)\n", phase, duration.Round(time.Millisecond), percentage)
		}
		fmt.Printf("%-25s: %10s\n", "Total", totalTime.Round(time.Millisecond))
		fmt.Println("========================\n")
	}

	// Execute SSH command
	sshCmd := exec.Command("ssh", sshArgs...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	err = sshCmd.Run()

	// Phase 9: Cleanup
	fmt.Println("\n⚡ Exiting Thunder instance ⚡")

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
