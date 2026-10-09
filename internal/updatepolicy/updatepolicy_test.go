package updatepolicy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Thunder-Compute/thunder-cli/internal/testutils"
)

func TestCheckOptionalUpdateUsesCache(t *testing.T) {
	testutils.SetHome(t, t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv("LOCALAPPDATA", cacheDir)

	plat := detectPlatform()
	version := "1.2.3"
	current := "1.2.0"
	minVersion := "1.1.0"
	expectedHash := strings.Repeat("a", 64)
	assetName := targetArchiveName(version, plat.OS)

	var manifestHits int32
	var checksumHits int32
	var minHits int32

	manifest := Manifest{
		Version: version,
		Channel: "stable",
		Assets: map[string]string{
			fmt.Sprintf("%s/%s", plat.OS, plat.Arch): "", // will fill after server starts
			"checksums":                              "",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest.json":
			atomic.AddInt32(&manifestHits, 1)
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(manifest); err != nil {
				t.Fatalf("encode manifest: %v", err)
			}
		case "/checksums.txt":
			atomic.AddInt32(&checksumHits, 1)
			fmt.Fprintf(w, "%s  %s\n", expectedHash, assetName)
		case "/min":
			atomic.AddInt32(&minHits, 1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"version": %q}`, minVersion)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	manifest.Assets[fmt.Sprintf("%s/%s", plat.OS, plat.Arch)] = srv.URL + "/" + assetName
	manifest.Assets["checksums"] = srv.URL + "/checksums.txt"

	t.Setenv("TNR_LATEST_URL", srv.URL+"/latest.json")
	t.Setenv("TNR_MIN_VERSION_URL", srv.URL+"/min")

	res, err := Check(context.Background(), current, true)
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if res.Mandatory {
		t.Fatalf("expected optional update, got mandatory")
	}
	if !res.Optional {
		t.Fatalf("expected optional update to be true")
	}
	if res.ExpectedSHA256 != expectedHash {
		t.Fatalf("expected checksum %s, got %s", expectedHash, res.ExpectedSHA256)
	}
	if res.ChecksumURL != srv.URL+"/checksums.txt" {
		t.Fatalf("unexpected checksum URL: %s", res.ChecksumURL)
	}

	if got := atomic.LoadInt32(&manifestHits); got != 1 {
		t.Fatalf("expected manifest hit 1, got %d", got)
	}
	if got := atomic.LoadInt32(&checksumHits); got != 1 {
		t.Fatalf("expected checksum hit 1, got %d", got)
	}
	if got := atomic.LoadInt32(&minHits); got != 1 {
		t.Fatalf("expected min-version hit 1, got %d", got)
	}

	// Second call without force should reuse cache.
	if _, err := Check(context.Background(), current, false); err != nil {
		t.Fatalf("Check (cached) error: %v", err)
	}
	if got := atomic.LoadInt32(&manifestHits); got != 1 {
		t.Fatalf("expected cached manifest hit 1, got %d", got)
	}
	if got := atomic.LoadInt32(&checksumHits); got != 1 {
		t.Fatalf("expected cached checksum hit 1, got %d", got)
	}
	if got := atomic.LoadInt32(&minHits); got != 1 {
		t.Fatalf("expected cached min-version hit 1, got %d", got)
	}

	// Force refresh to confirm mandatory evaluation.
	resForce, err := Check(context.Background(), "1.0.0", true)
	if err != nil {
		t.Fatalf("Check force error: %v", err)
	}
	if !resForce.Mandatory {
		t.Fatalf("expected mandatory update when below min version")
	}
}

// An unverifiable checksum must not hide the fact that an update exists: Check
// still reports the new version, but flags it as uninstallable so the caller can
// say so instead of silently doing nothing.
func TestCheckReportsUpdateButFlagsUnverifiableChecksum(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		checksum   string
	}{
		{
			name:       "unavailable",
			statusCode: http.StatusServiceUnavailable,
		},
		{
			name:       "missing",
			statusCode: http.StatusOK,
			checksum:   strings.Repeat("a", 64) + "  another-file.tar.gz\n",
		},
		{
			name:       "malformed",
			statusCode: http.StatusOK,
			checksum:   "not-a-sha256  %s\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.SetHome(t, t.TempDir())
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			t.Setenv("LOCALAPPDATA", t.TempDir())
			t.Setenv("TNR_MIN_VERSION_URL", "")

			plat := detectPlatform()
			version := "9.9.9"
			assetName := targetArchiveName(version, plat.OS)
			var serverURL string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/latest.json" {
					_ = json.NewEncoder(w).Encode(Manifest{
						Version: version,
						Assets: map[string]string{
							fmt.Sprintf("%s/%s", plat.OS, plat.Arch): serverURL + "/" + assetName,
							"checksums":                              serverURL + "/checksums.txt",
						},
					})
					return
				}
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					fmt.Fprint(w, strings.ReplaceAll(tt.checksum, "%s", assetName))
				}
			}))
			defer srv.Close()
			serverURL = srv.URL

			originalDownloadBase := githubReleaseDownloadBaseURL
			githubReleaseDownloadBaseURL = srv.URL
			defer func() { githubReleaseDownloadBaseURL = originalDownloadBase }()
			t.Setenv("TNR_LATEST_URL", srv.URL+"/latest.json")

			res, err := Check(context.Background(), "1.0.0", true)
			if err != nil {
				t.Fatalf("Check() error = %v, want the update still reported", err)
			}
			if !res.Mandatory && !res.Optional {
				t.Error("Check() did not report the available update")
			}
			if !res.ChecksumUnavailable {
				t.Fatal("Check() did not flag the checksum as unavailable")
			}
			if res.ChecksumError == nil {
				t.Error("Check() flagged an unavailable checksum with no reason")
			}
			if res.ExpectedSHA256 != "" {
				t.Errorf("ExpectedSHA256 = %q, want empty so nothing can be installed", res.ExpectedSHA256)
			}
		})
	}
}

// The checksum is only worth fetching once an update is actually available;
// fetching it on every command costs a round-trip that buys nothing.
func TestCheckSkipsChecksumFetchWhenUpToDate(t *testing.T) {
	testutils.SetHome(t, t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("TNR_MIN_VERSION_URL", "")

	plat := detectPlatform()
	version := "9.9.9"
	assetName := targetArchiveName(version, plat.OS)

	var checksumHits atomic.Int32
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest.json" {
			_ = json.NewEncoder(w).Encode(Manifest{
				Version: version,
				Assets: map[string]string{
					fmt.Sprintf("%s/%s", plat.OS, plat.Arch): serverURL + "/" + assetName,
					"checksums":                              serverURL + "/checksums.txt",
				},
			})
			return
		}
		checksumHits.Add(1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	serverURL = srv.URL

	originalDownloadBase := githubReleaseDownloadBaseURL
	githubReleaseDownloadBaseURL = srv.URL
	defer func() { githubReleaseDownloadBaseURL = originalDownloadBase }()
	t.Setenv("TNR_LATEST_URL", srv.URL+"/latest.json")

	// Current version == latest, so there is nothing to install.
	res, err := Check(context.Background(), version, true)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if res.Mandatory || res.Optional {
		t.Fatalf("Check() reported an update for the current version: %+v", res)
	}
	if got := checksumHits.Load(); got != 0 {
		t.Errorf("checksum was fetched %d times while up to date, want 0", got)
	}
}

func TestOptionalUpdateCacheHelpers(t *testing.T) {
	tmp := t.TempDir()
	testutils.SetHome(t, tmp)

	ts, err := ReadOptionalUpdateAttempt("1.2.3")
	if err != nil {
		t.Fatalf("initial read: %v", err)
	}
	if !ts.IsZero() {
		t.Fatalf("expected zero time, got %v", ts)
	}

	now := time.Unix(1700000000, 0).UTC()
	if err := WriteOptionalUpdateAttempt("1.2.3", now); err != nil {
		t.Fatalf("write attempt: %v", err)
	}

	readBack, err := ReadOptionalUpdateAttempt("1.2.3")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !readBack.Equal(now) {
		t.Fatalf("expected %v, got %v", now, readBack)
	}

	if err := ClearOptionalUpdateAttempt(); err != nil {
		t.Fatalf("clear optional cache: %v", err)
	}
	finalRead, err := ReadOptionalUpdateAttempt("1.2.3")
	if err != nil {
		t.Fatalf("final read: %v", err)
	}
	if !finalRead.IsZero() {
		t.Fatalf("expected zero time after clear, got %v", finalRead)
	}
}

func TestClassifyAsset(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"tnr_2.0.48_darwin_arm64.tar.gz", "macos/arm64"},
		{"tnr_2.0.48_darwin_amd64.tar.gz", "macos/amd64"},
		{"tnr_2.0.48_linux_amd64.tar.gz", "linux/amd64"},
		{"tnr_2.0.48_linux_arm64.tar.gz", "linux/arm64"},
		{"tnr_2.0.48_windows_amd64.zip", "windows/amd64"},
		{"tnr_2.0.48_windows_arm64.zip", "windows/arm64"},
		{"checksums.txt", "checksums"},
		// OS-specific checksums are not mapped (resolved via githubAssetAndChecksum)
		{"checksums-macos.txt", ""},
		{"checksums-linux.txt", ""},
		{"checksums-windows.txt", ""},
		// Installers are not mapped
		{"tnr_2.0.48_darwin_arm64.pkg", ""},
		{"tnr-2.0.48-amd64.msi", ""},
		{"tnr_2.0.48_linux_amd64.deb", ""},
		{"tnr_2.0.48_linux_amd64.rpm", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyAsset(tt.name)
			if got != tt.expected {
				t.Errorf("classifyAsset(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestFetchLatestFromGitHub(t *testing.T) {
	testutils.SetHome(t, t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tag_name": "v2.0.48",
			"assets": [
				{"name": "tnr_2.0.48_darwin_arm64.tar.gz", "browser_download_url": "https://example.com/tnr_2.0.48_darwin_arm64.tar.gz"},
				{"name": "tnr_2.0.48_linux_amd64.tar.gz", "browser_download_url": "https://example.com/tnr_2.0.48_linux_amd64.tar.gz"},
				{"name": "tnr_2.0.48_windows_amd64.zip", "browser_download_url": "https://example.com/tnr_2.0.48_windows_amd64.zip"},
				{"name": "checksums.txt", "browser_download_url": "https://example.com/checksums.txt"},
				{"name": "tnr_2.0.48_darwin_arm64.pkg", "browser_download_url": "https://example.com/tnr_2.0.48_darwin_arm64.pkg"}
			]
		}`)
	}))
	defer srv.Close()

	// Override the GitHub API URL for testing.
	origURL := githubReleasesLatestURL
	defer func() { setGitHubReleasesURL(origURL) }()
	setGitHubReleasesURL(srv.URL)

	man, err := fetchLatestFromGitHub(context.Background())
	if err != nil {
		t.Fatalf("fetchLatestFromGitHub: %v", err)
	}
	if man.Version != "v2.0.48" {
		t.Errorf("version = %q, want %q", man.Version, "v2.0.48")
	}
	if man.Channel != "stable" {
		t.Errorf("channel = %q, want %q", man.Channel, "stable")
	}
	if got := man.Assets["macos/arm64"]; got != "https://example.com/tnr_2.0.48_darwin_arm64.tar.gz" {
		t.Errorf("macos/arm64 = %q", got)
	}
	if got := man.Assets["linux/amd64"]; got != "https://example.com/tnr_2.0.48_linux_amd64.tar.gz" {
		t.Errorf("linux/amd64 = %q", got)
	}
	if got := man.Assets["checksums"]; got != "https://example.com/checksums.txt" {
		t.Errorf("checksums = %q", got)
	}
	// Installer (.pkg) should NOT be in assets
	if _, ok := man.Assets["macos/arm64/pkg"]; ok {
		t.Error("pkg installer should not be classified")
	}
}

