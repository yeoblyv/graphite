# Terminal

`Terminal` runs a shell (or any interactive program) attached to a real
pseudo-terminal and renders its output faithfully, including full-screen
programs like `vim` or `htop` that need real cursor control and an
alternate screen buffer — not just scrolling text. It's the most
structurally involved widget in the library (a pty, a hand-rolled VT100
interpreter, and a change to the core input loop all cooperate to make it
work), so like [Fader](fader.md) and [PianoRoll](pianoroll.md) it gets its
own page.

## Anatomy

```go
term, err := Graphite.NewTerminal(app, 0, 0, 0, 0, "/bin/bash", nil)
if err != nil {
	// the pty couldn't be created — a real OS-level failure, not
	// something to route into the terminal's own screen
}
win.AddWidget(term)
```

`x, y, w, h` follow `BaseWidget`'s usual convention — 0 or negative for
`w`/`h` stretches to fill the parent, exactly like any other widget.
`shell`/`args` is the command to run; the pty's initial size is a
placeholder (80×24) that `DrawRelative` immediately resizes to the
widget's actually resolved size on the first frame, the same "layout
resolves it, then the widget catches up" pattern `Image`'s `AutoSize`
uses.

Three pieces cooperate to make this work, each in its own file:

- **`pty.go`+`pty_darwin.go`/`pty_linux.go`/`pty_windows.go`** —
  `startPTY` spawns the child attached to a real pseudo-terminal (a
  genuine controlling terminal: `isatty(0)` is true, `SIGWINCH` on
  resize, its own prompt/cursor behavior — not just three pipes).
  Linux/macOS need different ioctl sequences for the one step of opening
  the master/slave pair; Windows has no equivalent kernel object at all,
  built entirely on ConPTY.
- **`vt100.go`/`vt100_ops.go`** — `vtScreen` is a VT100/xterm-subset
  interpreter: an incremental byte-stream parser (a pty read can split an
  escape sequence across chunks) driving a grid of cells, cursor,
  SGR-driven colors/attributes, scroll regions, and the alternate screen
  buffer.
- **`terminal.go`** — `Terminal` itself: renders `vtScreen`'s grid via
  `DrawRelative`, and forwards keystrokes back to the child via
  `WriteRaw`.

## Undistorted input: `RawInputReceiver`

A shell needs byte-for-byte fidelity with whatever the real terminal in
front of the user actually sent — application-cursor-mode arrows, Ctrl
combinations graphite's own `KeyCode` vocabulary doesn't cover, and so
on. Graphite normally decodes every keystroke into an `Event` before any
widget's `HandleEvent` sees it (see [events.md](events.md)), which would
lose exactly that fidelity if `Terminal` had to receive its input the
same way.

Instead, `Terminal` implements:

```go
type RawInputReceiver interface {
	Widget
	WriteRaw(p []byte)
}
```

`Application.Run` checks whether the currently focused widget implements
`RawInputReceiver` and, if so, routes raw bytes to it directly —
bypassing `parseANSI`'s `Event` decoding entirely for as long as it holds
focus. `HandleEvent` on a `RawInputReceiver` is effectively never called
for key input during that time; `Terminal`'s own is a no-op.

**Detaching focus:** if every keystroke reaches the terminal undecoded, a
plain Tab press would too — so a `Terminal`-focused window would have no
way out. `Ctrl+\` (ASCII FS, `0x1C`) is reserved as the one intercepted
byte: `Application.Run` strips it out of the stream before it reaches
`WriteRaw` and advances focus instead (the same as an ordinary Tab
keypress). It was chosen because it's a POSIX-conventional signal key
almost nothing uses interactively inside a shell.

**Gaining focus mid-click:** a `Terminal` created and focused from inside
a click's own handling (e.g. a menu item's Action) doesn't start raw
passthrough until that click's `EventMouseUp` has been decoded and routed
normally — `Application.Run` checks `Window.HasMouseCapture` first, so
the tail end of the very click that created the `Terminal` can't leak
into it as raw bytes.

**Mouse input is never raw.** Even once a `Terminal` has focus, an
incoming SGR mouse report (`Application.Run` recognizes it by its
`\x1b[<` prefix) is still decoded into an `Event` and routed the normal
way rather than handed to `WriteRaw` — only keyboard input actually goes
raw. Without this, clicking a different tab or the other pane would be
impossible while a `Terminal` has focus, since every byte (mouse reports
included) would go straight to it before `Window`'s own hit-testing ever
ran.

## Known limitations

- **No mouse support inside the terminal.** A mouse click is always
  decoded and routed normally, even while a `Terminal` has focus —
  otherwise there would be no way to click anything else (a different
  tab, the other pane, ...) while it does, since raw passthrough would
  swallow every byte, mouse reports included, before `Window`'s own
  hit-testing ever saw them. The tradeoff is that a program relying on
  terminal mouse reporting (`vim`'s or `tmux`'s mouse mode, say) won't
  see clicks made inside the `Terminal` widget itself.
- **No scrollback.** Only the visible grid is kept; scrolled-off lines
  are gone, the same way a bare VT100 terminal (as opposed to a modern
  terminal emulator with a history buffer) behaves.
- **No DEC line-drawing character sets.** A program that leans on them
  for box-drawing borders (some `ncurses` configurations) will show the
  raw ASCII designator characters instead of the box-drawing glyphs.
- **No reflow on resize.** `Resize` reallocates both grids fresh rather
  than rewrapping existing content to the new width — a genuinely hard
  problem real terminal emulators spend real effort on, out of scope
  here.

None of these affect the common case (an interactive shell, `vim`,
`htop`, `less`, a nested `ssh` session) — they're the corners intentionally
left for later rather than attempted up front.
