# Building a custom widget

Every widget in Graphite — including every one shipped in the library
itself — is built the same way: embed `BaseWidget`, and override only the
methods whose default behavior isn't what you want. This document walks
through that process using `showcase`'s `swatch` widget as a complete,
minimal, real example, then covers the less obvious parts (overlays,
containers, focus).

## The minimum: a static, non-interactive widget

`swatch` paints a solid color block with a centered, automatically
contrasting label — used in `showcase` both to visualize `Flex`
proportions and to display theme colors:

```go
type swatch struct {
	Graphite.BaseWidget
	Label string
	Fill  Graphite.Color
}

func newSwatch(label string, fill Graphite.Color) *swatch {
	return &swatch{BaseWidget: Graphite.NewBaseWidget(0, 0, 0, 0), Label: label, Fill: fill}
}

// contrastText picks black or white, whichever reads better against bg,
// using perceived luminance (ITU-R BT.601).
func contrastText(bg Graphite.Color) Graphite.Color {
	r, g, b := bg.Components()
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if luma > 140 {
		return Graphite.RGB(0, 0, 0)
	}
	return Graphite.RGB(255, 255, 255)
}

// DrawRelative implements Graphite.Widget.
func (s *swatch) DrawRelative(c *Graphite.Canvas, offX, offY, pW, pH int) {
	s.BaseWidget.DrawRelative(c, offX, offY, pW, pH) // resolve AbsX/AbsY/LastW/LastH first
	fg := contrastText(s.Fill)
	for y := 0; y < s.LastH; y++ {
		for x := 0; x < s.LastW; x++ {
			c.DrawCell(s.AbsX+x, s.AbsY+y, " ", s.Fill, fg)
		}
	}
	labelY := s.AbsY + s.LastH/2
	if labelY < s.AbsY+s.LastH {
		c.DrawText(s.AbsX+1, labelY, s.Label, s.Fill, fg)
	}
}
```

That's a complete, usable widget. It compiles against the `Widget`
interface because `BaseWidget` already implements every method except
`DrawRelative` — `HandleEvent` (default: ignore everything), `HitTest`
(default: rectangle test against `AbsX`/`AbsY`/`LastW`/`LastH`),
`SetFocus`/`HasFocus`/`CanFocus` (default: never focusable, since
`NewBaseWidget` leaves `IsFocusable` at its zero value `false`),
`GetChildren` (default: `nil`, no children), `SetVisible`/`IsVisible`,
`SetEnabled`/`IsEnabled`, `SetPosition`, `SetPercentLayout`,
`GetFixedW`/`GetFixedH`.

You use it exactly like any built-in widget:

```go
row := Graphite.NewFlex(0, 2, 0, 5, Graphite.FlexRow)
row.AddChild(newSwatch("weight 1", theme.Primary), 1)
row.AddChild(newSwatch("weight 2", theme.Success), 2)
panel.AddWidget(row)
```

`swatch` captures its color once, at construction, because it only ever
needs a fixed, deliberately-chosen color (that's the whole point of a
color-swatch widget). A widget that should restyle automatically when the
application calls `SetTheme` — the common case — reads the live palette
in `DrawRelative` via `c.Theme()` instead, exactly like a built-in widget
reads `c.theme`:

```go
func (w *myWidget) DrawRelative(c *Graphite.Canvas, offX, offY, pW, pH int) {
	w.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	theme := c.Theme()
	c.DrawText(w.AbsX, w.AbsY, w.Text, theme.BgWindow, theme.FgWindow)
}
```

### The one rule every `DrawRelative` override must follow

**Always call `s.BaseWidget.DrawRelative(c, offX, offY, pW, pH)` first**,
before drawing anything. This is what actually resolves `AbsX`/`AbsY`/
`LastW`/`LastH` from the fixed/percentage/stretch rules (see
[layout.md](layout.md)) against the offered box — skip it and every
coordinate your widget reads is stale or zero.

## Making it interactive

Add `IsFocusable = true` in the constructor and override `HandleEvent`:

