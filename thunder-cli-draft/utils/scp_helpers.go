package utils

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	scp "github.com/bramvdbogaerde/go-scp"
)

func GetLocalSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat local path: %w", err)
	}

	if !info.IsDir() {
		return info.Size(), nil
	}

	var totalSize int64
	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to walk directory: %w", err)
	}

	return totalSize, nil
}

func GetRemoteSize(client *SSHClient, path string) (int64, error) {
	// Expand ~ and environment variables
	expandedPath, err := ExpandRemotePath(client, path)
	if err != nil {
		return 0, err
	}

	checkCmd := fmt.Sprintf("test -f %s && echo file || echo dir", shellEscape(expandedPath))
	output, err := ExecuteSSHCommand(client, checkCmd)
	if err != nil {
		return 0, fmt.Errorf("failed to check path type: %w", err)
	}

	output = strings.TrimSpace(output)

	if output == "file" {
		cmd := fmt.Sprintf("stat --format=%%s %s", shellEscape(expandedPath))
		sizeStr, err := ExecuteSSHCommand(client, cmd)
		if err != nil {
			return 0, fmt.Errorf("failed to get file size: %w", err)
		}
		size, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse file size: %w", err)
		}
		return size, nil
	}

	cmd := fmt.Sprintf("du -sb %s | cut -f1", shellEscape(expandedPath))
	sizeStr, err := ExecuteSSHCommand(client, cmd)
	if err != nil {
		return 0, fmt.Errorf("failed to get directory size: %w", err)
	}

	size, err := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse directory size: %w", err)
	}

	return size, nil
}

func VerifyRemotePath(client *SSHClient, path string) (bool, error) {
	expandedPath, err := ExpandRemotePath(client, path)
	if err != nil {
		return false, err
	}

	cmd := fmt.Sprintf("test -e %s && echo exists || echo notfound", shellEscape(expandedPath))
	output, err := ExecuteSSHCommand(client, cmd)
	if err != nil {
		return false, fmt.Errorf("failed to verify path: %w", err)
	}

	return strings.TrimSpace(output) == "exists", nil
}

// ExpandRemotePath expands ~ and $VAR using shell evaluation
func ExpandRemotePath(client *SSHClient, path string) (string, error) {
	cmd := fmt.Sprintf("eval echo %s", shellEscape(path))
	output, err := ExecuteSSHCommand(client, cmd)
	if err != nil {
		return "", fmt.Errorf("failed to expand path: %w", err)
	}

	return strings.TrimSpace(output), nil
}

func PerformSCPUpload(client *SSHClient, localPath, remotePath string, progressCallback func(int64, int64)) error {
	if client == nil || client.GetClient() == nil {
		return fmt.Errorf("SSH client is not connected")
	}

	// Expand remote path
	expandedRemote, err := ExpandRemotePath(client, remotePath)
	if err != nil {
		return err
	}

	// Create SCP client
	scpClient, err := scp.NewClientBySSH(client.GetClient())
	if err != nil {
		return fmt.Errorf("failed to create SCP client: %w", err)
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local path: %w", err)
	}

	if info.IsDir() {
		return uploadDirectory(scpClient, localPath, expandedRemote, progressCallback)
	}

	return uploadFile(scpClient, localPath, expandedRemote, progressCallback)
}

func PerformSCPDownload(client *SSHClient, remotePath, localPath string, progressCallback func(int64, int64)) error {
	if client == nil || client.GetClient() == nil {
		return fmt.Errorf("SSH client is not connected")
	}

	// Expand remote path
	expandedRemote, err := ExpandRemotePath(client, remotePath)
	if err != nil {
		return err
	}

	checkCmd := fmt.Sprintf("test -f %s && echo file || test -d %s && echo dir || echo error",
		shellEscape(expandedRemote), shellEscape(expandedRemote))
	output, err := ExecuteSSHCommand(client, checkCmd)
	if err != nil {
		return fmt.Errorf("failed to check remote path type: %w", err)
	}

	pathType := strings.TrimSpace(output)
	if pathType == "error" {
		return fmt.Errorf("remote path does not exist or is not accessible")
	}

	scpClient, err := scp.NewClientBySSH(client.GetClient())
	if err != nil {
		return fmt.Errorf("failed to create SCP client: %w", err)
	}

	if pathType == "dir" {
		return downloadDirectory(client, scpClient, expandedRemote, localPath, progressCallback)
	}

	return downloadFile(scpClient, expandedRemote, localPath, progressCallback)
}

