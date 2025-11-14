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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Source describes where an update should be downloaded from.
// When AssetURL is provided, the updater downloads directly from that URL.
// Otherwise, ReleaseTag (or Version) is used to locate the asset on GitHub.
type Source struct {
	Version      string
	ReleaseTag   string
	AssetURL     string
	AssetName    string
	Checksum     string
	ChecksumURL  string
	Channel      string
	ExpectedSize int64
}

func MaybeStartBackgroundUpdate(ctx context.Context, currentVersion string) {
	if os.Getenv("TNR_NO_SELFUPDATE") == "1" {
		return
	}

	_ = finalizeWindowsSwap()

	exe, _ := currentExecutable()
	if isPMManaged(exe) {
		return
	}
	if !dirWritable(filepath.Dir(exe)) && runtime.GOOS != "windows" {
		return
	}

	latest, outdated, err := checkLatestVersion(ctx, currentVersion)
	if err != nil || !outdated {
		return
	}

	fmt.Printf("Updating tnr in background to %s…\n", latest)
	source := Source{
		ReleaseTag: latest,
		Version:    strings.TrimPrefix(strings.TrimPrefix(latest, "v"), "V"),
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = PerformUpdate(bgCtx, source)
	}()
}

// PerformUpdate downloads, verifies and installs the target version.
// When src.AssetURL is empty, the updater falls back to GitHub releases using src.ReleaseTag.
func PerformUpdate(ctx context.Context, src Source) error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}

	if isPMManaged(exe) {
		return errors.New("managed by package manager")
	}

	downloadURL, assetName, checksumsURL, version, expectedChecksum, err := resolveSource(ctx, src)
	if err != nil {
		return err
	}
	if assetName == "" {
		return errors.New("unable to determine asset name for update")
	}

	tmpDir, err := os.MkdirTemp("", "tnr-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(ctx, downloadURL, archivePath); err != nil {
		return fmt.Errorf("download asset: %w", err)
	}

	if expectedChecksum != "" {
		if err := verifyChecksumValue(archivePath, expectedChecksum); err != nil {
			return err
		}
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
		matches, _ := filepath.Glob(filepath.Join(extractedPath, "tnr*"))
		if len(matches) > 0 {
			newBinary = matches[0]
		} else {
			return fmt.Errorf("extracted binary not found")
		}
	}

	_ = os.Chmod(newBinary, 0o755)

	inst := detectInstaller()
	if err := inst.Install(ctx, exe, newBinary, version, src); err != nil {
		return err
	}
	return nil
}

func resolveSource(ctx context.Context, src Source) (downloadURL, assetName, checksumsURL, version, expectedChecksum string, err error) {
	expectedChecksum = strings.ToLower(strings.TrimSpace(src.Checksum))
	checksumsURL = strings.TrimSpace(src.ChecksumURL)
	version = strings.TrimSpace(src.Version)

	if src.AssetURL != "" {
		downloadURL = strings.TrimSpace(src.AssetURL)
		assetName = strings.TrimSpace(src.AssetName)
		if assetName == "" {
			assetName = fileNameFromURL(downloadURL)
		}
		if version == "" {
			version = deriveVersionFromName(assetName)
		}
		return
	}

	asset, ghChecksumsURL, ghVersion, err := resolveAssetForCurrentPlatform(ctx, src.ReleaseTag)
	if err != nil {
		return "", "", "", "", "", err
	}

	downloadURL = asset.BrowserDownloadURL
	assetName = asset.Name
	if checksumsURL == "" {
		checksumsURL = ghChecksumsURL
	}
	if version == "" {
		version = ghVersion
	}
	return
}

func verifyChecksumValue(filePath, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func fileNameFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return path.Base(u.Path)
}

func deriveVersionFromName(name string) string {
	sanitised := strings.TrimSpace(name)
	if sanitised == "" {
		return ""
	}
	parts := strings.Split(sanitised, "_")
	if len(parts) >= 2 {
		return strings.TrimPrefix(parts[1], "v")
	}
	return ""
}

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
	marker := filepath.Join(dir, ".tnr-update")
	
	if _, err := os.Stat(staged); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return nil
	}

	writable := dirWritable(dir)
	if writable {
		old := filepath.Join(dir, "tnr.old")
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
		if err := os.Rename(staged, exe); err != nil {
			_ = os.Rename(old, exe)
			return err
		}
		_ = os.Remove(marker)
		_ = removeOldBackupWithRetry(old)
		return nil
	}

	if err := runElevatedFinalizeHelper(context.Background(), dir); err != nil {
		return err
	}
	return nil
}

func removeOldBackupWithRetry(oldPath string) error {
	maxRetries := 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := os.Remove(oldPath)
		if err == nil {
			return nil
		}
		lastErr = err
		if i < maxRetries-1 {
			delay := time.Duration(100*(1<<i)) * time.Millisecond
			time.Sleep(delay)
		}
	}
	return lastErr
}

func FinalizeStagedWindowsUpdate() error {
	return finalizeWindowsSwap()
}

func checkLatestVersion(ctx context.Context, current string) (latest string, outdated bool, err error) {
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
	v = strings.TrimSpace(v)
	if v == "" {
		return out
	}
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	parts := strings.SplitN(v, "-", 2)[0]
	segs := strings.Split(parts, ".")
	for i := 0; i < len(segs) && i < 3; i++ {
		var n int
		fmt.Sscanf(segs[i], "%d", &n)
		out[i] = n
	}
	return out
}
