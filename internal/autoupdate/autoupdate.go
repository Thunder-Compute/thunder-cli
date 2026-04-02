package autoupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// PerformUpdate downloads, verifies and installs the target version.
// When src.AssetURL is empty, the updater falls back to GitHub releases using src.ReleaseTag.
func PerformUpdate(ctx context.Context, src Source) error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}

	if IsPMManaged(exe) {
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

func resolveSource(_ context.Context, src Source) (downloadURL, assetName, checksumsURL, version, expectedChecksum string, err error) {
	expectedChecksum = strings.ToLower(strings.TrimSpace(src.Checksum))
	checksumsURL = strings.TrimSpace(src.ChecksumURL)
	version = strings.TrimSpace(src.Version)

	if src.AssetURL == "" {
		return "", "", "", "", "", errors.New("no asset URL provided")
	}

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

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
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

	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	for _, f := range r.File {
		rel := filepath.Clean(f.Name)

		if filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
			return fmt.Errorf("zip entry has invalid path: %q", f.Name)
		}

		fp := filepath.Join(destAbs, rel)

		if !strings.HasPrefix(fp, destAbs+string(os.PathSeparator)) && fp != destAbs {
			return fmt.Errorf("zip entry escapes destination: %q", f.Name)
		}

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

	destAbs, err := filepath.Abs(dest)
	if err != nil {
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

		rel := filepath.Clean(hdr.Name)
		if filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
			return fmt.Errorf("tar entry has invalid path: %q", hdr.Name)
		}

		fp := filepath.Join(destAbs, rel)

		if !strings.HasPrefix(fp, destAbs+string(os.PathSeparator)) && fp != destAbs {
			return fmt.Errorf("tar entry escapes destination: %q", hdr.Name)
		}

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

// IsPMManaged reports whether the binary at binPath appears to be managed
// by a system package manager (Homebrew, Scoop, Winget).
func IsPMManaged(binPath string) bool {
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
func FinalizeWindowsSwap() error {
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

