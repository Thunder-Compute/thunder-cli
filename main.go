package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/Thunder-Compute/thunder-cli/api"
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

	os.Exit(cmd.Execute())
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
			if len(event.Exception) > 0 {
				msg := event.Exception[0].Value
				event.Fingerprint = []string{errorFingerprint(msg, hint.OriginalException)}
				cleanExceptionType(event, hint)
			}
			return event
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

func setUserContext(token string) {
	hash := sha256.Sum256([]byte(token))
	userID := hex.EncodeToString(hash[:8])

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetUser(sentry.User{
			ID: userID,
		})
	})
}

// cleanExceptionType replaces raw Go type names (*fmt.wrapError,
// *errors.errorString) with readable titles in Sentry issues.
func cleanExceptionType(event *sentry.Event, hint *sentry.EventHint) {
	if hint == nil || hint.OriginalException == nil {
		return
	}
	for i := range event.Exception {
		ex := &event.Exception[i]
		if !strings.HasPrefix(ex.Type, "*fmt.") && !strings.HasPrefix(ex.Type, "*errors.") {
			continue
		}
		// Use operation tag (set by scp, connect, etc.) or command tag as title.
		if op := event.Tags["operation"]; op != "" {
			ex.Type = op
		} else if command := event.Tags["command"]; command != "" {
			ex.Type = command + " error"
		}
	}
}

var statusCodeRe = regexp.MustCompile(`status (\d{3})`)

func errorFingerprint(msg string, original error) string {
	prefix := msg
	if idx := strings.Index(msg, ": "); idx > 0 {
		prefix = msg[:idx]
	}

	// Normalize to a slug.
	fp := strings.ToLower(prefix)
	fp = strings.ReplaceAll(fp, " ", "_")

	// Append HTTP status code so different API errors are separate issues.
	if m := statusCodeRe.FindStringSubmatch(msg); len(m) == 2 {
		fp += "_" + m[1]
	} else if original != nil {
		var apiErr *api.APIError
		if errors.As(original, &apiErr) {
			fp += "_" + strconv.Itoa(apiErr.StatusCode)
		}
	}

	return fp
}
