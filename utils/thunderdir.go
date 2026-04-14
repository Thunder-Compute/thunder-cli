package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var warnFallbackOnce sync.Once

// ThunderDir returns the directory used to store Thunder CLI state
// (config, SSH keys, locks). Resolution order:
//
//  1. $TNR_HOME if set
//  2. $HOME/.thunder
//  3. os.UserCacheDir()/thunder
//  4. os.TempDir()/thunder-<uid>
//
// The first writable candidate is created (0700) and returned. Writability
// is probed by creating a temp file — MkdirAll can succeed on an overlay
// filesystem where actual writes still fail with EROFS. When the preferred
// location ($HOME/.thunder) is not usable, a one-time warning is printed to
// stderr so agents in sandboxed environments know state isn't persistent.
func ThunderDir() (string, error) {
	if explicit := os.Getenv("TNR_HOME"); explicit != "" {
		return ensureWritableDir(explicit)
	}

	var candidates []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".thunder"))
	}
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		candidates = append(candidates, filepath.Join(cache, "thunder"))
	}
	candidates = append(candidates, tempFallback())

	var firstErr error
	for i, dir := range candidates {
		got, err := ensureWritableDir(dir)
		if err == nil {
			if i > 0 {
				preferred := candidates[0]
				warnFallbackOnce.Do(func() {
					fmt.Fprintf(os.Stderr,
						"tnr: %s is not writable, using %s for this session (set TNR_HOME to override)\n",
						preferred, got)
				})
			}
			return got, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return "", fmt.Errorf("no writable thunder directory: %w", firstErr)
}

// ThunderSubdir returns ThunderDir()/name, creating it if needed.
func ThunderSubdir(name string) (string, error) {
	base, err := ThunderDir()
	if err != nil {
		return "", err
	}
	sub := filepath.Join(base, name)
	if err := os.MkdirAll(sub, 0o700); err != nil {
		return "", err
	}
	return sub, nil
}

func ensureWritableDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return dir, nil
}

func tempFallback() string {
	name := "thunder"
	if uid := os.Getuid(); uid >= 0 {
		name = fmt.Sprintf("thunder-%d", uid)
	}
	return filepath.Join(os.TempDir(), name)
}
