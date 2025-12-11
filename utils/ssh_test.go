package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// testSSHServer represents a test SSH server
type testSSHServer struct {
	listener net.Listener
	config   *ssh.ServerConfig
	hostKey  ssh.Signer
	port     int
	stop     chan struct{}
}

// setupTestEnvironment creates a temporary directory and sets HOME environment variable
func setupTestEnvironment(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	cleanup := func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		} else {
			os.Unsetenv("HOME")
		}
	}

	return tmpDir, cleanup
}

// generateRSAKeyPair generates an RSA key pair for testing
// Returns the private key, signer, and public key
func generateRSAKeyPair(t *testing.T) (*rsa.PrivateKey, ssh.Signer, ssh.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)

	publicKey := signer.PublicKey()
	return privateKey, signer, publicKey
}

// savePrivateKeyToFile saves an RSA private key to a file in PEM format
func savePrivateKeyToFile(t *testing.T, privateKey *rsa.PrivateKey, path string) {
	der := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	keyData := pem.EncodeToMemory(pemBlock)

	err := os.WriteFile(path, keyData, 0600)
	require.NoError(t, err)
}

// setupSSHTestServer creates and starts an SSH test server that accepts connections
// using the provided client public key. Returns the server instance and a cleanup function.
func setupSSHTestServer(t *testing.T, clientPublicKey ssh.PublicKey) (*testSSHServer, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server, cleanup, err := startSSHTestServerWithListener(listener, clientPublicKey)
	require.NoError(t, err)
	return server, cleanup
}

func startSSHTestServerWithListener(listener net.Listener, clientPublicKey ssh.PublicKey) (*testSSHServer, func(), error) {
	hostSigner, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	hostKeySigner, err := ssh.NewSignerFromKey(hostSigner)
	if err != nil {
		return nil, nil, err
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), clientPublicKey.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("public key not accepted")
		},
	}
	config.AddHostKey(hostKeySigner)

	addr := listener.Addr().(*net.TCPAddr)
	server := &testSSHServer{
		listener: listener,
		config:   config,
		hostKey:  hostKeySigner,
		port:     addr.Port,
		stop:     make(chan struct{}),
	}

	go server.serve()

	time.Sleep(100 * time.Millisecond)

	cleanup := func() {
		close(server.stop)
		listener.Close()
	}

	return server, cleanup, nil
}

func (s *testSSHServer) serve() {
	for {
		select {
		case <-s.stop:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stop:
				return
			default:
				continue
			}
		}

		go s.handleConn(conn)
	}
}

func (s *testSSHServer) handleConn(conn net.Conn) {
	defer conn.Close()

	_, chans, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		return
	}

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		for req := range requests {
			if req.Type == "exec" {
				_ = req.Reply(true, nil)
				exitStatus := []byte{0, 0, 0, 0}
				_, _ = channel.SendRequest("exit-status", false, exitStatus)
				channel.Close()
				break
			}
			_ = req.Reply(false, nil)
		}
		channel.Close()
	}
}

// readKnownHosts reads the known_hosts file and returns its contents
func readKnownHosts(t *testing.T, knownHostsPath string) string {
	data, err := os.ReadFile(knownHostsPath)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(data)
}

// createKnownHostsEntry creates a known_hosts entry for a given host and key
func createKnownHostsEntry(t *testing.T, knownHostsPath string, host string, key ssh.PublicKey) {
	line := knownhosts.Line([]string{host}, key)

	var existingData []byte
	if data, err := os.ReadFile(knownHostsPath); err == nil {
		existingData = data
	}

	newData := append(existingData, []byte(line+"\n")...)
	err := os.WriteFile(knownHostsPath, newData, 0644)
	require.NoError(t, err)
}

