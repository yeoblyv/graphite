# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

Baseline for the first public release.

### Added

- Thread-safe cross-goroutine widget updates via `Application.Invoke`.
- A modal stack (`Application.SetModal`/`CloseModal`), replacing the earlier
  single-modal-window limitation.
- `Widget.IsEnabled()`, with `Window` withholding input from disabled
  widgets uniformly instead of relying on each widget to check itself.
- `Application.SetTheme`, replacing a package-level mutable theme variable.
- Cross-compile build tooling (`Makefile`, `build.ps1`) for
  windows/amd64, windows/386, linux/amd64, and linux/386.

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
