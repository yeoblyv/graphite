# Events and input

This document covers how raw terminal input becomes an `Event`, what the
`Event` types mean, and the routing rules `Window` applies (summarized
here, covered in full in [architecture.md](architecture.md#window-hosting-a-widget-tree)).

## `Event`

```go
type Event struct {
	Type     EventType
	Key      KeyCode
	CharCode rune
	MouseX   int
	MouseY   int
}
```

A single input notification. Which fields are meaningful depends on
`Type`:

- **Keyboard events** (`EventKey`): `Key` holds a non-printable key code
  (see below), or `KeyNone` with `CharCode` holding the actual printable
  rune the user typed (including non-ASCII — `parseANSI` decodes UTF-8).
- **Mouse events**: `MouseX`/`MouseY` hold the 0-indexed column/row the
  event occurred at (already converted from the terminal's 1-indexed SGR
  coordinates).

## `EventType`

```go
const (
	EventNone EventType = iota
	EventKey
	EventMouseDown
	EventMouseDrag
	EventMouseUp
	EventMouseScrollUp
	EventMouseScrollDown
	EventMouseRightDown
)
```

`EventNone` is returned by `pollEvent` when nothing arrived within its
poll timeout — it's how the render loop keeps ticking (servicing the idle
callback, animations, etc.) even with no input; it is never delivered to
a widget's `HandleEvent`.

`EventMouseDrag` is a motion report with the left button held, delivered
*only* to whichever widget was hit by the preceding `EventMouseDown` (see
`Window`'s mouse capture in [architecture.md](architecture.md)) —
regardless of where the pointer has moved to since, including outside
that widget's own bounds. `EventMouseUp` (button release) behaves the
same way, and additionally clears the capture.

`EventMouseRightDown` is a right-button press. It's hit-tested and
delivered the same way as a scroll event — once, to whatever's under the
pointer — but unlike `EventMouseDown` it never moves focus or starts a
mouse capture, since a right click has no corresponding drag/release to
capture for.

## `KeyCode`

```go
const (
	KeyNone      KeyCode = 0
	KeyTab       KeyCode = 9
	KeyEnter     KeyCode = 10
	KeyEscape    KeyCode = 27
	KeySpace     KeyCode = 32
	KeyBackspace KeyCode = 127
	KeyUp        KeyCode = 1001
	KeyDown      KeyCode = 1002
	KeyLeft      KeyCode = 1003
	KeyRight     KeyCode = 1004
	KeyDelete    KeyCode = 1005
	KeyCtrlC     KeyCode = 1006
	KeyCtrlV     KeyCode = 1007
	KeyCtrlX     KeyCode = 1008
	KeyF1        KeyCode = 1009
	KeyF2        KeyCode = 1010
	KeyF3        KeyCode = 1011
	KeyF4        KeyCode = 1012
	KeyF5        KeyCode = 1013
	KeyF6        KeyCode = 1014
	KeyF7        KeyCode = 1015
	KeyF8        KeyCode = 1016
	KeyF9        KeyCode = 1017
	KeyF10       KeyCode = 1018
	KeyF11       KeyCode = 1019
	KeyF12       KeyCode = 1020
	KeyInsert    KeyCode = 1021
)
```

Printable characters (letters, digits, punctuation, space is the one
exception — it gets its own `KeySpace`) arrive as `CharCode` with `Key`
left at `KeyNone`, not as a `KeyCode`. `Ctrl+C`/`Ctrl+V`/`Ctrl+X` are
wired specifically for clipboard operations in `InputBox` and `TextArea`
(via `github.com/atotto/clipboard`) — `Ctrl+C` copies the field's full
value, `Ctrl+X` cuts it, `Ctrl+V` pastes at the cursor, stripping
newlines for `InputBox` since it's single-line.

`F1`-`F12` are decoded from both encodings terminals actually send: the
SS3 form (`ESC O P`...`ESC O S`, xterm's encoding for `F1`-`F4`) and the
CSI-tilde form (`ESC [` + digits + `~`, used for `F5`-`F12` everywhere and
as an alternate encoding for `F1`-`F4` on some terminals). Neither
`Window` nor `Application` reserves any of them — see
[architecture.md](architecture.md) for `Tab`/`Escape`, the only two keys
that are — so a program is free to wire all twelve to its own commands,
e.g. a Total-Commander-style `F5` Copy/`F6` Move/`F8` Delete bar.

`KeyInsert` is decoded the same CSI-tilde way, for programs that use it
for the classic file-manager "tag/mark this row and move down" gesture.

## Two keys `Window`/`Application` reserve

- **`Tab`** advances focus through the window's flattened focus order
  (see [architecture.md](architecture.md#focus-order-and-tab)) — a
  widget's own `HandleEvent` never sees a `Tab` keypress, `Window`
  intercepts it first.
- **`Esc`** is handled by `Application.Run` before routing reaches a
  widget at all: it closes the topmost modal if one is open, otherwise
  quits the application. If you need `Esc` to do something else inside a
  specific widget, there currently isn't a way to intercept it before
  this top-level handling — design around it (e.g. use a different key
  for an in-widget "cancel" action) rather than trying to override it.

## Mouse input: SGR reporting

Graphite enables SGR mouse mode (`\033[?1002h\033[?1015h\033[?1006h`) on
`terminal.init()` — button-event reporting (presses, releases, and motion
*while a button is held*), not full pointer tracking that would report
every pixel the cursor crosses idly. `parseANSI` decodes the resulting
`\033[<btn;x;yM`/`\033[<btn;x;ym` sequences:

| SGR button code | Event |
|---|---|
| `0`, final byte `M` | `EventMouseDown` |
| `0`, final byte `m` | `EventMouseUp` |
| `2`, final byte `M` | `EventMouseRightDown` |
| `32` | `EventMouseDrag` |
| `64` | `EventMouseScrollUp` |
| `65` | `EventMouseScrollDown` |

Coordinates in the escape sequence are 1-indexed; `parseANSI` converts
them to the 0-indexed `MouseX`/`MouseY` every widget's layout math
expects.

## Building and testing input by hand

If you need to test a widget's input handling without a real terminal
(the library's own test suite does this extensively), you can either call
`widget.HandleEvent(Event{...})` directly with a synthetic `Event`, or, to
exercise the *entire* pipeline including `Window`'s hit-testing and mouse
capture, feed raw SGR bytes through `parseANSI` and then `Window.HandleEvent`:

```go
win := Graphite.NewWindow(60, 40, "test")
f := Graphite.NewFader(2, 2, 16, "CH1", Graphite.RGB(255, 200, 0))
win.AddWidget(f)

c := Graphite.NewCanvas()
c.Resize(80, 50)
win.Draw(c) // resolve layout so f.trackTop etc. are set

press := []byte(fmt.Sprintf("\033[<0;%d;%dM", f.AbsX+8+1, f.trackTop+1))
// (parseANSI/Window.HandleEvent are internal to the package; from outside
// the package, drive a real Application or call widget.HandleEvent directly)
```

This end-to-end approach is what caught real layout bugs during
development that unit-level `HandleEvent` calls alone did not — a widget
overlapping its neighbor only shows up once real rendering has resolved
`AbsX`/`AbsY`, not from calling `HandleEvent` with hand-picked coordinates.
