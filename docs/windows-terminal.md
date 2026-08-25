# Windows terminal notes

Graphite works on Windows out of the box, but Windows' terminal story has
one wrinkle worth understanding: there are two meaningfully different
console hosts in play, and they don't behave identically.

## Windows Terminal vs. the classic console host

- **Windows Terminal** (the modern app, `wt.exe`, default on Windows 11
  and commonly installed on Windows 10) interprets ANSI/VT100 escape
  sequences natively and correctly, the same as any Linux/macOS terminal
  emulator. Nothing extra is needed.
- **`conhost.exe`** (the classic console host behind a plain `cmd.exe` or
  PowerShell window when it is *not* running inside Windows Terminal)
  does **not** enable virtual-terminal escape interpretation by default
  on every Windows 10 build/configuration. Without it, every escape
  sequence Graphite writes — truecolor SGR codes, cursor positioning,
  alternate-screen-buffer switches — gets printed as literal text instead
  of being interpreted, and the screen fills with garbage like:

  ```
  [38;2;255;59;48m[48;2;16;16;16m...
  ```

## What Graphite does about it

`terminal.init()` (called at the start of `Application.Run()`) calls an
internal `enableVirtualTerminal()` before writing any escape sequence.
On Windows, this sets `ENABLE_VIRTUAL_TERMINAL_PROCESSING` on the stdout
console handle and `ENABLE_VIRTUAL_TERMINAL_INPUT` on the stdin handle via
`SetConsoleMode` (`golang.org/x/sys/windows`), so `conhost.exe` starts
interpreting the same escape sequences Windows Terminal already handles
natively. On every other platform, this call is a no-op (`sys_other.go`) —
every terminal emulator Graphite targets outside Windows already
interprets these sequences without help.

This is implemented as a Windows-specific file
(`sys_windows.go`, `//go:build windows`) alongside a `//go:build !windows`
stub (`sys_other.go`), so it compiles cleanly and does nothing extra on
non-Windows targets.

You do not need to do anything yourself to get this — it's automatic for
every `Application`. This document exists so that if you ever *do* see
raw escape codes printed as text (e.g. building against an unusually old
Windows build, or a non-standard terminal emulator that doesn't support
`SetConsoleMode`'s virtual-terminal flags at all), you know what's
happening and why, rather than assuming it's a bug in your own program.

## Why `golang.org/x/term`'s `MakeRaw` alone isn't enough

`terminal.init()` also calls `term.MakeRaw(int(os.Stdin.Fd()))` to put the
terminal into raw input mode (no line buffering, no local echo — Graphite
needs to see every keystroke and mouse report as it arrives). It's worth
being explicit that this is a *different* concern from virtual-terminal
processing: `MakeRaw` only affects how **input** is read from the
console; it does not touch **output** interpretation at all. Both calls
are needed on Windows — `MakeRaw` for raw input, `enableVirtualTerminal`
for the console to actually render the escape sequences Graphite writes —
and Graphite calls both for you, in the right order, before either mode
is relied on.

## Building for Windows from another platform

Cross-compiling for `windows/amd64` or `windows/386` from Linux/macOS
works with plain `go build` (`sys_windows.go`'s only external dependency,
`golang.org/x/sys/windows`, is pure Go — no cgo needed). See
[getting-started.md](getting-started.md#building) for the `make
build-all`/`build.ps1` cross-compile targets this repository ships.
