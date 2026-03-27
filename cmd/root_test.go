package cmd

import (
	"testing"

	"github.com/Thunder-Compute/thunder-cli/internal/updatepolicy"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// ── shouldSkipUpdateCheck ───────────────────────────────────────────────────

func TestShouldSkipUpdateCheck(t *testing.T) {
	tests := []struct {
		name     string
		buildCmd func() *cobra.Command
		want     bool
	}{
		{
			name: "nil command",
			buildCmd: func() *cobra.Command {
				return nil
			},
			want: false,
		},
		{
			name: "help command",
			buildCmd: func() *cobra.Command {
				return &cobra.Command{Use: "help"}
			},
			want: true,
		},
		{
			name: "completion command",
			buildCmd: func() *cobra.Command {
				return &cobra.Command{Use: "completion"}
			},
			want: true,
		},
		{
			name: "version command",
			buildCmd: func() *cobra.Command {
				return &cobra.Command{Use: "version"}
			},
			want: true,
		},
		{
			name: "normal command",
			buildCmd: func() *cobra.Command {
				return &cobra.Command{Use: "status"}
			},
			want: false,
		},
		{
			name: "annotated command",
			buildCmd: func() *cobra.Command {
				return &cobra.Command{
					Use:         "update",
					Annotations: map[string]string{"skipUpdateCheck": "true"},
				}
			},
			want: true,
		},
		{
			name: "child of help command",
			buildCmd: func() *cobra.Command {
				parent := &cobra.Command{Use: "help"}
				child := &cobra.Command{Use: "topic"}
				parent.AddCommand(child)
				return child
			},
			want: true,
		},
		{
			name: "parent annotated",
			buildCmd: func() *cobra.Command {
				parent := &cobra.Command{
					Use:         "update",
					Annotations: map[string]string{"skipUpdateCheck": "true"},
				}
				child := &cobra.Command{Use: "check"}
				parent.AddCommand(child)
				return child
			},
			want: true,
		},
		{
			name: "help flag set",
			buildCmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "status"}
				cmd.Flags().BoolP("help", "h", false, "help")
				_ = cmd.Flags().Set("help", "true")
				return cmd
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.buildCmd()
			assert.Equal(t, tt.want, shouldSkipUpdateCheck(cmd))
		})
	}
}

// ── releaseTag ──────────────────────────────────────────────────────────────

func TestReleaseTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		ver  string
		want string
	}{
		{name: "tag with v prefix", tag: "v1.2.3", want: "v1.2.3"},
		{name: "tag without v prefix", tag: "1.2.3", want: "v1.2.3"},
		{name: "tag with V prefix", tag: "V1.2.3", want: "V1.2.3"},
		{name: "empty tag falls back to version", tag: "", ver: "2.0.0", want: "v2.0.0"},
		{name: "both empty", tag: "", ver: "", want: ""},
		{name: "whitespace tag returns empty", tag: "  ", ver: "1.0.0", want: ""},
		{name: "tag with whitespace", tag: "  v1.5.0  ", want: "v1.5.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := releaseTag(updatepolicy.Result{
				LatestTag:     tt.tag,
				LatestVersion: tt.ver,
			})
			assert.Equal(t, tt.want, got)
		})
	}
}

// ── displayVersion ──────────────────────────────────────────────────────────

func TestDisplayVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"V1.2.3", "V1.2.3"},
		{"", "unknown"},
		{"  ", "unknown"},
		{"  1.0.0  ", "v1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, displayVersion(tt.input))
		})
	}
}

// ── isPMManaged / detectPackageManager ──────────────────────────────────────
// These are already tested in update_test.go but we add a few edge cases.

func TestIsPMManaged_EdgeCases(t *testing.T) {
	assert.False(t, isPMManaged("/usr/local/bin/tnr"))
	assert.False(t, isPMManaged("/home/user/bin/tnr"))
	assert.True(t, isPMManaged("/opt/homebrew/bin/tnr"))
}

func TestDetectPackageManager_EdgeCases(t *testing.T) {
	assert.Equal(t, "", detectPackageManager("/usr/local/bin/tnr"))
	assert.Equal(t, "homebrew", detectPackageManager("/opt/homebrew/Cellar/tnr/1.0/bin/tnr"))
}
