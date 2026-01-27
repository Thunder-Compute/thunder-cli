package sentry

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

// Config holds Sentry configuration options
type Config struct {
	DSN              string
	Environment      string   // "dev" or "production"
	Release          string   // e.g., "thunder-cli-v1.0.0"
	Debug            bool
	SampleRate       float64  // 0.0 to 1.0
	TracesSampleRate float64  // 0.0 to 1.0
	EnableProfiling  bool
	FilteredErrors   []string // Error messages to filter out

	// Enrichment data
	ServiceName string
	InstanceID  string
}

// Init initializes Sentry with the provided configuration
func Init(cfg Config, logProvider interface{}) error {
	if cfg.DSN == "" {
		return nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		Debug:            cfg.Debug,
		AttachStacktrace: true,
		SampleRate:       cfg.SampleRate,
		TracesSampleRate: cfg.TracesSampleRate,
		EnableTracing:    cfg.TracesSampleRate > 0,

		// BeforeSend hook for filtering and enrichment
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			// Filter out specified errors
			if event.Message != "" {
				for _, filtered := range cfg.FilteredErrors {
					if strings.Contains(event.Message, filtered) {
						return nil // Drop event
					}
				}
			}

			// Check exception messages too
			for _, exception := range event.Exception {
				for _, filtered := range cfg.FilteredErrors {
					if strings.Contains(exception.Value, filtered) {
						return nil
					}
				}
			}

			// Enrich with service metadata
			if event.Extra == nil {
				event.Extra = make(map[string]interface{})
			}
			event.Extra["service_name"] = cfg.ServiceName
			event.Extra["instance_id"] = cfg.InstanceID

			return event
		},
	})

	if err != nil {
		return fmt.Errorf("failed to initialize Sentry: %w", err)
	}

	// Set global tags
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("service", cfg.ServiceName)
		scope.SetTag("environment", cfg.Environment)
		if cfg.InstanceID != "" {
			scope.SetTag("instance_id", cfg.InstanceID)
		}
	})

	return nil
}

// Flush flushes buffered events with timeout
func Flush(timeout time.Duration) bool {
	return sentry.Flush(timeout)
}

// Shutdown gracefully shuts down Sentry
func Shutdown() {
	sentry.Flush(5 * time.Second)
}

// CaptureError captures an error with typed options
func CaptureError(err error, opts *EventOptions) *sentry.EventID {
	if err == nil {
		return nil
	}

	var eventID *sentry.EventID
	sentry.WithScope(func(scope *sentry.Scope) {
		if opts != nil {
			// Apply tags
			if opts.Tags != nil {
				for k, v := range opts.Tags.ToMap() {
					scope.SetTag(k, v)
				}
			}

			// Apply extra data
			if opts.Extra != nil {
				for k, v := range opts.Extra.ToMap() {
					scope.SetExtra(k, v)
				}
			}

			// Apply level
			if opts.Level != nil {
				scope.SetLevel(*opts.Level)
			}

			// Apply fingerprint
			if opts.Fingerprint != nil {
				scope.SetFingerprint(opts.Fingerprint)
			}
		}

		eventID = sentry.CaptureException(err)
	})
	return eventID
}

// AddBreadcrumb adds a breadcrumb for context tracking
// Breadcrumbs are global and attach to all subsequent events in the same scope
func AddBreadcrumb(category string, message string, data map[string]interface{}, level Level) {
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Type:      "default",
		Category:  category,
		Message:   message,
		Data:      data,
		Level:     sentry.Level(level),
		Timestamp: time.Now(),
	})
}

// Level is a Sentry severity level (re-exported for convenience)
type Level = sentry.Level

// Sentry level constants
const (
	LevelDebug   = sentry.LevelDebug
	LevelInfo    = sentry.LevelInfo
	LevelWarning = sentry.LevelWarning
	LevelError   = sentry.LevelError
	LevelFatal   = sentry.LevelFatal
)

// CapturePanic should be used in a defer statement to capture and report panics.
// It recovers from panic, reports to Sentry, flushes, and re-panics.
// Example: defer sentry.CapturePanic(&sentry.EventOptions{...})
func CapturePanic(opts *EventOptions) {
	if r := recover(); r != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetLevel(sentry.LevelFatal)

			if opts != nil {
				// Apply tags
				if opts.Tags != nil {
					for k, v := range opts.Tags.ToMap() {
						scope.SetTag(k, v)
					}
				}

				// Apply extra data
				if opts.Extra != nil {
					for k, v := range opts.Extra.ToMap() {
						scope.SetExtra(k, v)
					}
				}
			}

			sentry.CurrentHub().Recover(r)
		})
		sentry.Flush(5 * time.Second)
		panic(r) // Re-panic after capturing
	}
}

// GetEnvironment returns the environment string based on debug flag
func GetEnvironment(debug bool) string {
	if debug {
		return "dev"
	}
	env := os.Getenv("ENVIRONMENT")
	if env != "" {
		return env
	}
	return "production"
}

// GetRelease returns a release string for the service
func GetRelease(serviceName string) string {
	version := os.Getenv("SERVICE_VERSION")
	if version == "" {
		version = "unknown"
	}
	return fmt.Sprintf("%s-%s", serviceName, version)
}

// GetInstanceID returns an instance identifier
func GetInstanceID() string {
	// Try various environment variables for instance ID
	if id := os.Getenv("HOSTNAME"); id != "" {
		return id
	}
	if id := os.Getenv("AWS_LAMBDA_FUNCTION_NAME"); id != "" {
		return id
	}
	if id := os.Getenv("POD_NAME"); id != "" {
		return id
	}
	return "unknown"
}
