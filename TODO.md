# CLI TODO

## Terminal theme legibility (2026-07-28)

The CLI emits ANSI palette indices, never RGB, so the terminal's scheme decides
what every colour looks like. `internal/termcontrast` resolves the escapes we
actually emit against 10 real schemes and
`tui.TestRenderedTextStaysLegibleAcrossTerminalPalettes` gates them on every
build. Table headers, help text, success green and warning yellow are fixed;
the primary blue is not.

- [ ] **Move the remaining informational uses of the primary blue onto the
      terminal's default foreground.** It is still used for the list cursor
      `▶`, the selected row, inline info lines (`ℹ Restoring from a
      snapshot…`) and the login URL. All four carry information and all four
      collapse to 1.14:1 on macOS Terminal's Ocean profile and 1.76:1 on the
      Windows PowerShell default, so on those terminals `tnr create` and
      `tnr delete` show no visible selection indicator at all. No palette index
      fixes this — blue text on a blue background is a hue collision, not a
      light/dark mismatch, and resolving the terminal's actual palette needs an
      OSC 4 query that legacy conhost does not answer.

      The fix is to render those four in bold (or underlined, for the URL)
      default foreground and keep `theme.PrimaryColor` for borders and the
      spinner, where washing out costs nothing. `contrast_test.go` already
      covers all three TUI cases and logs them as `KNOWN:` instead of failing;
      drop the `known: true` flags once they are converted. This is a visible
      change to the CLI's look, so it needs a deliberate sign-off rather than
      being folded into the legibility fix.
