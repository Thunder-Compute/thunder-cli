package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/sentry"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/utils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var recursiveFlag bool

var scpCmd = &cobra.Command{
	Use:   "scp [source...] [destination]",
	Short: "Securely copy files between local machine and Thunder Compute instances",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("requires at least a source and destination")
		}

		sources := args[:len(args)-1]
		destination := args[len(args)-1]

		return runSCP(sources, destination)
	},
}

func init() {
	scpCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderSCPHelp(cmd)
	})

	rootCmd.AddCommand(scpCmd)
	scpCmd.Flags().BoolVarP(&recursiveFlag, "recursive", "r", false, "Recursively copy directories")
}

type PathInfo struct {
	Original   string
	InstanceID string
	Path       string
	IsRemote   bool
}

func parsePath(path string) (PathInfo, error) {
	info := PathInfo{
		Original: path,
	}

	// Windows edge cases: avoid treating "C:" or "\\server" as instance_id
	if runtime.GOOS == "windows" {
		if len(path) >= 2 && path[1] == ':' &&
			((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) {
			info.Path = path
			return info, nil
		}
		if strings.HasPrefix(path, "\\\\") {
			info.Path = path
			return info, nil
		}
	}

	// Remote path syntax: instance_id:/path/to/file
	parts := strings.SplitN(path, ":", 2)
	if len(parts) == 2 && parts[0] != "" {
		instanceID := parts[0]

		if isValidInstanceID(instanceID) {
			info.InstanceID = instanceID
			info.Path = parts[1]
			info.IsRemote = true
			return info, nil
		}
	}

	info.Path = path
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
	sentry.AddBreadcrumb("scp", "starting SCP operation", map[string]interface{}{
		"source_count": len(sources),
		"destination":  destination,
	}, sentry.LevelInfo)

	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	if config.Token == "" {
		return fmt.Errorf("no authentication token found. Please run 'tnr login'")
	}

	sourcePaths := make([]PathInfo, len(sources))
	for i, src := range sources {
		parsed, err := parsePath(src)
		if err != nil {
			sentry.AddBreadcrumb("scp", "path parsing failed", map[string]interface{}{
				"path":  src,
				"error": err.Error(),
			}, sentry.LevelError)
			return fmt.Errorf("failed to parse source path '%s': %w", src, err)
		}
		sourcePaths[i] = parsed
	}

	destPath, err := parsePath(destination)
	if err != nil {
		sentry.AddBreadcrumb("scp", "destination path parsing failed", map[string]interface{}{
			"path":  destination,
			"error": err.Error(),
		}, sentry.LevelError)
		return fmt.Errorf("failed to parse destination path '%s': %w", destination, err)
	}

	direction, instanceID, err := determineTransferDirection(sourcePaths, destPath)
	if err != nil {
		sentry.AddBreadcrumb("scp", "transfer direction error", map[string]interface{}{
			"error": err.Error(),
		}, sentry.LevelError)
		return err
	}

	sentry.AddBreadcrumb("scp", "transfer direction determined", map[string]interface{}{
		"direction":   direction,
		"instance_id": instanceID,
	}, sentry.LevelInfo)

	if len(sourcePaths) > 1 {
		if direction == "upload" {
			if !strings.HasSuffix(destPath.Path, "/") {
				return fmt.Errorf("destination must be a directory when copying multiple files (add trailing /)")
			}
		} else {
			if !strings.HasSuffix(destPath.Path, "/") && !strings.HasSuffix(destPath.Path, string(filepath.Separator)) {
				return fmt.Errorf("destination must be a directory when copying multiple files")
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	tui.InitCommonStyles(os.Stdout)

	scpModel := tui.NewSCPModel(direction, "Validating...")
	p := tea.NewProgram(
		scpModel,
		tea.WithContext(ctx),
		tea.WithOutput(os.Stdout),
	)

	tuiDone := make(chan error, 1)
	cancelCtx, cancel := context.WithCancel(context.Background())
	var finalModel tea.Model
	var wasCancelled bool

	go func() {
		var err error
		finalModel, err = p.Run()
		if err != nil {
			tuiDone <- err
		}
		if scpModel, ok := finalModel.(tui.SCPModel); ok && scpModel.Cancelled() {
			wasCancelled = true
			cancel()
		}
		close(tuiDone)
	}()

	time.Sleep(50 * time.Millisecond)

	client := api.NewClient(config.Token, config.APIURL)

	p.Send(tui.SCPPhaseMsg{Phase: tui.SCPPhaseConnecting})

	sentry.AddBreadcrumb("scp", "fetching instances", nil, sentry.LevelInfo)

	instances, err := client.ListInstances()
	if err != nil {
		sentry.AddBreadcrumb("scp", "failed to list instances", map[string]interface{}{
			"error": err.Error(),
		}, sentry.LevelError)
		p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("failed to list instances: %w", err)})
		<-tuiDone
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
		sentry.AddBreadcrumb("scp", "instance not found", map[string]interface{}{
			"instance_id": instanceID,
		}, sentry.LevelError)
		p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("instance '%s' not found", instanceID)})
		<-tuiDone
		return fmt.Errorf("instance '%s' not found", instanceID)
	}

	if targetInstance.Status != "RUNNING" {
		sentry.AddBreadcrumb("scp", "instance not running", map[string]interface{}{
			"instance_id": instanceID,
			"status":      targetInstance.Status,
		}, sentry.LevelError)
		p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("instance '%s' is not running (status: %s)", instanceID, targetInstance.Status)})
		<-tuiDone
		return fmt.Errorf("instance '%s' is not running (status: %s)", instanceID, targetInstance.Status)
	}

	sentry.AddBreadcrumb("scp", "instance validated", map[string]interface{}{
		"instance_id":   targetInstance.ID,
		"instance_name": targetInstance.Name,
		"instance_ip":   targetInstance.IP,
	}, sentry.LevelInfo)

	instanceName := fmt.Sprintf("%s (%s)", targetInstance.Name, targetInstance.ID)
	p.Send(tui.SCPInstanceNameMsg{InstanceName: instanceName})

	keyFile := utils.GetKeyFile(targetInstance.UUID)
	keyExists := utils.KeyExists(targetInstance.UUID)

	sentry.AddBreadcrumb("scp", "checking SSH keys", map[string]interface{}{
		"key_exists": keyExists,
	}, sentry.LevelInfo)

	if !keyExists {
		sentry.AddBreadcrumb("scp", "generating SSH key", nil, sentry.LevelInfo)

		keyResp, err := client.AddSSHKey(targetInstance.ID, nil)
		if err != nil {
			sentry.AddBreadcrumb("scp", "SSH key generation failed", map[string]interface{}{
				"error": err.Error(),
			}, sentry.LevelError)
			p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("failed to add SSH key: %w", err)})
			<-tuiDone
			return fmt.Errorf("failed to add SSH key: %w", err)
		}

		if keyResp.Key == nil {
			p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("no private key returned from server")})
			<-tuiDone
			return fmt.Errorf("no private key returned from server")
		}

		if err := utils.SavePrivateKey(targetInstance.UUID, *keyResp.Key); err != nil {
			sentry.AddBreadcrumb("scp", "SSH key save failed", map[string]interface{}{
				"error": err.Error(),
			}, sentry.LevelError)
			p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("failed to save private key: %w", err)})
			<-tuiDone
			return fmt.Errorf("failed to save private key: %w", err)
		}
		keyFile = utils.GetKeyFile(targetInstance.UUID)
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
		<-tuiDone
		PrintWarningSimple("User cancelled scp process")
		return nil
	}

	sentry.AddBreadcrumb("scp", "establishing SSH connection", map[string]interface{}{
		"ip":   targetInstance.IP,
		"port": targetInstance.Port,
	}, sentry.LevelInfo)

	sshClient, err := utils.RobustSSHConnectCtx(cancelCtx, targetInstance.IP, keyFile, targetInstance.Port, 60)
	if checkCancelled() {
		<-tuiDone
		PrintWarningSimple("User cancelled scp process")
		return nil
	}
	if err != nil {
		sentry.AddBreadcrumb("scp", "SSH connection failed", map[string]interface{}{
			"ip":         targetInstance.IP,
			"port":       targetInstance.Port,
			"error":      err.Error(),
			"error_type": string(utils.ClassifySSHError(err)),
		}, sentry.LevelError)
		p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("SSH connection failed: %w", err)})
		<-tuiDone
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	sentry.AddBreadcrumb("scp", "SSH connection established", nil, sentry.LevelInfo)

	if checkCancelled() {
		<-tuiDone
		PrintWarningSimple("User cancelled scp process")
		return nil
	}

	p.Send(tui.SCPPhaseMsg{Phase: tui.SCPPhaseCalculatingSize})
	_, err = calculateTotalSize(sshClient, sourcePaths, direction)
	if checkCancelled() {
		<-tuiDone
		PrintWarningSimple("User cancelled scp process")
		return nil
	}
	if err != nil {
		p.Send(tui.SCPErrorMsg{Err: err})
		<-tuiDone
		return err
	}

	if checkCancelled() {
		<-tuiDone
		PrintWarningSimple("User cancelled scp process")
		return nil
	}

	startTime := time.Now()
	p.Send(tui.SCPPhaseMsg{Phase: tui.SCPPhaseTransferring})

	sentry.AddBreadcrumb("scp", "starting file transfer", map[string]interface{}{
		"direction":    direction,
		"file_count":   len(sourcePaths),
		"instance_id":  instanceID,
		"instance_ip":  targetInstance.IP,
	}, sentry.LevelInfo)

	filesTransferred := 0
	var totalBytes int64

	progressCallback := func(bytesSent, bytesTotal int64) {
		if checkCancelled() {
			return
		}
		p.Send(tui.SCPProgressMsg{
			BytesSent:  bytesSent,
			BytesTotal: bytesTotal,
		})
	}

	if direction == "upload" {
		for _, src := range sourcePaths {
			if checkCancelled() {
				<-tuiDone
				PrintWarningSimple("User cancelled scp process")
				return nil
			}

			localPath := src.Path
			if strings.HasPrefix(localPath, "~/") {
				homeDir, _ := os.UserHomeDir()
				localPath = filepath.Join(homeDir, localPath[2:])
			}
			localPath = filepath.Clean(localPath)

			remotePath := destPath.Path
			if remotePath == "" {
				remotePath = "./" + filepath.Base(localPath)
			} else if len(sourcePaths) > 1 || strings.HasSuffix(remotePath, "/") {
				remotePath = strings.TrimSuffix(remotePath, "/") + "/" + filepath.Base(localPath)
			}

			if _, err := os.Stat(localPath); err != nil {
				p.Send(tui.SCPErrorMsg{Err: fmt.Errorf("local path not found: %s", localPath)})
				<-tuiDone
				return fmt.Errorf("local path not found: %s", localPath)
			}

			if checkCancelled() {
				<-tuiDone
				PrintWarningSimple("User cancelled scp process")
				return nil
			}

			err := utils.PerformSCPUploadCtx(cancelCtx, sshClient, localPath, remotePath, progressCallback)
			if checkCancelled() {
				<-tuiDone
				PrintWarningSimple("User cancelled scp process")
				return nil
			}
			if err != nil {
				sentry.AddBreadcrumb("scp", "upload failed", map[string]interface{}{
					"local_path":  localPath,
					"remote_path": remotePath,
					"error":       err.Error(),
				}, sentry.LevelError)
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
			if checkCancelled() {
				<-tuiDone
				PrintWarningSimple("User cancelled scp process")
				return nil
			}

			remotePath := src.Path

			exists, err := utils.VerifyRemotePath(sshClient, remotePath)
			if checkCancelled() {
				<-tuiDone
				PrintWarningSimple("User cancelled scp process")
				return nil
			}
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
			localPath := destPath.Path
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

			if checkCancelled() {
				<-tuiDone
				PrintWarningSimple("User cancelled scp process")
				return nil
			}

			err = utils.PerformSCPDownloadCtx(cancelCtx, sshClient, remotePath, localPath, progressCallback)
			if checkCancelled() {
				<-tuiDone
				PrintWarningSimple("User cancelled scp process")
				return nil
			}
			if err != nil {
				sentry.AddBreadcrumb("scp", "download failed", map[string]interface{}{
					"remote_path": remotePath,
					"local_path":  localPath,
					"error":       err.Error(),
				}, sentry.LevelError)
				p.Send(tui.SCPErrorMsg{Err: err})
				<-tuiDone
				return fmt.Errorf("download failed: %w", err)
			}

			filesTransferred++
			size, _ := utils.GetRemoteSize(sshClient, remotePath)
			totalBytes += size
		}
	}

	if checkCancelled() {
		<-tuiDone
		PrintWarningSimple("User cancelled scp process")
		return nil
	}

	duration := time.Since(startTime)

	sentry.AddBreadcrumb("scp", "transfer complete", map[string]interface{}{
		"direction":         direction,
		"files_transferred": filesTransferred,
		"bytes_transferred": totalBytes,
		"duration_ms":       duration.Milliseconds(),
	}, sentry.LevelInfo)

	p.Send(tui.SCPCompleteMsg{
		FilesTransferred: filesTransferred,
		BytesTransferred: totalBytes,
		Duration:         duration,
	})

	if err := <-tuiDone; err != nil {
		return err
	}

	if wasCancelled {
		PrintWarningSimple("User cancelled scp process")
		return nil
	}

	return nil
}

