//go:build windows

package autoupdate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	windowsUpdateHelperFlag = "--tnr-internal-update-helper"

	helperArgFrom    = "--from"
	helperArgTo      = "--to"
	helperArgVersion = "--version"

	installMetaFileName = ".install-meta.json"
)

type installMeta struct {
	InstallType string `json:"installType"`
	Source      string `json:"source,omitempty"`
	Version     string `json:"version,omitempty"`
}

// MaybeRunWindowsUpdateHelper checks whether the current invocation is the
// internal elevated update helper. When it is, it runs the helper flow and
// terminates the process with the appropriate exit code.
//
// It returns true when the helper flow was executed (and the process has
// already exited), and false for normal CLI invocations.
func MaybeRunWindowsUpdateHelper() bool {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != windowsUpdateHelperFlag {
		return false
	}

	code := runWindowsUpdateHelper(args[1:])
	os.Exit(code)
	return true
}

// windowsInstaller handles installation on Windows systems.
type windowsInstaller struct{}

// Install implements the installer interface for Windows systems.
func (w windowsInstaller) Install(ctx context.Context, exe, newBinary, version string, src Source) error {
	return installOnWindows(ctx, exe, newBinary, version, src)
}

// installOnWindows installs the downloaded update on Windows, taking into
// account MSI-managed installs and elevation requirements.
//
// It never overwrites the currently running executable directly. For manual
// installs it stages a tnr.new next to the binary and writes a marker so that
// the swap can be finalized on the next run. For MSI installs it triggers an
// MSI upgrade instead of touching MSI-managed files.
func installOnWindows(ctx context.Context, exe, newBinary, version string, src Source) error {
	dir := filepath.Dir(exe)

	installType := detectWindowsInstallType(exe)

	switch installType {
	case "msi":
		// MSI-managed installs must be upgraded via MSI to avoid confusing
		// Windows Installer and breaking Add/Remove Programs.
		return runWindowsMSIUpgrade(ctx, version)
	default:
		// Treat everything else as a manual install. We still honour MSI
		// metadata if present later.
		return stageWindowsUpdateWithElevation(ctx, dir, newBinary, version)
	}
}

// detectWindowsInstallType attempts to determine how tnr was installed.
// It prefers an on-disk metadata file, and falls back to checking the
// MSI registry key created by the WiX installer.
func detectWindowsInstallType(exePath string) string {
	dir := filepath.Dir(exePath)

	// 1) Prefer metadata file next to the binary.
	metaPath := filepath.Join(dir, installMetaFileName)
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta installMeta
		if err := json.Unmarshal(data, &meta); err == nil {
			t := strings.ToLower(strings.TrimSpace(meta.InstallType))
			if t != "" {
				return t
			}
		}
	}

	// 2) Fall back to MSI registry key.
	msiInstallDir, err := readMSIInstallDir()
	if err == nil && msiInstallDir != "" {
		if sameWindowsDir(msiInstallDir, dir) {
			return "msi"
		}
	}

	// 3) Default to manual.
	return "manual"
}

func readMSIInstallDir() (string, error) {
	// Matches the WiX installer definition in packaging/windows/app.wxs:
	// Root="HKLM"
	// Key="Software\Thunder Compute\{{ .ProjectName }}"
	// Name="InstallDir"
	const keyPath = `Software\Thunder Compute\tnr`

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	dir, _, err := k.GetStringValue("InstallDir")
	if err != nil {
		return "", err
	}
	return dir, nil
}

func sameWindowsDir(a, b string) bool {
	a = strings.TrimRight(strings.ToLower(filepath.Clean(a)), `\ /`)
	b = strings.TrimRight(strings.ToLower(filepath.Clean(b)), `\ /`)
	return a == b
}

// stageWindowsUpdateWithElevation stages a Windows update by placing a tnr.new
// file next to the current executable and writing the .tnr-update marker.
// When the install directory is not writable and the process is not already
// elevated, it re-launches itself via UAC to perform the staging step.
func stageWindowsUpdateWithElevation(ctx context.Context, dir, newBinary, version string) error {
	staged := filepath.Join(dir, "tnr.new")
	marker := filepath.Join(dir, ".tnr-update")

	// Fast path: writable directory from the current process (per-user installs).
	if dirWritable(dir) {
		if err := copyFile(newBinary, staged); err != nil {
			return fmt.Errorf("stage new binary: %w", err)
		}
		_ = os.WriteFile(marker, []byte(version), 0o644)
		fmt.Println("Update downloaded; will apply on next run.")
		return nil
	}

	// If we are already elevated (e.g. launched from an elevated shell),
	// we can stage directly even if the directory is normally protected.
	if isElevated() {
		if err := copyFile(newBinary, staged); err != nil {
			return fmt.Errorf("stage new binary (elevated): %w", err)
		}
		_ = os.WriteFile(marker, []byte(version), 0o644)
		fmt.Println("Update downloaded; will apply on next run.")
		return nil
	}

	// Otherwise, request elevation via UAC and have the elevated helper stage
	// the update for us.
	if err := runElevatedStagingHelper(ctx, dir, newBinary, version); err != nil {
		return err
	}

	fmt.Println("Update downloaded; will apply on next run after elevation.")
	return nil
}

