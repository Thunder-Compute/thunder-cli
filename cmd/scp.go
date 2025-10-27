/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joshuawatkins04/thunder-cli-draft/api"
	"github.com/joshuawatkins04/thunder-cli-draft/tui"
	"github.com/joshuawatkins04/thunder-cli-draft/utils"
	"github.com/spf13/cobra"
)

var recursiveFlag bool

var scpCmd = &cobra.Command{
	Use:   "scp [source...] [destination]",
	Short: "Securely copy files between local machine and Thunder Compute instances",
	Long: `Copy files and directories between your local machine and Thunder Compute instances.

Supports uploading/downloading files and directories with progress tracking.

Path Syntax:
  - Remote paths: instance_id:/path/to/file (e.g., 0:/home/ubuntu/myfile.py)
  - Local paths: Regular file system paths (e.g., ./myfile.py or /tmp/file.txt)

Examples:
  # Upload a single file
  tnr scp myfile.py 0:/home/ubuntu/

  # Download a file
  tnr scp 0:/home/ubuntu/results.txt ./

  # Upload multiple files
  tnr scp file1.py file2.py config.json 0:/home/ubuntu/

  # Upload a directory recursively
  tnr scp ./my-project/ 0:/home/ubuntu/projects/

  # Download a directory
  tnr scp 0:/home/ubuntu/data/ ./local-data/`,
	Args: cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Error: requires at least a source and destination\n")
			os.Exit(1)
		}

		sources := args[:len(args)-1]
		destination := args[len(args)-1]

		if err := runSCP(sources, destination); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(scpCmd)
	scpCmd.Flags().BoolVarP(&recursiveFlag, "recursive", "r", false, "Recursively copy directories")
}

type pathInfo struct {
	original   string
	instanceID string
	path       string
	isRemote   bool
}

// parsePath splits "instance_id:/path" into components, handling Windows edge cases
func parsePath(path string) (pathInfo, error) {
	info := pathInfo{
		original: path,
	}

	// Windows edge cases: avoid treating "C:" or "\\server" as instance_id
	if runtime.GOOS == "windows" {
		if len(path) >= 2 && path[1] == ':' &&
			((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) {
			info.path = path
			return info, nil
		}
		if strings.HasPrefix(path, "\\\\") {
			info.path = path
			return info, nil
		}
	}

	// Remote path syntax: instance_id:/path/to/file
	parts := strings.SplitN(path, ":", 2)
	if len(parts) == 2 && parts[0] != "" {
		instanceID := parts[0]

		if isValidInstanceID(instanceID) {
			info.instanceID = instanceID
			info.path = parts[1]
			info.isRemote = true
			return info, nil
		}
	}

	info.path = path
	return info, nil
}

// isValidInstanceID filters out protocol schemes (http:, file:) and absolute paths
func isValidInstanceID(s string) bool {
	if len(s) == 0 || len(s) > 20 {
		return false
	}

	if strings.ContainsAny(s, "/\\.") {
		return false
	}

	return true
}

func runSCP(sources []string, destination string) error {
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	sourcePaths := make([]pathInfo, len(sources))
	for i, src := range sources {
		parsed, err := parsePath(src)
		if err != nil {
			return fmt.Errorf("failed to parse source path '%s': %w", src, err)
		}
		sourcePaths[i] = parsed
	}

	destPath, err := parsePath(destination)
	if err != nil {
		return fmt.Errorf("failed to parse destination path '%s': %w", destination, err)
	}

	direction, instanceID, err := determineTransferDirection(sourcePaths, destPath)
	if err != nil {
		return err
	}

	client := api.NewClient(config.Token)
	instances, err := client.ListInstances()
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	var targetInstance *api.Instance
	for i, inst := range instances {
		if inst.ID == instanceID || inst.UUID == instanceID {
			targetInstance = &instances[i]
			break
		}
	}

	if targetInstance == nil {
		return fmt.Errorf("instance '%s' not found", instanceID)
	}

	if targetInstance.Status != "RUNNING" {
		return fmt.Errorf("instance '%s' is not running (status: %s)", instanceID, targetInstance.Status)
	}

	// Multiple files require destination to end with / (must be a directory)
	if len(sourcePaths) > 1 {
		if direction == "upload" {
			if !strings.HasSuffix(destPath.path, "/") {
				return fmt.Errorf("destination must be a directory when copying multiple files (add trailing /)")
			}
		} else {
			if !strings.HasSuffix(destPath.path, "/") && !strings.HasSuffix(destPath.path, string(filepath.Separator)) {
				return fmt.Errorf("destination must be a directory when copying multiple files")
			}
		}
	}

	keyFile := utils.GetKeyFile(targetInstance.UUID)
	if !utils.KeyExists(targetInstance.UUID) {
		keyResp, err := client.AddSSHKey(targetInstance.ID)
		if err != nil {
			return fmt.Errorf("failed to add SSH key: %w", err)
		}

		if err := utils.SavePrivateKey(targetInstance.UUID, keyResp.Key); err != nil {
			return fmt.Errorf("failed to save private key: %w", err)
		}
		keyFile = utils.GetKeyFile(targetInstance.UUID)
	}

	instanceName := fmt.Sprintf("%s (%s)", targetInstance.Name, targetInstance.ID)
	scpModel := tui.NewSCPModel(direction, instanceName)
	p := tea.NewProgram(scpModel)

	tuiDone := make(chan error, 1)
	go func() {
		_, err := p.Run()
		if err != nil {
			tuiDone <- err
		}
		close(tuiDone)
	}()

	time.Sleep(50 * time.Millisecond)

	p.Send(tui.SCPPhaseMsg{Phase: tui.SCPPhaseConnecting})
	sshClient, err := utils.RobustSSHConnect(targetInstance.IP, keyFile, targetInstance.Port, 60)
	if err != nil {
		p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("SSH connection failed: %w", err)})
		<-tuiDone
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	p.Send(tui.SCPPhaseMsg{Phase: tui.SCPPhaseCalculatingSize})
	_, err = calculateTotalSize(sshClient, sourcePaths, direction)
	if err != nil {
		p.Send(tui.SCPErrorMsg{Err: err})
		<-tuiDone
		return err
	}

	startTime := time.Now()
	p.Send(tui.SCPPhaseMsg{Phase: tui.SCPPhaseTransferring})

	filesTransferred := 0
	var totalBytes int64

	progressCallback := func(bytesSent, bytesTotal int64) {
		p.Send(tui.SCPProgressMsg{
			BytesSent:  bytesSent,
			BytesTotal: bytesTotal,
		})
	}

	if direction == "upload" {
		for _, src := range sourcePaths {
			localPath := src.path
			if strings.HasPrefix(localPath, "~/") {
				homeDir, _ := os.UserHomeDir()
				localPath = filepath.Join(homeDir, localPath[2:])
			}
			localPath = filepath.Clean(localPath)

			remotePath := destPath.path
			// Trailing slash means directory - append source filename
			if len(sourcePaths) > 1 || strings.HasSuffix(remotePath, "/") {
				remotePath = strings.TrimSuffix(remotePath, "/") + "/" + filepath.Base(localPath)
			}

			if _, err := os.Stat(localPath); err != nil {
				p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("local path not found: %s", localPath)})
				<-tuiDone
				return fmt.Errorf("local path not found: %s", localPath)
			}

			err := utils.PerformSCPUpload(sshClient, localPath, remotePath, progressCallback)
			if err != nil {
				p.Send(tui.SCPErrorMsg{Err: err})
				<-tuiDone
				return fmt.Errorf("upload failed: %w", err)
			}

			filesTransferred++
			size, _ := utils.GetLocalSize(localPath)
			totalBytes += size
		}
	} else {
		for _, src := range sourcePaths {
			remotePath := src.path

			exists, err := utils.VerifyRemotePath(sshClient, remotePath)
			if err != nil {
				p.Send(tui.SCPErrorMsg{Err: err})
				<-tuiDone
				return err
			}
			if !exists {
				err := fmt.Errorf("remote path not found: %s", remotePath)
				p.Send(tui.SCPErrorMsg{Err: err})
				<-tuiDone
				return err
			}

			// Determine local path
			localPath := destPath.path
			// If destination ends with / or \, it's a directory - append filename
			if len(sourcePaths) > 1 || strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, string(filepath.Separator)) {
				localPath = filepath.Join(localPath, filepath.Base(remotePath))
			}

			// Normalize local path
			if strings.HasPrefix(localPath, "~/") {
				homeDir, _ := os.UserHomeDir()
				localPath = filepath.Join(homeDir, localPath[2:])
			}
			localPath = filepath.Clean(localPath)

			parentDir := filepath.Dir(localPath)
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("failed to create directory: %w", err)})
				<-tuiDone
				return fmt.Errorf("failed to create directory: %w", err)
			}

			err = utils.PerformSCPDownload(sshClient, remotePath, localPath, progressCallback)
			if err != nil {
				p.Send(tui.SCPErrorMsg{Err: err})
				<-tuiDone
				return fmt.Errorf("download failed: %w", err)
			}

			filesTransferred++
			size, _ := utils.GetRemoteSize(sshClient, remotePath)
			totalBytes += size
		}
	}

	duration := time.Since(startTime)

	p.Send(tui.SCPCompleteMsg{
		FilesTransferred: filesTransferred,
		BytesTransferred: totalBytes,
		Duration:         duration,
	})

	if err := <-tuiDone; err != nil {
		return err
	}

	return nil
}

