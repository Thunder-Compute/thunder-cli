package utils

import "strings"

// "development" is the user-facing alias for the legacy wire value "prototyping".
// CLI accepts either; everything internal and the API still sees "prototyping".
const (
	modeInputDevelopment = "development"
	modeWirePrototyping  = "prototyping"
	modeWireProduction   = "production"
)

// NormalizeModeInput accepts user input ("development", "prototyping", "production",
// any case) and returns the wire value the API expects. Unknown input is returned
// lowercased so the caller can surface a validation error.
func NormalizeModeInput(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == modeInputDevelopment {
		return modeWirePrototyping
	}
	return s
}

// DisplayMode maps a wire mode value to its user-facing label (lowercase).
func DisplayMode(wire string) string {
	switch strings.ToLower(wire) {
	case modeWirePrototyping:
		return modeInputDevelopment
	default:
		return strings.ToLower(wire)
	}
}
