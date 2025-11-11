package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	cacheFileName = "latest.json"
	cacheDirName  = "tnr"
)

var cacheTTL = 24 * time.Hour

var (
	latestVersionFetcher = fetchLatestVersion
)

// Result describes the outcome of an update check.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	CheckedAt      time.Time
	Outdated       bool
	FromCache      bool
	Skipped        bool
	Reason         string
}

// Check determines whether a newer release is available for the current CLI version.
func Check(ctx context.Context, current string) (Result, error) {
	res := Result{
		CurrentVersion: strings.TrimSpace(current),
	}

	if res.CurrentVersion == "" || strings.EqualFold(res.CurrentVersion, "dev") {
		res.Skipped = true
		res.Reason = "development-build"
		return res, nil
	}

	currentSemver, err := parseVersion(res.CurrentVersion)
	if err != nil {
		res.Skipped = true
		res.Reason = "invalid-current-version"
		return res, nil
	}

	if cached, err := readCache(); err == nil && time.Since(cached.CheckedAt) < cacheTTL {
		res.LatestVersion = cached.LatestVersion
		res.CheckedAt = cached.CheckedAt
		res.FromCache = true
		res.Outdated = isOutdated(currentSemver, cached.LatestVersion)
		return res, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	versionStr, found, err := latestVersionFetcher(ctx)
	if err != nil {
		return res, err
	}
	if !found {
		return res, nil
	}

	res.LatestVersion = versionStr
	res.CheckedAt = time.Now()
	res.Outdated = isOutdated(currentSemver, res.LatestVersion)

	if err := writeCache(cachePayload{
		CheckedAt:     res.CheckedAt,
		LatestVersion: res.LatestVersion,
	}); err != nil && !errors.Is(err, os.ErrPermission) {
		// Non-fatal: loggable upstream if desired.
	}

	return res, nil
}

type cachePayload struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

func isOutdated(current *semver.Version, latest string) bool {
	if strings.TrimSpace(latest) == "" {
		return false
	}

	latestSemver, err := parseVersion(latest)
	if err != nil {
		return false
	}

	return current.LessThan(latestSemver)
}

func parseVersion(v string) (*semver.Version, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("empty version")
	}
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return semver.NewVersion(v)
}

func cachePath() (string, error) {
	if custom := os.Getenv("TNR_UPDATE_CACHE_DIR"); custom != "" {
		if err := os.MkdirAll(custom, 0o755); err != nil {
			return "", err
		}
		return filepath.Join(custom, cacheFileName), nil
	}

	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			if err != nil {
				return "", err
			}
			return "", homeErr
		}
		dir = filepath.Join(home, ".cache")
	}

	dir = filepath.Join(dir, cacheDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	return filepath.Join(dir, cacheFileName), nil
}

func readCache() (cachePayload, error) {
	path, err := cachePath()
	if err != nil {
		return cachePayload{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return cachePayload{}, err
	}

	var payload cachePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return cachePayload{}, err
	}

	return payload, nil
}

func writeCache(payload cachePayload) error {
	path, err := cachePath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func fetchLatestVersion(ctx context.Context) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/Thunder-Compute/thunder-cli/releases/latest", nil)
	if err != nil {
		return "", false, err
	}
	if token := strings.TrimSpace(os.Getenv("TNR_GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("github api status %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", false, err
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", false, nil
	}
	return tag, true, nil
}
