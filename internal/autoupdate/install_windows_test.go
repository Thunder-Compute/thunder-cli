//go:build windows

package autoupdate

import (
	"testing"
)

func TestQuoteWindowsArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "noQuotingNeeded",
			in:   `C:\tnr\tnr.exe`,
			want: `C:\tnr\tnr.exe`,
		},
		{
			name: "spaces",
			in:   `C:\Program Files (x86)\tnr\tnr.exe`,
			want: "\"C:\\Program Files (x86)\\tnr\\tnr.exe\"",
		},
		{
			name: "trailingBackslash",
			in:   `C:\Program Files (x86)\tnr\`,
			want: "\"C:\\Program Files (x86)\\tnr\\\\\"",
		},
		{
			name: "embeddedQuote",
			in:   `with"quote`,
			want: "\"with\\\"quote\"",
		},
		{
			name: "tabCharacter",
			in:   "tab\tseparated",
			want: "\"tab\tseparated\"",
		},
		{
			name: "emptyString",
			in:   "",
			want: "\"\"",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := quoteWindowsArg(tc.in); got != tc.want {
				t.Fatalf("quoteWindowsArg(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

