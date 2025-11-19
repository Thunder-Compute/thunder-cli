//go:build windows

package autoupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	windowsUpdateHelperFlag = "--tnr-internal-update-helper"

	helperArgFrom       = "--from"
	helperArgTo         = "--to"
	helperArgVersion    = "--version"
	helperArgFinalize   = "--finalize"
	helperArgLogFile    = "--log-file"
	helperArgUpdateMeta = "--update-meta"
)

type installMeta struct {
	InstallType string `json:"installType"`
	Source      string `json:"source,omitempty"`
	Version     string `json:"version,omitempty"`
}

func windowsMetaPath() string {
	base := filepath.Join(os.Getenv("ProgramData"), "Thunder-Compute", "tnr")
	_ = os.MkdirAll(base, 0o755)
	return filepath.Join(base, "install-meta.json")
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

func installOnWindows(ctx context.Context, exe, newBinary, version string, src Source) error {
	dir := filepath.Dir(exe)

	installType := detectWindowsInstallType(exe)

	switch installType {
	case "msi":
		return runWindowsMSIUpgrade(ctx, version)
	default:
		return stageWindowsUpdateWithElevation(ctx, dir, newBinary, version)
	}
}

func detectWindowsInstallType(exePath string) string {
	dir := filepath.Dir(exePath)

	metaPath := windowsMetaPath()
	if data, err := os.ReadFile(metaPath); err == nil {
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		var meta installMeta
		if err := json.Unmarshal(data, &meta); err == nil {
			t := strings.ToLower(strings.TrimSpace(meta.InstallType))
			if t != "" {
				return t
			}
		}
	}

	msiInstallDir, err := readMSIInstallDir()
	if err == nil && msiInstallDir != "" {
		if sameWindowsDir(msiInstallDir, dir) {
			return "msi"
		}
	}

	return "manual"
}

func readMSIInstallDir() (string, error) {
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

func stageWindowsUpdateWithElevation(ctx context.Context, dir, newBinary, version string) error {
	staged := filepath.Join(dir, "tnr.new")
	marker := filepath.Join(dir, ".tnr-update")

	if dirWritable(dir) {
		if err := copyFile(newBinary, staged); err != nil {
			return fmt.Errorf("stage new binary: %w", err)
		}
		_ = os.WriteFile(marker, []byte(version), 0o644)
		fmt.Println("Update downloaded; will apply on next run.")
		return nil
	}

	elevated := isElevated()
	if elevated {
		if err := copyFile(newBinary, staged); err != nil {
			return fmt.Errorf("stage new binary (elevated): %w", err)
		}
		_ = os.WriteFile(marker, []byte(version), 0o644)
		fmt.Println("Update downloaded; will apply on next run.")
		return nil
	}

	if err := runElevatedStagingHelper(ctx, dir, newBinary, version); err != nil {
		return err
	}

	fmt.Println("Update downloaded; will apply on next run after elevation.")
	return nil
}

func runWindowsMSIUpgrade(ctx context.Context, version string) error {
	exe, err := currentExecutable()
	if err != nil {
		return fmt.Errorf("failed to get current executable: %w", err)
	}
	installDir := filepath.Dir(exe)
	metaPath := windowsMetaPath()

	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Starting MSI upgrade to version %s\n", version)
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Install directory: %s\n", installDir)
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Metadata file path: %s\n", metaPath)

	var preservedMeta *installMeta
	if data, err := os.ReadFile(metaPath); err == nil {
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] Found existing metadata file, preserving it\n")
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
		var meta installMeta
		if err := json.Unmarshal(data, &meta); err == nil {
			preservedMeta = &meta
			fmt.Fprintf(os.Stderr, "[MSI Upgrade] Preserved metadata: installType=%s, source=%s, version=%s\n",
				meta.InstallType, meta.Source, meta.Version)
		} else {
			fmt.Fprintf(os.Stderr, "[MSI Upgrade] Failed to parse existing metadata: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] No existing metadata file found: %v\n", err)
	}

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

	persistentMsiDir := filepath.Join(os.TempDir(), "tnr-msi-upgrade")
	if err := os.MkdirAll(persistentMsiDir, 0o755); err != nil {
		return fmt.Errorf("create persistent MSI directory: %w", err)
	}
	persistentMsiPath := filepath.Join(persistentMsiDir, msiName)
	if err := copyFile(msiPath, persistentMsiPath); err != nil {
		return fmt.Errorf("copy MSI to persistent location: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Launching MSI installer: %s\n", persistentMsiPath)
	cmd := exec.Command("msiexec.exe",
		"/i", persistentMsiPath,
		"/passive",
		"/norestart",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MSI installer: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[MSI Upgrade] MSI installer process started (PID: %d)\n", cmd.Process.Pid)
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Waiting for MSI installer to complete...\n")

	err = cmd.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] MSI installer exited with error: %v\n", err)
		return fmt.Errorf("MSI installer failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[MSI Upgrade] MSI installer completed successfully\n")

	time.Sleep(2 * time.Second)
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Updating metadata file after MSI completion...\n")
	updateInstallMetaAfterMSI(installDir, version, preservedMeta)

	go func() {
		time.Sleep(5 * time.Second)
		_ = os.Remove(persistentMsiPath)
		_ = os.Remove(persistentMsiDir)
	}()

	fmt.Println("MSI upgrade completed. Please re-run your command.")
	return nil
}

func updateInstallMetaAfterMSI(installDir, version string, preservedMeta *installMeta) {
	metaPath := windowsMetaPath()
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Updating metadata file after MSI completion\n")
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Target path: %s\n", metaPath)

	var meta installMeta
	if preservedMeta != nil {
		meta.InstallType = preservedMeta.InstallType
		meta.Source = preservedMeta.Source
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] Using preserved metadata: installType=%s, source=%s\n",
			meta.InstallType, meta.Source)
	} else {
		meta.InstallType = "msi"
		meta.Source = "msi"
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] No preserved metadata, using defaults: installType=msi, source=msi\n")
	}
	meta.Version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")

	for i := 0; i < 5; i++ {
		if data, err := os.ReadFile(metaPath); err == nil {
			fmt.Fprintf(os.Stderr, "[MSI Upgrade] Found metadata file after MSI (attempt %d), reading it\n", i+1)
			data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
			var existingMeta installMeta
			if err := json.Unmarshal(data, &existingMeta); err == nil {
				fmt.Fprintf(os.Stderr, "[MSI Upgrade] Existing metadata: installType=%s, source=%s, version=%s\n",
					existingMeta.InstallType, existingMeta.Source, existingMeta.Version)
				if existingMeta.InstallType != "" {
					meta.InstallType = existingMeta.InstallType
				}
				if existingMeta.Source != "" {
					meta.Source = existingMeta.Source
				}
			} else {
				fmt.Fprintf(os.Stderr, "[MSI Upgrade] Failed to parse existing metadata: %v\n", err)
			}
			break
		} else {
			fmt.Fprintf(os.Stderr, "[MSI Upgrade] Metadata file not found (attempt %d): %v\n", i+1, err)
		}
		if i < 4 {
			time.Sleep(time.Second)
		}
	}

	metaJSON, err := json.MarshalIndent(meta, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] Failed to marshal metadata: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Writing metadata: installType=%s, source=%s, version=%s\n",
		meta.InstallType, meta.Source, meta.Version)

	metaDir := filepath.Dir(metaPath)
	writable := dirWritable(metaDir)
	elevated := isElevated()
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] Metadata directory writable: %v, Elevated: %v\n", writable, elevated)

	if !writable && !elevated {
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] Directory not writable and not elevated, using elevated helper\n")
		if err := updateInstallMetaWithElevation(metaDir, metaJSON); err != nil {
			fmt.Fprintf(os.Stderr, "[MSI Upgrade] Failed to update metadata with elevation: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] Successfully updated metadata using elevated helper\n")
		return
	}

	for i := 0; i < 5; i++ {
		if err := os.WriteFile(metaPath, metaJSON, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "[MSI Upgrade] Successfully wrote metadata file (attempt %d)\n", i+1)
			if _, err := os.Stat(metaPath); err == nil {
				fmt.Fprintf(os.Stderr, "[MSI Upgrade] Verified metadata file exists after write\n")
			} else {
				fmt.Fprintf(os.Stderr, "[MSI Upgrade] WARNING: Metadata file not found after write: %v\n", err)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "[MSI Upgrade] Failed to write metadata file (attempt %d): %v\n", i+1, err)
		if i < 4 {
			time.Sleep(time.Second)
		}
	}
	fmt.Fprintf(os.Stderr, "[MSI Upgrade] ERROR: Failed to write metadata file after all retries\n")
}