func TestSSHKnownHosts(t *testing.T) {
	// Test that connections succeed to unknown hosts without modifying known_hosts.
	// With known hosts verification disabled, the known_hosts file should remain empty.
	t.Run("unknown host auto-add", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnvironment(t)
		defer cleanup()

		clientPrivateKey, _, clientPublicKey := generateRSAKeyPair(t)
		keyFile := filepath.Join(tmpDir, "test_key")
		savePrivateKeyToFile(t, clientPrivateKey, keyFile)

		server, serverCleanup := setupSSHTestServer(t, clientPublicKey)
		defer serverCleanup()

		knownHostsPath := filepath.Join(tmpDir, ".ssh", "known_hosts")
		_ = os.MkdirAll(filepath.Dir(knownHostsPath), 0700)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, server.port, 5)
		require.NoError(t, err, "connection should succeed")
		require.NotNil(t, client, "client should not be nil")
		defer client.Close()

		knownHostsContent := readKnownHosts(t, knownHostsPath)
		assert.Empty(t, knownHostsContent, "known_hosts should not be modified when verification is disabled")
	})

	// Test that connections succeed to known hosts with matching keys.
	// The connection should succeed regardless of known_hosts file state since verification is disabled.
	t.Run("known host with matching key", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnvironment(t)
		defer cleanup()

		clientPrivateKey, _, clientPublicKey := generateRSAKeyPair(t)
		keyFile := filepath.Join(tmpDir, "test_key")
		savePrivateKeyToFile(t, clientPrivateKey, keyFile)

		server, serverCleanup := setupSSHTestServer(t, clientPublicKey)
		defer serverCleanup()

		knownHostsPath := filepath.Join(tmpDir, ".ssh", "known_hosts")
		_ = os.MkdirAll(filepath.Dir(knownHostsPath), 0700)
		createKnownHostsEntry(t, knownHostsPath, "127.0.0.1", server.hostKey.PublicKey())

		initialContent := readKnownHosts(t, knownHostsPath)
		initialEntryCount := strings.Count(initialContent, "127.0.0.1")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, server.port, 5)
		require.NoError(t, err, "connection should succeed with matching key")
		defer client.Close()

		finalContent := readKnownHosts(t, knownHostsPath)
		finalEntryCount := strings.Count(finalContent, "127.0.0.1")

		assert.GreaterOrEqual(t, finalEntryCount, initialEntryCount,
			"known_hosts should contain at least the initial entry")
	})

	// Test that connections succeed even when known_hosts contains a mismatched key.
	// This verifies that known hosts verification is disabled and connections are not blocked by key mismatches.
	t.Run("known host with mismatched key should succeed after fix", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnvironment(t)
		defer cleanup()

		clientPrivateKey, _, clientPublicKey := generateRSAKeyPair(t)
		keyFile := filepath.Join(tmpDir, "test_key")
		savePrivateKeyToFile(t, clientPrivateKey, keyFile)

		server, serverCleanup := setupSSHTestServer(t, clientPublicKey)
		defer serverCleanup()

		knownHostsPath := filepath.Join(tmpDir, ".ssh", "known_hosts")
		_ = os.MkdirAll(filepath.Dir(knownHostsPath), 0700)

		_, _, wrongKey := generateRSAKeyPair(t)
		hostnameWithPort := fmt.Sprintf("127.0.0.1:%d", server.port)
		createKnownHostsEntry(t, knownHostsPath, hostnameWithPort, wrongKey)

		wrongKeyMarshal := wrongKey.Marshal()
		serverKeyMarshal := server.hostKey.PublicKey().Marshal()
		assert.NotEqual(t, wrongKeyMarshal, serverKeyMarshal, "test keys should be different")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, server.port, 5)
		require.NoError(t, err, "connection should succeed even with mismatched key (known hosts verification disabled)")
		require.NotNil(t, client, "client should not be nil")
		defer client.Close()

		assert.NotNil(t, client.GetClient(), "SSH client should be established")
	})

	// Test that connections succeed when using hostname with port format.
	// Verifies that known hosts verification is disabled regardless of hostname format.
	t.Run("hostname with port", func(t *testing.T) {
		tmpDir, cleanup := setupTestEnvironment(t)
		defer cleanup()

		clientPrivateKey, _, clientPublicKey := generateRSAKeyPair(t)
		keyFile := filepath.Join(tmpDir, "test_key")
		savePrivateKeyToFile(t, clientPrivateKey, keyFile)

		server, serverCleanup := setupSSHTestServer(t, clientPublicKey)
		defer serverCleanup()

		knownHostsPath := filepath.Join(tmpDir, ".ssh", "known_hosts")
		_ = os.MkdirAll(filepath.Dir(knownHostsPath), 0700)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, server.port, 5)
		require.NoError(t, err, "connection should succeed")
		require.NotNil(t, client, "client should not be nil")
		defer client.Close()

		knownHostsContent := readKnownHosts(t, knownHostsPath)
		assert.Empty(t, knownHostsContent, "known_hosts should not be modified when verification is disabled")
	})
}

