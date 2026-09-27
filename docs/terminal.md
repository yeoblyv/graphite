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
for *key* input during that time — but a scroll wheel notch still reaches
it as a normally-decoded `Event` (see Mouse input below), which is how
`Terminal.HandleEvent` ends up handling `EventMouseScrollUp`/`Down` for
[Scrollback](#scrollback) despite raw passthrough being active.

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

## Scrollback

`vtScreen` keeps up to `Terminal.SetScrollbackLimit`'s row count (default
`DefaultScrollbackLines`, 2000) of primary-screen output that's scrolled
off the top — a mouse-wheel notch or `PageUp`/`PageDown` scrolls the view
up into it, the same as any real terminal emulator's history buffer. A
small corner badge (`↑ 120/2000`, say) appears while scrolled, and
disappears the moment the view returns to the live bottom.

Two things make this **not** just "always keep every row you print,
forever":

- **Only a full-screen scroll of the primary grid captures anything.** A
  program that sets a narrower `DECSTBM` scroll region (a pager keeping a
  status line pinned at the bottom, say) scrolling *that* region doesn't
  touch scrollback — matching what a real terminal's history buffer
  captures. Neither does anything happening on the **alternate screen**:
  `vim`, `htop`, `less`, and friends manage their own full-screen display
  and have no scrollback of their own for this widget to show instead.
- **It's capped and trimmed from the front**, so a long-running shell
  doesn't grow `Terminal`'s memory use without bound.

Because of the alternate-screen exclusion above, scrolling behaves
differently depending on what's currently running:

- **Primary screen (an ordinary shell prompt):** the mouse wheel and
  `PageUp`/`PageDown` scroll `Terminal`'s own scrollback. `PageUp`/
  `PageDown` are intercepted *before* `WriteRaw` — the CSI-tilde bytes
  (`Terminal.HandleScroll`, checked via a small `Scrollable` interface in
  `app.go`) never reach the shell at all while this widget has focus.
- **Alternate screen (`vim`, `htop`, `less`, ...):** `Terminal.HandleScroll`
  declines (returns `false`), so `PageUp`/`PageDown` fall through to raw
  passthrough and reach the program unaltered, exactly as if `Terminal`
  weren't intercepting them. The mouse wheel, however, doesn't reach such
  a program either way — forwarding it as the program's *own* mouse input
  would need tracking whether the program itself enabled mouse reporting
  (`DECSET` 1000/1002/1003), which this doesn't attempt.

## Known limitations

- **No mouse support inside the terminal.** A mouse click is always
  decoded and routed normally, even while a `Terminal` has focus —
  otherwise there would be no way to click anything else (a different
  tab, the other pane, ...) while it does, since raw passthrough would
  swallow every byte, mouse reports included, before `Window`'s own
  hit-testing ever saw them. The tradeoff is that a program relying on
  terminal mouse reporting (`vim`'s or `tmux`'s mouse mode, say) won't
  see clicks made inside the `Terminal` widget itself.
- **Scrollback's mouse-wheel forwarding while a full-screen program is
  running is one-way only.** The mouse wheel adjusts `Terminal`'s own
  scrollback while the primary screen is active, but doesn't reach a
  program on the alternate screen (`vim`, `htop`, `less`) as its own
  mouse input either way — see [Scrollback](#scrollback) below for what
  does and doesn't reach such a program.
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
