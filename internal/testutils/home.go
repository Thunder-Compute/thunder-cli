package testutils

import "testing"

// SetHome isolates os.UserHomeDir on both Unix and Windows. The environment is
// restored automatically when the test completes.
func SetHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}
