/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	"github.com/Thunder-Compute/thunder-cli/cmd"
	"github.com/Thunder-Compute/thunder-cli/internal/autoupdate"
	"github.com/Thunder-Compute/thunder-cli/internal/console"
	"github.com/Thunder-Compute/thunder-cli/internal/version"
	"github.com/Thunder-Compute/thunder-cli/sentry"
	sentrygo "github.com/getsentry/sentry-go"
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
	defer sentry.Shutdown()

	// Wrap execution with panic recovery
	defer sentry.CapturePanic(&sentry.EventOptions{
		Tags: sentry.NewTags().
			Set("command", "root").
			Set("version", version.BuildVersion),
	})

	cmd.Execute()
}

func initSentry() error {
	// DSN is injected at build time - if empty, Sentry is disabled
	if version.SentryDSN == "" {
		return nil
	}

	// Load config for user context only
	cfg, _ := cmd.LoadConfig()

	// Get configuration values
	environment := getEnvironment()
	sampleRate := getSampleRate()
	tracesSampleRate := getTracesSampleRate()
	release := fmt.Sprintf("thunder-cli@%s", version.BuildVersion)

	// Initialize Sentry with build-injected DSN
	err := sentry.Init(sentry.Config{
		DSN:              version.SentryDSN,
		Environment:      environment,
		Release:          release,
		Debug:            false, // Never debug in production
		SampleRate:       sampleRate,
		TracesSampleRate: tracesSampleRate,
		EnableProfiling:  false,
		ServiceName:      "thunder-cli",
		InstanceID:       getInstanceID(),
		FilteredErrors:   getFilteredErrors(),
	}, nil)

	if err != nil {
		return fmt.Errorf("failed to initialize Sentry: %w", err)
	}

	// Set user context (privacy-safe)
	if cfg != nil && cfg.Token != "" {
		setUserContext(cfg.Token)
	}

	// Set global context tags
	sentrygo.ConfigureScope(func(scope *sentrygo.Scope) {
		scope.SetTag("os", runtime.GOOS)
		scope.SetTag("arch", runtime.GOARCH)
		scope.SetTag("go_version", runtime.Version())
		scope.SetTag("build_commit", version.BuildCommit)
		if cfg != nil {
			scope.SetTag("api_url", cfg.APIURL)
		}
	})

	return nil
}

func getEnvironment() string {
	// Check if running from development build
	if version.BuildVersion == "dev" {
		return "dev"
	}

	return "production"
}

func getSampleRate() float64 {
	return 1.0 // Capture all errors
}

func getTracesSampleRate() float64 {
	return 0.1 // Sample 10% of traces
}

func getInstanceID() string {
	// Try various environment variables for instance ID
	if id := os.Getenv("HOSTNAME"); id != "" {
		return id
	}
	if id := os.Getenv("COMPUTERNAME"); id != "" {
		return id
	}
	return "unknown"
}

func getFilteredErrors() []string {
	return []string{
		"user cancelled",
		"context canceled",
		"operation cancelled by user",
		"authentication cancelled",
	}
}

func setUserContext(token string) {
	// Hash the token to create anonymous but unique user ID
	hash := sha256.Sum256([]byte(token))
	userID := hex.EncodeToString(hash[:8])

	sentrygo.ConfigureScope(func(scope *sentrygo.Scope) {
		scope.SetUser(sentrygo.User{
			ID: userID, // Hashed, not personally identifiable
		})
	})
}
