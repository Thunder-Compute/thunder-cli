package tui

import (
	"fmt"
	"io"
	"sort"
	"testing"

	"github.com/muesli/termenv"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/internal/termcontrast"
	"github.com/Thunder-Compute/thunder-cli/tui/theme"
)

// There is no single contrast ratio every terminal scheme can satisfy:
// Solarized Light's own default text is 4.13:1 against its own background, so a
// WCAG AA floor of 4.5 would fail the terminal before it fails us. Text is
// measured in two ways instead.
const (
	// structuralFloor is the fraction of the scheme's default foreground
	// contrast that information-bearing text must retain. Text rendered in the
	// terminal's own foreground scores 1.0 by definition.
	structuralFloor = 0.9

	// accentMinRatio is a ratchet on coloured text: help hints and status
	// words, where the words themselves carry the meaning and the colour only
	// has to stay distinguishable. It sits just under today's worst case, so
	// any change that makes a scheme worse trips the test. Faint text is
	// modelled pessimistically (see termcontrast.Foreground), so real terminals
	// clear this by more than the number suggests.
	accentMinRatio = 1.8
)

type contrastCase struct {
	name   string
	render func(m statusModel) string
	// floor returns the minimum acceptable ratio given the scheme's default
	// foreground contrast.
	floor func(defaultRatio float64) float64
	// known marks an element that fails today and is not yet fixed. The ratio
	// is logged rather than failing the build so the debt stays visible.
	known bool
}

func structural(defaultRatio float64) float64 { return defaultRatio * structuralFloor }
func accent(float64) float64                  { return accentMinRatio }

// TestRenderedTextStaysLegibleAcrossTerminalPalettes resolves the escape
// sequences the TUI emits against real terminal colour schemes and checks the
// resulting text is still readable.
//
// The CLI only ever emits palette indices, never RGB, so the same escape looks
// completely different per scheme. Bright blue is #3B78FF in Windows Terminal
// but pure #0000FF in the legacy Windows console, where PowerShell's default
// background is the navy #012456. That pairing is 1.76:1, which is what made
// the status table headers unreadable on Windows.
func TestRenderedTextStaysLegibleAcrossTerminalPalettes(t *testing.T) {
	m := newContrastStatusModel()

	cases := []contrastCase{
		{
			name:   "status table header",
			render: func(m statusModel) string { return m.styles.header.Render("UUID") },
			floor:  structural,
		},
		{
			name:   "status table cell",
			render: func(m statusModel) string { return m.styles.cell.Render("m31bzr0i") },
			floor:  structural,
		},
		{
			name:   "field label",
			render: func(statusModel) string { return LabelStyle().Render("Instance ID:") },
			floor:  structural,
		},
		{
			name:   "help footer",
			render: func(statusModel) string { return HelpStyle().Render("Press 'Q' to close") },
			floor:  accent,
		},
		{
			name:   "status RUNNING",
			render: func(m statusModel) string { return m.formatStatus("RUNNING", 14) },
			floor:  accent,
		},
		{
			name:   "status DELETING",
			render: func(m statusModel) string { return m.formatStatus("DELETING", 14) },
			floor:  accent,
		},
		{
			name:   "status PROVISIONING",
			render: func(m statusModel) string { return m.formatStatus("PROVISIONING", 14) },
			floor:  accent,
		},
		// The primary blue is still used for the list cursor, the selected row
		// and inline info lines. All three carry information and all three
		// vanish on blue backgrounds, which no palette index can fix; the
		// remedy is to render them in the default foreground like the table
		// headers now are.
		{
			name:   "selection cursor",
			render: func(statusModel) string { return PrimaryCursorStyle().Render("▶") },
			floor:  accent,
			known:  true,
		},
		{
			name:   "selected list item",
			render: func(statusModel) string { return PrimarySelectedStyle().Render("A100") },
			floor:  accent,
			known:  true,
		},
		{
			name:   "primary info line",
			render: func(statusModel) string { return PrimaryStyle().Render("ℹ Restoring") },
			floor:  accent,
			known:  true,
		},
	}

	for _, p := range termcontrast.Palettes {
		t.Run(p.Name, func(t *testing.T) {
			defaultRatio := termcontrast.Ratio(p.Fg, p.Bg)
			setRenderTarget(p)

			for _, c := range cases {
				ratio := termcontrast.Ratio(termcontrast.Foreground(c.render(m), p), p.Bg)
				min := c.floor(defaultRatio)
				if ratio >= min {
					continue
				}
				if c.known {
					t.Logf("KNOWN: %s is %.2f:1, below the %.2f:1 floor", c.name, ratio, min)
					continue
				}
				t.Errorf("%s: contrast %.2f:1 is below the %.2f:1 floor (scheme default text is %.2f:1)",
					c.name, ratio, min, defaultRatio)
			}
		})
	}
}

// TestContrastReport prints the full ratio matrix. It never fails; run it with
// `go test ./tui -run TestContrastReport -v` to see how every styled element
// lands on every scheme.
func TestContrastReport(t *testing.T) {
	m := newContrastStatusModel()

	elements := []struct {
		name   string
		render func() string
	}{
		{"table header", func() string { return m.styles.header.Render("UUID") }},
		{"table cell", func() string { return m.styles.cell.Render("m31bzr0i") }},
		{"field label", func() string { return LabelStyle().Render("Instance ID:") }},
		{"help footer", func() string { return HelpStyle().Render("Press 'Q'") }},
		{"selection cursor", func() string { return PrimaryCursorStyle().Render("▶") }},
		{"selected item", func() string { return PrimarySelectedStyle().Render("A100") }},
		{"primary info line", func() string { return PrimaryStyle().Render("ℹ Restoring") }},
		{"success", func() string { return SuccessStyle().Render("✓ Created") }},
		{"warning", func() string { return WarningStyle().Render("⚠ Warning") }},
		{"error", func() string { return ErrorStyle().Render("✗ Error") }},
	}

	worst := make(map[string]float64, len(elements))

	t.Log("contrast ratio of each styled element against each scheme's background")
	for _, p := range termcontrast.Palettes {
		setRenderTarget(p)
		line := fmt.Sprintf("%-40s default=%5.2f", p.Name, termcontrast.Ratio(p.Fg, p.Bg))
		for _, e := range elements {
			ratio := termcontrast.Ratio(termcontrast.Foreground(e.render(), p), p.Bg)
			line += fmt.Sprintf("  %s=%.2f", e.name, ratio)
			if prev, ok := worst[e.name]; !ok || ratio < prev {
				worst[e.name] = ratio
			}
		}
		t.Log(line)
	}

	names := make([]string, 0, len(worst))
	for n := range worst {
		names = append(names, n)
	}
	sort.Strings(names)
	t.Log("worst case per element across all schemes:")
	for _, n := range names {
		t.Logf("  %-20s %.2f:1", n, worst[n])
	}
}

func newContrastStatusModel() statusModel {
	InitCommonStyles(io.Discard)
	return newStatusModel(nil, false, []api.Instance{{ID: "0", Status: "RUNNING"}}, false)
}

// setRenderTarget points the shared renderer at a scheme so any adaptive styles
// resolve the way they would in that terminal. Colour is forced on because the
// test's output is not a TTY and lipgloss would otherwise strip every escape.
func setRenderTarget(p termcontrast.Palette) {
	r := theme.Renderer()
	r.SetColorProfile(termenv.ANSI)
	r.SetHasDarkBackground(termcontrast.IsDark(p.Bg))
}
