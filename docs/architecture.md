# Architecture

Graphite has five core pieces: `Application`, `Canvas`, `Window`,
`Widget`/`BaseWidget`, and `Event`. This document explains what each one
owns and how they cooperate in the render/input loop. Layout
(`Flex`/`Panel`/percentage positioning) and theming (`Color`/`Theme`) are
big enough topics to get their own documents — see
[layout.md](layout.md) and [theming.md](theming.md).

## `Application`: the composition root

```go
app := Graphite.NewApplication()
```

`Application` owns the terminal (raw mode, alternate screen, mouse
reporting), the `Canvas`, the active `Window`, the modal stack, and the
idle callback. It is the one piece of mutable state a program constructs
— there is no package-level global to reason about, and constructing a
second `Application` in the same process (not a supported use case, but
worth knowing) would not interfere with the first's fields.

Key methods:

| Method | Purpose |
|---|---|
| `SetTheme(t Theme)` | Replace the color palette, forcing a full redraw. |
| `SetWindow(win *Window)` | Set the non-modal window drawn behind any open modals. |
| `SetModal(mod *Window)` / `CloseModal()` | Push/pop the modal stack — see [modals.md](modals.md). |
| `SetStatus(text string)` | Set the status bar text along the bottom row. |
| `SetIdleCallback(cb func())` | Set a function called repeatedly once the user has been idle — see below. |
| `SetOnQuitRequested(fn func())` | Override what Escape does with no modal open — see below. |
| `Invoke(fn func())` | Queue `fn` to run on the main loop — the only safe way to touch widget state from another goroutine. |
| `ShowMessage(title, message string, style ButtonStyle)` | Open a single-button modal dialog. |
| `Quit()` | Stop `Run` after the current frame. |
| `Run()` | Start the render/input loop (blocks). |
| `Suspend()` / `Resume()` | Temporarily hand the terminal back to cooked mode, e.g. before shelling out to an interactive subprocess. |

### The render/input loop

`Run()` does, every iteration, until `Quit()` is called or the process is
torn down:

