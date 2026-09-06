# Theming

Graphite renders everything in 24-bit truecolor. There is no legacy
16/256-color palette or fallback path — every color in the library is a
`Color`, and every widget reads its colors from a `Theme` set on the
`Application`.

## `Color`

```go
type Color int32
```

A `Color` packs 8-bit red, green, and blue components into a single
`int32` as `(R<<16)|(G<<8)|B`. You build one of three ways:

```go
c1 := Graphite.RGB(255, 59, 48)   // explicit components
c2 := Graphite.Hex("#FF3B30")     // "#RRGGBB" or "RRGGBB"
c3 := Graphite.ColorNone          // sentinel: "leave this color unchanged"
```

`Hex` panics on a malformed string (not 6 hex digits, or not valid hex) —
a bad hex literal is treated as a programming error to catch at
development time, not a runtime condition a caller needs to recover from.
Only pass `Hex` a string literal or a value you've already validated.

`ColorNone` is a sentinel value (`-1`), not a real color. It's what
`Canvas.DrawCell`/`DrawText`/etc. treat as "don't touch this color" when
passed for `bg` or `fg` — pass it for one and a real color for the other
to update only one half of a cell's appearance. `GphPixel.Bg` also uses it
to mean "transparent, blend with whatever's already drawn there" (see
[images.md](images.md)).

### Inspecting and deriving colors

```go
r, g, b := c.Components()   // unpack back to 8-bit components
dimmer := c.Darken(0.3)     // scaled 30% of the way towards black
```

`Components()` is what you'd use to, say, pick black or white text based
on a background's perceived luminance (see `showcase`'s `contrastText`
helper, which computes ITU-R BT.601 luma from `Components()` and picks
whichever of black/white reads better). `Darken(pct)` scales towards
black — `0` returns the color unchanged, `1` returns black — and is how
several widgets derive an "inactive" variant of an accent color without
needing a second explicit color field: `Fader`'s inactive Mute/Solo icon
buttons use `activeColor.Darken(0.65)`, and unfocused danger buttons use
`theme.Danger.Darken(0.3)`.

## `Theme`

```go
type Theme struct {
	BgScreen   Color // the canvas background, outside any window
	BgWindow   Color // a window's interior background
	FgWindow   Color // default text color on that background
	BgWidget   Color // an unfocused interactive widget's background (input fields, list rows, ...)
	BgFocused  Color // a focused widget's background
	FgFocused  Color // text color on BgFocused
	Primary    Color // accent color: selection highlights, progress fill, active tab, ...
	Success    Color // affirmative/positive semantic color
	Danger     Color // destructive/error semantic color
	Warning    Color // caution semantic color
	Disabled   Color // a disabled widget's background
	FgDisabled Color // muted/secondary text (placeholders, disabled text, scrollbar track, ...)
	Accent     Color // a secondary accent distinct from Primary; no built-in widget reads it
}
```

Every widget's `DrawRelative` reads these fields off `c.theme` (the
`Canvas`'s current theme) rather than hardcoding colors, so replacing the
`Theme` restyles the entire application uniformly. A custom widget defined
outside package `Graphite` reads the same live values through the public
`c.Theme()` getter (`c.theme` itself is unexported) — see
[custom-widgets.md](custom-widgets.md).

### `DefaultTheme()`

```go
func DefaultTheme() Theme
```

A new `Canvas` (and so a new `Application`) starts with this theme until
`SetTheme` overrides it: a GitHub-Dark-inspired palette — near-black
background, soft light-gray text, a blue accent, and green/red/amber for
success/danger/warning.

```go
Theme{
	BgScreen:   RGB(13, 17, 23),
	BgWindow:   RGB(22, 27, 34),
	FgWindow:   RGB(201, 209, 217),
	BgWidget:   RGB(33, 40, 48),
	BgFocused:  RGB(56, 139, 253),
	FgFocused:  RGB(255, 255, 255),
	Primary:    RGB(56, 139, 253),
	Success:    RGB(63, 185, 80),
	Danger:     RGB(248, 81, 73),
	Warning:    RGB(210, 153, 34),
	Disabled:   RGB(48, 54, 61),
	FgDisabled: RGB(110, 118, 129),
	Accent:     RGB(188, 140, 255),
}
```

### Setting a custom theme

```go
app := Graphite.NewApplication()
app.SetTheme(Graphite.Theme{
	BgScreen:   Graphite.RGB(46, 52, 64),
	BgWindow:   Graphite.RGB(59, 66, 82),
	FgWindow:   Graphite.RGB(216, 222, 233),
	BgWidget:   Graphite.RGB(67, 76, 94),
	BgFocused:  Graphite.RGB(136, 192, 208),
	FgFocused:  Graphite.RGB(46, 52, 64),
	Primary:    Graphite.RGB(136, 192, 208),
	Success:    Graphite.RGB(163, 190, 140),
	Danger:     Graphite.RGB(191, 97, 106),
	Warning:    Graphite.RGB(235, 203, 139),
	Disabled:   Graphite.RGB(76, 86, 106),
	FgDisabled: Graphite.RGB(143, 153, 168),
	Accent:     Graphite.RGB(180, 142, 173),
})
```

`SetTheme` swaps every field at once — you always provide a complete
`Theme`, not a partial override, since a `Theme` value has no notion of
"unset." Start from `Graphite.DefaultTheme()` and mutate the fields you
want to change if you only want to tweak a couple of colors:

```go
theme := Graphite.DefaultTheme()
theme.Primary = Graphite.RGB(255, 105, 180)
app.SetTheme(theme)
```

`SetTheme` also forces a full redraw on the next frame, so the change is
visible immediately rather than only on cells that happen to redraw
anyway.

### Designing a theme: what actually needs contrast

A theme only reads correctly if a handful of pairs stay legible against
each other:

- `FgWindow` on `BgWindow` — most static text.
- `FgFocused` on `BgFocused` — anything focused (inputs, buttons, list
  selection).
- `FgWindow` (or white/black, computed via `Components()`) on `Primary`,
  `Success`, `Danger`, and `Warning` — these back solid-color chips
  (active tab, filled progress bar, styled buttons) more often than they
  sit as text-on-background.
- `FgDisabled` on `BgWindow` and on `Disabled` — needs to read as
  "muted" without disappearing entirely (placeholder text, scrollbar
  track, disabled labels all use it).

A minimal way to validate a new theme quickly: run `showcase` with
`app.SetTheme(yourTheme)` swapped in — its Theme tab renders every field
as a labeled swatch with automatically contrasting text, and its Widgets
tab exercises every focus/disabled/selected state those colors need to
stay legible in.