func uploadFile(scpClient scp.Client, localPath, remotePath string, progressCallback func(int64, int64)) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := info.Size()
	var bytesSent int64

	reader := &progressReader{
		reader: file,
		callback: func(n int) {
			bytesSent += int64(n)
			if progressCallback != nil {
				progressCallback(bytesSent, fileSize)
			}
		},
	}

	ctx := context.Background()
	err = scpClient.CopyFile(ctx, reader, remotePath, fmt.Sprintf("0%o", info.Mode().Perm()))
	if err != nil {
		return fmt.Errorf("SCP upload failed: %w", err)
	}

	return nil
}

func uploadDirectory(scpClient scp.Client, localPath, remotePath string, progressCallback func(int64, int64)) error {
	totalSize, err := GetLocalSize(localPath)
	if err != nil {
		return err
	}

	var bytesSent int64

	err = filepath.Walk(localPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(remotePath, relPath)
		targetPath = filepath.ToSlash(targetPath) // Unix paths use forward slashes

		if info.IsDir() {
			return nil // go-scp creates directories implicitly
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		reader := &progressReader{
			reader: file,
			callback: func(n int) {
				bytesSent += int64(n)
				if progressCallback != nil {
					progressCallback(bytesSent, totalSize)
				}
			},
		}

		ctx := context.Background()
		return scpClient.CopyFile(ctx, reader, targetPath, fmt.Sprintf("0%o", info.Mode().Perm()))
	})

	return err
}

func downloadFile(scpClient scp.Client, remotePath, localPath string, progressCallback func(int64, int64)) error {
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer file.Close()

	// Note: go-scp doesn't provide download progress callbacks
	ctx := context.Background()
	err = scpClient.CopyFromRemote(ctx, file, remotePath)
	if err != nil {
		return fmt.Errorf("SCP download failed: %w", err)
	}

	if progressCallback != nil {
		info, _ := file.Stat()
		progressCallback(info.Size(), info.Size())
	}

	return nil
}

func downloadDirectory(client *SSHClient, scpClient scp.Client, remotePath, localPath string, progressCallback func(int64, int64)) error {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	cmd := fmt.Sprintf("find %s -type f", shellEscape(remotePath))
	output, err := ExecuteSSHCommand(client, cmd)
	if err != nil {
		return fmt.Errorf("failed to list remote files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(output), "\n")
	if len(files) == 0 || (len(files) == 1 && files[0] == "") {
		return nil
	}

	totalSize, err := GetRemoteSize(client, remotePath)
	if err != nil {
		return err
	}

	var bytesReceived int64

	for _, remoteFile := range files {
		if remoteFile == "" {
			continue
		}

		relPath := strings.TrimPrefix(remoteFile, remotePath)
		relPath = strings.TrimPrefix(relPath, "/")
		localFile := filepath.Join(localPath, filepath.FromSlash(relPath))

		if err := os.MkdirAll(filepath.Dir(localFile), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		file, err := os.Create(localFile)
		if err != nil {
			return fmt.Errorf("failed to create local file: %w", err)
		}

		ctx := context.Background()
		err = scpClient.CopyFromRemote(ctx, file, remoteFile)
		file.Close()

		if err != nil {
			return fmt.Errorf("failed to download %s: %w", remoteFile, err)
		}

		info, _ := os.Stat(localFile)
		bytesReceived += info.Size()
		if progressCallback != nil {
			progressCallback(bytesReceived, totalSize)
		}
	}

	return nil
}

type progressReader struct {
	reader   *os.File
	callback func(int)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.callback != nil {
		pr.callback(n)
	}
	return n, err
}

// shellEscape wraps in single quotes and escapes embedded single quotes
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