1. **Drain the invoke queue** — run every callback queued via `Invoke`
   since the last frame (see [Concurrency](#concurrency-and-invoke)
   below).
2. **Resize the canvas** to the current terminal size (`GetTerminalSize`),
   a no-op if unchanged.
3. **Clear** the canvas back buffer to the theme's screen background.
4. **Draw** the active window, then every open modal on top of it, then
   the status bar (if set).
5. **Render** — diff the back buffer against what was last actually
   written to the terminal, and emit only the changed cells (see
   [Canvas](#canvas-double-buffering-and-diffing) below).
6. **Poll for input** (`pollEvent`, waits up to 10ms).
7. **Run the idle callback**, if 300ms have passed with no input.
8. **Route the event**: `Esc` closes the topmost modal if one is open;
   with no modal open, it quits the application, unless
   `SetOnQuitRequested` set an override — in that case the override is
   called instead of `Quit`, and is itself responsible for deciding
   whether and when to call `Quit` (typically after confirming via
   `ShowConfirm`). Every other event goes to the topmost modal if one is
   open, otherwise to the active window.

The terminal is always restored to cooked mode on return from `Run`,
including on panic (`defer app.term.restore()`), so a crashing program
doesn't leave the user's shell in raw mode with mouse reporting still
enabled.

### The idle callback

```go
app.SetIdleCallback(func() {
	// called repeatedly once ~300ms have passed with no keyboard/mouse input
})
```

This is how every example in the repository animates or polls: `showcase`
updates a `ProgressBar` and fakes a VU meter signal from it; `orqestor` (a
real system volume mixer built on Graphite, not in this repository) reads
live WASAPI audio levels and pushes them into `Fader.SetLevel` from it.
It fires on every loop iteration once the idle threshold has passed — not
once — so it is the right place for continuous polling, not a one-shot
timer.

### Concurrency and `Invoke`

Widget state is only safe to read and write from the goroutine running
`Run()`. If you have a background goroutine — a streaming subprocess, a
network client, a hardware poller — that needs to update a widget, do not
touch the widget directly from that goroutine. Instead:

```go
go func() {
	for line := range subprocessOutput {
		app.Invoke(func() {
			textArea.SetText(textArea.Text + line + "\n")
		})
	}
}()
```

`Invoke` queues the callback under a mutex; `drainInvokeQueue` swaps the
queue out and unlocks *before* running any callback, so a callback is
free to call `Invoke` again itself without deadlocking. Every callback
queued this way runs on the main loop's goroutine, right before the next
frame is drawn — so by the time `Render()` runs, the widget state it
reads is consistent.

This same rule extends to any thread-affine external API a widget's
callback talks to. A real example: a Windows volume-mixer application
built on Graphite needs every WASAPI (COM) call to happen on the same OS
thread that called `CoInitializeEx`. Since `Fader` callbacks
(`OnChange`/`OnMuteChange`/`OnSoloChange`/`OnDoubleClick`) and the idle
callback are both invoked synchronously from `Run()`'s own goroutine, the
fix is simply `runtime.LockOSThread()` once in `main()` before starting
the app — no `Invoke` needed, because the callbacks were already running
on the right goroutine. The rule to remember: **all widget/thread-affine
state must be touched only from inside a widget callback or the idle
callback, unless it's marshaled through `Invoke`.**

## `Canvas`: double-buffering and diffing

```go
type Cell struct {
	Symbol  string
	BgColor Color
	FgColor Color
}
```

`Canvas` is a `width * height` grid of `Cell`s, kept in two buffers: the
back buffer widgets draw into every frame, and a front buffer recording
what was last actually sent to the terminal. `Render()` walks the grid,
compares each cell against the front buffer, and only emits the ANSI
bytes needed to update cells that actually changed — a cursor-position
escape only when the cursor isn't already where the next write needs it,
and an SGR color escape only when the color actually changed from the
previous cell written. This keeps the amount of data written to the
terminal proportional to what's visually different between frames, not to
the terminal's total size.

Widgets draw via `Canvas`'s methods rather than touching cells directly
in most cases:

| Method | Behavior |
|---|---|
| `DrawCell(x, y, symbol, bg, fg)` | Writes one glyph. Out-of-bounds coordinates are silently ignored. `ColorNone` for `bg` or `fg` leaves that color unchanged. Assumes `symbol` is exactly one terminal column wide. |
| `DrawText(x, y, text, bg, fg)` | Writes a string, advancing correctly for double-width runes (CJK etc. — see `runeWidth`). |
| `DrawTextBounded(x, y, maxW, text, bg, fg)` | Like `DrawText`, but truncates with `…` if `text` would exceed `maxW` columns. |
| `DrawTextWrapped(x, y, maxW, text, bg, fg) int` | Word-wraps `text` to `maxW` columns, honoring `\n` as a hard break; returns the number of lines drawn. |
| `GetCellBg(x, y) Color` | Reads back the current background at a cell — used to blend decorations like window drop-shadows with whatever is already there. |

A full redraw (`\033[2J\033[H` plus every cell) happens on the very first
frame and after any `Resize` (i.e. the terminal was resized); every other
frame only writes the diff.

### Double-width runes

`DrawText` and friends call an internal `runeWidth` helper so CJK and
other double-width characters advance the cursor by two columns instead
of one, and the column immediately to the right of a wide glyph is marked
as a "continuation" cell that `Render` skips over (writing into it would
have the terminal overwrite half of the wide glyph it just drew).
`InputBox` and `TextArea`'s cursor math is rune-count based rather than
display-column based, so very wide input can drift slightly out of sync
with the visual cursor position while editing — a known, minor
limitation rather than a bug to work around.

### Escape-sequence sanitization

Any text a widget draws that reaches `DrawCell` as a single-rune string
is checked against `unicode.IsControl` and replaced with a space if it's
a raw control character. This matters because `Render` writes `Symbol`
to the real terminal byte-for-byte: a widget displaying untrusted text
(streamed subprocess output into a `TextArea`, for example) could
otherwise inject arbitrary ANSI/OSC escape sequences — title-bar spoofing,
color resets, or worse — into the user's terminal. Multi-rune strings
(the box-drawing and block-element glyphs the widgets themselves draw)
pass through unchanged, since those are trusted, not user-supplied.

## `Widget` and `BaseWidget`

```go
type Widget interface {
	DrawRelative(c *Canvas, offX, offY, pW, pH int)
	DrawOverlay(c *Canvas, offX, offY, pW, pH int)
	HandleEvent(ev Event)
	HitTest(mx, my int) bool

	SetFocus(f bool)
	CanFocus() bool
	HasFocus() bool
	GetChildren() []Widget
	SetVisible(v bool)
	IsVisible() bool
	SetEnabled(e bool)
	IsEnabled() bool
	SetPosition(x, y int)
	SetPercentLayout(pctX, pctY, pctW, pctH int)
	GetFixedH() int
	GetFixedW() int
}
```

Every UI element in Graphite implements this interface. You almost never
implement all of it by hand: every concrete widget embeds `BaseWidget`,
which implements every method except `DrawRelative` (and, for
interactive widgets, `HandleEvent`) — see
[custom-widgets.md](custom-widgets.md) for how to build your own widget
this way.

`BaseWidget` holds the fields every widget needs regardless of what it
draws:

```go
type BaseWidget struct {
	X, Y, Width, Height       int  // fixed position/size
	PctX, PctY, PctW, PctH    int  // percentage position/size (see layout.md)
	IsFocusable, IsFocused    bool
	Enabled, Visible          bool
	AbsX, AbsY, LastW, LastH  int  // resolved by the most recent DrawRelative
}
```

`AbsX`/`AbsY`/`LastW`/`LastH` are the widget's *last resolved* absolute
position and size on the canvas — set by `DrawRelative` every frame, and
what `HitTest` (mouse routing) and a widget's own drawing code both read.
Layout resolution (fixed vs. percentage, stretch, negative-offset
anchoring) is covered in full in [layout.md](layout.md).

`DrawOverlay` is the one `BaseWidget` method concrete widgets sometimes
override without needing to reimplement anything else: its default does
nothing, and it exists so a widget like `ComboBox` can draw its dropdown
list *after* every sibling widget has drawn, so the dropdown correctly
appears on top instead of being drawn over by whatever comes next in the
same container.

## `Window`: hosting a widget tree

```go
win := Graphite.NewWindow(50, 10, " My Window ")
win.AddWidget(someWidget)
app.SetWindow(win)
```

`Window` is a bordered, titled, centered container by default. It holds a
flat list of top-level `Children` (which may themselves be containers —
`Panel`, `Flex`, `GroupBox` — with their own nested children) and owns two
things no individual widget can do on its own: **focus order** and
**event routing**.

### Chrome: bordered dialog vs. full-screen application

```go
win := Graphite.NewWindow(50, 10, " My Window ")   // ChromeBordered (default): centered, bordered, drop shadow
win := Graphite.NewFullscreenWindow()              // ChromeBorderless: fills the canvas exactly, no decoration
```

`Window.Chrome` is `ChromeBordered` (the zero value) unless you opt into
`ChromeBorderless` — every existing window, and every window a future
`NewWindow` call creates, keeps drawing exactly as before. Reach for
`ChromeBorderless` when `Window` is the *application's* main window rather
than a floating dialog (a commander-style file manager, a full-screen
dashboard): it skips the border, drop shadow, and title bar entirely and
sizes itself to the canvas on every frame, ignoring
`FixedW`/`FixedH`/`PctW`/`PctH`. `PaddingX`/`PaddingY` still apply (0 by
default here, vs. `NewWindow`'s 4/2) if you want breathing room around the
edge without a visible frame.

### Focus order and Tab

`getFlatFocusables()` walks the whole tree (recursing into
`GetChildren()`) and returns every visible, focusable widget in traversal
order — this is also Tab order. `AddWidget` automatically focuses the
first focusable widget if nothing already has focus, so you don't need to
call `SetFocus` yourself for the common case of "focus the first field."

### Mouse routing and implicit capture

`Window.HandleEvent` is where hit-testing and mouse capture live:

- **`EventMouseDown`**: walks `Children` in order, hit-testing every
  visible widget (and its children, recursively, if the parent itself was
  hit); the *last* match found wins ties, matching `Draw`'s own
  last-drawn-on-top order. This is what lets `MenuStrip`'s open dropdown
  (or any widget whose `HitTest` expands past its normal bounds while an
  overlay is showing) win clicks over a sibling it visually covers — as
  long as that widget was added *after* the sibling it can overlap; add it
  before instead and the sibling wins the tie every time. Once a target is
  found and it's enabled, it becomes focused (any previously focused
  widget is unfocused first) and receives the event. The window also
  remembers this widget as `mouseCapture`.
- **`EventMouseDrag`** / **`EventMouseUp`**: go straight to whatever
  widget is currently captured, bypassing hit-testing entirely. This is
  what lets you drag a `Fader` handle or a scrollbar thumb past the
  widget's own bounds and keep dragging it — exactly what every desktop
  GUI toolkit does. Capture is released on `EventMouseUp`.
- **`EventMouseScrollUp`/`EventMouseScrollDown`**: hit-tested like
  `EventMouseDown`, but don't affect capture or focus.

`ClearMouseCapture()` exists for one specific situation: `Application`
calls it whenever a modal opens or closes, so a mouse-down that triggered
opening a modal doesn't leave a stale capture that then routes the
modal's first click somewhere in the window underneath it.

### Keyboard routing

Non-mouse events go to whichever widget currently has focus, except
`KeyTab`, which `Window` intercepts itself to advance focus through the
flattened order (wrapping around). Disabled widgets never receive an
event, regardless of what their own `HandleEvent` would otherwise do —
this is enforced by `Window`, not by each widget having to check
`Enabled` itself.

## Event

```go
type Event struct {
	Type     EventType
	Key      KeyCode
	CharCode rune
	MouseX   int
	MouseY   int
}
```

See [events.md](events.md) for the full list of `EventType`/`KeyCode`
values and how raw terminal input is decoded into these.

## Where layout and theming fit in

Two things this document deliberately doesn't cover, because they're
substantial on their own:

- **Layout** — how `X`/`Y`/`Width`/`Height` vs. `PctX`/`PctY`/`PctW`/`PctH`
  resolve, the "stretch when `Width`/`Height` ≤ 0" rule every widget
  follows, negative offsets as right/bottom anchors, and the `Panel` and
  `Flex` containers. See [layout.md](layout.md).
- **Theming** — `Color`, `Theme`'s fields, `DefaultTheme()`, and how to
  build a custom palette. See [theming.md](theming.md).
