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
	htemplate "html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thunder-Compute/thunder-cli/tui"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
	"github.com/spf13/cobra"
)

const (
	authURL     = "https://console.thundercompute.com/login/vscode"
	callbackURL = "http://127.0.0.1"
)

const authSuccessHTML = `
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Authentication Successful</title>
		<style>
			* {
				margin: 0;
				padding: 0;
				box-sizing: border-box;
			}

			html, body {
				font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif, 'Apple Color Emoji', 'Segoe UI Emoji';
				background: #0a0a0a;
				color: #fafafa;
				min-height: 100vh;
				display: flex;
				flex-direction: column;
				align-items: center;
				justify-content: center;
				padding: 20px;
				text-rendering: optimizeLegibility;
				-webkit-font-smoothing: antialiased;
				-moz-osx-font-smoothing: grayscale;
			}

			.logo-container {
				margin-bottom: 32px;
				display: flex;
				justify-content: center;
				align-items: center;
			}

			.logo-container svg {
				width: 120px;
				height: 120px;
			}

			h1 {
				font-size: 28px;
				font-weight: 700;
				color: #fafafa;
				margin-bottom: 12px;
				letter-spacing: -0.02em;
				text-align: center;
				display: flex;
				align-items: center;
				gap: 12px;
				justify-content: center;
			}

			.message {
				font-size: 16px;
				line-height: 1.6;
				color: #a3a3a3;
				margin-bottom: 24px;
				text-align: center;
				max-width: 400px;
			}
		</style>
	</head>
	<body>
		<div class="logo-container">
			<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" fill="none">
				<path d="M122.5 112.5L20 256L236 68H174L222.5 0L72.5 83.5L193 84L113.5 153.5L154.5 96H50L20 112.5H122.5Z" fill="#369EFF"/>
				<path d="M222.5 0L73 83.5L193 84L113.5 153.5L154.5 96H50L20 112.5H122.5L20 256L236 68H174L222.5 0Z" fill="#369EFF"/>
			</svg>
		</div>
		
		<h1>
			Authentication Successful!
		</h1>
		
		<p class="message">
			You can now close this window and return to your terminal.
		</p>
	</body>
	</html>
`

const authFailedHTML = `
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Authentication Failed</title>
		<style>
			* {
				margin: 0;
				padding: 0;
				box-sizing: border-box;
			}

			html, body {
				font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif, 'Apple Color Emoji', 'Segoe UI Emoji';
				background: #0a0a0a;
				color: #fafafa;
				min-height: 100vh;
				display: flex;
				flex-direction: column;
				align-items: center;
				justify-content: center;
				padding: 20px;
				text-rendering: optimizeLegibility;
				-webkit-font-smoothing: antialiased;
				-moz-osx-font-smoothing: grayscale;
			}

			.logo-container {
				margin-bottom: 32px;
				display: flex;
				justify-content: center;
				align-items: center;
			}

			.logo-container svg {
				width: 120px;
				height: 120px;
			}

			h1 {
				font-size: 28px;
				font-weight: 700;
				margin-bottom: 12px;
				letter-spacing: -0.02em;
				text-align: center;
			}

			.message {
				font-size: 16px;
				line-height: 1.6;
				color: #a3a3a3;
				margin-bottom: 24px;
				text-align: center;
				max-width: 400px;
			}

			.error {
				background: rgba(239, 68, 68, 0.1);
				border: 1px solid rgba(239, 68, 68, 0.3);
				border-radius: 8px;
				padding: 12px 16px;
				color: #fca5a5;
				margin-bottom: 16px;
				word-break: break-word;
			}
		</style>
	</head>
	<body>
		<div class="logo-container">
			<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" fill="none">
				<path d="M122.5 112.5L20 256L236 68H174L222.5 0L72.5 83.5L193 84L113.5 153.5L154.5 96H50L20 112.5H122.5Z" fill="#369EFF"/>
				<path d="M222.5 0L73 83.5L193 84L113.5 153.5L154.5 96H50L20 112.5H122.5L20 256L236 68H174L222.5 0Z" fill="#369EFF"/>
			</svg>
		</div>

		<h1>Authentication Failed</h1>
		<div class="error">Error: {{.Error}}</div>
		<p class="message">You can now close this window and return to your terminal.</p>
	</body>
	</html>
`

var (
	authSuccessTemplate = htemplate.Must(htemplate.New("success").Parse(authSuccessHTML))
	authFailedTemplate  = htemplate.Must(htemplate.New("failed").Parse(authFailedHTML))
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

var loginToken string

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
	loginCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderLoginHelp(cmd)
	})

	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().StringVar(&loginToken, "token", "", "Authenticate directly with a token instead of opening browser")
}

func runLogin() error {
	config, err := LoadConfig()
	if err == nil && config.Token != "" {
		fmt.Println("User already logged in. Log out to sign into a different account.")
		return nil
	}

	if loginToken != "" {
		authResp := AuthResponse{
			Token: loginToken,
		}
		if err := saveConfig(authResp); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
		fmt.Println("✓ Successfully authenticated with Thunder Compute!")
		return nil
	}

	return runInteractiveLogin()
}

func runInteractiveLogin() error {
	state, err := generateState()
	if err != nil {
		return fmt.Errorf("failed to generate state: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port, authChan, errChan, cleanup, err := startCallbackServerWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	defer cleanup()

	returnURI := fmt.Sprintf("%s:%d/callback", callbackURL, port)
	authURLWithParams := buildAuthURL(state, returnURI)

	model := tui.NewLoginModel(authURLWithParams)
	p := tea.NewProgram(model)

	go func() {
		select {
		case authResp := <-authChan:
			tui.SendLoginSuccess(p, authResp.Token)
		case err := <-errChan:
			tui.SendLoginError(p, err)
		case <-ctx.Done():
			tui.SendLoginCancel(p)
		case <-time.After(5 * time.Minute):
			tui.SendLoginError(p, fmt.Errorf("authentication timeout after 5 minutes"))
		}
	}()

	if err := openBrowser(authURLWithParams); err != nil {
		fmt.Printf("Failed to open browser automatically: %v\n", err)
	}

	_, err = p.Run()
	if err != nil {
		if model.State() == tui.LoginStateCancelled {
			fmt.Println("User cancelled authentication")
			return nil
		}
		return fmt.Errorf("TUI error: %w", err)
	}

	if model.State() == tui.LoginStateSuccess {
		authResp := AuthResponse{
			Token: model.Token(),
		}
		if err := saveConfig(authResp); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
		return nil
	}

	if model.State() == tui.LoginStateCancelled {
		fmt.Println("User cancelled authentication")
		return nil
	}
	if model.State() == tui.LoginStateError {
		return model.Error()
	}

	return fmt.Errorf("authentication failed")
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

func startCallbackServerWithContext(ctx context.Context) (int, <-chan AuthResponse, <-chan error, func(), error) {
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
			w.WriteHeader(http.StatusUnauthorized)
			authFailedTemplate.Execute(w, map[string]string{"Error": errorParam})
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
		authSuccessTemplate.Execute(w, nil)
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	cleanup := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
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

// logoutCmd represents the logout command
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from Thunder Compute",
	Long:  `Log out from Thunder Compute and remove saved authentication credentials.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runLogout(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	logoutCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		helpmenus.RenderLogoutHelp(cmd)
	})

	rootCmd.AddCommand(logoutCmd)
}

func runLogout() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".thunder", "config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("You are not logged in.")
		return nil
	}

	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	fmt.Println("✓ Successfully logged out from Thunder Compute!")
	return nil
}
