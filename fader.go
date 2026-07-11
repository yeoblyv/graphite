package Graphite

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// -------------------------------------------------------------------------
// Fader
// -------------------------------------------------------------------------

// FaderTick is one labeled position along a Fader's scale. It is purely a
// visual label — this library has no real audio pipeline, so Percent is
// just where along the 0-100 track the label is drawn, not a dB conversion.
type FaderTick struct {
	Label   string
	Percent float64 // 0-100, where 100 is the top of the track.
}

// defaultFaderTicks mirrors a typical mixing-console fader scale.
func defaultFaderTicks() []FaderTick {
	return []FaderTick{
		{"+3", 100}, {"0", 87}, {"-3", 74}, {"-6", 61}, {"-9", 48},
		{"-12", 35}, {"-18", 22}, {"-24", 11}, {"-30", 0},
	}
}

// Fader is a vertical channel-strip control: a draggable, clickable gain
// fader (Value, 0-100), an optional independent VU meter (Level, set via
// SetLevel — distinct from Value, the way a real mixer's meter shows the
// actual signal while the fader only sets gain), an optional latching clip
// LED, and optional icon-only Mute/Solo buttons sharing one row, and a
// colored channel label. Height defaults to 0, so — like Panel — a Fader
// stretches to fill whatever vertical space its parent (e.g. Flex) offers;
// set Height explicitly for a fixed size instead.
type Fader struct {
	BaseWidget

	ChannelName string
	LabelColor  Color

	Value float64 // 0-100, the fader's own gain position.
	Level float64 // 0-100, independent live VU level; see SetLevel.

	Clipping      bool
	ClipThreshold float64 // Level at/above which SetLevel latches Clipping. Zero means 100.

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

	lastClickAt  time.Time
	lastClickRow int

	// Row/column geometry resolved by the most recent DrawRelative, so
	// HandleEvent can work out which sub-region a click landed in without
	// redoing layout math. -1 means that row isn't currently shown.
	trackTop, trackHeight int
	clipRow, buttonsRow   int
}

// NewFader creates a Fader at (x, y) with the given width, channel name, and
// label color; Height starts at 0 (stretch to fill the parent — see the
// Fader doc comment). All optional sub-features (meter, clip LED, mute,
// solo) start enabled; Value starts at 80 (near the "0" mark on the default
// ticks, matching where a real fader normally sits — not pinned at the top).
func NewFader(x, y, w int, channelName string, labelColor Color) *Fader {
	base := NewBaseWidget(x, y, w, 0)
	base.IsFocusable = true
	return &Fader{
		BaseWidget:    base,
		ChannelName:   channelName,
		LabelColor:    labelColor,
		Value:         80,
		ClipThreshold: 100,
		ShowMeter:     true,
		ShowClip:      true,
		ShowMute:      true,
		ShowSolo:      true,
		Ticks:         defaultFaderTicks(),
	}
}

// SetLevel sets the independent live VU level (0-100), clamped, and latches
// Clipping once it reaches ClipThreshold. Clipping stays latched — even if
// Level later drops back down — until ClearClip is called or the clip
// indicator is clicked, so a brief peak isn't missed, matching how a real
// clip light behaves.
func (f *Fader) SetLevel(level float64) {
	f.Level = math.Min(math.Max(level, 0), 100)
	threshold := f.ClipThreshold
	if threshold <= 0 {
		threshold = 100
	}
	if f.Level >= threshold {
		f.Clipping = true
	}
}

// ClearClip resets the latched clip indicator.
func (f *Fader) ClearClip() {
	f.Clipping = false
}

// setValue clamps and applies a new fader position, firing OnChange if it
// actually changed.
func (f *Fader) setValue(v float64) {
	v = math.Min(math.Max(v, 0), 100)
	if v == f.Value {
		return
	}
	f.Value = v
	if f.OnChange != nil {
		f.OnChange(f.Value)
	}
}

// rowForPercent maps a 0-100 value to the screen row it renders at within
// the track (100 at the top row, 0 at the bottom row).
func (f *Fader) rowForPercent(pct float64) int {
	if f.trackHeight <= 1 {
		return f.trackTop
	}
	pct = math.Min(math.Max(pct, 0), 100)
	offset := int(pct / 100 * float64(f.trackHeight-1))
	return f.trackTop + f.trackHeight - 1 - offset
}

// percentForRow is rowForPercent's inverse.
func (f *Fader) percentForRow(row int) float64 {
	if f.trackHeight <= 1 {
		return 0
	}
	rel := f.trackTop + f.trackHeight - 1 - row
	return float64(rel) / float64(f.trackHeight-1) * 100
}

// setValueFromRow sets Value to whatever percentage the given screen row
// represents — used by both a track click (jump) and a drag.
func (f *Fader) setValueFromRow(row int) {
	f.setValue(f.percentForRow(row))
}

// faderZoneColor picks the meter color for a given level percentage: green
// below 60, amber 60-85, red at or above 85 — reusing Theme's existing
// Success/Warning/Danger fields rather than adding new ones.
func faderZoneColor(theme Theme, pct float64) Color {
	switch {
	case pct >= 85:
		return theme.Danger
	case pct >= 60:
		return theme.Warning
	default:
		return theme.Success
	}
}

