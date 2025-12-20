package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var ErrPersistentAuthFailure = errors.New("persistent SSH authentication failure: remote SSH keys may be missing")

type SSHRetryStatus string

const (
	SSHStatusDialing    SSHRetryStatus = "dial"
	SSHStatusHandshake  SSHRetryStatus = "handshake"
	SSHStatusAuth       SSHRetryStatus = "auth"
	SSHStatusKeyParse   SSHRetryStatus = "key_parse"
	SSHStatusUnexpected SSHRetryStatus = "unexpected"
	SSHStatusSuccess    SSHRetryStatus = "success"
)

const (
	PersistentAuthMaxAttempts = 3
	PersistentAuthTimeout     = 10 * time.Second
)

type SSHRetryInfo struct {
	Status      SSHRetryStatus
	Attempt     int
	MaxAttempts int
	Error       error
	Message     string
}

type SSHProgressCallback func(info SSHRetryInfo)

type SSHConnectOptions struct {
	DetectPersistentAuthFailure bool
}

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
		Timeout:         10 * time.Second,
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
	return RobustSSHConnectWithOptions(ctx, ip, keyFile, port, maxWait, callback, nil)
}

func RobustSSHConnectWithOptions(ctx context.Context, ip, keyFile string, port int, maxWait int, callback SSHProgressCallback, opts *SSHConnectOptions) (*SSHClient, error) {
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
	authBackoff := 500 * time.Millisecond
	maxAuthBackoff := 2 * time.Second
	attempt := 0
	var lastErr error

	detectPersistentAuth := opts != nil && opts.DetectPersistentAuthFailure
	consecutiveAuthFailures := 0
	var firstAuthFailureTime time.Time

	for {
		attempt++
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("SSH connection cancelled")
		}
		if time.Now().After(deadline) {
			return nil, timeoutError(maxWait, lastErr)
		}

		remaining := time.Until(deadline)
		dialTimeout := remaining
		if dialTimeout > 5*time.Second {
			dialTimeout = 5 * time.Second
		}
		if dialTimeout <= 0 {
			return nil, timeoutError(maxWait, lastErr)
		}

		dialer := &net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: 10 * time.Second,
		}

		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr != nil {
			lastErr = dialErr
			consecutiveAuthFailures = 0
			firstAuthFailureTime = time.Time{}

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

		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

		type connResult struct {
			cc    ssh.Conn
			chans <-chan ssh.NewChannel
			reqs  <-chan *ssh.Request
			err   error
		}

		connChan := make(chan connResult, 1)
		go func() {
			cc, chans, reqs, err := ssh.NewClientConn(conn, address, config)
			connChan <- connResult{cc: cc, chans: chans, reqs: reqs, err: err}
		}()

		select {
		case <-ctx.Done():
			conn.Close()
			return nil, fmt.Errorf("SSH connection cancelled")
		case result := <-connChan:
			if result.err == nil {
				_ = conn.SetDeadline(time.Time{})
				if callback != nil {
					callback(SSHRetryInfo{
						Status:  SSHStatusSuccess,
						Attempt: attempt,
						Message: "SSH connection established",
					})
				}
				return &SSHClient{client: ssh.NewClient(result.cc, result.chans, result.reqs)}, nil
			}

			conn.Close()
			lastErr = result.err
			errStatus := ClassifySSHError(result.err)

			if errStatus == SSHStatusAuth {
				consecutiveAuthFailures++
				if firstAuthFailureTime.IsZero() {
					firstAuthFailureTime = time.Now()
				}

				if detectPersistentAuth {
					authFailureDuration := time.Since(firstAuthFailureTime)
					if consecutiveAuthFailures >= PersistentAuthMaxAttempts || authFailureDuration >= PersistentAuthTimeout {
						if callback != nil {
							callback(SSHRetryInfo{
								Status:  SSHStatusAuth,
								Attempt: attempt,
								Error:   ErrPersistentAuthFailure,
								Message: "Persistent authentication failure detected",
							})
						}
						return nil, ErrPersistentAuthFailure
					}
				}
			} else {
				consecutiveAuthFailures = 0
				firstAuthFailureTime = time.Time{}
			}

			if shouldRetrySSH(result.err) {
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
						Error:   result.err,
						Message: msg,
					})
				}

				retryBackoff := backoff
				if errStatus == SSHStatusAuth {
					retryBackoff = authBackoff
				}
				if err := sleepWithContext(ctx, retryBackoff); err != nil {
					return nil, fmt.Errorf("SSH connection cancelled")
				}
				if errStatus == SSHStatusAuth {
					authBackoff = minDuration(authBackoff*2, maxAuthBackoff)
				} else {
					backoff = minDuration(backoff*2, maxBackoff)
				}
				continue
			}

			if callback != nil {
				callback(SSHRetryInfo{
					Status:  errStatus,
					Attempt: attempt,
					Error:   result.err,
					Message: fmt.Sprintf("SSH connection failed: %v", result.err),
				})
			}
			return nil, fmt.Errorf("SSH connection failed: %w", result.err)
		}
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
		if netErr.Timeout() {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return messageContainsAny(msg,
		"connection refused",
		"no route to host",
		"operation timed out",
		"i/o timeout",
		"connection reset",
		"broken pipe",
		"network is unreachable",
	)
}

