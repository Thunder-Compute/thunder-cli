package utils

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
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
	hostSigner, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	hostKeySigner, err := ssh.NewSignerFromKey(hostSigner)
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if bytes.Equal(key.Marshal(), clientPublicKey.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("public key not accepted")
		},
	}
	config.AddHostKey(hostKeySigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := listener.Addr().(*net.TCPAddr)
	port := addr.Port

	server := &testSSHServer{
		listener: listener,
		config:   config,
		hostKey:  hostKeySigner,
		port:     port,
		stop:     make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-server.stop:
				return
			default:
				conn, err := listener.Accept()
				if err != nil {
					select {
					case <-server.stop:
						return
					default:
						continue
					}
				}

				go func() {
					_, chans, reqs, err := ssh.NewServerConn(conn, config)
					if err != nil {
						return
					}

					go ssh.DiscardRequests(reqs)

					for newChannel := range chans {
						if newChannel.ChannelType() != "session" {
							newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
							continue
						}

						channel, _, err := newChannel.Accept()
						if err != nil {
							continue
						}

						channel.Close()
					}
				}()
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	cleanup := func() {
		close(server.stop)
		listener.Close()
	}

	return server, cleanup
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

// createMismatchedKnownHostsEntry creates a known_hosts entry with a different key
// than the server's actual host key, used for testing key mismatch scenarios.
func createMismatchedKnownHostsEntry(t *testing.T, knownHostsPath string, host string) {
	_, _, wrongKey := generateRSAKeyPair(t)
	createKnownHostsEntry(t, knownHostsPath, host, wrongKey)
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
		os.MkdirAll(filepath.Dir(knownHostsPath), 0700)

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
		os.MkdirAll(filepath.Dir(knownHostsPath), 0700)
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
		os.MkdirAll(filepath.Dir(knownHostsPath), 0700)

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
		os.MkdirAll(filepath.Dir(knownHostsPath), 0700)

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
