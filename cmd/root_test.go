package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/internal/autoupdate"
	"github.com/Thunder-Compute/thunder-cli/internal/updatepolicy"
	"github.com/Thunder-Compute/thunder-cli/tui"
	"github.com/Thunder-Compute/thunder-cli/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func TestIsUserError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "usage sentinel",
			err:  fmt.Errorf("create failed: %w", ErrUsage),
			want: true,
		},
		{
			name: "tui cancellation sentinel",
			err:  fmt.Errorf("modify failed: %w", tui.ErrCancelled),
			want: true,
		},
		{
			name: "transfer user sentinel",
			err:  fmt.Errorf("scp failed: %w", utils.ErrTransferUser),
			want: true,
		},
		{
			name: "transfer cancelled sentinel",
			err:  fmt.Errorf("scp failed: %w", utils.ErrTransferCancelled),
			want: true,
		},
		{
			name: "ssh unreachable sentinel",
			err:  fmt.Errorf("connect failed: %w", utils.ErrSSHUnreachable),
			want: true,
		},
		{
			name: "persistent auth sentinel",
			err:  fmt.Errorf("connect failed: %w", utils.ErrPersistentAuthFailure),
			want: true,
		},
		{
			name: "key unreadable sentinel",
			err:  fmt.Errorf("connect failed: %w", utils.ErrKeyUnreadable),
			want: true,
		},
		{
			name: "api transport sentinel",
			err:  fmt.Errorf("status failed: %w", api.ErrTransport),
			want: true,
		},
		{
			name: "api error",
			err:  fmt.Errorf("status failed: %w", &api.APIError{StatusCode: 500, Message: "server unavailable"}),
			want: true,
		},
		{
			name: "pflag missing flag",
			err:  rootFlagError(t, "--missing"),
			want: true,
		},
		{
			name: "pflag value required",
			err:  rootFlagError(t, "--name"),
			want: true,
		},
		{
			name: "pflag invalid value",
			err:  rootFlagError(t, "--count=bad"),
			want: true,
		},
		{
			name: "pflag invalid syntax",
			err:  rootFlagError(t, "---bad"),
			want: true,
		},
		{
			name: "context cancelled",
			err:  fmt.Errorf("operation failed: %w", context.Canceled),
			want: true,
		},
		{
			name: "context deadline",
			err:  fmt.Errorf("operation failed: %w", context.DeadlineExceeded),
			want: true,
		},
		{
			name: "net timeout",
			err:  fmt.Errorf("operation failed: %w", &net.DNSError{IsTimeout: true}),
			want: true,
		},
		{
			name: "substring fallback",
			err:  errors.New("exec failed: executable file not found"),
			want: true,
		},
		{
			name: "internal error",
			err:  errors.New("nil pointer while rendering status"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUserError(tt.err))
		})
	}
}

func pflagError(t *testing.T, arg string) error {
	t.Helper()

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.String("name", "", "name")
	flags.Int("count", 0, "count")

	err := flags.Parse([]string{arg})
	if err == nil {
		t.Fatalf("expected pflag.Parse(%q) to fail", arg)
	}
	return err
}

func rootFlagError(t *testing.T, arg string) error {
	t.Helper()

	err := rootCmd.FlagErrorFunc()(nil, pflagError(t, arg))
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("root flag error should wrap ErrUsage, got %v", err)
	}
	return err
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
	assert.False(t, autoupdate.IsPMManaged("/usr/local/bin/tnr"))
	assert.False(t, autoupdate.IsPMManaged("/home/user/bin/tnr"))
	assert.True(t, autoupdate.IsPMManaged("/opt/homebrew/bin/tnr"))
}

func TestDetectPackageManager_EdgeCases(t *testing.T) {
	assert.Equal(t, "", detectPackageManager("/usr/local/bin/tnr"))
	assert.Equal(t, "homebrew", detectPackageManager("/opt/homebrew/Cellar/tnr/1.0/bin/tnr"))
}