func determineTransferDirection(sources []PathInfo, dest PathInfo) (direction string, instanceID string, err error) {
	remoteCount := 0
	var remoteInstanceID string

	for _, src := range sources {
		if src.IsRemote {
			remoteCount++
			if remoteInstanceID == "" {
				remoteInstanceID = src.InstanceID
			} else if remoteInstanceID != src.InstanceID {
				return "", "", fmt.Errorf("cannot transfer between multiple instances")
			}
		}
	}

	if dest.IsRemote {
		if remoteCount > 0 {
			return "", "", fmt.Errorf("cannot transfer from remote to remote")
		}
		return "upload", dest.InstanceID, nil
	}

	if remoteCount == 0 {
		return "", "", fmt.Errorf("no remote path specified (use instance_id:/path/to/file)")
	}

	if remoteCount != len(sources) {
		return "", "", fmt.Errorf("all sources must be from the same location (all local or all from same instance)")
	}

	return "download", remoteInstanceID, nil
}

func calculateTotalSize(client *utils.SSHClient, sources []PathInfo, direction string) (int64, error) {
	var totalSize int64

	for _, src := range sources {
		var size int64
		var err error

		if direction == "upload" {
			localPath := src.Path
			if strings.HasPrefix(localPath, "~/") {
				homeDir, _ := os.UserHomeDir()
				localPath = filepath.Join(homeDir, localPath[2:])
			}
			localPath = filepath.Clean(localPath)

			size, err = utils.GetLocalSize(localPath)
			if err != nil {
				return 0, fmt.Errorf("failed to get size of %s: %w", src.Original, err)
			}
		} else {
			size, err = utils.GetRemoteSize(client, src.Path)
			if err != nil {
				return 0, fmt.Errorf("failed to get size of %s: %w", src.Original, err)
			}
		}

		totalSize += size
	}

	return totalSize, nil
}
