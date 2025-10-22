package utils

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient wraps an SSH connection
type SSHClient struct {
	client *ssh.Client
}

// Close closes the SSH connection
func (s *SSHClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// testSocketConnection checks if we can connect to the SSH port
func testSocketConnection(ip string, port int) bool {
	address := fmt.Sprintf("%s:%d", ip, port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// RobustSSHConnect establishes an SSH connection with retry logic
func RobustSSHConnect(ip, keyFile string, port int, maxWait int) (*SSHClient, error) {
	// Read private key
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", ip, port)
	startTime := time.Now()

	fmt.Println("Establishing SSH connection...")

	for {
		elapsed := time.Since(startTime)
		if elapsed > time.Duration(maxWait)*time.Second {
			return nil, fmt.Errorf("SSH connection timeout after %d seconds", maxWait)
		}

		// Test socket connection first
		if !testSocketConnection(ip, port) {
			fmt.Print(".")
			time.Sleep(1 * time.Second)
			continue
		}

		// Try SSH connection
		client, err := ssh.Dial("tcp", address, config)
		if err == nil {
			fmt.Println("\n✓ SSH connection established")
			return &SSHClient{client: client}, nil
		}

		// Retry on common errors
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "no route to host") ||
			strings.Contains(err.Error(), "i/o timeout") {
			fmt.Print(".")
			time.Sleep(1 * time.Second)
			continue
		}

		// For other errors, return immediately
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
}

// ExecuteSSHCommand runs a command on the remote host
func ExecuteSSHCommand(client *SSHClient, command string) (string, error) {
	if client == nil || client.client == nil {
		return "", fmt.Errorf("SSH client is not connected")
	}

	session, err := client.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// CheckActiveSessions counts the number of active SSH sessions on the instance
func CheckActiveSessions(client *SSHClient) (int, error) {
	output, err := ExecuteSSHCommand(client, "who | grep 'pts/' | wc -l")
	if err != nil {
		return 0, err
	}

	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(output), "%d", &count)
	if err != nil {
		return 0, fmt.Errorf("failed to parse session count: %w", err)
	}

	return count, nil
}

// UploadFile uploads a file to the remote host using SFTP
func UploadFile(client *SSHClient, localPath, remotePath string) error {
	if client == nil || client.client == nil {
		return fmt.Errorf("SSH client is not connected")
	}

	// Read local file
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Create remote file using cat
	session, err := client.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	if err := session.Start(fmt.Sprintf("cat > %s", remotePath)); err != nil {
		return fmt.Errorf("failed to start cat command: %w", err)
	}

	if _, err := stdin.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// DownloadFile downloads a file from the remote host
func DownloadFile(client *SSHClient, remotePath, localPath string) error {
	if client == nil || client.client == nil {
		return fmt.Errorf("SSH client is not connected")
	}

	session, err := client.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := session.Start(fmt.Sprintf("cat %s", remotePath)); err != nil {
		return fmt.Errorf("failed to start cat command: %w", err)
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	if err := os.WriteFile(localPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}

	return nil
}