// determineTransferDirection detects upload vs download and validates source consistency
func determineTransferDirection(sources []pathInfo, dest pathInfo) (direction string, instanceID string, err error) {
	remoteCount := 0
	var remoteInstanceID string

	for _, src := range sources {
		if src.isRemote {
			remoteCount++
			if remoteInstanceID == "" {
				remoteInstanceID = src.instanceID
			} else if remoteInstanceID != src.instanceID {
				return "", "", fmt.Errorf("cannot transfer between multiple instances")
			}
		}
	}

	if dest.isRemote {
		if remoteCount > 0 {
			return "", "", fmt.Errorf("cannot transfer from remote to remote")
		}
		return "upload", dest.instanceID, nil
	}

	if remoteCount == 0 {
		return "", "", fmt.Errorf("no remote path specified (use instance_id:/path/to/file)")
	}

	if remoteCount != len(sources) {
		return "", "", fmt.Errorf("all sources must be from the same location (all local or all from same instance)")
	}

	return "download", remoteInstanceID, nil
}

func calculateTotalSize(client *utils.SSHClient, sources []pathInfo, direction string) (int64, error) {
	var totalSize int64

	for _, src := range sources {
		var size int64
		var err error

		if direction == "upload" {
			localPath := src.path
			if strings.HasPrefix(localPath, "~/") {
				homeDir, _ := os.UserHomeDir()
				localPath = filepath.Join(homeDir, localPath[2:])
			}
			localPath = filepath.Clean(localPath)

			size, err = utils.GetLocalSize(localPath)
			if err != nil {
				return 0, fmt.Errorf("failed to get size of %s: %w", src.original, err)
			}
		} else {
			size, err = utils.GetRemoteSize(client, src.path)
			if err != nil {
				return 0, fmt.Errorf("failed to get size of %s: %w", src.original, err)
			}
		}

		totalSize += size
	}

	return totalSize, nil
}
