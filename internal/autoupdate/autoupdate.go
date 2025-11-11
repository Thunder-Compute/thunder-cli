package autoupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func MaybeStartBackgroundUpdate(ctx context.Context, currentVersion string) {
	if os.Getenv("TNR_NO_SELFUPDATE") == "1" {
		return
	}

	// Best-effort finalize any staged Windows update first.
	_ = finalizeWindowsSwap()

	exe, _ := currentExecutable()
	if isPMManaged(exe) {
		return
	}
	if !dirWritable(filepath.Dir(exe)) && runtime.GOOS != "windows" {
		// On Unix, we need a writable directory to atomically replace.
		return
	}

	// Shallow dependency to avoid import cycle: call updatecheck.Check via function pointer.
	latest, outdated, err := checkLatestVersion(ctx, currentVersion)
	if err != nil || !outdated {
		return
	}

	fmt.Printf("Updating tnr in background to %s…\n", latest)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := PerformUpdate(bgCtx, latest, false /*useSudo*/); err != nil {
			// Quietly ignore errors in background; user can run manual self-update.
		}
	}()
}

// PerformUpdate downloads, verifies and installs the target version.
// If latestTag is empty, the latest release will be used.
func PerformUpdate(ctx context.Context, latestTag string, useSudo bool) error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}

	if isPMManaged(exe) {
		return errors.New("managed by package manager")
	}

	asset, checksumsURL, version, err := resolveAssetForCurrentPlatform(ctx, latestTag)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "tnr-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(ctx, asset.BrowserDownloadURL, archivePath); err != nil {
		return fmt.Errorf("download asset: %w", err)
	}

	if checksumsURL != "" {
		checksumsPath := filepath.Join(tmpDir, "checksums.txt")
		if err := downloadFile(ctx, checksumsURL, checksumsPath); err == nil {
			if err := verifyChecksum(archivePath, checksumsPath); err != nil {
				return fmt.Errorf("checksum verification failed: %w", err)
			}
		}
	}

	extractedPath := filepath.Join(tmpDir, "tnr-extracted")
	if err := extractArchive(archivePath, extractedPath); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	var newBinary string
	if runtime.GOOS == "windows" {
		newBinary = filepath.Join(extractedPath, "tnr.exe")
	} else {
		newBinary = filepath.Join(extractedPath, "tnr")
	}
	if _, err := os.Stat(newBinary); err != nil {
		// Fallback: find first file starting with tnr
		matches, _ := filepath.Glob(filepath.Join(extractedPath, "tnr*"))
		if len(matches) > 0 {
			newBinary = matches[0]
		} else {
			return fmt.Errorf("extracted binary not found")
		}
	}

	// Ensure executable
	_ = os.Chmod(newBinary, 0o755)

	// Install
	if runtime.GOOS == "windows" {
		dir := filepath.Dir(exe)
		staged := filepath.Join(dir, "tnr.new")
		if err := copyFile(newBinary, staged); err != nil {
			return fmt.Errorf("stage new binary: %w", err)
		}
		// Write marker to attempt swap on next run
		_ = os.WriteFile(filepath.Join(dir, ".tnr-update"), []byte(version), 0o644)
		fmt.Println("Update downloaded; will apply on next run.")
		return nil
	}

	// Unix: replace atomically
	dir := filepath.Dir(exe)
	if !dirWritable(dir) && !useSudo {
		return fmt.Errorf("installation path requires elevated permissions: %s", dir)
	}

	if useSudo {
		tmpTarget := filepath.Join(filepath.Dir(newBinary), filepath.Base(exe))
		if err := copyFile(newBinary, tmpTarget); err != nil {
			return err
		}
		cmd := exec.Command("sudo", "mv", tmpTarget, exe)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sudo install failed: %w", err)
		}
		fmt.Printf("Successfully updated to version %s\n", version)
		return nil
	}

	tmpTarget := filepath.Join(dir, ".tnr-tmp")
	if err := copyFile(newBinary, tmpTarget); err != nil {
		return err
	}
	if err := os.Chmod(tmpTarget, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmpTarget, exe); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}
	fmt.Printf("Successfully updated to version %s\n", version)
	return nil
}