func updateInstallMetaWithElevation(metaDir string, metaJSON []byte) error {
	exe, err := currentExecutable()
	if err != nil {
		return fmt.Errorf("failed to get current executable: %w", err)
	}

	metaDir, err = filepath.Abs(metaDir)
	if err != nil {
		return err
	}

	tmpMetaFile := filepath.Join(os.TempDir(), "tnr-meta-"+fmt.Sprintf("%d", time.Now().UnixNano())+".json")
	if err := os.WriteFile(tmpMetaFile, metaJSON, 0o644); err != nil {
		return fmt.Errorf("failed to write temp metadata file: %w", err)
	}
	defer os.Remove(tmpMetaFile)

	args := []string{
		windowsUpdateHelperFlag,
		helperArgUpdateMeta,
		helperArgTo, metaDir,
		helperArgFrom, tmpMetaFile,
	}

	psCommand := buildPowerShellElevationCommand(exe, args)

	cmd := exec.Command("powershell.exe",
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
			return fmt.Errorf("elevated metadata update helper failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to launch elevated metadata update helper: %w", err)
	}

	return nil
}

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

func runElevatedStagingHelper(ctx context.Context, targetDir, newBinary, version string) error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}

	newBinary, err = filepath.Abs(newBinary)
	if err != nil {
		return err
	}
	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	persistentBinaryDir := filepath.Join(os.TempDir(), "tnr-update-binary")
	if err := os.MkdirAll(persistentBinaryDir, 0o755); err != nil {
		return fmt.Errorf("create persistent binary directory: %w", err)
	}
	persistentBinary := filepath.Join(persistentBinaryDir, "tnr.exe")
	if err := copyFile(newBinary, persistentBinary); err != nil {
		return fmt.Errorf("copy binary to persistent location: %w", err)
	}

	args := []string{
		windowsUpdateHelperFlag,
		helperArgFrom, persistentBinary,
		helperArgTo, targetDir,
		helperArgVersion, version,
	}

	psCommand := buildPowerShellElevationCommand(exe, args)

	helperLogFile := filepath.Join(os.TempDir(), "tnr-helper-staging.log")
	args = append(args, helperArgLogFile, helperLogFile)

	psCommand = buildPowerShellElevationCommand(exe, args)

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
			_ = os.Remove(persistentBinary)
			_ = os.Remove(persistentBinaryDir)
			return fmt.Errorf("elevated staging helper failed with exit code %d", exitErr.ExitCode())
		}
		_ = os.Remove(persistentBinary)
		_ = os.Remove(persistentBinaryDir)
		return fmt.Errorf("failed to launch elevated staging helper: %w", err)
	}
	_ = os.Remove(helperLogFile)

	defer func() {
		_ = os.Remove(persistentBinary)
		_ = os.Remove(persistentBinaryDir)
	}()

	staged := filepath.Join(targetDir, "tnr.new")
	if _, err := os.Stat(staged); err != nil {
		return fmt.Errorf("staged file was not created: %w", err)
	}
	return nil
}

