/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

const (
	authURL     = "https://console.thundercompute.com/login/vscode"
	callbackURL = "http://127.0.0.1"
)

type AuthResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
}

type Config struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Thunder Compute",
	Long:  `Login to Thunder Compute by authenticating through your browser. This will open your default browser to complete the authentication process.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runLogin(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin() error {
	state, err := generateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	port, authChan, errChan, cleanup, err := startCallbackServer()
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer cleanup()

	returnURI := fmt.Sprintf("%s:%d/callback", callbackURL, port)
	authURLWithParams := buildAuthURL(state, returnURI)

	fmt.Println("Opening browser for authentication...")
	fmt.Printf("If the browser doesn't open automatically, visit:\n%s\n\n", authURLWithParams)

	if err := openBrowser(authURLWithParams); err != nil {
		fmt.Printf("Failed to open browser automatically: %v\n", err)
	}

	fmt.Println("Waiting for authentication...")

	select {
	case authResp := <-authChan:
		if err := saveConfig(authResp); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
		fmt.Println("✓ Successfully authenticated with Thunder Compute!")
		return nil
	case err := <-errChan:
		return fmt.Errorf("authentication failed: %w", err)
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("authentication timeout after 5 minutes")
	}
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func buildAuthURL(state, returnURI string) string {
	params := url.Values{}
	params.Add("state", state)
	params.Add("return_uri", returnURI)
	return fmt.Sprintf("%s?%s", authURL, params.Encode())
}

func startCallbackServer() (int, <-chan AuthResponse, <-chan error, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, nil, nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port

	authChan := make(chan AuthResponse, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		refreshToken := r.URL.Query().Get("refresh_token")
		errorParam := r.URL.Query().Get("error")

		if errorParam != "" {
			errChan <- fmt.Errorf("authentication error: %s", errorParam)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `
				<html>
				<head><title>Authentication Failed</title></head>
				<body>
					<h1>Authentication Failed</h1>
					<p>Error: %s</p>
					<p>You can close this window.</p>
				</body>
				</html>
			`, errorParam)
			return
		}

		if token == "" {
			errChan <- fmt.Errorf("no token received")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "No token received")
			return
		}

		authResp := AuthResponse{
			Token:        token,
			RefreshToken: refreshToken,
		}

		authChan <- authResp

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html>
			<head><title>Authentication Successful</title></head>
			<body>
				<h1>Authentication Successful!</h1>
				<p>You have successfully authenticated with Thunder Compute.</p>
				<p>You can close this window and return to your terminal.</p>
			</body>
			</html>
		`)
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}

	return port, authChan, errChan, cleanup, nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func saveConfig(authResp AuthResponse) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".thunder")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.json")

	config := Config{
		Token:        authResp.Token,
		RefreshToken: authResp.RefreshToken,
	}

	if authResp.ExpiresIn > 0 {
		config.ExpiresAt = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".thunder", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
