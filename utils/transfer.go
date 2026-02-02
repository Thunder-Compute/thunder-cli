package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// SCPTransfer uses the native scp command for file transfers.
// Works on Windows 10+, macOS, and Linux without additional dependencies.
func SCPTransfer(ctx context.Context, keyFile, ip string, port int, localPath, remotePath string, upload bool) error {
	remote := fmt.Sprintf("ubuntu@%s:%s", ip, remotePath)

	args := []string{
		"-i", keyFile,
		"-P", fmt.Sprintf("%d", port),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-r", // recursive for directories
	}

	if upload {
		args = append(args, localPath, remote)
	} else {
		args = append(args, remote, localPath)
	}

	cmd := exec.CommandContext(ctx, "scp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