func runElevatedFinalizeHelper(ctx context.Context, targetDir string) error {
	exe, err := currentExecutable()
	if err != nil {
		return err
	}

	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return err
	}

	args := []string{
		windowsUpdateHelperFlag,
		helperArgFinalize,
		helperArgTo, targetDir,
	}

	helperLogFile := filepath.Join(os.TempDir(), "tnr-helper-finalize.log")
	args = append(args, helperArgLogFile, helperLogFile)

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
			_ = os.Remove(helperLogFile)
			return fmt.Errorf("elevated finalize helper failed with exit code %d", exitErr.ExitCode())
		}
		_ = os.Remove(helperLogFile)
		return fmt.Errorf("failed to launch elevated finalize helper: %w", err)
	}

	_ = os.Remove(helperLogFile)

	return nil
}

func buildPowerShellElevationCommand(exe string, args []string) string {
	psExe := psQuote(exe)

	var arrayElements []string
	for _, arg := range args {
		arrayElements = append(arrayElements, psQuote(QuoteWindowsArg(arg)))
	}
	psArgsArray := "@(" + strings.Join(arrayElements, ",") + ")"

	script := fmt.Sprintf(`$argArray = %s; $p = Start-Process -FilePath %s -ArgumentList $argArray -Verb RunAs -Wait -PassThru; exit $p.ExitCode`, psArgsArray, psExe)
	return script
}

