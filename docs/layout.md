# Layout

Every widget resolves its on-screen position and size the same way, via
`BaseWidget.DrawRelative`. This document explains that resolution in
full, then covers the three layout containers built on top of it: `Panel`,
`Flex`, and `GroupBox`.

## Fixed positioning

```go
label := Graphite.NewLabel(2, 4, "Hello")
```

`X`/`Y` are a fixed offset from the parent's content origin, in columns
and rows. This is what most `New*` constructors set from their `x, y`
parameters.

### Negative offsets anchor from the far edge

A negative `X` or `Y` is not "off-screen" — it's an offset from the
parent's *far* edge instead of its origin. `X: -1` means "one column from
the right edge of the parent's content area"; `Y: -2` means "two rows
from the bottom." This is how `showcase`'s file picker positions its
Cancel/Open buttons and filter combo box relative to the dialog's
bottom-right corner regardless of the dialog's actual size:

```go
mod.AddWidget(Graphite.NewButton(-20, -2, "Open", Graphite.BtnSuccess, onOpen))
mod.AddWidget(Graphite.NewButton(-10, -2, "Cancel", Graphite.BtnDefault, onCancel))
```

## Percentage positioning

```go
panel.SetPercentLayout(0, 0, 48, 90)
```

`SetPercentLayout(pctX, pctY, pctW, pctH)` switches a widget to
position/size relative to its parent's content area instead of fixed
columns/rows. A value of `0` for any one of the four leaves the
corresponding fixed `X`/`Y`/`Width`/`Height` in effect instead — so you
can mix, e.g., a percentage width with a fixed `Y` offset. This is the
standard way `showcase` lays out two side-by-side columns:

```go
colLeft := Graphite.NewPanel(0, 2, 0, 0)
colLeft.SetPercentLayout(0, 0, 48, 90)   // left half, 90% of parent's height

colRight := Graphite.NewPanel(0, 2, 0, 0)
colRight.SetPercentLayout(52, 0, 48, 90) // right half, starting at 52%
```

## The stretch rule

This is the single most important rule to understand about Graphite's
layout, because it governs how `Flex` and any container that offers a
widget more space than it was given actually behaves:

> **A widget with `Width` (or `Height`) `<= 0` stretches to fill
> whatever space its parent offers it. A widget with a positive fixed
> `Width`/`Height` always keeps that exact size, no matter how much space
> the parent offers.**

Concretely, from `BaseWidget.DrawRelative`:

```go
b.LastW = b.Width
if b.PctW > 0 {
	b.LastW = (pW * b.PctW) / 100
} else if b.Width <= 0 {
	// A zero or negative fixed Width means "stretch to fill the
	// remaining parent width", with the value acting as a right margin.
	b.LastW = pW - (b.AbsX - offX) + b.Width
}
```

So `Width: 0` stretches to fill exactly what's left; `Width: -2` stretches
to fill what's left *minus 2 columns* (a right margin). `Height` works
identically along the vertical axis. Most container-friendly widgets
(`Fader`, `Panel`, `Flex` itself, `ListBox`/`TextArea` when constructed
with `h=0`) rely on this to fill whatever their parent gives them without
the caller doing arithmetic.

The result is then clamped so a widget can never report a size larger
than what its parent actually has left — a widget cannot bleed outside
its parent's bounds this way, only fail to fill them if the parent ran
out of room.

## `Panel`: a plain layout box

```go
panel := Graphite.NewPanel(0, 2, 0, 0)
panel.SetPercentLayout(0, 0, 100, 90)
panel.AddWidget(Graphite.NewLabel(0, 0, "Inside the panel"))
```

