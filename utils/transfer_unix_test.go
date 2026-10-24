//go:build !windows

package utils

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRsyncTransferPreservesKeyPath(t *testing.T) {
	rsync, err := exec.LookPath("rsync")
	if err != nil {
		t.Skip("rsync is not installed")
	}
	binDir := t.TempDir()
	// Replace only SSH: real rsync must parse -e and complete its protocol
	// locally, so this catches quoting errors without opening a connection.
	ssh := `#!/bin/sh
if [ "$1" != "-i" ] || [ "$2" != "$TNR_TEST_KEY" ]; then
    echo "SSH identity path was split or changed" >&2
    exit 42
fi
shift 2
while [ "$#" -gt 0 ]; do
    case "$1" in
        -p|-o|-l) shift 2 ;;
        ubuntu@127.0.0.1|127.0.0.1)
            shift
            [ "$1" = "rsync" ] || exit 43
            shift
            exec "$TNR_TEST_RSYNC" "$@"
            ;;
        *) exit 44 ;;
    esac
done
exit 45
`
	if err := os.WriteFile(filepath.Join(binDir, "ssh"), []byte(ssh), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TNR_TEST_RSYNC", rsync)
	// Keep transfer paths separate from the identity-path cases: the SSH
	// stand-in executes rsync directly rather than through a remote shell.
	transferDir := t.TempDir()

	for _, name := range []string{"plain", "home with spaces", "owner's home", `home "quoted"`, `home\with\backslashes`, "home\twith\ttabs", "home $literal`name`"} {
		for _, upload := range []bool{true, false} {
			direction := "download"
			if upload {
				direction = "upload"
			}
			t.Run(name+"/"+direction, func(t *testing.T) {
				dir, err := os.MkdirTemp(transferDir, "transfer-")
				if err != nil {
					t.Fatal(err)
				}
				keyFile := filepath.Join(dir, name, "key")
				if err := os.MkdirAll(filepath.Dir(keyFile), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(keyFile, []byte("synthetic test key"), 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("TNR_TEST_KEY", keyFile)
				source := filepath.Join(dir, "source")
				destination := filepath.Join(dir, "destination")
				const content = "Thunder transfer regression\n"
				if err := os.WriteFile(source, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				localPath, remotePath := source, destination
				if !upload {
					localPath, remotePath = destination, source
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var output bytes.Buffer
				if err := rsyncTransfer(ctx, keyFile, "127.0.0.1", 22, localPath, remotePath, upload, &output, &output); err != nil {
					t.Fatalf("transfer failed: %v\n%s", err, output.String())
				}
				got, err := os.ReadFile(destination)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != content {
					t.Fatalf("transferred content = %q, want %q", got, content)
				}
			})
		}
	}
}