func psQuote(s string) string {
	s = strings.ReplaceAll(s, `'`, `''`)
	return "'" + s + "'"
}

// quoteWindowsArg applies Windows command-line quoting rules so that Start-Process
// forwards each argument as a single token even when it contains spaces, tabs,
// double quotes, or trailing backslashes
func quoteWindowsArg(arg string) string {
	if !needsWindowsQuoting(arg) {
		return arg
	}

	var b strings.Builder
	b.Grow(len(arg) + 2)
	b.WriteByte('"')

	backslashes := 0
	for i := 0; i < len(arg); i++ {
		c := arg[i]
		switch c {
		case '\\':
			backslashes++
		case '"':
			for j := 0; j < backslashes*2+1; j++ {
				b.WriteByte('\\')
			}
			backslashes = 0
			b.WriteByte('"')
		default:
			for j := 0; j < backslashes; j++ {
				b.WriteByte('\\')
			}
			backslashes = 0
			b.WriteByte(c)
		}
	}

	for j := 0; j < backslashes*2; j++ {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return b.String()
}

func needsWindowsQuoting(arg string) bool {
	if arg == "" {
		return true
	}
	for i := 0; i < len(arg); i++ {
		switch arg[i] {
		case ' ', '\t', '"':
			return true
		}
	}
	return false
}

func runWindowsUpdateHelper(args []string) int {
	var logFile *os.File
	var logFilePath string
	for i := 0; i < len(args); i++ {
		if args[i] == helperArgLogFile {
			i++
			if i < len(args) {
				logFilePath = args[i]
				if f, err := os.Create(logFilePath); err == nil {
					logFile = f
					defer logFile.Close()
				}
			}
			break
		}
	}

	writeLog := func(format string, a ...interface{}) {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprint(os.Stderr, msg)
		if logFile != nil {
			fmt.Fprint(logFile, msg)
		}
	}

	elevated := isElevated()
	if !elevated {
		writeLog("tnr update helper: elevation required but not granted (UAC may have been cancelled).\n")
		return 1
	}

	if len(args) > 0 && args[0] == helperArgFinalize {
		toDir, err := parseFinalizeArgs(args[1:])
		if err != nil {
			writeLog("tnr update helper: %v\n", err)
			return 1
		}
		if err := finalizeUpdateInDir(toDir); err != nil {
			writeLog("tnr update helper: failed to finalize update: %v\n", err)
			return 1
		}
		return 0
	}

	if len(args) > 0 && args[0] == helperArgUpdateMeta {
		toDir, fromFile, err := parseUpdateMetaArgs(args[1:])
		if err != nil {
			writeLog("tnr update helper: %v\n", err)
			return 1
		}
		if err := updateMetaInDir(toDir, fromFile); err != nil {
			writeLog("tnr update helper: failed to update metadata: %v\n", err)
			return 1
		}
		return 0
	}

	from, toDir, version, err := parseHelperArgs(args)
	if err != nil {
		writeLog("tnr update helper: %v\n", err)
		return 1
	}

	if err := stageUpdateInDir(from, toDir, version); err != nil {
		writeLog("tnr update helper: failed to stage update: %v\n", err)
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
		case helperArgLogFile:
			i++
			if i >= len(args) {
				return "", "", "", errors.New("missing value for --log-file")
			}
		default:
			return "", "", "", fmt.Errorf("unexpected argument: %s", args[i])
		}
	}

	if from == "" || toDir == "" {
		return "", "", "", errors.New("missing required arguments")
	}
	return from, toDir, version, nil
}

