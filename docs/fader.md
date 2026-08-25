# Fader

`Fader` is a vertical channel-strip control modeled on a real mixing
console: a draggable gain fader, an independent VU meter, a latching clip
indicator, and icon-only Mute/Solo buttons, all under a colored channel
label. It's the most involved widget in the library, so it gets its own
document. `showcase`'s Mixer tab and a real Windows system-volume-mixer
application built on Graphite are both good references alongside this
page.

## Anatomy

```go
mic := Graphite.NewFader(0, 0, 16, "MIC 1", Graphite.RGB(235, 203, 139))
mic.OnDoubleClick = func() {
	Graphite.ShowValueEditor(app, "MIC 1 Value", mic.Value, 0, 100, func(v float64) {
		mic.Value = v
	})
}
```

From top to bottom, a `Fader` draws:

1. **Label row** — a colored `●` dot (`LabelColor`) and `ChannelName`.
2. **Value row** — the current `Value` as `NNN%`, dimmed (`FgDisabled`).
3. **Clip row** *(if `ShowClip`)* — `⬤ Clipping`, dark red when idle,
   bright red when `Clipping` is latched.
4. **Track** — tick labels (if `ShowMeter`), the VU meter column, and the
   draggable fader rail/handle, filling all remaining vertical space.
5. **Buttons row** *(if `ShowMute` or `ShowSolo`)* — Mute and/or Solo as
   icon-only buttons sharing one row.

```go
type Fader struct {
	BaseWidget

	ChannelName string
	LabelColor  Color

	Value float64 // 0-100, the fader's own gain position
	Level float64 // 0-100, independent live VU level; see SetLevel

	Clipping      bool
	ClipThreshold float64 // Level at/above which SetLevel latches Clipping; 0 means 100

	ShowMeter bool
	ShowClip  bool
	ShowMute  bool
	ShowSolo  bool

	Muted  bool
	Soloed bool

	Ticks []FaderTick

	OnChange      func(value float64)
	OnMuteChange  func(muted bool)
	OnSoloChange  func(soloed bool)
	OnDoubleClick func()
}

func NewFader(x, y, w int, channelName string, labelColor Color) *Fader
```

