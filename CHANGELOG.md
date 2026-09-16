# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

Baseline for the first public release.

### Fixed

- A mouse click was swallowed as raw bytes by a focused
  `RawInputReceiver` (a `Terminal`) the same way keyboard input is,
  making it impossible to click anything else — a different tab, the
  other pane — while a `Terminal` had focus, since `Window`'s own
  hit-testing never ran. `Application.Run` now recognizes an SGR mouse
  report by its `\x1b[<` prefix and always decodes and routes it
  normally, regardless of who has focus; only keyboard input still goes
  raw.
- A `RawInputReceiver` that gained focus as a side effect of the very
  click that's still in flight (e.g. a menu item whose action creates
  and focuses a new `Terminal`) could have that same click's trailing
  `EventMouseUp` redirected to it as raw bytes instead of being decoded
  and routed to whatever actually captured the gesture — visibly, the
  tail end of a mouse-click escape sequence leaking into the terminal
  as typed garbage. `Application.Run` now checks the new
  `Window.HasMouseCapture` first and defers to normal decoded routing
  while a gesture is still open, regardless of who currently has focus.

### Added

- Internationalization: `Application.T` resolves every string the
  library's own `ShowMessage`/`ShowConfirm`/`ShowTextEditor`/
  `ShowValueEditor`/`ShowFilePicker` dialogs draw against `Application`'s
  current `Locale` (`SetLocale`, default `LocaleEnglish`), with a prepared
  `Catalog` shipped for 15 languages (English, Ukrainian, Russian, German,
  French, Spanish, Portuguese, Italian, Polish, Dutch, Turkish, Czech,
  Japanese, Chinese, Korean). The same `Locale`/`Catalog`/`T` mechanism is
  a general-purpose key-to-text standard a program uses to translate its
  own UI, not just the library's: `Application.SetTranslations` merges a
  `Catalog` into a locale's dictionary (repeated calls add to it rather
  than replacing it, so a translation can be built up incrementally), and
  `Catalog.Merge` composes several Go-native `Catalog` values (e.g. one
  per file under a project's own `locales` package) into one — every
  translation is a plain Go value compiled into the binary, so a duplicate
  or misspelled key is a `go build`/`go vet` failure rather than something
  discovered at runtime, and nothing needs shipping or loading from disk
  alongside the executable. See [docs/i18n.md](docs/i18n.md).
- `InputBox.Masked` (and the `NewPasswordBox` constructor that sets it), for
  a password/passphrase field: every character of `Value` renders as `•`
  instead of itself, and `Ctrl+C`/`Ctrl+X` never put the real value on the
  OS clipboard (`Ctrl+X` on a masked box does nothing at all, rather than
  still clearing the field with nowhere recoverable for it to go). Editing,
  cursor movement, and `Ctrl+V` paste are unaffected — only what gets drawn
  and what a copy/cut can reach change. An unmasked `InputBox` is untouched.
- `MenuItem.Separator` (an inert divider row) and `MenuItem.SubItems` (a
  nested flyout opening to the item's right instead of running `Action`,
  one level deep). Both default to their zero value (`false`/`nil`), so
  every existing `MenuStrip` is unaffected.
- `Terminal`, a widget that runs a shell (or any interactive program)
  attached to a real pseudo-terminal and renders its output faithfully —
  full-screen programs (`vim`, `htop`, `less`, a nested `ssh` session)
  included, via a hand-rolled VT100/xterm interpreter (`vtScreen`:
  incremental parsing, SGR colors/attributes, scroll regions, the
  alternate screen buffer) and per-OS pty spawning (`startPTY`: real
  ioctls on Linux/macOS, ConPTY on Windows). Genuinely undistorted
  keyboard passthrough needed a core input-loop change too: a widget
  implementing the new `RawInputReceiver` interface receives raw bytes
  directly while focused, bypassing `parseANSI`'s `Event` decoding
  entirely, with `Ctrl+\` reserved to detach focus (advances it, like
  Tab) since a `Terminal` would otherwise consume every keystroke,
  including the one that would normally escape it. See
  [docs/terminal.md](docs/terminal.md). No new dependencies — hand-rolled
  per the project owner's explicit choice over an existing VT100 library.
- `Theme.Info`, a third accent distinct from both `Primary` and `Accent`,
  for a transient "here's a result" highlight (e.g. a search match) that
  would otherwise have to reuse a color already carrying a different
  meaning. No built-in widget reads it, the same as `Accent`.
- `EventMouseRightDown`, decoded from the SGR mouse right-button press
  report that was previously undecoded (silently dropped). It's
  hit-tested and delivered like a scroll event — no focus change, no
  mouse capture, since there's no corresponding drag/release to capture
  for — letting a widget wire a secondary click-driven action (e.g. a
  quick toggle) distinct from its primary `EventMouseDown` behavior.
- `Button.BgColor`/`FgColor`, letting a button carry its own accent color
  in its idle state instead of only being distinguishable once focused —
  useful for a toolbar-style button that would otherwise blend into a
  plain list background. `FgColor` left at `ColorNone` auto-picks a
  contrasting text color via `Color.ContrastText`. Both default to
  `ColorNone` (theme-driven, unchanged from before), and neither affects
  focused, disabled, or unfocused-`BtnDanger` rendering, so no existing
  `Button` is affected.
- `Color.ContrastText()`, returning black or white for readable text on
  an arbitrary background — the exact computation `showcase`'s own
  `contrastText` helper already duplicated locally, promoted so a widget
  coloring itself from something other than a fixed theme field doesn't
  have to re-derive it.
- `MenuStrip.BgColor`/`FgColor`, letting the strip and its dropdown share
  one flat accent color instead of the theme's `BgWidget`/`BgWindow` —
  `FgColor` left at `ColorNone` auto-picks a contrasting text color via
  `Color.ContrastText`. Both default to `ColorNone` (theme-driven,
  unchanged from before), so no existing `MenuStrip` is affected.
- `Window.Chrome` / `ChromeBorderless` / `NewFullscreenWindow()`: an
  edge-to-edge window mode with no border, drop shadow, or title bar, for
  an application's main window rather than a floating dialog.
  `ChromeBordered` (the zero value) is unchanged and remains the default,
  so every existing `NewWindow` caller is unaffected.
- `Theme.Accent`, a second accent color distinct from `Primary`. No
  built-in widget reads it — it's for a custom widget that needs to color
  two different things without one borrowing the other's meaning (e.g.
  `Primary` for pane focus/cursor, `Accent` for a multi-selection tag
  marker).
- `Canvas.Theme()`, so a custom widget defined outside package `Graphite`
  can read the live palette in its own `DrawRelative` — previously only
  widgets inside the package could, via the unexported `c.theme` field,
  so a third-party widget had no way to restyle automatically when
  `SetTheme` changes the palette.
- `KeyF1`-`KeyF12`, decoded from both the SS3 (`F1`-`F4`) and CSI-tilde
  (`F5`-`F12`, and an alternate `F1`-`F4` encoding) escape sequences
  terminals send for function keys. Previously an `F`-key's escape
  sequence was misread as a lone `Escape` keypress (closing a modal or
  quitting the application) followed by its remaining bytes leaking
  through as ordinary characters.
- `KeyInsert`, decoded from its CSI-tilde escape sequence the same way.
- `ShowConfirm`, a Yes/No confirmation modal, and `ShowTextEditor`,
  `ShowValueEditor`'s free-form-text counterpart for prompting a single
  line of arbitrary text (e.g. naming a file or directory) instead of a
  bounded number.
- `Application.SetOnQuitRequested`, letting a program intercept Escape
  when no modal is open (e.g. to show a confirmation dialog) instead of
  quitting immediately.
- Thread-safe cross-goroutine widget updates via `Application.Invoke`.
- A modal stack (`Application.SetModal`/`CloseModal`), replacing the earlier
  single-modal-window limitation.
- `Widget.IsEnabled()`, with `Window` withholding input from disabled
  widgets uniformly instead of relying on each widget to check itself.
- `Application.SetTheme`, replacing a package-level mutable theme variable.
- Cross-compile build tooling (`Makefile`, `build.ps1`) for
  windows/amd64, windows/386, linux/amd64, and linux/386.
- Truecolor `Color` (`RGB`/`Hex`/`Darken`), a `Flex` layout container, and
  unicode-width-aware text rendering.
- Real mouse drag: `EventMouseDrag`/`EventMouseUp`, decoded from SGR
  button-event mouse tracking, and implicit mouse capture in `Window` so a
  drag keeps reaching the widget that was pressed even after the pointer
  leaves its bounds.
- `Fader`, a draggable/clickable channel-strip widget (independent VU meter
  via `SetLevel`, latching clip indicator, Mute/Solo, colored label,
  double-click-to-edit via `ShowFaderValueEditor`) built on top of the new
  mouse capture.
- `PianoRoll`, a playable piano keyboard (horizontal or vertical, an
  adaptive key count with a configurable floor, mouse click/drag,
  PC-keyboard input, and a `NoteOn`/`NoteOff` API for driving it
  programmatically), plus two optional companion modules —
  `graphite/audio` (real sine-wave synthesis and playback via `oto`) and
  `graphite/midi` (real MIDI input devices via `gomidi`/`rtmidi`) — kept
  as separate Go modules so their dependencies (and, for `graphite/midi`,
  its cgo requirement) aren't imposed on consumers who don't need them.
- `FuzzParseANSI`, `FuzzTextAreaBuildLines`, `FuzzSanitizeGlyph` fuzz the
  three hand-rolled parsers.

### Fixed

- `ListBox`'s selected-row text stayed the fixed `theme.FgWindow` regardless
  of the row's highlight color (`theme.Primary`), so a theme whose `Primary`
  and `FgWindow` are both light — a bright accent color on a light-text
  theme — rendered the selection as unreadable light-on-light. The
  foreground is now computed from the highlight via `Color.ContrastText`,
  the same auto-contrast `Button`/`MenuStrip` already use for their own
  custom colors.
- `ShowValueEditor`/`ShowConfirm`/`ShowTextEditor`'s OK/Yes/Cancel/No
  buttons were unclickable by mouse: each dialog positioned its button row
  using the modal's own requested height, without accounting for
  `PaddingY` shrinking the content area a child's Y actually resolves
  against. `BaseWidget.DrawRelative`'s parent-bounds clamp then capped
  each button's resolved height at 0 — still focusable and
  Enter-submittable (keyboard routing goes by focus, not `HitTest`), but
  `HitTest` requires a positive height, so no click could ever land. Fixed
  by anchoring the button row to the bottom of the content area instead of
  computing its position from the window's own height.
- Terminal escape-sequence injection: `Canvas.DrawCell` now strips control
  characters (e.g. a raw ESC arriving via untrusted subprocess output
  streamed into a `TextArea`) before they reach the real terminal.
- A stale `Selected` index on `ListBox`/`TodoList` no longer panics when a
  caller reassigns `Items` to a shorter slice without resetting `Selected`.
- A data race between background goroutines and the render loop, present in
  every example that streamed progress into a widget from a goroutine
  (fixed by `Application.Invoke`, above).

### Changed

- All source comments and documentation translated to English; every
  exported type, function, and method now carries a godoc comment.
- Package identifier capitalized from `graphite` to `Graphite`; callers now
  write `Graphite.NewApplication()` etc. The import path itself
  (`github.com/yeoblyv/graphite`) is unchanged.
