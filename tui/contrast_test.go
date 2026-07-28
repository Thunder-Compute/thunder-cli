package tui

import (
	"fmt"
	"io"
	"sort"
	"testing"

	"github.com/muesli/termenv"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/internal/termcontrast"
	helpmenus "github.com/Thunder-Compute/thunder-cli/tui/help-menus"
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
	render func() string
	// floor returns the minimum acceptable ratio given the scheme's default
	// foreground contrast.
	floor func(defaultRatio float64) float64
}

func structural(defaultRatio float64) float64 { return defaultRatio * structuralFloor }
func accent(float64) float64                  { return accentMinRatio }

// TestRenderedTextStaysLegibleAcrossTerminalPalettes resolves the escape
// sequences the CLI emits against real terminal colour schemes and checks the
// resulting text is still readable.
//
// The CLI only ever emits palette indices, never RGB, so the same escape looks
// completely different per scheme. Bright blue is #3B78FF in Windows Terminal
// but pure #0000FF in the legacy Windows console, where PowerShell's default
// background is the navy #012456. That pairing is 1.76:1, which is what made
// the status table headers unreadable on Windows.
func TestRenderedTextStaysLegibleAcrossTerminalPalettes(t *testing.T) {
	for _, c := range contrastCases() {
		for _, p := range termcontrast.Palettes {
			t.Run(p.Name, func(t *testing.T) {
				setRenderTarget(p)
				defaultRatio := termcontrast.Ratio(p.Fg, p.Bg)
				ratio := termcontrast.WorstRatio(c.render(), p)

				if min := c.floor(defaultRatio); ratio < min {
					t.Errorf("%s: contrast %.2f:1 is below the %.2f:1 floor (scheme default text is %.2f:1)",
						c.name, ratio, min, defaultRatio)
				}
			})
		}
	}
}

func contrastCases() []contrastCase {
	m := newContrastStatusModel()

	return []contrastCase{
		// Status view.
		{"status table header", func() string { return m.styles.header.Render("UUID") }, structural},
		{"status table cell", func() string { return m.styles.cell.Render("m31bzr0i") }, structural},
		{"field label", func() string { return LabelStyle().Render("Instance ID:") }, structural},
		{"help footer", func() string { return HelpStyle().Render("Press 'Q' to close") }, accent},
		{"status RUNNING", func() string { return m.formatStatus("RUNNING", 14) }, accent},
		{"status DELETING", func() string { return m.formatStatus("DELETING", 14) }, accent},
		{"status PROVISIONING", func() string { return m.formatStatus("PROVISIONING", 14) }, accent},

		// Help menus.
		{"help section heading", func() string { return helpmenus.SectionStyle.Render("COMMANDS") }, structural},
		{"help description", func() string { return helpmenus.DescStyle.Render("Create an instance") }, structural},
		{"help link", func() string { return helpmenus.LinkStyle.Render("https://www.thundercompute.com/docs") }, structural},
		{"help flag", func() string { return helpmenus.FlagStyle.Render("--gpu") }, accent},

		{"help banner heading", func() string { return helpmenus.HeaderStyle.Render("tnr create") }, structural},
		{"help command label", func() string { return helpmenus.CommandStyle.Render("Docs") }, structural},
		{"help example command", func() string { return helpmenus.CommandTextStyle.Render("tnr status") }, structural},
		{"help example comment", func() string { return helpmenus.ExampleStyle.Render("# monitor") }, accent},

		// Selection. The ▶ marker and the weight carry it, so neither depends
		// on a colour the scheme is free to reinterpret.
		{"selection cursor", func() string { return CursorStyle().Render("▶") }, structural},
		{"selected list item", func() string { return SelectedStyle().Render("A100") }, structural},
		{"panel title", func() string { return TitleStyle().Render("Select a GPU") }, structural},
		// A spinner that cannot be seen cannot distinguish "working" from
		// "hung", so it is held to the same floor as text.
		{"spinner", func() string { return SpinnerStyle().Render("⣽") }, structural},
		// A box frame that vanishes leaves padded text floating with no
		// grouping, so it is held to the same floor. Borders emit no escape of
		// their own; this fails the moment someone sets BorderForeground.
		{"panel border", func() string { return NewPanelStyles().Panel.Render("x") }, structural},
	}
}

// TestContrastReport prints the full ratio matrix. It never fails; run it with
// `go test ./tui -run TestContrastReport -v` to see how every styled element
// lands on every scheme.
func TestContrastReport(t *testing.T) {
	cases := contrastCases()
	worst := make(map[string]float64, len(cases))

	t.Log("contrast ratio of each styled element against each scheme's background")
	for _, p := range termcontrast.Palettes {
		setRenderTarget(p)
		line := fmt.Sprintf("%-40s default=%5.2f", p.Name, termcontrast.Ratio(p.Fg, p.Bg))
		for _, c := range cases {
			ratio := termcontrast.WorstRatio(c.render(), p)
			line += fmt.Sprintf("  %s=%.2f", c.name, ratio)
			if prev, ok := worst[c.name]; !ok || ratio < prev {
				worst[c.name] = ratio
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
		t.Logf("  %-22s %.2f:1", n, worst[n])
	}
}

func newContrastStatusModel() statusModel {
	InitCommonStyles(io.Discard)
	helpmenus.InitHelpStyles(io.Discard)
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