func TestRobustSSHConnectCtxRetriesUntilServerAvailable(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientPrivateKey, _, clientPublicKey := generateRSAKeyPair(t)
	keyFile := filepath.Join(tmpDir, "retry_key")
	savePrivateKeyToFile(t, clientPrivateKey, keyFile)

	// Reserve a port and release it so initial dials see connection refused.
	tmpListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := tmpListener.Addr().(*net.TCPAddr).Port
	tmpListener.Close()

	serverReady := make(chan func(), 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			serverReady <- func() {}
			return
		}
		server, srvCleanup, err := startSSHTestServerWithListener(listener, clientPublicKey)
		if err != nil {
			listener.Close()
			serverReady <- func() {}
			return
		}
		serverReady <- func() {
			srvCleanup()
		}
		_ = server // keep reference alive until cleanup executes
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, port, 5)
	require.NoError(t, err)
	require.NotNil(t, client)
	client.Close()

	cleanupServer := <-serverReady
	cleanupServer()
}

func TestRobustSSHConnectCtxHonorsContextCancellation(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientPrivateKey, _, _ := generateRSAKeyPair(t)
	keyFile := filepath.Join(tmpDir, "cancel_key")
	savePrivateKeyToFile(t, clientPrivateKey, keyFile)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, 65500, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestVerifySSHConnectionCtxSuccess(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientPrivateKey, _, clientPublicKey := generateRSAKeyPair(t)
	keyFile := filepath.Join(tmpDir, "verify_key")
	savePrivateKeyToFile(t, clientPrivateKey, keyFile)

	server, serverCleanup := setupSSHTestServer(t, clientPublicKey)
	defer serverCleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := VerifySSHConnectionCtx(ctx, "127.0.0.1", keyFile, server.port)
	require.NoError(t, err)
}

func TestVerifySSHConnectionCtxRespectsContext(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientPrivateKey, _, _ := generateRSAKeyPair(t)
	keyFile := filepath.Join(tmpDir, "verify_cancel_key")
	savePrivateKeyToFile(t, clientPrivateKey, keyFile)

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	err := VerifySSHConnectionCtx(ctx, "127.0.0.1", keyFile, 65501)
	require.Error(t, err)
	// Error can be either "SSH verification failed" or "SSH connection cancelled" depending on timing
	assert.True(t, strings.Contains(err.Error(), "SSH verification failed") ||
		strings.Contains(err.Error(), "SSH connection cancelled"),
		"unexpected error: %s", err.Error())
}

func TestNewSSHConfigErrors(t *testing.T) {
	t.Run("missing key file", func(t *testing.T) {
		_, err := newSSHConfig("ubuntu", filepath.Join(t.TempDir(), "missing"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read private key")
	})

	t.Run("invalid key contents", func(t *testing.T) {
		dir := t.TempDir()
		keyPath := filepath.Join(dir, "bad_key")
		require.NoError(t, os.WriteFile(keyPath, []byte("not a key"), 0600))
		_, err := newSSHConfig("ubuntu", keyPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse private key")
	})
}

func TestRobustSSHConnectCtxTimeoutVsCancellation(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientPrivateKey, _, _ := generateRSAKeyPair(t)
	keyFile := filepath.Join(tmpDir, "timeout_key")
	savePrivateKeyToFile(t, clientPrivateKey, keyFile)

	t.Run("timeout", func(t *testing.T) {
		ctx := context.Background()
		_, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, 65520, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_, err := RobustSSHConnectCtx(ctx, "127.0.0.1", keyFile, 65521, 5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cancelled")
	})
}

func TestRobustSSHConnectCtxAuthErrorRetries(t *testing.T) {
	// Auth errors are now retried (key may still be propagating to instance)
	// so this test verifies the retry behavior with a wrong key eventually times out
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	acceptedKey, acceptedSigner, acceptedPublic := generateRSAKeyPair(t)
	_ = acceptedSigner // keep for clarity
	rejectedKey, _, _ := generateRSAKeyPair(t)

	acceptedKeyPath := filepath.Join(tmpDir, "accepted")
	savePrivateKeyToFile(t, acceptedKey, acceptedKeyPath)

	rejectedKeyPath := filepath.Join(tmpDir, "rejected")
	savePrivateKeyToFile(t, rejectedKey, rejectedKeyPath)

	server, serverCleanup := setupSSHTestServer(t, acceptedPublic)
	defer serverCleanup()

	// Use a short timeout since auth errors are now retried
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := RobustSSHConnectCtx(ctx, "127.0.0.1", rejectedKeyPath, server.port, 3)
	require.Error(t, err)
	// Should either timeout or be cancelled (auth errors are retried until timeout)
	assert.True(t, strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "cancelled"),
		"unexpected error: %s", err.Error())
}

// customNetError implements net.Error with configurable behavior
type customNetError struct {
	msg       string
	timeout   bool
	temporary bool
}

func (e *customNetError) Error() string   { return e.msg }
func (e *customNetError) Timeout() bool   { return e.timeout }
func (e *customNetError) Temporary() bool { return e.temporary }

func TestShouldRetryDial(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, shouldRetryDial(nil))
	})

	t.Run("timeout net error", func(t *testing.T) {
		err := &customNetError{msg: "timeout", timeout: true}
		assert.True(t, shouldRetryDial(err))
	})

	t.Run("temporary net error", func(t *testing.T) {
		assert.True(t, shouldRetryDial(fmt.Errorf("dial tcp 1.2.3.4:22: connection reset by peer")))
	})

	t.Run("connection refused", func(t *testing.T) {
		assert.True(t, shouldRetryDial(fmt.Errorf("dial tcp 1.2.3.4:22: connection refused")))
	})

	t.Run("no route to host", func(t *testing.T) {
		assert.True(t, shouldRetryDial(fmt.Errorf("dial tcp 1.2.3.4:22: no route to host")))
	})

	t.Run("operation timed out", func(t *testing.T) {
		assert.True(t, shouldRetryDial(fmt.Errorf("dial tcp: operation timed out")))
	})

	t.Run("non retryable", func(t *testing.T) {
		assert.False(t, shouldRetryDial(fmt.Errorf("ssh: authentication failed")))
	})
}

func TestShouldRetrySSH(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		assert.False(t, shouldRetrySSH(nil))
	})

	t.Run("connection closed", func(t *testing.T) {
		assert.True(t, shouldRetrySSH(fmt.Errorf("ssh: connection closed")))
	})

	t.Run("kex exchange issue", func(t *testing.T) {
		assert.True(t, shouldRetrySSH(fmt.Errorf("kex_exchange_identification: Connection closed")))
	})

	t.Run("handshake failed", func(t *testing.T) {
		assert.True(t, shouldRetrySSH(fmt.Errorf("ssh: handshake failed")))
	})

	t.Run("auth errors are retryable", func(t *testing.T) {
		// Auth errors are now retried (key may still be propagating to instance)
		assert.True(t, shouldRetrySSH(fmt.Errorf("ssh: unable to authenticate")))
	})

	t.Run("non retryable", func(t *testing.T) {
		// Some errors should not be retried
		assert.False(t, shouldRetrySSH(fmt.Errorf("some random unrelated error")))
	})
}

