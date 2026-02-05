package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Thunder-Compute/thunder-cli/api"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/spf13/cobra"
)

var scpCmd = &cobra.Command{
	Use:          "scp [source...] [destination]",
	Short:        "Copy files between local machine and Thunder Compute instances",
	Args:         cobra.MinimumNArgs(2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
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
}

type PathInfo struct {
	Original   string
	InstanceID string
	Path       string
	IsRemote   bool
}

func parsePath(path string) PathInfo {
	info := PathInfo{Original: path}

	// Windows drive letters
	if runtime.GOOS == "windows" && len(path) >= 2 && path[1] == ':' {
		info.Path = path
		return info
	}

	// Remote: instance_id:/path
	if parts := strings.SplitN(path, ":", 2); len(parts) == 2 && isValidInstanceID(parts[0]) {
		info.InstanceID = parts[0]
		info.Path = parts[1]
		info.IsRemote = true
		return info
	}

	info.Path = path
	return info
}

func isValidInstanceID(s string) bool {
	return len(s) > 0 && len(s) <= 20 && !strings.ContainsAny(s, "/\\.")
}

func runSCP(sources []string, destination string) error {
	config, err := LoadConfig()
	if err != nil || config.Token == "" {
		return fmt.Errorf("not authenticated. Please run 'tnr login' first")
	}

	sourcePaths := make([]PathInfo, len(sources))
	for i, src := range sources {
		sourcePaths[i] = parsePath(src)
	}
	destPath := parsePath(destination)

	direction, instanceID, err := determineTransferDirection(sourcePaths, destPath)
	if err != nil {
		return err
	}

	client := api.NewClient(config.Token, config.APIURL)
	instances, err := client.ListInstances()
	if err != nil {
		return utils.WrapAPIError(err, "failed to list instances")
	}

	var target *api.Instance
	for i, inst := range instances {
		if inst.ID == instanceID || inst.UUID == instanceID {
			target = &instances[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("instance '%s' not found", instanceID)
	}
	if target.Status != "RUNNING" {
		return fmt.Errorf("instance '%s' is not running (status: %s)", instanceID, target.Status)
	}

	keyFile := utils.GetKeyFile(target.UUID)
	if !utils.KeyExists(target.UUID) {
		keyResp, err := client.AddSSHKey(target.ID)
		if err != nil {
			return fmt.Errorf("failed to add SSH key: %w", err)
		}
		if keyResp.Key != nil {
			if err := utils.SavePrivateKey(target.UUID, *keyResp.Key); err != nil {
				return fmt.Errorf("failed to save private key: %w", err)
			}
		}
		keyFile = utils.GetKeyFile(target.UUID)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Transfer each source
	for _, src := range sourcePaths {
		localPath := src.Path
		remotePath := destPath.Path

		if direction == "upload" {
			if strings.HasPrefix(localPath, "~/") {
				homeDir, _ := os.UserHomeDir()
				localPath = filepath.Join(homeDir, localPath[2:])
			}
			if remotePath == "" {
				remotePath = "./"
			}
			fmt.Printf("Uploading %s to %s:%s\n", localPath, target.Name, remotePath)
		} else {
			remotePath = src.Path
			localPath = destPath.Path
			if strings.HasPrefix(localPath, "~/") {
				homeDir, _ := os.UserHomeDir()
				localPath = filepath.Join(homeDir, localPath[2:])
			}
			fmt.Printf("Downloading %s:%s to %s\n", target.Name, remotePath, localPath)
		}

		err := utils.Transfer(ctx, keyFile, target.GetIP(), target.Port, localPath, remotePath, direction == "upload")
		if err != nil {
			return err
		}
	}

	fmt.Println("Transfer complete")
	return nil
}

func determineTransferDirection(sources []PathInfo, dest PathInfo) (string, string, error) {
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
		return "", "", fmt.Errorf("no remote path specified (use instance_id:/path)")
	}

	return "download", remoteInstanceID, nil
}
