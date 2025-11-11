package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileCreatesExecutable(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "tnr")
	if err := os.WriteFile(src, []byte("original-binary"), 0o700); err != nil {
		t.Fatalf("failed to write source executable: %v", err)
	}

	dst := filepath.Join(dstDir, "tnr")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile returned error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(data) != "original-binary" {
		t.Fatalf("unexpected destination contents: %q", string(data))
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat destination: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("expected destination permissions 755, got %v", info.Mode().Perm())
	}
}

func TestCopyFileFailsForMissingSource(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "tnr")
	if err := copyFile("non-existent", dst); err == nil {
		t.Fatal("expected error when source file is missing")
	}
}