func parseFinalizeArgs(args []string) (toDir string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case helperArgTo:
			i++
			if i >= len(args) {
				return "", errors.New("missing value for --to")
			}
			toDir = args[i]
		case helperArgLogFile:
			i++
			if i >= len(args) {
				return "", errors.New("missing value for --log-file")
			}
		default:
			return "", fmt.Errorf("unexpected argument: %s", args[i])
		}
	}
	if toDir == "" {
		return "", errors.New("missing required arguments")
	}
	return toDir, nil
}

func parseUpdateMetaArgs(args []string) (toDir, fromFile string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case helperArgTo:
			i++
			if i >= len(args) {
				return "", "", errors.New("missing value for --to")
			}
			toDir = args[i]
		case helperArgFrom:
			i++
			if i >= len(args) {
				return "", "", errors.New("missing value for --from")
			}
			fromFile = args[i]
		default:
			return "", "", fmt.Errorf("unexpected argument: %s", args[i])
		}
	}
	if toDir == "" || fromFile == "" {
		return "", "", errors.New("missing required arguments")
	}
	return toDir, fromFile, nil
}

func updateMetaInDir(toDir, fromFile string) error {
	toDir, err := filepath.Abs(toDir)
	if err != nil {
		return err
	}
	fromFile, err = filepath.Abs(fromFile)
	if err != nil {
		return err
	}

	metaData, err := os.ReadFile(fromFile)
	if err != nil {
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	metaPath := windowsMetaPath()
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	if err := os.WriteFile(metaPath, metaData, 0o644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

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

func finalizeUpdateInDir(toDir string) error {
	toDir, err := filepath.Abs(toDir)
	if err != nil {
		return err
	}

	staged := filepath.Join(toDir, "tnr.new")
	if _, err := os.Stat(staged); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	exe := filepath.Join(toDir, "tnr.exe")
	old := filepath.Join(toDir, "tnr.old")
	marker := filepath.Join(toDir, ".tnr-update")

	if err := os.MkdirAll(toDir, 0o755); err != nil {
		return err
	}

	_ = os.Remove(old)

	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}

	if err := os.Rename(staged, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("rename staged binary: %w", err)
	}

	_ = os.Remove(marker)
	_ = removeOldBackup(old)

	return nil
}

func removeOldBackup(oldPath string) error {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := os.Remove(oldPath); err == nil {
			return nil
		}
		if i < maxRetries-1 {
			delay := time.Duration(100*(1<<i)) * time.Millisecond
			time.Sleep(delay)
		}
	}
	return os.Remove(oldPath)
}

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

func TryFinalizeStagedUpdateImmediately(ctx context.Context, exePath string) (bool, error) {
	dir := filepath.Dir(exePath)
	staged := filepath.Join(dir, "tnr.new")

	if _, err := os.Stat(staged); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	writable := dirWritable(dir)
	elevated := isElevated()

	if writable || elevated {
		if err := finalizeUpdateInDir(dir); err != nil {
			return false, fmt.Errorf("failed to finalize update: %w", err)
		}
		return true, nil
	}

	if err := runElevatedFinalizeHelper(ctx, dir); err != nil {
		return false, nil
	}

	if _, err := os.Stat(staged); err == nil {
		return false, nil
	}

	return true, nil
}
