// Package termcontrast resolves the ANSI escape sequences the CLI emits into
// concrete RGB values for a set of real-world terminal colour schemes, so that
// tests can assert text stays legible regardless of the user's theme.
//
// The CLI never sends RGB. Styles such as lipgloss.Color("12") emit a palette
// index (SGR 94, "bright blue"), and the terminal decides what that index looks
// like. A colour that reads well on one scheme can be invisible on another:
// bright blue is #3B78FF in Windows Terminal but pure #0000FF in the legacy
// Windows console, where the default background is the navy #012456.
package termcontrast

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// RGB is a 24-bit colour.
type RGB struct{ R, G, B uint8 }

// Palette is one terminal colour scheme: its default foreground and background
// plus the 16 ANSI slots in SGR order (black, red, green, yellow, blue,
// magenta, cyan, white, then the eight bright variants).
type Palette struct {
	Name string
	Fg   RGB
	Bg   RGB
	ANSI [16]RGB
}

func hex(s string) RGB {
	v, err := strconv.ParseUint(strings.TrimPrefix(s, "#"), 16, 32)
	if err != nil {
		panic("termcontrast: bad hex colour " + s)
	}
	return RGB{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}
}

func ansi16(c ...string) [16]RGB {
	var out [16]RGB
	for i, s := range c {
		out[i] = hex(s)
	}
	return out
}

// Palettes are the schemes we hold ourselves to. They cover the defaults a user
// is most likely to be sitting in on Windows, macOS and Linux, plus the light
// schemes that catch the opposite failure mode.
var Palettes = []Palette{
	// The Windows PowerShell shortcut keeps the legacy console palette but
	// overrides two slots: DarkMagenta becomes the navy #012456 and is used as
	// the background, DarkYellow becomes #EEEDF0 and is used as the foreground.
	// Bright blue stays pure #0000FF, which is why blue text vanishes here.
	{
		Name: "Windows PowerShell (legacy conhost)",
		Fg:   hex("#EEEDF0"),
		Bg:   hex("#012456"),
		ANSI: ansi16(
			"#000000", "#800000", "#008000", "#EEEDF0", "#000080", "#012456", "#008080", "#C0C0C0",
			"#808080", "#FF0000", "#00FF00", "#FFFF00", "#0000FF", "#FF00FF", "#00FFFF", "#FFFFFF",
		),
	},
	{
		Name: "cmd.exe (legacy conhost)",
		Fg:   hex("#C0C0C0"),
		Bg:   hex("#000000"),
		ANSI: ansi16(
			"#000000", "#800000", "#008000", "#808000", "#000080", "#800080", "#008080", "#C0C0C0",
			"#808080", "#FF0000", "#00FF00", "#FFFF00", "#0000FF", "#FF00FF", "#00FFFF", "#FFFFFF",
		),
	},
	{
		Name: "Windows Terminal (Campbell)",
		Fg:   hex("#CCCCCC"),
		Bg:   hex("#0C0C0C"),
		ANSI: campbell,
	},
	{
		Name: "Windows Terminal (Campbell PowerShell)",
		Fg:   hex("#CCCCCC"),
		Bg:   hex("#012456"),
		ANSI: campbell,
	},
	{
		Name: "macOS Terminal (Basic)",
		Fg:   hex("#000000"),
		Bg:   hex("#FFFFFF"),
		ANSI: appleTerminal,
	},
	{
		Name: "macOS Terminal (Ocean)",
		Fg:   hex("#FFFFFF"),
		Bg:   hex("#224FBC"),
		ANSI: appleTerminal,
	},
	{
		Name: "Solarized Dark",
		Fg:   hex("#839496"),
		Bg:   hex("#002B36"),
		ANSI: solarized,
	},
	{
		Name: "Solarized Light",
		Fg:   hex("#657B83"),
		Bg:   hex("#FDF6E3"),
		ANSI: solarized,
	},
	{
		Name: "VS Code terminal (Dark+)",
		Fg:   hex("#CCCCCC"),
		Bg:   hex("#1E1E1E"),
		ANSI: ansi16(
			"#000000", "#CD3131", "#0DBC79", "#E5E510", "#2472C8", "#BC3FBC", "#11A8CD", "#E5E5E5",
			"#666666", "#F14C4C", "#23D18B", "#F5F543", "#3B8EEA", "#D670D6", "#29B8DB", "#E5E5E5",
		),
	},
	{
		Name: "GNOME Terminal (Tango dark)",
		Fg:   hex("#D3D7CF"),
		Bg:   hex("#300A24"),
		ANSI: ansi16(
			"#2E3436", "#CC0000", "#4E9A06", "#C4A000", "#3465A4", "#75507B", "#06989A", "#D3D7CF",
			"#555753", "#EF2929", "#8AE234", "#FCE94F", "#729FCF", "#AD7FA8", "#34E2E2", "#EEEEEC",
		),
	},
}