// --- helpers ---

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func resolveAssetForCurrentPlatform(ctx context.Context, tag string) (ghAsset, string, string, error) {
	release, err := fetchRelease(ctx, tag)
	if err != nil {
		return ghAsset{}, "", "", err
	}

	osKey := runtime.GOOS
	archKey := runtime.GOARCH
	if archKey == "amd64" {
		archKey = "amd64"
	}
	// Prefer OS-specific checksums file
	var checksumsURL string
	for _, a := range release.Assets {
		l := strings.ToLower(a.Name)
		if strings.Contains(l, "checksums") && strings.Contains(l, osKey) {
			checksumsURL = a.BrowserDownloadURL
			break
		}
	}
	if checksumsURL == "" {
		for _, a := range release.Assets {
			if strings.Contains(strings.ToLower(a.Name), "checksums") {
				checksumsURL = a.BrowserDownloadURL
				break
			}
		}
	}

	// Select the asset matching current OS/arch
	var candidate ghAsset
	for _, a := range release.Assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, osKey) && strings.Contains(name, archKey) {
			if osKey == "windows" && strings.HasSuffix(name, ".zip") {
				candidate = a
				break
			}
			if (osKey == "linux" || osKey == "darwin") && (strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz")) {
				candidate = a
				break
			}
		}
	}
	if candidate.Name == "" {
		return ghAsset{}, "", "", fmt.Errorf("no release asset for %s/%s", osKey, archKey)
	}

	return candidate, checksumsURL, release.TagName, nil
}

func fetchRelease(ctx context.Context, tag string) (ghRelease, error) {
	url := "https://api.github.com/repos/Thunder-Compute/thunder-cli/releases/latest"
	if strings.TrimSpace(tag) != "" {
		url = "https://api.github.com/repos/Thunder-Compute/thunder-cli/releases/tags/" + tag
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ghRelease{}, err
	}
	if token := strings.TrimSpace(os.Getenv("TNR_GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ghRelease{}, fmt.Errorf("github api status %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ghRelease{}, err
	}
	return rel, nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if token := strings.TrimSpace(os.Getenv("TNR_GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func verifyChecksum(filePath, checksumsPath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	sum := hex.EncodeToString(h.Sum(nil))

	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, sum) {
			return nil
		}
	}
	return fmt.Errorf("sha256 %s not found in checksums", sum)
}

func extractArchive(archivePath, destDir string) error {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractZip(archivePath, destDir)
	}
	// assume tar.gz or tgz
	return extractTarGz(archivePath, destDir)
}

func extractZip(path, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		fp := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fp, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func extractTarGz(path, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		fp := filepath.Join(dest, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fp, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func dirWritable(dir string) bool {
	test := filepath.Join(dir, ".tnr-write-test")
	if err := os.WriteFile(test, []byte("x"), 0o644); err != nil {
		return false
	}
	_ = os.Remove(test)
	return true
}

func isPMManaged(binPath string) bool {
	p := strings.ToLower(binPath)
	return strings.Contains(p, "/opt/homebrew/") ||
		strings.Contains(p, "/usr/local/cellar/") ||
		strings.Contains(p, `\scoop\apps\`) ||
		strings.Contains(p, "windowsapps")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// finalizeWindowsSwap attempts to replace tnr.exe with a previously staged tnr.new.
// It is a no-op on non-Windows platforms.
func finalizeWindowsSwap() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	staged := filepath.Join(dir, "tnr.new")
	if _, err := os.Stat(staged); err != nil {
		return nil
	}
	// Try atomic rename chain: move current to .old, then staged to exe.
	old := filepath.Join(dir, "tnr.old")
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(staged, exe); err != nil {
		// Try to rollback
		_ = os.Rename(old, exe)
		return err
	}
	_ = os.Remove(filepath.Join(dir, ".tnr-update"))
	_ = os.Remove(old)
	return nil
}

// checkLatestVersion calls into the updatecheck.Check without importing it directly to avoid cycles.
// Implemented here minimally by duplicating the HTTP call used by updatecheck.
func checkLatestVersion(ctx context.Context, current string) (latest string, outdated bool, err error) {
	// Fast path: reuse the same endpoint logic used by internal/updatecheck.
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/Thunder-Compute/thunder-cli/releases/latest", nil)
	if err != nil {
		return "", false, err
	}
	if token := strings.TrimSpace(os.Getenv("TNR_GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
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
	latest = strings.TrimSpace(payload.TagName)
	if latest == "" || strings.EqualFold(strings.TrimSpace(current), "dev") || strings.TrimSpace(current) == "" {
		return "", false, nil
	}
	// Compare semver (strip leading v)
	cv := strings.TrimPrefix(strings.TrimPrefix(current, "v"), "V")
	lv := strings.TrimPrefix(strings.TrimPrefix(latest, "v"), "V")
	outdated = versionLess(cv, lv)
	return latest, outdated, nil
}

func versionLess(a, b string) bool {
	ap := parseParts(a)
	bp := parseParts(b)
	for i := 0; i < 3; i++ {
		if ap[i] != bp[i] {
			return ap[i] < bp[i]
		}
	}
	return false
}

func parseParts(v string) [3]int {
	var out [3]int
	parts := strings.SplitN(v, "-", 2)[0]
	segs := strings.Split(parts, ".")
	for i := 0; i < len(segs) && i < 3; i++ {
		var n int
		fmt.Sscanf(segs[i], "%d", &n)
		out[i] = n
	}
	return out
}