// runWindowsMSIUpgrade performs an MSI-based upgrade for MSI-managed installs.
// It downloads the MSI for the target version and invokes msiexec to perform
// the upgrade. Windows will handle UAC prompts for elevation.
func runWindowsMSIUpgrade(ctx context.Context, version string) error {
	msiURL, msiName := windowsMSIURL(version)
	if msiURL == "" {
		return errors.New("unable to determine MSI URL for upgrade")
	}

	tmpDir, err := os.MkdirTemp("", "tnr-msi-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	msiPath := filepath.Join(tmpDir, msiName)
	if err := downloadFile(ctx, msiURL, msiPath); err != nil {
		return fmt.Errorf("download MSI: %w", err)
	}

	// Run msiexec; let Windows handle any required UAC prompt.
	cmd := exec.CommandContext(ctx, "msiexec.exe",
		"/i", msiPath,
		"/qn",
		"/norestart",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("msi upgrade failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("msi upgrade failed: %w", err)
	}

	fmt.Println("Successfully upgraded via MSI. Please re-run your command.")
	return nil
}

// windowsMSIURL constructs the expected MSI URL for the given version and
// current architecture, matching the GoReleaser configuration.
func windowsMSIURL(version string) (string, string) {
	v := strings.TrimSpace(version)
	if v == "" {
		return "", ""
	}

	fileVersion := strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	tag := v
	if !strings.HasPrefix(strings.ToLower(tag), "v") {
		tag = "v" + fileVersion
	}

	arch := runtime.GOARCH
	msiName := fmt.Sprintf("tnr-%s-%s.msi", fileVersion, arch)
	url := fmt.Sprintf(
		"https://github.com/Thunder-Compute/thunder-cli/releases/download/%s/%s",
		tag, msiName,
	)
	return url, msiName
}

// runElevatedStagingHelper launches an elevated instance of tnr in a special
// helper mode to stage the update in a directory that normally requires
// administrative rights (e.g. Program Files).
func runElevatedStagingHelper(ctx context.Context, targetDir, newBinary, version string) error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}

	// The helper will receive absolute paths.
	newBinary, err = filepath.Abs(newBinary)
	if err != nil {
		return err
	}
	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	args := []string{
		windowsUpdateHelperFlag,
		helperArgFrom, newBinary,
		helperArgTo, targetDir,
		helperArgVersion, version,
	}

	psCommand := buildPowerShellElevationCommand(exe, args)

	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		psCommand,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Exit code 0xC0000142 and similar often indicate the user declined UAC.
			return fmt.Errorf("elevated staging helper failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to launch elevated staging helper: %w", err)
	}

	return nil
}

// buildPowerShellElevationCommand builds a PowerShell one-liner that launches
// the current executable with elevation (UAC) and waits for completion,
// propagating its exit code.
func buildPowerShellElevationCommand(exe string, args []string) string {
	psExe := psQuote(exe)
	psArgs := psQuote(strings.Join(args, " "))

	// Use Start-Process -Verb RunAs to trigger UAC, wait for completion and
	// propagate the exit code to the PowerShell process (and thus to Go).
	script := fmt.Sprintf(`$p = Start-Process -FilePath %s -ArgumentList %s -Verb RunAs -Wait -PassThru; exit $p.ExitCode`, psExe, psArgs)
	return script
}

func psQuote(s string) string {
	// Single-quote and escape existing single quotes for PowerShell.
	s = strings.ReplaceAll(s, `'`, `''`)
	return "'" + s + "'"
}

func runWindowsUpdateHelper(args []string) int {
	if !isElevated() {
		fmt.Fprintln(os.Stderr, "tnr update helper: elevation required but not granted (UAC may have been cancelled).")
		return 1
	}

	from, toDir, version, err := parseHelperArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tnr update helper: %v\n", err)
		return 1
	}

	if err := stageUpdateInDir(from, toDir, version); err != nil {
		fmt.Fprintf(os.Stderr, "tnr update helper: failed to stage update: %v\n", err)
		return 1
	}

	return 0
}

func parseHelperArgs(args []string) (from, toDir, version string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case helperArgFrom:
			i++
			if i >= len(args) {
				return "", "", "", errors.New("missing value for --from")
			}
			from = args[i]
		case helperArgTo:
			i++
			if i >= len(args) {
				return "", "", "", errors.New("missing value for --to")
			}
			toDir = args[i]
		case helperArgVersion:
			i++
			if i >= len(args) {
				return "", "", "", errors.New("missing value for --version")
			}
			version = args[i]
		}
	}

	if from == "" || toDir == "" {
		return "", "", "", errors.New("missing required arguments")
	}
	return from, toDir, version, nil
}

// stageUpdateInDir performs the actual work of copying the new binary into
// the target directory and writing the update marker. It assumes it is
// running elevated and that the directory may normally be protected.
func stageUpdateInDir(from, toDir, version string) error {
	from, err := filepath.Abs(from)
	if err != nil {
		return err
	}
	toDir, err = filepath.Abs(toDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(toDir, 0o755); err != nil {
		return err
	}

	staged := filepath.Join(toDir, "tnr.new")
	marker := filepath.Join(toDir, ".tnr-update")

	if err := copyFile(from, staged); err != nil {
		return fmt.Errorf("copy new binary: %w", err)
	}
	if err := os.WriteFile(marker, []byte(version), 0o644); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}

	return nil
}

// isElevated returns true when the current process is running with
// administrative privileges.
func isElevated() bool {
	token := windows.GetCurrentProcessToken()
	defer token.Close()

	var elevation struct {
		TokenIsElevated uint32
	}
	var outLen uint32
	err := windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elevation)),
		uint32(unsafe.Sizeof(elevation)),
		&outLen,
	)
	if err != nil {
		return false
	}
	return elevation.TokenIsElevated != 0
}
