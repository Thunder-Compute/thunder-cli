package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSHClient struct {
	client *ssh.Client
}

func (s *SSHClient) GetClient() *ssh.Client {
	return s.client
}

func (s *SSHClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func testSocketConnection(ip string, port int) bool {
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func newSSHConfig(user, keyFile string) (*ssh.ClientConfig, error) {
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	knownHostsPath := filepath.Join(sshDir, "known_hosts")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		if err := os.WriteFile(knownHostsPath, []byte{}, 0644); err != nil {
			return nil, fmt.Errorf("failed to create known_hosts file: %w", err)
		}
	}

	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts: %w", err)
	}

	wrappedCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := hostKeyCallback(hostname, remote, key)
		if err == nil {
			return nil
		}

		errMsg := err.Error()

		if strings.Contains(errMsg, "host key mismatch") || strings.Contains(errMsg, "changed") {
			return fmt.Errorf("host key changed for %s - possible security issue: %w", hostname, err)
		}

		if strings.Contains(errMsg, "key is unknown") || strings.Contains(errMsg, "not in known_hosts") {
			hostForKnownHosts := hostname
			if host, _, err := net.SplitHostPort(hostname); err == nil {
				hostForKnownHosts = host
			}
			line := knownhosts.Line([]string{hostForKnownHosts}, key)
			f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to append to known_hosts: %w", err)
			}
			if _, err := fmt.Fprintln(f, line); err != nil {
				f.Close()
				return fmt.Errorf("failed to write to known_hosts: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("failed to close known_hosts: %w", err)
			}
			return nil
		}

		return err
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: wrappedCallback,
		Timeout:         15 * time.Second,
	}, nil
}

func RobustSSHConnect(ip, keyFile string, port int, maxWait int) (*SSHClient, error) {
	return RobustSSHConnectCtx(context.Background(), ip, keyFile, port, maxWait)
}

func RobustSSHConnectCtx(ctx context.Context, ip, keyFile string, port int, maxWait int) (*SSHClient, error) {
	config, err := newSSHConfig("ubuntu", keyFile)
	if err != nil {
		return nil, err
	}

	address := net.JoinHostPort(ip, strconv.Itoa(port))
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("SSH connection cancelled")
		default:
		}
		elapsed := time.Since(startTime)
		if elapsed > time.Duration(maxWait)*time.Second {
			return nil, fmt.Errorf("SSH connection timeout after %d seconds", maxWait)
		}

		if !testSocketConnection(ip, port) {
			time.Sleep(1 * time.Second)
			continue
		}

		dialer := &net.Dialer{}
		var conn net.Conn
		d := make(chan struct{})
		var dialErr error
		go func() {
			conn, dialErr = dialer.Dial("tcp", address)
			close(d)
		}()

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("SSH connection cancelled")
		case <-d:
			// proceed
		}

		if dialErr != nil {
			if strings.Contains(dialErr.Error(), "connection refused") ||
				strings.Contains(dialErr.Error(), "no route to host") ||
				strings.Contains(dialErr.Error(), "i/o timeout") {
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, fmt.Errorf("SSH dial failed: %w", dialErr)
		}

		cc, chans, reqs, err := ssh.NewClientConn(conn, address, config)
		if err == nil {
			return &SSHClient{client: ssh.NewClient(cc, chans, reqs)}, nil
		}

		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "no route to host") ||
			strings.Contains(err.Error(), "i/o timeout") {
			time.Sleep(1 * time.Second)
			continue
		}

		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
}

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

// CheckActiveSessions counts active SSH sessions (pts/ terminals)
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

// UploadFile uploads a single file via SSH stdin pipe
func UploadFile(client *SSHClient, localPath, remotePath string) error {
	if client == nil || client.client == nil {
		return fmt.Errorf("SSH client is not connected")
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

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

func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "unable to authenticate") ||
		strings.Contains(errMsg, "no supported methods remain") ||
		strings.Contains(errMsg, "ssh: handshake failed")
}