`NewFader` starts every optional feature enabled (`ShowMeter`, `ShowClip`,
`ShowMute`, `ShowSolo` all `true`), `Value` at `80` (near the "0" mark on
the default dB-style ticks — where a real fader normally rests, not
pinned to the top), and `Height` at `0` — like `Panel`, a `Fader`
stretches to fill whatever vertical space its parent offers unless you
set `Height` explicitly (see [layout.md](layout.md#the-stretch-rule)).
Turn off any feature you don't want per-instance, as `showcase` does for
its `DESKTOP` (no solo) and `AUX` (bare fader, no meter/clip/mute/solo)
channels:

```go
desktop := Graphite.NewFader(0, 0, 16, "DESKTOP", Graphite.RGB(136, 192, 208))
desktop.ShowSolo = false

aux := Graphite.NewFader(0, 0, 12, "AUX", Graphite.RGB(191, 97, 106))
aux.ShowMeter, aux.ShowClip, aux.ShowMute, aux.ShowSolo = false, false, false, false
```

## `Value` vs. `Level`: two independent numbers

This is the single most important thing to understand about `Fader`, and
the reason it has two separate 0-100 fields instead of one:

- **`Value`** is the fader's *gain position* — what the user sets by
  dragging the handle, or what you set programmatically to reflect gain
  changed elsewhere. It only changes via a drag/click/keyboard nudge (which
  fires `OnChange`) or by your code assigning it directly.
- **`Level`** is the *live signal meter* — set via `SetLevel`, completely
  independent of `Value`. On a real mixing console, the meter shows how
  loud the actual audio is right now; the fader only controls how much
  gain is applied. Turning a fader down doesn't move the meter — much
  louder source audio does. `showcase` demonstrates this explicitly by
  driving `Level` from a sine wave in the idle callback while `Value`
  stays wherever the user last left it:

```go
app.SetIdleCallback(func() {
	elapsed := time.Since(startTime).Seconds()
	micFader.SetLevel(55 + 40*math.Sin(elapsed*2)) // meter moves on its own
	// mic.Value is untouched — the fader itself doesn't move
})
```

```go
func (f *Fader) SetLevel(level float64)
```

Clamps to `[0, 100]` and, as a side effect, updates `Clipping` (see
below). There is no `SetValue` — assign `Value` directly if you need to
change gain from outside a user drag (`ShowValueEditor`'s `onConfirm`
does exactly this).

## Clipping

```go
Clipping      bool
ClipThreshold float64 // 0 means "use the default, 85"
```

`SetLevel` latches `Clipping = true` the moment `Level >= ClipThreshold`,
and — unlike a real hardware clip LED, which usually needs a manual
reset — **clears it automatically** the moment `Level` drops back below
the threshold. `ClipThreshold` defaults to `0`, which `SetLevel` treats as
`faderDangerZonePercent` (`85`) — the same point the VU meter itself
turns red (see [Meter zone coloring](#meter-zone-coloring) below), so the
clip LED lighting up and the meter entering its red zone are the same
event by default, not two independently-tuned thresholds. Set
`ClipThreshold` to something else — say, `100`, to only latch on a true
full-scale peak — if you want them to diverge.

Clicking the clip row (`ClearClip()`) manually clears the latch early,
independent of the current `Level` — useful if you want a "peak hold that
the user acknowledges" UX rather than a purely live indicator.

## Meter zone coloring

Every row of the track, when `ShowMeter` is true, is colored by the
*value that row represents*, not by the current `Level` — `faderZoneColor`
maps a row's percentage to `theme.Success` (below 60), `theme.Warning`
(60 up to `faderDangerZonePercent`), or `theme.Danger` (at or above it).
Rows above the current `Level` are dimmed (`.Darken(0.75)`), so the meter
reads as a filled bar that's already colored red/amber/green in the zones
it *would* light up in, dimmed everywhere it currently isn't — rather
than a single flat color that changes based on the peak.

## `Ticks`: the scale labels

```go
type FaderTick struct {
	Label   string
	Percent float64 // 0-100, where 100 is the top of the track
}

func defaultFaderTicks() []FaderTick // "+3" at 100 down to "-30" at 0
```

`Ticks` is purely cosmetic — there is no real audio pipeline behind this
library, so `Percent` positions are not a dB conversion of anything, just
where along the 0-100 track a label is drawn. `NewFader` seeds a
plausible mixing-console-style scale (`+3` at the top down to `-30` near
the bottom); replace `f.Ticks` with your own `[]FaderTick` for a
different scale, or set it to `nil` to draw no tick labels (leaving more
horizontal room for the meter/track column).

## Mute and Solo

```go
Muted  bool
Soloed bool
OnMuteChange func(muted bool)
OnSoloChange func(soloed bool)
```

Both are plain toggles on the widget itself — clicking the left half of
the buttons row toggles `Muted` and fires `OnMuteChange`; the right half
toggles `Soloed` and fires `OnSoloChange`. **`Fader` has no built-in
cross-channel solo behavior** — soloing channel A muting every *other*
channel B, C, D, and restoring their prior mute state when nothing is
soloed anymore, is application logic you write in `OnSoloChange`, not
something `Fader` coordinates for you. A real implementation (from a
Windows system-volume-mixer app built on Graphite) looks like this:

```go
soloed := make(map[uint32]bool) // which channels are currently soloed

func setSolo(pid uint32, isSoloed bool) {
	wasActive := len(soloed) > 0
	if isSoloed {
		soloed[pid] = true
	} else {
		delete(soloed, pid)
	}
	nowActive := len(soloed) > 0

	switch {
	case !wasActive && nowActive:
		// first solo this round: snapshot everyone's current mute state
		for _, ch := range channels {
			ch.preSoloMuted = ch.Muted()
		}
		fallthrough
	case nowActive:
		// force-mute everyone not soloed
		for id, ch := range channels {
			ch.SetMuted(!soloed[id])
		}
	case wasActive:
		// last solo turned off: restore what each channel's mute state was
		// right before the first solo — not an unconditional unmute, so a
		// channel the user had deliberately muted stays muted
		for _, ch := range channels {
			ch.SetMuted(ch.preSoloMuted)
		}
	}
}
```

The key detail worth keeping if you copy this pattern: restore from a
**snapshot taken at the moment the first solo activated**, not an
unconditional unmute — otherwise soloing and un-soloing would clobber a
mute state the user had set deliberately before soloing anything.

Icon buttons render with a **dark tint of their own accent color** when
inactive (`activeColor.Darken(0.65)`) rather than a shared gray — Mute is
red-tinted, Solo is amber-tinted — specifically so the two stay visually
distinguishable from each other even when both are off, and so their
"this is inactive but not disabled" state is still readable at a glance.

Set `ShowMute`/`ShowSolo` to `false` to omit either button — a lone
remaining button then fills the whole row instead of sharing half of it.
`showcase`'s `DESKTOP` channel (`ShowSolo = false`) is the reference for
"soloing everything doesn't make sense for this channel" — the same
reasoning a MASTER/system-output channel should generally apply.

## Interaction

| Input | Effect |
|---|---|
| Click/drag on the track | Jump/drag `Value` to that row's percentage |
| `Up`/`Right` (focused) | `Value += 2` |
| `Down`/`Left` (focused) | `Value -= 2` |
| Double-click on the track | Fires `OnDoubleClick` instead of jumping `Value` — see below |
| Click on the clip row | `ClearClip()` |
| Click left half of buttons row | Toggle `Muted`, fire `OnMuteChange` |
| Click right half of buttons row | Toggle `Soloed`, fire `OnSoloChange` |

A double-click is two clicks within 400ms *and* within 1 row of each
other vertically (`isDoubleClick`) — a third rapid click starts a fresh
pair rather than chaining into a second double-click. `OnChange` never
fires for the click that triggers a double-click (it returns before
calling `setValueFromRow`), so wiring `OnDoubleClick` to open a precise
numeric-entry modal doesn't also jump `Value` to wherever the first click
of the pair happened to land.

Dragging works past the widget's own bounds because of `Window`'s
implicit mouse capture (see
[architecture.md](architecture.md#mouse-routing-and-implicit-capture)) —
once the track is hit by the initial press, every subsequent
`EventMouseDrag` goes straight to this `Fader` regardless of where the
pointer actually is, exactly like dragging a slider handle past a real
window's edge in a desktop GUI.

## `ShowValueEditor`: typing an exact value

```go
func ShowValueEditor(app *Application, title string, current, min, max float64, onConfirm func(float64))
```

A modal with a labeled `InputBox` pre-filled with `current`; `OK`, the
`Enter` key (via `InputBox.OnSubmit`), both validate and call `onConfirm`.
An invalid or out-of-range entry shows an inline error label instead of
closing the modal, so the user can correct it without restarting. This
isn't `Fader`-specific — it's a general "type a number in `[min, max]`"
modal (see [modals.md](modals.md)) — but wiring it to `OnDoubleClick` is
the standard way to pair a `Fader`'s draggable coarse control with a
precise typed one:

```go
f.OnDoubleClick = func() {
	Graphite.ShowValueEditor(app, f.ChannelName+" Volume", f.Value, 0, 100, func(v float64) {
		f.Value = v
		// also push v to whatever backing state f.Value represents,
		// since assigning f.Value directly does not fire OnChange
	})
}
```

Note `onConfirm` assigning `f.Value` directly does **not** fire
`OnChange` (only a user drag/click/nudge does, via the internal `setValue`
helper) — if `OnChange` is what actually applies the gain to some backing
system, call that logic explicitly inside `onConfirm` too, as shown above.

## Sizing multiple channels in a row

A row of `Fader`s is normally built with `Flex`. Whether you want them
packed at a fixed width or stretched to fill the row evenly is the same
`weight` decision covered in [layout.md](layout.md#weighted-vs-natural-size-children) —
give every `Fader` an explicit `Width` (e.g. `16`-`20`) and
`weight: 0` for channel strips that stay a fixed, readable width no
matter how many are added; leave `Width` at `0` and use `weight: 1` if you
want them to always fill the available row width evenly instead.

```go
channels := Graphite.NewFlex(0, 2, 0, 0, Graphite.FlexRow)
channels.Gap = 2
channels.AddChild(mic, 1)
channels.AddChild(desktop, 1)
channels.AddChild(aux, 1)
```
