package autoupdate

import "testing"

func TestParseParts(t *testing.T) {
	tests := []struct {
		in   string
		want [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.0.0", [3]int{1, 0, 0}},
		{"2.0.0-beta.1", [3]int{2, 0, 0}},
		{"0.0.9", [3]int{0, 0, 9}},
	}
	for _, tt := range tests {
		if got := parseParts(tt.in); got != tt.want {
			t.Fatalf("parseParts(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestVersionLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.2.0", "1.2.0", false},
		{"1.10.0", "1.2.0", false},
		{"0.9.9", "1.0.0", true},
	}
	for _, tt := range tests {
		if got := versionLess(tt.a, tt.b); got != tt.want {
			t.Fatalf("versionLess(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
