package updatepolicy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// debugf prints diagnostics when TNR_UPDATE_DEBUG=1
func debugf(format string, args ...any) {
	if os.Getenv("TNR_UPDATE_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[tnr-update] "+format+"\n", args...)
	}
}

const (
	cacheDirName           = "tnr"
	minVersionCacheName    = "min_version.json"
	manifestCacheName      = "latest_manifest.json"
	checksumsCacheTemplate = "checksums_%s_%s.json" // version, os

	cacheTTL = 24 * time.Hour

	defaultLatestPath = "/tnr/releases/latest.json"

	minVersionEnvKey = "TNR_MIN_VERSION_URL"
)

var defaultBases = []string{
	"https://gettnr.com",
}

// Result captures the outcome of the update policy evaluation.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	LatestTag      string
	MinVersion     string

	Mandatory bool
	Optional  bool
	Reason    string

	AssetURL       string
	ChecksumURL    string
	ExpectedSHA256 string
}

// Check evaluates whether the CLI needs to update, using cached metadata when possible.
// If force is true, cached values are ignored and fresh network calls are made.
func Check(ctx context.Context, currentVersion string, force bool) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	res := Result{
		CurrentVersion: normalizeVersion(currentVersion),
	}

	if strings.EqualFold(res.CurrentVersion, "dev") || res.CurrentVersion == "" {
		// Development builds skip update checks entirely.
		return res, nil
	}

	manifest, err := fetchLatestManifest(ctx, force)
	if err != nil {
		return res, fmt.Errorf("fetch latest manifest: %w", err)
	}
	res.LatestTag = manifest.Version
	res.LatestVersion = normalizeVersion(manifest.Version)

	platform := detectPlatform()
	assetURL, checksumURL := manifest.AssetFor(platform)

	if assetURL == "" {
		assetURL = defaultAssetURL(manifest, platform)
	}
	if checksumURL == "" {
		checksumURL = defaultChecksumURL(manifest, platform, assetURL)
	}

	// GitHub fallback for asset/checksum when manifest did not include them
	if assetURL == "" || checksumURL == "" {
		ghAsset, ghChecksum := githubAssetAndChecksum(manifest.Version, platform)
		if assetURL == "" {
			assetURL = ghAsset
		}
		if checksumURL == "" {
			checksumURL = ghChecksum
		}
	}

	res.AssetURL = assetURL
	candidates := checksumCandidates(checksumURL, assetURL, manifest.Version, platform.OS)
	// Always include the GitHub OS-specific checksum as an additional candidate
	if _, ghChecksum := githubAssetAndChecksum(manifest.Version, platform); ghChecksum != "" {
		candidates = append(candidates, ghChecksum)
	}

	expectedChecksum, usedURL, err := fetchChecksum(ctx, manifest.Version, platform.OS, candidates, force)
	if err == nil {
		res.ExpectedSHA256 = expectedChecksum
		res.ChecksumURL = usedURL
	}
	debugf("asset: %s", res.AssetURL)
	if res.ChecksumURL != "" && res.ExpectedSHA256 != "" {
		debugf("checksums: %s (sha256=%s)", res.ChecksumURL, res.ExpectedSHA256)
	}

	// Decide minimum CLI version
	if url := resolveMinVersionURL(); url != "" {
		if minVersion, err := fetchMinVersion(ctx, force); err == nil {
			res.MinVersion = normalizeVersion(minVersion)
			debugf("min-version: using %s from %s", res.MinVersion, url)
		} else if force {
			return res, fmt.Errorf("fetch min version: %w", err)
		}
	} else {
		res.MinVersion = res.LatestVersion
		debugf("min-version: defaulting to latest %s", res.MinVersion)
	}

	currentSemver, errCur := semver.NewVersion(res.CurrentVersion)
	if errCur != nil {
		return res, fmt.Errorf("parse current version: %w", errCur)
	}

	if res.MinVersion != "" {
		minSemver, errMin := semver.NewVersion(res.MinVersion)
		if errMin == nil && currentSemver.LessThan(minSemver) {
			res.Mandatory = true
			res.Optional = false
			res.Reason = "min-version"
			return res, nil
		}
	}

	if res.LatestVersion != "" {
		latestSemver, errLatest := semver.NewVersion(res.LatestVersion)
		if errLatest == nil && currentSemver.LessThan(latestSemver) {
			res.Optional = true
			res.Reason = "new-version"
		}
	}

	return res, nil
}