// Ensures tests run with consistent platform data when executed on different OS/arch.
func init() {
	// Ensure the helper functions behave deterministically during tests.
	_ = runtime.GOOS
	_ = runtime.GOARCH
}

// Forced updates are opt-in via TNR_MIN_VERSION_URL. With no min-version source
// configured, being behind latest is an optional update, never a mandatory one —
// a mandatory update the CLI cannot complete exits(1) and leaves it unusable.
func TestCheckDoesNotForceUpdateWithoutMinVersionSource(t *testing.T) {
	testutils.SetHome(t, t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv(minVersionEnvKey, "")

	plat := detectPlatform()
	version := "9.9.9"
	assetName := targetArchiveName(version, plat.OS)
	contents := []byte("release archive")
	sum := sha256.Sum256(contents)

	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest.json" {
			_ = json.NewEncoder(w).Encode(Manifest{
				Version: version,
				Assets: map[string]string{
					fmt.Sprintf("%s/%s", plat.OS, plat.Arch): serverURL + "/" + assetName,
					"checksums":                              serverURL + "/checksums.txt",
				},
			})
			return
		}
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), assetName)
	}))
	defer srv.Close()
	serverURL = srv.URL

	originalDownloadBase := githubReleaseDownloadBaseURL
	githubReleaseDownloadBaseURL = srv.URL
	defer func() { githubReleaseDownloadBaseURL = originalDownloadBase }()
	t.Setenv("TNR_LATEST_URL", srv.URL+"/latest.json")

	res, err := Check(context.Background(), "1.0.0", true)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if res.Mandatory {
		t.Errorf("Check() forced an update with %s unset (reason=%q)", minVersionEnvKey, res.Reason)
	}
	if !res.Optional {
		t.Error("Check() did not offer the available update")
	}
	if res.MinVersion != "" {
		t.Errorf("MinVersion = %q, want empty when no min-version source is configured", res.MinVersion)
	}
}
