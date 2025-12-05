package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHRetryStatus string

const (
	SSHStatusDialing    SSHRetryStatus = "dial"
	SSHStatusHandshake  SSHRetryStatus = "handshake"
	SSHStatusAuth       SSHRetryStatus = "auth"
	SSHStatusKeyParse   SSHRetryStatus = "key_parse"
	SSHStatusUnexpected SSHRetryStatus = "unexpected"
	SSHStatusSuccess    SSHRetryStatus = "success"
)

type SSHRetryInfo struct {
	Status      SSHRetryStatus
	Attempt     int
	MaxAttempts int
	Error       error
	Message     string
}

type SSHProgressCallback func(info SSHRetryInfo)

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

func newSSHConfig(user, keyFile string) (*ssh.ClientConfig, error) {
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}, nil
}

func RobustSSHConnect(ip, keyFile string, port int, maxWait int) (*SSHClient, error) {
	return RobustSSHConnectCtx(context.Background(), ip, keyFile, port, maxWait)
}

func RobustSSHConnectCtx(ctx context.Context, ip, keyFile string, port int, maxWait int) (*SSHClient, error) {
	return RobustSSHConnectWithProgress(ctx, ip, keyFile, port, maxWait, nil)
}

// RobustSSHConnectWithProgress establishes an SSH connection with retry logic and progress callbacks.
// The callback is invoked on each retry attempt with structured status information.
func RobustSSHConnectWithProgress(ctx context.Context, ip, keyFile string, port int, maxWait int, callback SSHProgressCallback) (*SSHClient, error) {
	config, err := newSSHConfig("ubuntu", keyFile)
	if err != nil {
		if callback != nil {
			callback(SSHRetryInfo{
				Status:  SSHStatusKeyParse,
				Attempt: 0,
				Error:   err,
				Message: "Failed to parse SSH private key",
			})
		}
		return nil, err
	}

	address := net.JoinHostPort(ip, strconv.Itoa(port))
	deadline := time.Now().Add(time.Duration(maxWait) * time.Second)
	backoff := time.Second
	maxBackoff := 10 * time.Second
	attempt := 0
	var lastErr error

	for {
		attempt++
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("SSH connection cancelled")
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return nil, fmt.Errorf("SSH connection timeout after %d seconds: %w", maxWait, lastErr)
			}
			return nil, fmt.Errorf("SSH connection timeout after %d seconds", maxWait)
		}

		remaining := time.Until(deadline)
		dialTimeout := remaining
		if dialTimeout > 10*time.Second {
			dialTimeout = 10 * time.Second
		}
		if dialTimeout <= 0 {
			if lastErr != nil {
				return nil, fmt.Errorf("SSH connection timeout after %d seconds: %w", maxWait, lastErr)
			}
			return nil, fmt.Errorf("SSH connection timeout after %d seconds", maxWait)
		}

		dialer := &net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 10 * time.Second,
		}

		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr != nil {
			lastErr = dialErr
			if shouldRetryDial(dialErr) {
				if callback != nil {
					callback(SSHRetryInfo{
						Status:  SSHStatusDialing,
						Attempt: attempt,
						Error:   dialErr,
						Message: "Waiting for instance to be ready...",
					})
				}
				if err := sleepWithContext(ctx, backoff); err != nil {
					return nil, fmt.Errorf("SSH connection cancelled")
				}
				backoff = minDuration(backoff*2, maxBackoff)
				continue
			}
			return nil, fmt.Errorf("SSH dial failed: %w", dialErr)
		}

		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
		cc, chans, reqs, err := ssh.NewClientConn(conn, address, config)
		if err == nil {
			_ = conn.SetDeadline(time.Time{})
			if callback != nil {
				callback(SSHRetryInfo{
					Status:  SSHStatusSuccess,
					Attempt: attempt,
					Message: "SSH connection established",
				})
			}
			return &SSHClient{client: ssh.NewClient(cc, chans, reqs)}, nil
		}

		conn.Close()
		lastErr = err
		errStatus := ClassifySSHError(err)

		if shouldRetrySSH(err) {
			if callback != nil {
				msg := "Retrying SSH connection..."
				switch errStatus {
				case SSHStatusAuth:
					msg = "Authentication failed, retrying..."
				case SSHStatusHandshake:
					msg = "SSH handshake failed, retrying..."
				case SSHStatusDialing:
					msg = "Connection interrupted, retrying..."
				}
				callback(SSHRetryInfo{
					Status:  errStatus,
					Attempt: attempt,
					Error:   err,
					Message: msg,
				})
			}
			if err := sleepWithContext(ctx, backoff); err != nil {
				return nil, fmt.Errorf("SSH connection cancelled")
			}
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}

		if callback != nil {
			callback(SSHRetryInfo{
				Status:  errStatus,
				Attempt: attempt,
				Error:   err,
				Message: fmt.Sprintf("SSH connection failed: %v", err),
			})
		}
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func shouldRetryDial(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "operation timed out") ||
		strings.Contains(msg, "i/o timeout")
}

func shouldRetrySSH(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Network and connection errors
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "operation timed out") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "kex_exchange_identification") ||
		strings.Contains(msg, "connection closed") ||
		strings.Contains(msg, "handshake failed") ||
		strings.Contains(msg, "kex") {
		return true
	}

	// Auth errors are retried because keys may still be propagating to the instance
	if strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain") {
		return true
	}
	return false
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if ctx == nil {
		time.Sleep(d)
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// VerifySSHConnectionCtx ensures a fresh SSH connection succeeds before we hand
// control off to the system SSH binary.
func VerifySSHConnectionCtx(ctx context.Context, ip, keyFile string, port int) error {
	const (
		maxAttempts = 3
		retryDelay  = 2 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		client, err := RobustSSHConnectCtx(ctx, ip, keyFile, port, 30)
		if err == nil {
			_, cmdErr := ExecuteSSHCommand(client, "true")
			client.Close()
			if cmdErr == nil {
				return nil
			}
			lastErr = cmdErr
		} else {
			lastErr = err
		}

		if attempt < maxAttempts {
			if err := sleepWithContext(ctx, retryDelay); err != nil {
				return fmt.Errorf("SSH connection cancelled")
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown verification error")
	}

	return fmt.Errorf("SSH verification failed: %w", lastErr)
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

// IsKeyParseError checks if the error is due to a corrupt or invalid private key file
func IsKeyParseError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "failed to parse private key") ||
		strings.Contains(errMsg, "no key found") ||
		strings.Contains(errMsg, "ssh: no key found") ||
		strings.Contains(errMsg, "asn1:") ||
		strings.Contains(errMsg, "illegal base64")
}

// IsNetworkError checks if the error is a network connectivity issue
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "operation timed out") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "network is unreachable")
}

// ClassifySSHError determines the type of SSH error for reporting
func ClassifySSHError(err error) SSHRetryStatus {
	if err == nil {
		return SSHStatusSuccess
	}
	if IsKeyParseError(err) {
		return SSHStatusKeyParse
	}
	if IsAuthError(err) {
		return SSHStatusAuth
	}
	if IsNetworkError(err) {
		return SSHStatusDialing
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "handshake") ||
		strings.Contains(msg, "kex") ||
		strings.Contains(msg, "connection reset") {
		return SSHStatusHandshake
	}
	return SSHStatusUnexpected
}