// -------- Manifest & Version Fetching --------

type manifest struct {
	Version string            `json:"version"`
	Channel string            `json:"channel"`
	Assets  map[string]string `json:"assets"`
}

// Manifest is exported for testing
type Manifest = manifest

func (m manifest) AssetFor(p platform) (assetURL, checksumURL string) {
	if m.Assets == nil {
		return "", ""
	}
	if v, ok := m.Assets[fmt.Sprintf("%s/%s", p.OS, p.Arch)]; ok {
		assetURL = v
	}
	if v, ok := m.Assets["checksums"]; ok {
		checksumURL = v
	}
	return
}

func fetchLatestManifest(ctx context.Context, force bool) (manifest, error) {
	if !force {
		if cached, ok := readManifestCache(); ok {
			debugf("manifest: using cached latest manifest version=%s", cached.Version)
			return cached, nil
		}
	}

	urls := resolveLatestURLs()
	var lastErr error
	for _, u := range urls {
		if u == "" {
			continue
		}
		debugf("manifest: trying %s", u)
		man, err := fetchManifestFromURL(ctx, u)
		if err == nil {
			_ = writeManifestCache(man)
			debugf("manifest: using %s (version=%s)", u, man.Version)
			return man, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("no manifest URL candidates")
	}
	return manifest{}, lastErr
}

func fetchManifestFromURL(ctx context.Context, manifestURL string) (manifest, error) {
	body, err := httpGet(ctx, manifestURL)
	if err != nil {
		return manifest{}, err
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return manifest{}, err
	}

	var man manifest
	if err := json.Unmarshal(data, &man); err != nil {
		return manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if man.Version == "" {
		return manifest{}, errors.New("manifest missing version")
	}
	return man, nil
}

type minVersionPayload struct {
	Version string `json:"version"`
}

func fetchMinVersion(ctx context.Context, force bool) (string, error) {
	if !force {
		if v, ok := readMinVersionCache(); ok {
			debugf("min-version: using cached %s", v)
			return v, nil
		}
	}

	minURL := resolveMinVersionURL()
	if minURL == "" {
		// Opt-in behavior: if no min-version URL is configured, skip mandatory checks.
		debugf("min-version: disabled (no %s)", minVersionEnvKey)
		return "", nil
	}

	body, err := httpGet(ctx, minURL)
	if err != nil {
		return "", err
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	var payload minVersionPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("decode min version: %w", err)
	}
	if payload.Version == "" {
		return "", errors.New("min version payload missing version")
	}
	_ = writeMinVersionCache(payload.Version)
	debugf("min-version: fetched %s from %s", payload.Version, minURL)
	return payload.Version, nil
}

// -------- Checksums --------

func fetchChecksum(ctx context.Context, version, osName string, candidates []string, force bool) (checksum string, usedURL string, err error) {
	if !force {
		if checksum, usedURL, ok := readChecksumCache(version, osName); ok {
			return checksum, usedURL, nil
		}
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		checksum, err := fetchChecksumFromURL(ctx, version, osName, candidate)
		if err == nil {
			return checksum, candidate, nil
		}
		if !errors.Is(err, errChecksumNotFound) {
			// Try next candidate; remember last error.
			usedURL = candidate
		}
	}
	if err == nil {
		err = errors.New("unable to locate checksum")
	}
	return "", usedURL, err
}

var errChecksumNotFound = errors.New("checksum not found")

func fetchChecksumFromURL(ctx context.Context, version, osName, checksumURL string) (string, error) {
	body, err := httpGet(ctx, checksumURL)
	if err != nil {
		return "", err
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	targetFile := targetArchiveName(version, osName)
	checksum, err := parseChecksum(string(data), targetFile)
	if err != nil {
		if errors.Is(err, errChecksumNotFound) {
			return "", err
		}
		return "", fmt.Errorf("parse checksum: %w", err)
	}
	_ = writeChecksumCache(version, osName, checksum, checksumURL)
	return checksum, nil
}

func parseChecksum(data, target string) (string, error) {
	target = strings.TrimSpace(target)
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[len(parts)-1]
		if name == target || strings.HasSuffix(name, "/"+target) || strings.HasSuffix(name, "\\"+target) {
			return parts[0], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%w: %s", errChecksumNotFound, target)
}

// -------- Helpers --------

type platform struct {
	OS   string
	Arch string
	Ext  string
}

// Platform is exported for testing
type Platform = platform

// detectPlatform detects the current platform
func detectPlatform() platform {
	var p platform
	switch runtime.GOOS {
	case "darwin":
		p.OS = "macos"
		p.Ext = ".tar.gz"
	case "linux":
		p.OS = "linux"
		p.Ext = ".tar.gz"
	case "windows":
		p.OS = "windows"
		p.Ext = ".zip"
	default:
		p.OS = runtime.GOOS
		p.Ext = ".tar.gz"
	}

	switch runtime.GOARCH {
	case "amd64":
		p.Arch = "amd64"
	case "arm64":
		p.Arch = "arm64"
	default:
		p.Arch = runtime.GOARCH
	}
	return p
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// targetArchiveName generates target archive name
func targetArchiveName(version, osName string) string {
	version = normalizeVersion(version)
	fileOS := osName
	if osName == "macos" {
		fileOS = "darwin"
	}
	return fmt.Sprintf("tnr_%s_%s_%s%s", version, fileOS, detectPlatform().Arch, detectPlatform().Ext)
}

func checksumCandidates(explicitURL, assetURL, version string, osName string) []string {
	candidates := make([]string, 0, 4)
	if explicitURL != "" {
		candidates = append(candidates, explicitURL)
	}
	releaseRoot := deriveReleaseRoot(assetURL)
	if releaseRoot != "" {
		candidates = append(candidates,
			fmt.Sprintf("%s/checksums-%s.txt", releaseRoot, osName),
			fmt.Sprintf("%s/checksums.txt", releaseRoot),
			fmt.Sprintf("%s/%s/checksums-%s.txt", releaseRoot, osName, osName),
		)
	}
	return candidates
}

func deriveReleaseRoot(assetURL string) string {
	if assetURL == "" {
		return ""
	}
	u, err := url.Parse(assetURL)
	if err != nil {
		return ""
	}
	dir := path.Dir(u.Path) // .../<version>/<os>
	dir = path.Dir(dir)     // .../<version>
	u.Path = dir
	return strings.TrimSuffix(u.String(), "/")
}

func defaultAssetURL(man manifest, p platform) string {
	if p.OS == "" || p.Arch == "" || man.Version == "" {
		return ""
	}
	base := defaultManifestBase()
	if base == "" {
		return ""
	}
	version := normalizeVersion(man.Version)
	fileOS := p.OS
	if p.OS == "macos" {
		fileOS = "darwin"
	}
	return fmt.Sprintf("%s/tnr/releases/%s/%s/tnr_%s_%s_%s%s", base, version, p.OS, version, fileOS, p.Arch, p.Ext)
}

func defaultChecksumURL(man manifest, p platform, assetURL string) string {
	if assetURL == "" {
		return ""
	}
	releaseRoot := deriveReleaseRoot(assetURL)
	if releaseRoot == "" {
		return ""
	}
	return fmt.Sprintf("%s/checksums-%s.txt", releaseRoot, p.OS)
}

// githubAssetAndChecksum builds GitHub release URLs for the asset and OS-specific checksum file.
// Tag is v{version}, filenames use plain {version}.
func githubAssetAndChecksum(version string, p platform) (assetURL, checksumURL string) {
	fileVersion := normalizeVersion(version)
	tag := fileVersion
	if !strings.HasPrefix(tag, "v") && !strings.HasPrefix(tag, "V") {
		tag = "v" + tag
	}
	fileOS := p.OS
	if p.OS == "macos" {
		fileOS = "darwin"
	}
	assetURL = fmt.Sprintf(
		"https://github.com/Thunder-Compute/thunder-cli/releases/download/%s/tnr_%s_%s_%s%s",
		tag, fileVersion, fileOS, p.Arch, p.Ext,
	)
	checkName := "checksums.txt"
	switch p.OS {
	case "macos":
		checkName = "checksums-macos.txt"
	case "linux":
		checkName = "checksums-linux.txt"
	case "windows":
		checkName = "checksums-windows.txt"
	}
	checksumURL = fmt.Sprintf(
		"https://github.com/Thunder-Compute/thunder-cli/releases/download/%s/%s",
		tag, checkName,
	)
	return
}

func resolveLatestURLs() []string {
	var urls []string
	if v := os.Getenv("TNR_LATEST_URL"); v != "" {
		urls = append(urls, strings.TrimSpace(v))
	} else if base := os.Getenv("TNR_DOWNLOAD_BASE"); base != "" {
		base = strings.TrimRight(strings.TrimSpace(base), "/")
		urls = append(urls, base+defaultLatestPath)
	}
	for _, base := range defaultBases {
		base = strings.TrimRight(base, "/")
		urls = append(urls, base+defaultLatestPath)
	}
	return dedupe(urls)
}

func resolveMinVersionURL() string {
	if v := strings.TrimSpace(os.Getenv(minVersionEnvKey)); v != "" {
		return v
	}
	return ""
}

func defaultManifestBase() string {
	if base := strings.TrimSpace(os.Getenv("TNR_DOWNLOAD_BASE")); base != "" {
		return strings.TrimRight(base, "/")
	}
	// Return first non-empty default base (Cloudflare R2 via gettnr.com)
	for _, base := range defaultBases {
		if base != "" {
			return strings.TrimRight(base, "/")
		}
	}
	return ""
}

func dedupe(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// -------- Cache helpers --------

type manifestCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Manifest  manifest  `json:"manifest"`
}

type minVersionCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Version   string    `json:"version"`
}

type checksumCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Checksum  string    `json:"checksum"`
	URL       string    `json:"url"`
}

func readManifestCache() (manifest, bool) {
	var cached manifestCache
	if !readJSONCache(manifestCacheName, &cached) {
		return manifest{}, false
	}
	if time.Since(cached.CheckedAt) > cacheTTL {
		return manifest{}, false
	}
	return cached.Manifest, true
}

func writeManifestCache(man manifest) error {
	return writeJSONCache(manifestCacheName, manifestCache{
		CheckedAt: time.Now(),
		Manifest:  man,
	})
}

func readMinVersionCache() (string, bool) {
	var cached minVersionCache
	if !readJSONCache(minVersionCacheName, &cached) {
		return "", false
	}
	if time.Since(cached.CheckedAt) > cacheTTL {
		return "", false
	}
	return cached.Version, true
}

func writeMinVersionCache(version string) error {
	return writeJSONCache(minVersionCacheName, minVersionCache{
		CheckedAt: time.Now(),
		Version:   version,
	})
}

func readChecksumCache(version, osName string) (checksum, url string, ok bool) {
	name := fmt.Sprintf(checksumsCacheTemplate, version, osName)
	var cached checksumCache
	if !readJSONCache(name, &cached) {
		return "", "", false
	}
	if time.Since(cached.CheckedAt) > cacheTTL {
		return "", "", false
	}
	return cached.Checksum, cached.URL, true
}

func writeChecksumCache(version, osName, checksum, url string) error {
	name := fmt.Sprintf(checksumsCacheTemplate, version, osName)
	return writeJSONCache(name, checksumCache{
		CheckedAt: time.Now(),
		Checksum:  checksum,
		URL:       url,
	})
}

func cachePath(name string) (string, error) {
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
	return filepath.Join(dir, name), nil
}

func readJSONCache(name string, dst any) bool {
	path, err := cachePath(name)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return false
	}
	return true
}

func writeJSONCache(name string, payload any) error {
	path, err := cachePath(name)
	if err != nil {
		return err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// -------- Networking --------

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

func httpGet(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("http %d: %s (%s)", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}
	return resp.Body, nil
}
