# Graphite

![go version](https://img.shields.io/badge/go-%3E%3D1.25-00ADD8)
![license](https://img.shields.io/badge/license-MIT-green)

A small terminal UI (TUI) widget library for Go: a double-buffered canvas
with a diffing renderer, a `Widget` interface with focus/enabled/visible
state handled for you, and a set of ready-made widgets (labels, buttons,
checkboxes, input boxes, text areas, list boxes, todo lists, tabs, progress
bars, panels for layout, modal windows).

## Quickstart

```bash
git clone <this-repo-url>
cd graphite
go run ./demo
```

Minimal usage from your own program:

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

`Esc` quits the application (or closes the topmost modal, if one is open);
`Tab` cycles focus; arrow keys and mouse clicks both work out of the box.

## Building

This repo ships two ways to build, so you don't need to install anything you
don't already have:

- **`make`** (Linux/macOS/CI, or Windows with Make installed):
  ```bash
  make build          # compile everything for the host platform
  make build-all       # cross-compile the example programs for
                        # windows/amd64, windows/386, linux/amd64, linux/386
  make test            # go test ./...
  make lint             # golangci-lint, if installed
  ```
- **PowerShell** (native on Windows, no extra tools):
  ```powershell
  ./build.ps1                      # cross-compile for all four targets
  ./build.ps1 -Target linux-amd64  # a single target
  ./build.ps1 -Target Test         # go test ./...
  ```

Both write cross-compiled binaries to `dist/<goos>_<goarch>/`.

## Architecture

- **`Application`** owns the terminal, the canvas, the active window, and the
  modal stack, and drives the render/input loop (`Run`). It is the one
  composition root a program constructs — there is no package-level mutable
  state to reason about.
- **`Canvas`** is a double-buffered grid of cells; `Render` diffs the two
  buffers and writes only what changed.
- **`Widget`** is the interface every UI element implements; `BaseWidget`
  handles the mechanical parts (layout resolution, focus/enabled/visible) so
  concrete widgets only implement drawing and event handling.
- **`Window`** hosts a widget tree, resolves Tab order, and routes mouse and
  keyboard events.
- Updating a widget from a background goroutine (e.g. streaming subprocess
  output, as `demo` does) must go through `Application.Invoke`, the only
  thread-safe way to touch widget state from outside the render loop.

## Security notes

See the answer given directly to the maintainer covering the two hardening
fixes applied before this release (terminal escape-sequence sanitization in
`Canvas.DrawCell`, and bounds-checked widget selection indices) and the
threat model this library does and does not cover.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
