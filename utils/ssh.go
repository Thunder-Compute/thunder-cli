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

	"github.com/Thunder-Compute/thunder-cli/internal/clierr"
	"golang.org/x/crypto/ssh"
)

var ErrPersistentAuthFailure = clierr.New("persistent SSH authentication failure")

// ErrSSHUnreachable marks SSH reachability failures (TCP port closed, dial
// timeout, handshake timeout). These indicate the instance is not ready yet
// or the user's network/firewall is blocking the connection — not a CLI bug,
// so they are filtered out of Sentry reporting.
var ErrSSHUnreachable = clierr.New("SSH unreachable")

// ErrKeyUnreadable marks failures to open/read the locally cached SSH private
// key file. The file exists (KeyExists passed) but the OS denied the read —
// typically NTFS ACLs or antivirus blocking access on Windows. This is a local
// environment issue, not a CLI bug, so it is filtered out of Sentry reporting
// and surfaced to the user with actionable guidance.
var ErrKeyUnreadable = clierr.New("cached SSH key could not be read")

type SSHRetryStatus string

const (
	SSHStatusDialing    SSHRetryStatus = "dial"
	SSHStatusHandshake  SSHRetryStatus = "handshake"
	SSHStatusAuth       SSHRetryStatus = "auth"
	SSHStatusKeyParse   SSHRetryStatus = "key_parse"
	SSHStatusUnexpected SSHRetryStatus = "unexpected"
	SSHStatusSuccess    SSHRetryStatus = "success"

	PersistentAuthMaxAttempts = 3
	PersistentAuthTimeout     = 10 * time.Second

	sshInitialBackoff     = time.Second
	sshMaxBackoff         = 10 * time.Second
	sshInitialAuthBackoff = 500 * time.Millisecond
	sshMaxAuthBackoff     = 2 * time.Second
	sshMaxDialTimeout     = 5 * time.Second
	sshHandshakeTimeout   = 10 * time.Second
	sshKeepAlive          = 10 * time.Second
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
	// PersistentAuthTimeout overrides the default timeout for persistent auth
	// failure detection. Use a longer value after key regeneration to allow
	// time for key propagation before declaring failure.
	PersistentAuthTimeout time.Duration
	// PersistentAuthMaxAttempts overrides the consecutive-auth-failure count that
	// trips ErrPersistentAuthFailure. Zero uses the package default
	// (PersistentAuthMaxAttempts); a negative value disables the attempt-count
	// trigger so PersistentAuthTimeout alone governs the grace window. The
	// post-regeneration retry sets this negative — otherwise the 3-strike cap
	// fires after ~3s and the freshly added key never gets its full timeout.
	PersistentAuthMaxAttempts int
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
		return nil, fmt.Errorf("%w: failed to read private key (%w) — this is usually a file-permission or antivirus issue; "+
			"delete that file and reconnect, or check that your security software isn't blocking the .thunder folder", ErrKeyUnreadable, err)
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
		emitSSHProgress(callback, SSHStatusKeyParse, 0, err, "Failed to parse SSH private key")
		return nil, err
	}

	address := net.JoinHostPort(ip, strconv.Itoa(port))
	connectConfig := normalizeSSHConnectOptions(opts)
	retryState := newSSHRetryState(maxWait)

	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, sshConnectionCancelledError()
		}
		if retryState.expired() {
			return nil, timeoutError(maxWait, retryState.lastErr)
		}

		dialTimeout := retryState.dialTimeout()
		if dialTimeout <= 0 {
			return nil, timeoutError(maxWait, retryState.lastErr)
		}

		conn, dialErr := dialSSH(ctx, address, dialTimeout)
		if dialErr != nil {
			retryState.recordDialError(dialErr)

			if shouldRetryDial(dialErr) {
				emitSSHProgress(callback, SSHStatusDialing, attempt, dialErr, "Waiting for instance to be ready...")
				if err := sleepWithContext(ctx, retryState.backoffFor(SSHStatusDialing)); err != nil {
					return nil, sshConnectionCancelledError()
				}
				retryState.advanceBackoff(SSHStatusDialing)
				continue
			}
			return nil, fmt.Errorf("SSH dial failed: %w", dialErr)
		}

		client, sshErr := handshakeSSHClient(ctx, conn, address, config)
		if errors.Is(sshErr, errSSHConnectionCancelled) {
			return nil, sshConnectionCancelledError()
		}
		if sshErr == nil {
			emitSSHProgress(callback, SSHStatusSuccess, attempt, nil, "SSH connection established")
			return client, nil
		}

		errStatus := ClassifySSHError(sshErr)
		retryState.recordSSHError(sshErr, errStatus)

		if errStatus == SSHStatusAuth && retryState.persistentAuthFailed(connectConfig) {
			emitSSHProgress(callback, SSHStatusAuth, attempt, ErrPersistentAuthFailure, "Persistent authentication failure detected")
			return nil, ErrPersistentAuthFailure
		}

		if shouldRetrySSH(sshErr) {
			emitSSHProgress(callback, errStatus, attempt, sshErr, retryMessageForStatus(errStatus))
			if err := sleepWithContext(ctx, retryState.backoffFor(errStatus)); err != nil {
				return nil, sshConnectionCancelledError()
			}
			retryState.advanceBackoff(errStatus)
			continue
		}

		emitSSHProgress(callback, errStatus, attempt, sshErr, fmt.Sprintf("SSH connection failed: %v", sshErr))
		return nil, fmt.Errorf("SSH connection failed: %w", sshErr)
	}
}