var campbell = ansi16(
	"#0C0C0C", "#C50F1F", "#13A10E", "#C19C00", "#0037DA", "#881798", "#3A96DD", "#CCCCCC",
	"#767676", "#E74856", "#16C60C", "#F9F1A5", "#3B78FF", "#B4009E", "#61D6D6", "#F2F2F2",
)

var appleTerminal = ansi16(
	"#000000", "#C23621", "#25BC24", "#ADAD27", "#492EE1", "#D338D3", "#33BBC8", "#CBCCCD",
	"#818383", "#FC391F", "#31E722", "#EAEC23", "#5833FF", "#F935F8", "#14F0F0", "#E9EBEB",
)

// Solarized reuses its base greys for the bright slots, so "bright blue" is the
// grey #839496 rather than a brighter blue.
var solarized = ansi16(
	"#073642", "#DC322F", "#859900", "#B58900", "#268BD2", "#D33682", "#2AA198", "#EEE8D5",
	"#002B36", "#CB4B16", "#586E75", "#657B83", "#839496", "#6C71C4", "#93A1A1", "#FDF6E3",
)

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// Foreground resolves the foreground colour that the first visible text in
// rendered will actually be painted in under p. It walks the SGR codes in
// order and stops at the first printable character, so the trailing reset that
// lipgloss appends does not erase the answer. Text with no explicit colour
// resolves to the palette's default foreground.
func Foreground(rendered string, p Palette) RGB {
	fg, faint := p.Fg, false
	pos := 0
	for _, loc := range sgrPattern.FindAllStringSubmatchIndex(rendered, -1) {
		if hasVisibleText(rendered[pos:loc[0]]) {
			break
		}
		fg, faint = applySGR(fg, faint, rendered[loc[2]:loc[3]], p)
		pos = loc[1]
	}
	if faint {
		// Terminals vary in how they dim SGR 2; some ignore it entirely. Half
		// way to the background is a pessimistic stand-in, so a ratio measured
		// here is a lower bound on what the user actually sees.
		fg = blend(fg, p.Bg, 0.5)
	}
	return fg
}

func blend(a, b RGB, t float64) RGB {
	mix := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return RGB{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B)}
}

func hasVisibleText(s string) bool {
	return strings.TrimSpace(s) != ""
}

func applySGR(fg RGB, faint bool, params string, p Palette) (RGB, bool) {
	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			continue
		}
		switch {
		case n == 0:
			fg, faint = p.Fg, false
		case n == 2:
			faint = true
		case n == 22:
			faint = false
		case n == 39:
			fg = p.Fg
		case n >= 30 && n <= 37:
			fg = p.ANSI[n-30]
		case n >= 90 && n <= 97:
			fg = p.ANSI[n-90+8]
		case n == 38 && i+2 < len(fields) && fields[i+1] == "5":
			idx, _ := strconv.Atoi(fields[i+2])
			fg = xterm256(idx, p)
			i += 2
		case n == 38 && i+4 < len(fields) && fields[i+1] == "2":
			r, _ := strconv.Atoi(fields[i+2])
			g, _ := strconv.Atoi(fields[i+3])
			b, _ := strconv.Atoi(fields[i+4])
			fg = RGB{uint8(r), uint8(g), uint8(b)}
			i += 4
		}
	}
	return fg, faint
}

// xterm256 maps a 256-colour index to RGB. The first 16 come from the palette;
// the rest are the fixed 6x6x6 cube and greyscale ramp.
func xterm256(i int, p Palette) RGB {
	switch {
	case i < 16:
		return p.ANSI[i]
	case i < 232:
		i -= 16
		steps := []uint8{0, 95, 135, 175, 215, 255}
		return RGB{steps[i/36], steps[(i/6)%6], steps[i%6]}
	case i < 256:
		v := uint8(8 + (i-232)*10)
		return RGB{v, v, v}
	}
	return p.Fg
}

// IsDark reports whether a background should be treated as dark, matching the
// threshold termenv uses when it answers lipgloss.HasDarkBackground.
func IsDark(bg RGB) bool { return luminance(bg) < 0.5 }

// Ratio is the WCAG 2.x contrast ratio between two colours, from 1 (identical)
// to 21 (black on white).
func Ratio(a, b RGB) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c RGB) float64 {
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

func channel(v uint8) float64 {
	f := float64(v) / 255
	if f <= 0.03928 {
		return f / 12.92
	}
	return math.Pow((f+0.055)/1.055, 2.4)
}
