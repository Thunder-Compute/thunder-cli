# CLI TODO

## Terminal theme legibility (2026-07-28)

The CLI emits ANSI palette indices, never RGB, so the terminal's scheme decides
what every colour looks like. `internal/termcontrast` resolves the escapes we
actually emit against 10 real schemes and
`tui.TestRenderedTextStaysLegibleAcrossTerminalPalettes` asserts on every build.

The governing rule: no information may depend on a colour rendering the way we
expect. Colour may make something faster to find, never possible to read.
`NO_COLOR=1 tnr status` and `NO_COLOR=1 tnr --help` are the cheap check, since
lipgloss honours it through `EnvColorProfile()`. If the output loses hierarchy
with colour stripped, it is leaning on something the terminal does not
guarantee.

There is no accent colour left. Every use of it turned out to carry
information, so `theme.PrimaryColor`, `theme.Primary()` and the `primaryStyle`
mirror in `tui` are all gone and the binary emits no blue at all.

- [ ] **Every `tnr` invocation blocks for 5 seconds on a terminal that does not
      answer OSC 11.** Confirmed by timing the real binary under a PTY with no
      responder: `tnr --version` takes 5.02s. The stack shows it happens before
      `main()`:

          bubbletea@v1.3.10/tea_init.go:21  func init() { _ = lipgloss.HasDarkBackground() }
          -> lipgloss.HasDarkBackground -> termenv Output.BackgroundColor
          -> termStatusReport -> waitForData(OSCTimeout = 5s)

      Bubble Tea forces the query in package init deliberately, to stop the
      query racing its own terminal acquisition later; the comment says it is
      removed in v2. Nothing we do can guard an init in a dependency, so the
      options are upgrading Bubble Tea or vendoring a patch.

      Scope, measured rather than guessed. `termStatusReport` sends the OSC
      query *and* `ESC[6n` (cursor position), then reads two responses. CPR is
      the safety net: a terminal that answers it but not OSC 11 returns
      `ErrStatusReport` immediately. Timing `tnr --version` under a pty with a
      scripted responder:

          answers OSC 11 + CPR (any modern terminal)   0.04s
          answers CPR only (no OSC 11 support)         0.01s
          answers nothing (pty with no emulator)       5.02s
          TERM=dumb                                    0.02s
          stdout not a TTY (piped)                     0.01s

      So it needs a foreground TTY, `TERM` not `dumb`, that answers neither
      query. Real terminal emulators all implement CPR, including the Linux VT
      console, tmux, screen and Emacs term, so they are not affected. What is
      left is a pty with nothing emulating behind it: `docker run -t` without
      `-i`, CI harnesses that allocate a pty to force colour while stdin is
      /dev/null, and test rigs. termenv also returns early when the process is
      not in the foreground process group, so `tnr ... &` is unaffected.
      Windows is likely unaffected, since termenv uses the console API there
      rather than an OSC query.

### Done

- [x] Status table headers, section headings and `tnr ports list` headers moved
      from the accent blue to bold default foreground.
- [x] `theme.Neutral()` moved from colour 8 to `Faint`. Colour 8 is the
      *background* in Solarized, so help footers were invisible there at 1.00:1.
- [x] Semantic colours are `lipgloss.AdaptiveColor` (success `2`/`10`, warning
      `3`/`11`, error `1`/`9`). Neither half works alone: the bright variants
      fall to 1.67:1 and 1.27:1 on a light background, the normal ones read as
      muddy on a dark one. Adaptive is affordable only because Bubble Tea's
      init already forces the background query, so `theme.Init` seeds the
      renderer from `lipgloss.HasDarkBackground()` instead of letting it query
      again. Verified that this adds no second `OSCTimeout`: `tnr --version`
      takes 5.01s before and 5.02s after. Never let a second renderer resolve
      an AdaptiveColor without seeding it the same way.
- [x] Dropped `Bold(true)` from the success and warning styles, and taught
      `termcontrast` to model "bold is bright". Many terminals implement bold by
      promoting the colour to its bright slot rather than by using a bold font
      (conhost always, macOS Terminal's "Use bright colors for bold text",
      Windows Terminal's `intenseTextStyle`), which silently undid the 10 → 2
      and 11 → 3 change: `1;32` was drawn as bright green anyway, back to
      1.67:1 on a white background. Confirmed on macOS Terminal with
      `printf '\e[32mplain\e[0m \e[1;32mbold\e[0m \e[92mbright\e[0m'`, where
      bold matched bright. `WorstRatio` now resolves both interpretations and
      the test asserts against the less legible one. Error keeps its bold: slot
      9 is already bright, so there is nothing to promote it to.
- [x] List cursor `▶`, selected row and panel titles moved to bold default
      foreground. `PrimaryTitleStyle` / `PrimaryCursorStyle` /
      `PrimarySelectedStyle` renamed to `TitleStyle` / `CursorStyle` /
      `SelectedStyle`, and the unused `PrimaryStyle` accessor removed, so no
      name still implies the accent colour.
- [x] Help menus: `HeaderStyle`, `CommandStyle` and `CommandTextStyle` moved to
      bold default foreground, `ExampleStyle` to faint italic, `DescStyle` to
      `theme.Body()`.
- [x] Login URL underlined on the default foreground instead of coloured.
- [x] `RESTORING` and `MIGRATING` share the warning colour with the other
      transitional states instead of the accent blue.
- [x] Panel and login input boxes keep the accent border but no longer set an
      accent foreground that leaked onto their contents.
- [x] Box borders dropped `BorderForeground` entirely, so frames render in the
      terminal's default foreground. Lip Gloss applies `Faint` only to a box's
      contents, never to its border characters, so a recessive frame is not
      expressible; an uncoloured border emits no escape at all, which is the
      safe end of that trade. A frame that vanishes leaves padded text floating
      with no grouping, which is the same "lost if it disappears" test the
      spinner failed. The semantic warning-box border keeps its adaptive
      yellow. `contrast_test.go` guards it: re-adding a blue border fails on
      eight schemes.
- [x] Spinners moved off the accent onto bold default foreground, via a named
      `tui.SpinnerStyle` that `contrast_test.go` now guards. Originally filed
      alongside borders as decoration, which was wrong: a spinner's motion is
      the only signal that the process is alive rather than hung, so losing it
      to a 1.1:1 accent on a blue background loses information. Borders keep
      the accent, because an invisible border costs nothing but the grouping
      cue. The rule is "does the user lose information if it disappears", not
      "is it decoration".
- [x] Reviewed and kept as is: `helpmenus.FlagStyle` stays bright red. It is
      1.97:1 on macOS Terminal's Ocean, the only scheme where it is tight (next
      worst is Solarized Dark at 3.26), because bright red on a saturated blue
      is an inherently muddy pairing. Checked by eye on Ocean against
      `tnr status -h` and it reads fine; the flag column is also distinguished
      by position and the leading `--`.
- [x] Reviewed and kept as is: the `Press 'Q' to …` hint at `status.go:402`
      shares its faint style with the timestamp above it, so the one actionable
      line on the screen is styled like a passive one. It clears the contrast
      floor and reads fine in practice, so the hierarchy is deliberate.