type sshConnectConfig struct {
	detectPersistentAuthFailure bool
	persistentAuthTimeout       time.Duration
	persistentAuthMaxAttempts   int
}

func normalizeSSHConnectOptions(opts *SSHConnectOptions) sshConnectConfig {
	config := sshConnectConfig{
		detectPersistentAuthFailure: false,
		persistentAuthTimeout:       PersistentAuthTimeout,
		persistentAuthMaxAttempts:   PersistentAuthMaxAttempts,
	}
	if opts == nil {
		return config
	}

	config.detectPersistentAuthFailure = opts.DetectPersistentAuthFailure
	if opts.PersistentAuthTimeout > 0 {
		config.persistentAuthTimeout = opts.PersistentAuthTimeout
	}
	if opts.PersistentAuthMaxAttempts != 0 {
		config.persistentAuthMaxAttempts = opts.PersistentAuthMaxAttempts
	}
	return config
}

type sshRetryState struct {
	deadline                time.Time
	backoff                 time.Duration
	authBackoff             time.Duration
	lastErr                 error
	consecutiveAuthFailures int
	firstAuthFailureTime    time.Time
}

func newSSHRetryState(maxWait int) *sshRetryState {
	return &sshRetryState{
		deadline:    time.Now().Add(time.Duration(maxWait) * time.Second),
		backoff:     sshInitialBackoff,
		authBackoff: sshInitialAuthBackoff,
	}
}

func (s *sshRetryState) expired() bool {
	return time.Now().After(s.deadline)
}

func (s *sshRetryState) dialTimeout() time.Duration {
	remaining := time.Until(s.deadline)
	if remaining > sshMaxDialTimeout {
		return sshMaxDialTimeout
	}
	return remaining
}

func (s *sshRetryState) recordDialError(err error) {
	s.lastErr = err
	s.resetAuthFailures()
}

func (s *sshRetryState) recordSSHError(err error, status SSHRetryStatus) {
	s.lastErr = err
	if status == SSHStatusAuth {
		s.consecutiveAuthFailures++
		if s.firstAuthFailureTime.IsZero() {
			s.firstAuthFailureTime = time.Now()
		}
		return
	}
	s.resetAuthFailures()
}

func (s *sshRetryState) resetAuthFailures() {
	s.consecutiveAuthFailures = 0
	s.firstAuthFailureTime = time.Time{}
}