```go
func newSwatch(label string, fill Graphite.Color) *swatch {
	base := Graphite.NewBaseWidget(0, 0, 0, 0)
	base.IsFocusable = true
	return &swatch{BaseWidget: base, Label: label, Fill: fill}
}

func (s *swatch) HandleEvent(ev Graphite.Event) {
	if ev.Type == Graphite.EventMouseDown || (ev.Type == Graphite.EventKey && ev.Key == Graphite.KeyEnter) {
		// respond to activation
	}
}
```

`Window` handles the mechanics for you once `IsFocusable` is `true` —
`Tab` order, focus highlighting via whatever `IsFocused` you read in
`DrawRelative`, and routing `EventMouseDown`/keyboard events to
`HandleEvent` only while enabled. You never need to check `Enabled`
yourself inside `HandleEvent` — `Window` withholds events from disabled
widgets before your code ever runs.

## Drawing on top of siblings: `DrawOverlay`

If your widget needs to draw something that must appear *above* whatever
widgets come after it in the same container — a dropdown, a tooltip, a
popup menu — override `DrawOverlay` instead of trying to do it in
`DrawRelative`. `Window.Draw` calls every child's `DrawRelative` first (in
order), then every child's `DrawOverlay` (also in order) — so anything
drawn in `DrawOverlay` always ends up on top of every widget's normal
content, regardless of container order. `ComboBox`'s open dropdown list
and `MenuStrip`'s open category dropdown are both built this way — see
their entries in [widgets.md](widgets.md) for the pattern, and
[architecture.md](architecture.md#widget-and-basewidget) for why this
two-pass draw exists.

## Building a container: `GetChildren`

A widget that holds other widgets (like `Panel`) needs to override
`GetChildren()` so `Window` can descend into it for focus order and
hit-testing:

```go
type MyContainer struct {
	Graphite.BaseWidget
	Children []Graphite.Widget
}

func (c *MyContainer) GetChildren() []Graphite.Widget { return c.Children }

func (c *MyContainer) DrawRelative(canvas *Graphite.Canvas, offX, offY, pW, pH int) {
	c.BaseWidget.DrawRelative(canvas, offX, offY, pW, pH)
	for _, child := range c.Children {
		if child.IsVisible() {
			child.DrawRelative(canvas, c.AbsX, c.AbsY, c.LastW, c.LastH)
		}
	}
}
```

Note the child's offered box is the container's own *resolved* box
(`c.AbsX, c.AbsY, c.LastW, c.LastH`), not the box the container itself
was offered — this is what makes a child's percentage/stretch layout
resolve relative to the container's actual size rather than its parent's.
If your container also needs `DrawOverlay` propagation for its children
(so a `ComboBox` nested inside it still draws its dropdown on top
correctly), forward that too, the same way `Panel` and `GroupBox` do —
see their source for the two-line pattern.

## Checklist

- [ ] Embed `Graphite.BaseWidget` (not a pointer — `BaseWidget` is
      embedded by value in every built-in widget).
- [ ] Call `NewBaseWidget(x, y, w, h)` in your constructor; set
      `IsFocusable = true` there if the widget accepts input.
- [ ] Override `DrawRelative`, calling the embedded `BaseWidget`'s version
      *first*, then draw using `AbsX`/`AbsY`/`LastW`/`LastH`.
- [ ] Override `HandleEvent` if the widget responds to input — switch on
      `ev.Type` (`EventKey`, `EventMouseDown`, `EventMouseDrag`, ...).
- [ ] Override `DrawOverlay` only if something needs to render above
      sibling widgets.
- [ ] Override `GetChildren` (and forward `DrawRelative`/`DrawOverlay` to
      each child) if the widget is itself a container.
- [ ] Override `HitTest` only if the widget's clickable area isn't a
      simple rectangle over `AbsX/AbsY/LastW/LastH` (see `ComboBox` and
      `MenuStrip` in [widgets.md](widgets.md) for real examples — both
      expand their hit area while a dropdown they own is open).

Reading `swatch` in `showcase/main.go`, and any one built-in widget's
source close to what you're building (`Checkbox` for a simple toggle,
`ListBox` for a scrollable selection list, `ComboBox` for an overlay-based
popup), is the fastest way to see this pattern in full working code
before writing your own.