// DrawRelative implements Widget.
func (f *Fader) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	f.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	labelBg := c.theme.BgWindow
	c.DrawText(f.AbsX, f.AbsY, "●", labelBg, f.LabelColor)
	c.DrawText(f.AbsX+2, f.AbsY, f.ChannelName, labelBg, c.theme.FgWindow)
	c.DrawText(f.AbsX, f.AbsY+1, fmt.Sprintf("%3.0f%%", f.Value), labelBg, c.theme.FgDisabled)

	row := f.AbsY + 2
	bottom := f.AbsY + f.LastH

	f.clipRow = -1
	if f.ShowClip {
		f.clipRow = row
		row++
	}

	reserved := 0
	if f.ShowMute || f.ShowSolo {
		reserved = 1
	}

	f.trackTop = row
	f.trackHeight = (bottom - reserved) - row
	if f.trackHeight < 1 {
		f.trackHeight = 1
	}

	f.buttonsRow = -1
	if reserved > 0 {
		f.buttonsRow = f.trackTop + f.trackHeight
	}

	f.drawClipIndicator(c)
	f.drawTrack(c)
	f.drawButtonsRow(c)
}

// drawClipIndicator paints a centered LED-style dot: dim when idle, red
// when Clipping is latched — imitating a hardware clip light rather than a
// labeled button.
func (f *Fader) drawClipIndicator(c *Canvas) {
	if !f.ShowClip {
		return
	}
	bg := c.theme.BgWidget
	for x := 0; x < f.LastW; x++ {
		c.DrawCell(f.AbsX+x, f.clipRow, " ", bg, bg)
	}
	led := c.theme.FgDisabled
	if f.Clipping {
		led = c.theme.Danger
	}
	ledX := f.AbsX + max(0, (f.LastW-1)/2)
	c.DrawText(ledX, f.clipRow, "⬤", bg, led)
}

// drawButtonsRow paints Mute and Solo as icon-only buttons sharing one row
// — Mute ("🄼") on the left, Solo ("🅂") on the right — or a single centered
// icon if only one of them is shown.
func (f *Fader) drawButtonsRow(c *Canvas) {
	if f.buttonsRow < 0 {
		return
	}
	bg := c.theme.BgWidget
	for x := 0; x < f.LastW; x++ {
		c.DrawCell(f.AbsX+x, f.buttonsRow, " ", bg, bg)
	}

	switch {
	case f.ShowMute && f.ShowSolo:
		half := f.LastW / 2
		f.drawIconButton(c, f.AbsX, half, "🄼", f.Muted, c.theme.Danger)
		f.drawIconButton(c, f.AbsX+half, f.LastW-half, "🅂", f.Soloed, c.theme.Warning)
	case f.ShowMute:
		f.drawIconButton(c, f.AbsX, f.LastW, "🄼", f.Muted, c.theme.Danger)
	case f.ShowSolo:
		f.drawIconButton(c, f.AbsX, f.LastW, "🅂", f.Soloed, c.theme.Warning)
	}
}

// drawIconButton fills a w-wide segment of the buttons row starting at x
// with icon, centered, colored activeColor when active.
func (f *Fader) drawIconButton(c *Canvas, x, w int, icon string, active bool, activeColor Color) {
	bg, fg := c.theme.BgWidget, c.theme.FgDisabled
	if active {
		bg, fg = activeColor, RGB(255, 255, 255)
	}
	for i := 0; i < w; i++ {
		c.DrawCell(x+i, f.buttonsRow, " ", bg, fg)
	}
	iconX := x + max(0, (w-1)/2)
	c.DrawText(iconX, f.buttonsRow, icon, bg, fg)
}

// drawTrack renders the tick labels, VU meter, and fader rail/handle.
func (f *Fader) drawTrack(c *Canvas) {
	tickW, meterW, gap := 0, 0, 0
	if f.ShowMeter {
		tickW, meterW, gap = 4, 2, 1
	}
	trackX := f.AbsX + tickW + meterW + gap
	trackW := f.LastW - (tickW + meterW + gap)
	if trackW < 1 {
		trackW = 1
	}

	tickRows := make(map[int]string, len(f.Ticks))
	if f.ShowMeter {
		for _, tick := range f.Ticks {
			tickRows[f.rowForPercent(tick.Percent)] = tick.Label
		}
	}

	handleRow := f.rowForPercent(f.Value)

	for i := 0; i < f.trackHeight; i++ {
		row := f.trackTop + i
		rowPercent := f.percentForRow(row)

		if f.ShowMeter {
			zone := faderZoneColor(c.theme, rowPercent)
			if rowPercent > f.Level {
				zone = zone.Darken(0.75)
			}
			for mx := 0; mx < meterW; mx++ {
				c.DrawCell(f.AbsX+tickW+mx, row, " ", zone, zone)
			}
			if label, ok := tickRows[row]; ok {
				for len([]rune(label)) < tickW-1 {
					label = " " + label
				}
				c.DrawText(f.AbsX, row, label, c.theme.BgWindow, c.theme.FgDisabled)
			}
		}

		if row == handleRow {
			handleBg := RGB(200, 200, 200)
			if f.IsFocused {
				handleBg = c.theme.BgFocused
			}
			for hx := 0; hx < trackW; hx++ {
				ch := "▬"
				if hx == trackW/2 {
					ch = "│"
				}
				c.DrawCell(trackX+hx, row, ch, handleBg, f.LabelColor)
			}
			continue
		}

		mid := trackX + trackW/2
		for tx := trackX; tx < trackX+trackW; tx++ {
			if tx == mid {
				c.DrawCell(tx, row, "│", c.theme.BgWidget, c.theme.FgDisabled)
			} else {
				c.DrawCell(tx, row, " ", c.theme.BgWidget, c.theme.BgWidget)
			}
		}
	}
}