func (s *sshRetryState) persistentAuthFailed(config sshConnectConfig) bool {
	if !config.detectPersistentAuthFailure {
		return false
	}

	authFailureDuration := time.Since(s.firstAuthFailureTime)
	hitAttemptLimit := config.persistentAuthMaxAttempts > 0 && s.consecutiveAuthFailures >= config.persistentAuthMaxAttempts
	hitTimeLimit := authFailureDuration >= config.persistentAuthTimeout
	return hitAttemptLimit || hitTimeLimit
}

func (s *sshRetryState) backoffFor(status SSHRetryStatus) time.Duration {
	if status == SSHStatusAuth {
		return s.authBackoff
	}
	return s.backoff
}

func (s *sshRetryState) advanceBackoff(status SSHRetryStatus) {
	if status == SSHStatusAuth {
		s.authBackoff = minDuration(s.authBackoff*2, sshMaxAuthBackoff)
		return
	}
	s.backoff = minDuration(s.backoff*2, sshMaxBackoff)
}

func emitSSHProgress(callback SSHProgressCallback, status SSHRetryStatus, attempt int, err error, message string) {
	if callback == nil {
		return
	}
	callback(SSHRetryInfo{
		Status:  status,
		Attempt: attempt,
		Error:   err,
		Message: message,
	})
}

func retryMessageForStatus(status SSHRetryStatus) string {
	switch status {
	case SSHStatusAuth:
		return "Authentication failed, retrying..."
	case SSHStatusHandshake:
		return "SSH handshake failed, retrying..."
	case SSHStatusDialing:
		return "Connection interrupted, retrying..."
	default:
		return "Retrying SSH connection..."
	}
}

func dialSSH(ctx context.Context, address string, timeout time.Duration) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: sshKeepAlive,
	}
	return dialer.DialContext(ctx, "tcp", address)
}

var errSSHConnectionCancelled = errors.New("SSH connection cancelled")

func sshConnectionCancelledError() error {
	return fmt.Errorf("SSH connection cancelled")
}

type sshHandshakeResult struct {
	cc    ssh.Conn
	chans <-chan ssh.NewChannel
	reqs  <-chan *ssh.Request
	err   error
}

func handshakeSSHClient(ctx context.Context, conn net.Conn, address string, config *ssh.ClientConfig) (*SSHClient, error) {
	_ = conn.SetDeadline(time.Now().Add(sshHandshakeTimeout))

	connChan := make(chan sshHandshakeResult, 1)
	go func() {
		cc, chans, reqs, err := ssh.NewClientConn(conn, address, config)
		connChan <- sshHandshakeResult{cc: cc, chans: chans, reqs: reqs, err: err}
	}()

	select {
	case <-ctx.Done():
		conn.Close()
		return nil, errSSHConnectionCancelled
	case result := <-connChan:
		if result.err != nil {
			conn.Close()
			return nil, result.err
		}
		_ = conn.SetDeadline(time.Time{})
		return &SSHClient{client: ssh.NewClient(result.cc, result.chans, result.reqs)}, nil
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
		return fmt.Errorf("%w: SSH connection timeout after %d seconds: %w", ErrSSHUnreachable, maxWait, lastErr)
	}
	return fmt.Errorf("%w: SSH connection timeout after %d seconds", ErrSSHUnreachable, maxWait)
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
			return fmt.Errorf("%w: TCP port check cancelled: %w", ErrSSHUnreachable, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: TCP port %s not available after %v", ErrSSHUnreachable, address, overallTimeout)
		}

		remaining := time.Until(deadline)
		attemptTimeout := remaining
		if attemptTimeout > 5*time.Second {
			attemptTimeout = 5 * time.Second
		}
		if attemptTimeout <= 0 {
			return fmt.Errorf("%w: TCP port %s not available after %v", ErrSSHUnreachable, address, overallTimeout)
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
			return fmt.Errorf("%w: TCP port check failed: %w", ErrSSHUnreachable, err)
		}

		// Exponential backoff with cap
		if err := sleepWithContext(ctx, backoff); err != nil {
			return fmt.Errorf("%w: TCP port check cancelled: %w", ErrSSHUnreachable, err)
		}
		backoff = minDuration(backoff*2, maxBackoff)
	}
}