func TestErrPersistentAuthFailure(t *testing.T) {
	t.Run("errors.Is works with ErrPersistentAuthFailure", func(t *testing.T) {
		assert.True(t, errors.Is(ErrPersistentAuthFailure, ErrPersistentAuthFailure))
	})

	t.Run("errors.Is works with wrapped ErrPersistentAuthFailure", func(t *testing.T) {
		wrapped := fmt.Errorf("connection failed: %w", ErrPersistentAuthFailure)
		assert.True(t, errors.Is(wrapped, ErrPersistentAuthFailure))
	})

	t.Run("errors.Is returns false for other errors", func(t *testing.T) {
		assert.False(t, errors.Is(fmt.Errorf("some other error"), ErrPersistentAuthFailure))
		assert.False(t, errors.Is(fmt.Errorf("ssh: unable to authenticate"), ErrPersistentAuthFailure))
		assert.False(t, errors.Is(nil, ErrPersistentAuthFailure))
	})
}

func TestRobustSSHConnectWithOptionsPersistentAuthDetection(t *testing.T) {
	// Test that persistent auth failure is detected when DetectPersistentAuthFailure is enabled
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Use accepted key for server, but connect with rejected key to trigger auth failures
	acceptedKey, _, acceptedPublic := generateRSAKeyPair(t)
	rejectedKey, _, _ := generateRSAKeyPair(t)
	_ = acceptedKey // keep reference

	rejectedKeyPath := filepath.Join(tmpDir, "rejected")
	savePrivateKeyToFile(t, rejectedKey, rejectedKeyPath)

	server, serverCleanup := setupSSHTestServer(t, acceptedPublic)
	defer serverCleanup()

	t.Run("persistent auth failure detected when enabled", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		opts := &SSHConnectOptions{
			DetectPersistentAuthFailure: true,
		}

		callbackCalled := false
		callback := func(info SSHRetryInfo) {
			callbackCalled = true
		}

		_, err := RobustSSHConnectWithOptions(ctx, "127.0.0.1", rejectedKeyPath, server.port, 60, callback, opts)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrPersistentAuthFailure), "should return ErrPersistentAuthFailure, got: %v", err)
		assert.True(t, callbackCalled, "callback should have been called")
	})

	t.Run("auth failures retry until timeout when detection disabled", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// No options means DetectPersistentAuthFailure is false
		_, err := RobustSSHConnectWithOptions(ctx, "127.0.0.1", rejectedKeyPath, server.port, 3, nil, nil)

		require.Error(t, err)
		assert.False(t, errors.Is(err, ErrPersistentAuthFailure), "should NOT return ErrPersistentAuthFailure")
		// Should timeout or be cancelled instead
		assert.True(t, strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "cancelled"),
			"expected timeout or cancelled, got: %v", err)
	})
}

func TestPersistentAuthThresholds(t *testing.T) {
	// Verify the constants are sensible
	assert.GreaterOrEqual(t, PersistentAuthMaxAttempts, 3, "should require at least 3 attempts")
	assert.LessOrEqual(t, PersistentAuthMaxAttempts, 10, "should not require too many attempts")
	assert.GreaterOrEqual(t, PersistentAuthTimeout, 10*time.Second, "should wait at least 10 seconds")
	assert.LessOrEqual(t, PersistentAuthTimeout, 60*time.Second, "should not wait more than 60 seconds")
}