`Panel` groups children under a shared position/size without being
focusable itself — only its children are. It's the standard way to carve
out a labeled region (a column, a tab's content area) that other widgets
position themselves inside using coordinates relative to the panel
instead of the whole window. `GroupBox` (below) is the same idea with a
drawn border and label.

## `Flex`: CSS-flexbox-style distribution

```go
row := Graphite.NewFlex(0, 2, 0, 5, Graphite.FlexRow)
row.Gap = 1
row.AddChild(swatchA, 1)
row.AddChild(swatchB, 2)
row.AddChild(swatchC, 1)
panel.AddWidget(row)
```

`Flex` distributes space among its children along one axis —
`Graphite.FlexRow` (left-to-right; each child spans the container's full
height) or `Graphite.FlexColumn` (top-to-bottom; each spans the full
width) — instead of requiring you to compute fixed pixel or percentage
offsets by hand. `Gap` inserts that many columns/rows of empty space
between consecutive *visible* children (hiding a child via `SetVisible(false)`
closes the gap it would otherwise leave, matching CSS's `display: none`).

### Weighted vs. natural-size children

```go
func (f *Flex) AddChild(w Widget, weight int)
```

This is the one API surface in `Flex` worth understanding precisely,
because it interacts with the stretch rule above:

- **`weight <= 0`**: the child gets its own *natural* size along the main
  axis — `GetFixedW()` for `FlexRow`, `GetFixedH()` for `FlexColumn` (both
  fall back to `1` if the widget's own `Width`/`Height` isn't a positive
  fixed value). `Flex` advances its layout cursor by exactly that size
  plus `Gap`, so children with a fixed width/height pack tightly
  left-to-right (or top-to-bottom) and any space left over in the
  container stays empty rather than being redistributed.
- **`weight > 0`**: the child's offered size is `remaining space *
  weight / totalWeight`, where `remaining` is whatever's left after every
  fixed-size (`weight <= 0`) sibling has taken its natural size. Multiple
  weighted children split the remainder proportionally to their weight —
  a 1:2:1 split behaves exactly like CSS `flex: 1`/`flex: 2`/`flex: 1`.

Crucially, the *offered* size from a weighted child is not the same as
its *rendered* size: per the stretch rule, a child with its own positive
fixed `Width`/`Height` keeps that size regardless of what `Flex` offers
it — so giving a `Fader` both `weight: 1` and an explicit `Width: 20`
means it never actually grows past 20 columns even in a row with room to
spare, but the *cursor* still advances by the larger weighted allotment,
which can leave a visible gap before the next child. If you want channel
strips that are genuinely packed tight at their fixed width instead of
spaced out with slack, use `weight <= 0` (e.g. `0`), not `weight: 1`,
alongside a fixed `Width`:

```go
channels := Graphite.NewFlex(0, 0, 0, 0, Graphite.FlexRow)
channels.Gap = 3
channels.AddChild(masterFader, 0) // masterFader has a fixed Width; packs tight
for _, f := range channelFaders {
	channels.AddChild(f, 0) // same — left-packed, doesn't stretch across the row
}
```

Conversely, if you want children to *actually* fill all available width
evenly regardless of their own `Width` field, leave their `Width` at `0`
(stretch) and give them `weight > 0` — this is what `showcase`'s Layout
tab demonstrates with plain color swatches.

### `Clear()`: rebuilding a `Flex`'s children at runtime

```go
flex.Clear()          // empties the child list
flex.AddChild(a, 0)
flex.AddChild(b, 0)
```

For a `Flex` whose contents change at runtime — a dynamic list of mixer
channels, for instance — `Clear()` followed by re-adding every child is
the supported pattern, rather than trying to track and mutate individual
child indices. It's simpler and there's no equivalent "remove one child"
API by design.

## `GroupBox`: a bordered, labeled `Panel`

```go
gb := Graphite.NewGroupBox(0, 21, 40, 8, "GroupBox Example")
gb.AddWidget(Graphite.NewLabel(0, 0, "Widgets inside GroupBox:"))
gb.AddWidget(Graphite.NewButton(0, 2, "Action", Graphite.BtnInfo, nil))
```

`GroupBox` is functionally a `Panel` that also draws a border and a label
across the top edge. Children are positioned relative to the box's
*interior*, which is inset by 2 columns/rows from the box's own
`X, Y` to leave room for the border and label — position your first child
at `Y: 0` or later (not negative), and expect roughly 2 fewer usable
columns/rows than the box's own `w, h`.

## Recap

| Need | Use |
|---|---|
| A fixed offset from the parent's top-left | `X`, `Y` (constructor args) |
| A fixed offset from the parent's bottom-right | Negative `X`/`Y` |
| Position/size relative to the parent's actual dimensions | `SetPercentLayout(x, y, w, h)` |
| Fill whatever space is left | `Width`/`Height` `<= 0` (the default for many widgets) |
| A plain grouping box, no border | `Panel` |
| A bordered, labeled grouping box | `GroupBox` |
| Distribute children along one axis by weight, CSS-flexbox-style | `Flex` |
| Children packed at their own natural/fixed size | `Flex.AddChild(w, 0)` |
| Children sharing leftover space proportionally | `Flex.AddChild(w, weight > 0)` |
| Rebuild a `Flex`'s children at runtime | `Flex.Clear()` + re-`AddChild` |