func shouldRetrySSH(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Network and connection errors
	if messageContainsAny(msg,
		"connection refused",
		"no route to host",
		"operation timed out",
		"i/o timeout",
		"connection reset",
		"kex_exchange_identification",
		"connection closed",
		"handshake failed",
		"kex",
	) {
		return true
	}

	// Auth errors are retried because keys may still be propagating to the instance
	if messageContainsAny(msg,
		"unable to authenticate",
		"no supported methods remain",
	) {
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

// ExecuteSSHCommandStdoutOnly executes a command and returns only stdout, filtering out ld.so.preload errors from stderr
// This prevents stderr pollution from breaking output parsing when /etc/ld.so.preload references a missing binary
func ExecuteSSHCommandStdoutOnly(client *SSHClient, command string) (string, error) {
	if client == nil || client.client == nil {
		return "", fmt.Errorf("SSH client is not connected")
	}

	session, err := client.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := session.Start(command); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Read stdout and stderr concurrently
	var stdoutData, stderrData []byte
	var stderrErr error
	done := make(chan bool, 2)

	go func() {
		var err error
		stdoutData, err = io.ReadAll(stdout)
		if err != nil {
			// Log but don't fail - stderr filtering will handle errors
		}
		done <- true
	}()

	go func() {
		stderrData, stderrErr = io.ReadAll(stderr)
		done <- true
	}()

	// Wait for both reads to complete
	<-done
	<-done

	// Wait for command to finish
	cmdErr := session.Wait()

	// Filter out ld.so.preload errors from stderr (these are benign when binary is missing)
	stderrStr := string(stderrData)
	if stderrErr == nil && stderrStr != "" {
		// Check if stderr contains only ignorable Thunder-specific errors
		stderrLines := strings.Split(strings.TrimSpace(stderrStr), "\n")
		hasNonIgnorableErrors := false
		for _, line := range stderrLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Ignore ld.so.preload errors and other benign Thunder warnings
			isIgnorable := strings.Contains(line, "ld.so: object") ||
				strings.Contains(line, "cannot be preloaded") ||
				strings.Contains(line, "ignored") ||
				strings.Contains(line, "install: cannot remove") ||
				strings.Contains(line, "Device or resource busy") ||
				strings.Contains(line, "chown: changing ownership") ||
				strings.Contains(line, "Read-only file system") ||
				strings.Contains(line, "chown: cannot dereference") ||
				strings.Contains(line, "No such file or directory")
			if !isIgnorable {
				hasNonIgnorableErrors = true
				break
			}
		}
		// If there are non-ignorable errors, return them
		if hasNonIgnorableErrors && cmdErr != nil {
			return "", fmt.Errorf("command failed: %w (stderr: %s)", cmdErr, stderrStr)
		}
	}

	if cmdErr != nil && !strings.Contains(cmdErr.Error(), "exit status") {
		return "", fmt.Errorf("command failed: %w", cmdErr)
	}

	// Return stdout only (stderr errors are filtered/ignored)
	return string(stdoutData), nil
}

// CheckActiveSessions counts active SSH sessions (pts/ terminals)
func CheckActiveSessions(client *SSHClient) (int, error) {
	// Use stdout-only and redirect stderr to avoid ld.so.preload error pollution
	output, err := ExecuteSSHCommandStdoutOnly(client, "who | grep 'pts/' | wc -l 2>/dev/null")
	if err != nil {
		return 0, err
	}

	// Filter out any remaining ld.so.preload errors
	output = filterLdSoErrors(output)

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
	errMsg := strings.ToLower(err.Error())
	return messageContainsAny(errMsg,
		"unable to authenticate",
		"no supported methods remain",
		"ssh: handshake failed",
	)
}

// IsKeyParseError checks if the error is due to a corrupt or invalid private key file
func IsKeyParseError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	return messageContainsAny(errMsg,
		"failed to parse private key",
		"no key found",
		"ssh: no key found",
		"asn1:",
		"illegal base64",
	)
}

// IsNetworkError checks if the error is a network connectivity issue
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return messageContainsAny(msg,
		"connection refused",
		"no route to host",
		"operation timed out",
		"i/o timeout",
		"network is unreachable",
	)
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
	if messageContainsAny(msg, "handshake", "kex", "connection reset") {
		return SSHStatusHandshake
	}
	return SSHStatusUnexpected
}

func messageContainsAny(msg string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}

func timeoutError(maxWait int, lastErr error) error {
	if lastErr != nil {
		return fmt.Errorf("SSH connection timeout after %d seconds: %w", maxWait, lastErr)
	}
	return fmt.Errorf("SSH connection timeout after %d seconds", maxWait)
}

func WaitForTCPPort(ctx context.Context, host string, port int, overallTimeout time.Duration) error {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(overallTimeout)
	backoff := 1 * time.Second
	maxBackoff := 10 * time.Second
	attempt := 0

	for {
		attempt++
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("TCP port check cancelled: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("TCP port %s not available after %v", address, overallTimeout)
		}

		remaining := time.Until(deadline)
		attemptTimeout := remaining
		if attemptTimeout > 5*time.Second {
			attemptTimeout = 5 * time.Second
		}
		if attemptTimeout <= 0 {
			return fmt.Errorf("TCP port %s not available after %v", address, overallTimeout)
		}

		dialer := &net.Dialer{
			Timeout: attemptTimeout,
		}

		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err == nil {
			conn.Close()
			return nil
		}

		// Only retry on connection-related errors
		if !shouldRetryDial(err) {
			return fmt.Errorf("TCP port check failed: %w", err)
		}

		// Exponential backoff with cap
		if err := sleepWithContext(ctx, backoff); err != nil {
			return fmt.Errorf("TCP port check cancelled: %w", err)
		}
		backoff = minDuration(backoff*2, maxBackoff)
	}
}
