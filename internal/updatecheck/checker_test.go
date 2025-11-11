package updatecheck

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckUsesCacheWhenFresh(t *testing.T) {
	t.Setenv("TNR_UPDATE_CACHE_DIR", t.TempDir())

	originalFetcher := latestVersionFetcher
	defer func() {
		latestVersionFetcher = originalFetcher
	}()

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

	originalFetcher := latestVersionFetcher
	defer func() {
		latestVersionFetcher = originalFetcher
	}()

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
	current, err := parseVersion("1.0.0")
	if err != nil {
		t.Fatalf("parseVersion failed: %v", err)
	}

	if !isOutdated(current, "1.0.1") {
		t.Fatal("expected 1.0.0 to be outdated compared to 1.0.1")
	}
	if isOutdated(current, "1.0.0") {
		t.Fatal("expected same version to not be outdated")
	}
	if isOutdated(current, "invalid") {
		t.Fatal("invalid version should not cause outdated=true")
	}
}

func TestCachePathHonorsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TNR_UPDATE_CACHE_DIR", dir)

	path, err := cachePath()
	if err != nil {
		t.Fatalf("cachePath returned error: %v", err)
	}

	expected := filepath.Join(dir, cacheFileName)
	if path != expected {
		t.Fatalf("expected cache path %s, got %s", expected, path)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("override directory should exist: %v", err)
	}
}

func TestFetchLatestVersionPropagatesError(t *testing.T) {
	originalFetcher := latestVersionFetcher
	defer func() {
		latestVersionFetcher = originalFetcher
	}()

	expectedErr := errors.New("network down")
	latestVersionFetcher = func(ctx context.Context) (string, bool, error) {
		return "", false, expectedErr
	}

	_, err := Check(context.Background(), "1.0.0")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func init() {
	// Ensure cache TTL is respected during tests by reducing the duration.
	cacheTTL = 10 * time.Second
}
