# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

Baseline for the first public release.

### Added

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
