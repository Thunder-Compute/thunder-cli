package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThunderDirHappyPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TNR_HOME", "")

	got, err := ThunderDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, ".thunder"), got)
	assert.DirExists(t, got)
}

func TestThunderDirTNRHomeOverride(t *testing.T) {
	tmp := t.TempDir()
	override := filepath.Join(tmp, "custom-state")
	t.Setenv("HOME", tmp)
	t.Setenv("TNR_HOME", override)

	got, err := ThunderDir()
	require.NoError(t, err)
	assert.Equal(t, override, got)
	assert.DirExists(t, got)
}

func TestThunderDirTNRHomeHardFails(t *testing.T) {
	// TNR_HOME is an explicit override with no fallback.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "file-not-dir")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0600))

	t.Setenv("HOME", tmp)
	t.Setenv("TNR_HOME", blocker)

	_, err := ThunderDir()
	assert.Error(t, err)
}

func TestThunderDirFallsBackToUserCacheDir(t *testing.T) {
	tmp := t.TempDir()
	// Make $HOME/.thunder unusable by planting a regular file there.
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".thunder"), []byte("x"), 0600))

	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "xdg-cache"))
	t.Setenv("TNR_HOME", "")

	got, err := ThunderDir()
	require.NoError(t, err)
	assert.DirExists(t, got)

	cacheBase, err := os.UserCacheDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cacheBase, "thunder"), got)
}

func TestSavePrivateKeyUsesFallback(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, ".thunder"), []byte("x"), 0600))
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "xdg-cache"))
	t.Setenv("TNR_HOME", "")

	require.NoError(t, SavePrivateKey("test-uuid", "PRIVATE KEY CONTENT"))
	assert.True(t, KeyExists("test-uuid"))

	data, err := os.ReadFile(GetKeyFile("test-uuid"))
	require.NoError(t, err)
	assert.Equal(t, "PRIVATE KEY CONTENT", string(data))
}
