package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// SessionConfig holds configuration for an interactive SSH session.
type SessionConfig struct {
	Client *SSHClient
	Ports  []int    // local ports to forward (each maps localhost:port -> remote localhost:port)
	Stdin  *os.File // typically os.Stdin
	Stdout *os.File // typically os.Stdout
	Stderr *os.File // typically os.Stderr
}

// RunInteractiveSession starts a PTY-allocated shell session with local port forwarding.
// It blocks until the session ends (user exits shell, connection drops, or ctx is cancelled).
func RunInteractiveSession(ctx context.Context, cfg SessionConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sshClient := cfg.Client.GetClient()

	// Start port forwarding before raw mode so bind errors print cleanly.
	for _, port := range cfg.Ports {
		if err := startPortForward(ctx, sshClient, port); err != nil {
			fmt.Fprintf(cfg.Stderr, "Warning: could not forward port %d: %v\n", port, err)
		}
	}

	// Get terminal size before entering raw mode.
	// Use stdout fd — on Windows, the screen buffer size is on the output handle.
	outFd := int(cfg.Stdout.Fd())
	width, height, err := term.GetSize(outFd)
	if err != nil {
		width, height = 80, 24
	}

	fd := int(cfg.Stdin.Fd())
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return fmt.Errorf("failed to set raw terminal mode: %w", err)
		}
		defer term.Restore(fd, oldState)
	}

	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	sessionStdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	sessionStdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	sessionStderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	go startKeepalive(ctx, sshClient)
	go watchWindowSize(ctx, outFd, session)

	// Only wait for stdout/stderr — stdin blocks on os.Stdin.Read() which
	// won't unblock when the remote session ends.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(cfg.Stdout, sessionStdout)
	}()
	go func() {
		defer wg.Done()
		io.Copy(cfg.Stderr, sessionStderr)
	}()
	go pipeStdin(cfg.Stdin, sessionStdin)

	err = session.Wait()
	cancel()
	wg.Wait()

	return err
}

// startPortForward sets up local port forwarding: listens on 127.0.0.1:localPort
// and tunnels each connection through the SSH client to localhost:localPort on the remote.
func startPortForward(ctx context.Context, sshClient *ssh.Client, localPort int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", localPort, err)
	}

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	go func() {
		for {
			localConn, err := listener.Accept()
			if err != nil {
				return // listener closed
			}
			go forwardConnection(sshClient, localConn, localPort)
		}
	}()

	return nil
}

// startKeepalive sends periodic keepalives to keep the connection alive and
// detect when it's dead. Each request runs in a goroutine with a timeout so
// that a single lost reply packet doesn't block the loop - without this,
// SendRequest blocks for minutes (waiting for TCP retransmission to give up),
// during which no keepalives are sent and NAT/firewall entries can expire.
// The connection is only considered dead after multiple consecutive failures.
func startKeepalive(ctx context.Context, client *ssh.Client) {
	const (
		keepaliveInterval    = 15 * time.Second
		keepaliveTimeout     = 10 * time.Second
		maxConsecutiveMisses = 3
	)
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	consecutiveMisses := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			done := make(chan error, 1)
			go func() {
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					return // transport is dead
				}
				consecutiveMisses = 0
			case <-time.After(keepaliveTimeout):
				consecutiveMisses++
				if consecutiveMisses >= maxConsecutiveMisses {
					return // connection is dead
				}
				// Reply was lost but connection may still be alive - keep trying.
				// The next SendRequest will generate fresh traffic to keep NAT entries alive.
			case <-ctx.Done():
				return
			}
		}
	}
}

// pipeStdin copies local stdin to the remote session stdin. Uses a manual
// read loop instead of io.Copy because on Windows, Ctrl+Z causes ReadConsole
// to return EOF — we must keep reading after that or the user can never type again.
func pipeStdin(local *os.File, remote io.WriteCloser) {
	buf := make([]byte, 1024)
	for {
		n, err := local.Read(buf)
		if n > 0 {
			if _, writeErr := remote.Write(buf[:n]); writeErr != nil {
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				continue
			}
			remote.Close()
			return
		}
	}
}

func forwardConnection(sshClient *ssh.Client, localConn net.Conn, remotePort int) {
	defer localConn.Close()

	remoteConn, err := sshClient.Dial("tcp", fmt.Sprintf("localhost:%d", remotePort))
	if err != nil {
		return
	}
	defer remoteConn.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()
	<-done
}