// HandleEvent implements Widget: arrow keys nudge Value, a track click
// jumps to that position (or fires OnDoubleClick on a fast second click at
// the same spot), a drag continues updating Value the same way a click
// would — Window's mouse capture guarantees Fader keeps receiving
// EventMouseDrag even once the pointer leaves its own bounds — and clicks
// on the clip LED clear it, and clicks on the buttons row toggle Mute or
// Solo depending on which half was hit.
func (f *Fader) HandleEvent(ev Event) {
	switch ev.Type {
	case EventKey:
		switch ev.Key {
		case KeyUp, KeyRight:
			f.setValue(f.Value + 2)
		case KeyDown, KeyLeft:
			f.setValue(f.Value - 2)
		}
	case EventMouseDown:
		f.handleClick(ev)
	case EventMouseDrag:
		if f.trackHeight > 0 {
			f.setValueFromRow(ev.MouseY)
		}
	}
}

func (f *Fader) handleClick(ev Event) {
	switch {
	case f.ShowClip && ev.MouseY == f.clipRow:
		f.ClearClip()
	case ev.MouseY == f.buttonsRow && (f.ShowMute || f.ShowSolo):
		f.handleButtonsClick(ev.MouseX)
	case ev.MouseY >= f.trackTop && ev.MouseY < f.trackTop+f.trackHeight:
		if f.isDoubleClick(ev) {
			if f.OnDoubleClick != nil {
				f.OnDoubleClick()
			}
			return
		}
		f.setValueFromRow(ev.MouseY)
	}
}

// handleButtonsClick toggles Mute or Solo depending on which half of the
// buttons row mouseX falls in, mirroring drawButtonsRow's layout.
func (f *Fader) handleButtonsClick(mouseX int) {
	isMute := f.ShowMute
	if f.ShowMute && f.ShowSolo {
		isMute = mouseX < f.AbsX+f.LastW/2
	}

	if isMute {
		f.Muted = !f.Muted
		if f.OnMuteChange != nil {
			f.OnMuteChange(f.Muted)
		}
		return
	}
	f.Soloed = !f.Soloed
	if f.OnSoloChange != nil {
		f.OnSoloChange(f.Soloed)
	}
}

// isDoubleClick reports whether ev lands close enough in time and position
// to the previous click to count as a double-click, updating the tracked
// last-click state either way.
func (f *Fader) isDoubleClick(ev Event) bool {
	now := time.Now()
	rowDelta := ev.MouseY - f.lastClickRow
	if rowDelta < 0 {
		rowDelta = -rowDelta
	}
	isDouble := !f.lastClickAt.IsZero() && now.Sub(f.lastClickAt) < 400*time.Millisecond && rowDelta <= 1

	f.lastClickAt, f.lastClickRow = now, ev.MouseY
	if isDouble {
		// A third rapid click starts a fresh pair instead of chaining into
		// another double-click.
		f.lastClickAt = time.Time{}
	}
	return isDouble
}

// ShowFaderValueEditor opens a modal titled title with an input field
// pre-filled with current (formatted to one decimal place). Enter/click on
// OK parses it as a number clamped to 0-100 and calls onConfirm; Cancel or
// Esc closes without calling it. This is what Fader.OnDoubleClick is
// typically wired to.
func ShowFaderValueEditor(app *Application, title string, current float64, onConfirm func(float64)) {
	mod := NewWindow(44, 9, " "+title+" ")
	mod.AddWidget(NewLabel(2, 1, "Value (0-100):"))

	input := NewInputBox(2, 3, 30, "")
	input.Value = fmt.Sprintf("%.1f", current)
	input.CursorPos = len([]rune(input.Value))
	mod.AddWidget(input)

	errLbl := NewLabel(2, 5, "")
	mod.AddWidget(errLbl)

	confirm := func() {
		v, err := strconv.ParseFloat(strings.TrimSpace(input.Value), 64)
		if err != nil || v < 0 || v > 100 {
			errLbl.SetText("Enter a number between 0 and 100.")
			return
		}
		app.CloseModal()
		onConfirm(v)
	}

	mod.AddWidget(NewButton(2, 7, "OK", BtnSuccess, confirm))
	mod.AddWidget(NewButton(14, 7, "Cancel", BtnDefault, func() {
		app.CloseModal()
	}))

	app.SetModal(mod)
}
