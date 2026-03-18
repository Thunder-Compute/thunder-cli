package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/Thunder-Compute/thunder-cli/cmd"
	"github.com/Thunder-Compute/thunder-cli/internal/autoupdate"
	"github.com/Thunder-Compute/thunder-cli/internal/console"
	"github.com/Thunder-Compute/thunder-cli/internal/version"
)

func main() {
	// On Windows, this allows the same binary to act as an elevated helper
	// process for staging updates when triggered via UAC. On other platforms
	// this is a no-op.
	if autoupdate.MaybeRunWindowsUpdateHelper() {
		return
	}

	console.Init()

	_ = initSentry()
	defer sentry.Flush(5 * time.Second)

	// Wrap execution with panic recovery
	defer func() {
		if r := recover(); r != nil {
			sentry.CurrentHub().Recover(r)
			sentry.Flush(5 * time.Second)
			panic(r)
		}
	}()

	cmd.Execute()
}

func initSentry() error {
	// DSN is injected at build time - if empty, Sentry is disabled
	if version.SentryDSN == "" {
		return nil
	}

	// Load config for user context only
	cfg, _ := cmd.LoadConfig()

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              version.SentryDSN,
		Environment:      getEnvironment(),
		Release:          fmt.Sprintf("thunder-cli@%s", version.BuildVersion),
		Debug:            false,
		AttachStacktrace: true,
		SampleRate:       1.0,
		TracesSampleRate: 0.1,
		EnableTracing:    true,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			return filterSentryEvent(event)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize Sentry: %w", err)
	}

	// Set user context (privacy-safe)
	if cfg != nil && cfg.Token != "" {
		setUserContext(cfg.Token)
	}

	// Set global context tags
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("os", runtime.GOOS)
		scope.SetTag("arch", runtime.GOARCH)
		scope.SetTag("go_version", runtime.Version())
		scope.SetTag("build_commit", version.BuildCommit)
		scope.SetTag("service", "thunder-cli")
		scope.SetTag("instance_id", getInstanceID())
		if cfg != nil {
			scope.SetTag("api_url", cfg.APIURL)
		}
	})

	return nil
}

func getEnvironment() string {
	if version.BuildVersion == "dev" {
		return "dev"
	}
	return "production"
}

func getInstanceID() string {
	if id := os.Getenv("HOSTNAME"); id != "" {
		return id
	}
	if id := os.Getenv("COMPUTERNAME"); id != "" {
		return id
	}
	return "unknown"
}

// Noisy Sentry errors we should ignore — these are expected user errors,
// not bugs worth investigating.
var ignoredErrors = []string{
	// Auth errors (user needs to login / token expired)
	"not authenticated",
	"authentication failed",
	"token validation failed",

	// CLI usage errors
	"unknown flag",
	"unknown command",
	"unknown shorthand flag",
	"flag needs an argument",
	"invalid argument",
	"not a tty",

	// User file/instance errors
	"local file not found",
	"remote file not found",
	"not found",
	"not running",
	"transfer cancelled",

	// Network errors
	"context deadline exceeded",
	"connection reset",
	"ECONNRESET",
	"i/o timeout",
}

func filterSentryEvent(event *sentry.Event) *sentry.Event {
	for _, ex := range event.Exception {
		text := ex.Type + " " + ex.Value
		for _, pattern := range ignoredErrors {
			if strings.Contains(strings.ToLower(text), strings.ToLower(pattern)) {
				return nil
			}
		}
	}
	return event
}

func setUserContext(token string) {
	hash := sha256.Sum256([]byte(token))
	userID := hex.EncodeToString(hash[:8])

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{
			ID: userID,
		})
	})
}
