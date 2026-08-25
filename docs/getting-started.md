# Getting started

Graphite is a terminal UI (TUI) library for Go. It renders truecolor
(24-bit RGB) widgets into the terminal's alternate screen buffer, reads
keyboard and SGR mouse input, and gives you a `Widget` tree, a `Flex`
layout container, and a set of ready-made widgets so you don't have to
hand-roll cursor math for every screen.

## Requirements

- Go 1.25 or later.
- A terminal that supports the alternate screen buffer, 24-bit truecolor
  SGR sequences, and SGR mouse reporting (mode `1002`/`1006`). Every modern
  terminal emulator does — iTerm2, Windows Terminal, GNOME Terminal,
  Alacritty, Kitty, WezTerm, tmux (with `set -g mouse on`), and so on. There
  is no legacy 16/256-color fallback path.
- On Windows specifically, see [windows-terminal.md](windows-terminal.md)
  for a note about `conhost.exe` (a plain `cmd.exe`/PowerShell window not
  running inside Windows Terminal) needing virtual-terminal processing
  enabled — Graphite does this for you automatically, but it's worth
  understanding if you ever see raw escape codes printed as text.

## Install

```bash
go get github.com/yeoblyv/graphite
```

Note the import path resolves to the identifier `Graphite` (capitalized),
not `graphite`, because the package itself is declared as `package
Graphite`. Every example in this documentation uses that capitalization.

## Minimal program

```go
package main

import "github.com/yeoblyv/graphite"

func main() {
	app := Graphite.NewApplication()

	win := Graphite.NewWindow(50, 10, " Hello ")
	win.AddWidget(Graphite.NewLabel(2, 2, "Hello, terminal!"))
	win.AddWidget(Graphite.NewButton(2, 4, "Quit", Graphite.BtnDefault, func() {
		app.Quit()
	}))

	app.SetWindow(win)
	app.Run()
}
```

Run it, and you get a bordered, centered window with a label and a
button. `Tab` moves focus between the label (not focusable, so it's
skipped) and the button; `Enter` or a mouse click activates the button;
`Esc` quits the application (or closes the topmost modal, if one is open).

## Anatomy of every Graphite program

1. Create an `Application` with `Graphite.NewApplication()`.
2. Optionally call `app.SetTheme(...)` to replace the default color
   palette (see [theming.md](theming.md)).
3. Build a `Window` with `Graphite.NewWindow(w, h, title)`, add `Widget`s
   to it with `win.AddWidget(...)`.
4. Call `app.SetWindow(win)`.
5. Call `app.Run()`. This blocks until the user quits or `app.Quit()` is
   called from a callback.

Everything else — reading input, diffing the screen, redrawing, focus
order, mouse capture — is handled for you. See
[architecture.md](architecture.md) for how that loop actually works.

## Example programs in this repository

The repository ships three full example programs you can run directly to
see Graphite in action:

```bash
git clone <this-repo-url>
cd graphite
go run ./showcase   # every widget, Flex layout, and a custom theme in one window
go run ./gphedit     # a GPH image editor/converter built with Graphite
go run ./promo       # an animated logo/splash screen
```

`showcase` is the most useful one to read alongside this documentation —
it is a tabbed window exercising every widget type, `Flex` layout, theme
customization, modals, and the `Fader`/image widgets, and every code
example in this documentation set either comes directly from it or
mirrors its style.

## Building

This repo ships two ways to build, so you don't need to install anything
you don't already have:

- **`make`** (Linux/macOS/CI, or Windows with Make installed):
  ```bash
  make build           # compile everything for the host platform
  make build-all        # cross-compile the example programs for
                         # windows/amd64, windows/386, linux/amd64, linux/386
  make test             # go test ./...
  make lint              # golangci-lint, if installed
  ```
- **PowerShell** (native on Windows, no extra tools):
  ```powershell
  ./build.ps1                      # cross-compile for all four targets
  ./build.ps1 -Target linux-amd64  # a single target
  ./build.ps1 -Target Test         # go test ./...
  ```

Both write cross-compiled binaries to `dist/<goos>_<goarch>/`.

## Where to go next

- [architecture.md](architecture.md) — how `Application`, `Canvas`,
  `Window`, and `Widget` fit together, and the render/input loop.
- [layout.md](layout.md) — fixed vs. percentage positioning, the stretch
  rule every widget follows, `Panel`, `Flex`, and `GroupBox`.
- [widgets.md](widgets.md) — every widget in the library, with examples.
- [fader.md](fader.md) — the channel-strip mixer control, in depth.
- [modals.md](modals.md) — dialogs, confirmations, and the file picker.
- [theming.md](theming.md) — `Color`, `Theme`, and building your own palette.
- [events.md](events.md) — how keyboard and mouse input is modeled and routed.
- [images.md](images.md) — the GPH pseudographics image format.
- [custom-widgets.md](custom-widgets.md) — building your own widget.
