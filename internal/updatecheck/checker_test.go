package updatecheck

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCheckUsesCacheWhenFresh(t *testing.T) {
	t.Setenv("TNR_UPDATE_CACHE_DIR", t.TempDir())

	// Save original fetcher and restore it after the test
	originalFetcher := latestVersionFetcher
	defer func() { latestVersionFetcher = originalFetcher }()

	callCount := 0
	latestVersionFetcher = func(ctx context.Context) (string, bool, error) {
		callCount++
		return "1.2.3", true, nil
	}

	result, err := Check(context.Background(), "1.0.0")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if result.LatestVersion != "1.2.3" {
		t.Fatalf("expected latest version 1.2.3, got %s", result.LatestVersion)
	}
	if !result.Outdated {
		t.Fatal("expected result to be marked outdated")
	}
	if result.FromCache {
		t.Fatal("expected first call to be a live fetch")
	}
	if callCount != 1 {
		t.Fatalf("expected fetcher to be called once, got %d", callCount)
	}

	latestVersionFetcher = func(ctx context.Context) (string, bool, error) {
		t.Fatal("fetcher should not be called when cache is fresh")
		return "", false, nil
	}

	cachedResult, err := Check(context.Background(), "1.0.0")
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !cachedResult.FromCache {
		t.Fatal("expected result to come from cache")
	}
	if cachedResult.LatestVersion != "1.2.3" {
		t.Fatalf("expected cached latest version 1.2.3, got %s", cachedResult.LatestVersion)
	}
}

func TestCheckSkipsDevelopmentBuilds(t *testing.T) {
	t.Setenv("TNR_UPDATE_CACHE_DIR", t.TempDir())

	// Save original fetcher and restore it after the test
	originalFetcher := latestVersionFetcher
	defer func() { latestVersionFetcher = originalFetcher }()

	latestVersionFetcher = func(ctx context.Context) (string, bool, error) {
		t.Fatal("fetcher should not be called for development builds")
		return "", false, nil
	}

	res, err := Check(context.Background(), "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped {
		t.Fatal("expected development build to skip update check")
	}
}

func TestIsOutdatedComparison(t *testing.T) {
	// Now we can test isOutdated directly since we're in the same package
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"1.0.0", "1.2.3", true},
		{"1.2.3", "1.0.0", false},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", true},
		{"2.0.0", "1.9.9", false},
		{"1.0.0", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.current+" vs "+tt.latest, func(t *testing.T) {
			currentSemver, err := parseVersion(tt.current)
			if err != nil {
				t.Fatalf("parseVersion(%q) failed: %v", tt.current, err)
			}
			got := isOutdated(currentSemver, tt.latest)
			if got != tt.want {
				t.Errorf("isOutdated(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string // Expected parsed version string
		err   bool
	}{
		{"1.0.0", "1.0.0", false},
		{"v1.0.0", "1.0.0", false},
		{"V1.0.0", "1.0.0", false},
		{"  1.0.0  ", "1.0.0", false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseVersion(tt.input)
			if tt.err {
				if err == nil {
					t.Errorf("parseVersion(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVersion(%q) failed: %v", tt.input, err)
			}
			if got.String() != tt.want {
				t.Errorf("parseVersion(%q) = %v, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestCachePathHonorsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TNR_UPDATE_CACHE_DIR", dir)

	// Test that cache directory is honored by checking cachePath directly
	path, err := cachePath()
	if err != nil {
		t.Fatalf("cachePath() failed: %v", err)
	}

	if !strings.Contains(path, dir) {
		t.Fatalf("cachePath() = %q, expected it to contain %q", path, dir)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("override directory should exist: %v", err)
	}
}

func TestFetchLatestVersionPropagatesError(t *testing.T) {
	t.Setenv("TNR_UPDATE_CACHE_DIR", t.TempDir())

	// Save original fetcher and restore it after the test
	originalFetcher := latestVersionFetcher
	defer func() { latestVersionFetcher = originalFetcher }()

	expectedErr := errors.New("network down")
	latestVersionFetcher = func(ctx context.Context) (string, bool, error) {
		return "", false, expectedErr
	}

	_, err := Check(context.Background(), "1.0.0")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Check may wrap the error, so we check if it contains our error message
	if !strings.Contains(err.Error(), "network down") {
		t.Fatalf("expected error containing 'network down', got %v", err)
	}
}
