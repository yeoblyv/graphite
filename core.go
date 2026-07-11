package Graphite

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// KeyCode identifies a non-printable key reported by an Event.
type KeyCode int

// Recognized non-printable key codes. Printable characters arrive via
// Event.CharCode instead, with Key left as KeyNone.
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
)

// EventType discriminates the kind of input an Event carries.
type EventType int

// Supported event kinds.
const (
	EventNone EventType = iota
	EventKey
	EventMouseDown
	// EventMouseDrag is a motion report with the left button held, deliv-
	// ered only to whichever widget was hit by the preceding EventMouseDown
	// (see Window's mouse capture), regardless of where the pointer moves.
	EventMouseDrag
	// EventMouseUp is a button release. Like EventMouseDrag, it goes to the
	// widget that captured the preceding EventMouseDown, and ends capture.
	EventMouseUp
)

// Event is a single input notification delivered to the focused widget (for
// keyboard input) or to whichever widget is hit-tested under the pointer
// (for mouse input).
type Event struct {
	Type     EventType
	Key      KeyCode
	CharCode rune
	MouseX   int
	MouseY   int
}

// Color is a 24-bit truecolor value, packed as (R<<16)|(G<<8)|B. Canvas.Render
// emits it as a truecolor ANSI SGR sequence, which every terminal capable of
// running this library already supports (rendering here already depends on
// the alternate screen buffer and SGR mouse mode, both modern-terminal-only
// features — there is no legacy 16/256-color fallback path).
type Color int32

// ColorNone means "leave the existing color unchanged" — passed to
// Canvas.DrawCell/DrawText for bg or fg to only touch the other one.
const ColorNone Color = -1

// RGB builds a Color from 8-bit red, green, and blue components.
func RGB(r, g, b uint8) Color {
	return Color(int32(r)<<16 | int32(g)<<8 | int32(b))
}

// Hex parses a "#RRGGBB" or "RRGGBB" string into a Color. It panics on a
// malformed string, since a bad hex literal is a programming error to be
// caught at development time, not a runtime condition to recover from.
func Hex(s string) Color {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		panic(fmt.Sprintf("Graphite.Hex: %q is not a 6-digit hex color", s))
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		panic(fmt.Sprintf("Graphite.Hex: %q is not a valid hex color: %v", s, err))
	}
	return Color(int32(v))
}

// Components unpacks the red, green, and blue bytes of c, for callers that
// need to inspect or recombine a color (e.g. picking a readable text color
// against an arbitrary background).
func (c Color) Components() (r, g, b uint8) {
	return uint8(c >> 16), uint8(c >> 8), uint8(c)
}

// Darken returns c scaled towards black by pct (0 keeps c unchanged, 1
// returns black). It replaces arithmetic like "subtract 10 from an ANSI
// code", which has no equivalent once colors are RGB instead of palette
// indices — e.g. a button wants a dimmer variant of its Danger color when
// unfocused.
func (c Color) Darken(pct float64) Color {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	r, g, b := c.Components()
	scale := 1 - pct
	return RGB(
		uint8(float64(r)*scale),
		uint8(float64(g)*scale),
		uint8(float64(b)*scale),
	)
}

// Theme is the color palette a Canvas renders widgets with.
type Theme struct {
	BgScreen   Color
	BgWindow   Color
	FgWindow   Color
	BgWidget   Color
	BgFocused  Color
	FgFocused  Color
	Primary    Color
	Success    Color
	Danger     Color
	Warning    Color
	Disabled   Color
	FgDisabled Color
}

// DefaultTheme returns the built-in color palette used by a new Canvas until
// Application.SetTheme overrides it: a GitHub-Dark-inspired scheme (near-
// black background, soft light-gray text, a blue accent, and green/red/amber
// semantic colors for success/danger/warning states).
func DefaultTheme() Theme {
	return Theme{
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
	}
}

// Cell is a single terminal character position: its glyph plus foreground
// and background color.
type Cell struct {
	Symbol  string
	BgColor Color
	FgColor Color

	// continuation marks the right-hand placeholder cell after a
	// double-width glyph (see Canvas.DrawText). Render skips it entirely:
	// the terminal already consumed this column when it drew the wide
	// glyph to its left.
	continuation bool
}

// NotEqual reports whether c would render differently from other.
func (c Cell) NotEqual(other Cell) bool {
	return c.Symbol != other.Symbol || c.BgColor != other.BgColor ||
		c.FgColor != other.FgColor || c.continuation != other.continuation
}

// Canvas is a double-buffered terminal grid. Widgets draw into the back
// buffer; Render diffs it against the front buffer and emits only the ANSI
// sequences needed to bring the terminal up to date.
type Canvas struct {
	width, height       int
	buffer, frontBuffer []Cell
	theme               Theme
	forceRedraw         bool
}

// NewCanvas creates an empty Canvas using DefaultTheme. Call Resize before
// drawing to it; Application does this automatically on every frame.
func NewCanvas() *Canvas { return &Canvas{theme: DefaultTheme(), forceRedraw: true} }

// Resize changes the canvas dimensions, reallocating both buffers and
// forcing a full redraw on the next Render. It is a no-op when the
// dimensions are unchanged, since terminal size is polled every frame.
func (c *Canvas) Resize(w, h int) {
	if c.width == w && c.height == h {
		return
	}
	c.width, c.height = w, h
	c.buffer = make([]Cell, w*h)
	c.frontBuffer = make([]Cell, w*h)
	for i := range c.buffer {
		c.buffer[i] = Cell{Symbol: " ", BgColor: c.theme.BgScreen, FgColor: c.theme.FgWindow}
		c.frontBuffer[i] = Cell{Symbol: "", BgColor: ColorNone, FgColor: ColorNone}
	}
	c.forceRedraw = true
}

// Clear resets every cell in the back buffer to the screen background,
// ready for the next frame's widgets to draw over it.
func (c *Canvas) Clear() {
	for i := range c.buffer {
		c.buffer[i] = Cell{Symbol: " ", BgColor: c.theme.BgScreen, FgColor: c.theme.FgWindow}
	}
}

// sanitizeGlyph guards against writing a raw control character into a
// cell's Symbol. Render emits Symbol to the real terminal byte-for-byte, so
// a widget displaying untrusted text (e.g. subprocess output streamed into
// a TextArea) could otherwise inject arbitrary ANSI/OSC escape sequences —
// title-bar spoofing, color/state resets, or worse — into the user's
// terminal. Any single rune classified as a control character is replaced
// with a visible placeholder; multi-rune strings (the box-drawing and
// block-element glyphs the widgets themselves draw) pass through unchanged.
func sanitizeGlyph(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == len(s) && unicode.IsControl(r) {
		return " "
	}
	return s
}

// DrawCell writes a single glyph at (x, y). Coordinates outside the canvas
// are silently ignored so widgets do not need to bounds-check every write.
// Passing ColorNone for bg or fg leaves that color unchanged. s is assumed
// to occupy exactly one terminal column — every direct caller in this
// package already passes single-column glyphs (box drawing, block
// elements); free-form text should go through DrawText instead, which
// accounts for double-width runes.
func (c *Canvas) DrawCell(x, y int, s string, bg, fg Color) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		idx := y*c.width + x
		c.buffer[idx].Symbol = sanitizeGlyph(s)
		c.buffer[idx].continuation = false
		if bg != ColorNone {
			c.buffer[idx].BgColor = bg
		}
		if fg != ColorNone {
			c.buffer[idx].FgColor = fg
		}
	}
}

// drawContinuation marks (x, y) as the right half of the double-width glyph
// drawn immediately to its left, so Render skips it and nothing else draws
// into a column the terminal itself already consumed.
func (c *Canvas) drawContinuation(x, y int, bg, fg Color) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		idx := y*c.width + x
		c.buffer[idx] = Cell{Symbol: "", BgColor: bg, FgColor: fg, continuation: true}
	}
}

// DrawText writes text starting at (x, y), advancing one column for a
// normal-width rune and two for a double-width one (e.g. CJK), so
// callers don't need to compute display width themselves.
func (c *Canvas) DrawText(x, y int, text string, bg, fg Color) {
	cursor := x
	for _, r := range text {
		w := runeWidth(r)
		c.DrawCell(cursor, y, string(r), bg, fg)
		if w >= 2 {
			c.drawContinuation(cursor+1, y, bg, fg)
		}
		if w < 1 {
			w = 1
		}
		cursor += w
	}
}

// Render diffs the back buffer against what was last drawn to the terminal
// and writes only the changed cells, minimizing the bytes sent per frame.
// A resize or the first frame forces every cell to be rewritten.
func (c *Canvas) Render() {
	var out strings.Builder
	if c.forceRedraw {
		out.WriteString("\033[2J\033[H")
	}
	cBg, cFg, lX, lY := ColorNone, ColorNone, -2, -2
	for y := 0; y < c.height; y++ {
		for x := 0; x < c.width; x++ {
			idx := y*c.width + x
			if c.buffer[idx].continuation {
				// The terminal already consumed this column rendering the
				// wide glyph to its left; emitting anything here (even a
				// cursor move) would overwrite half of that glyph.
				continue
			}
			if c.buffer[idx].NotEqual(c.frontBuffer[idx]) || c.forceRedraw {
				// Only emit a cursor-move sequence when the previous write
				// didn't already leave the cursor at this position.
				if x != lX+1 || y != lY {
					fmt.Fprintf(&out, "\033[%d;%dH", y+1, x+1)
				}
				if c.buffer[idx].BgColor != cBg || c.buffer[idx].FgColor != cFg {
					fr, fg, fb := c.buffer[idx].FgColor.Components()
					br, bg, bb := c.buffer[idx].BgColor.Components()
					fmt.Fprintf(&out, "\033[38;2;%d;%d;%d;48;2;%d;%d;%dm", fr, fg, fb, br, bg, bb)
					cBg, cFg = c.buffer[idx].BgColor, c.buffer[idx].FgColor
				}
				out.WriteString(c.buffer[idx].Symbol)
				c.frontBuffer[idx] = c.buffer[idx]
				lX, lY = x, y
			}
		}
	}
	if out.Len() > 0 {
		fmt.Print(out.String() + "\033[0m")
	}
	c.forceRedraw = false
}

// GetCellBg returns the background color at (x, y), or the theme's screen
// background for out-of-bounds coordinates. Widgets use this to blend
// decorations (e.g. window shadows) with whatever is already underneath.
func (c *Canvas) GetCellBg(x, y int) Color {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		return c.buffer[y*c.width+x].BgColor
	}
	return c.theme.BgScreen
}

// Width returns the canvas width in columns.
func (c *Canvas) Width() int { return c.width }

// Height returns the canvas height in rows.
func (c *Canvas) Height() int { return c.height }

// Widget is the contract every UI element implements to participate in
// layout, drawing, focus, and event routing. Custom widgets are built by
// embedding BaseWidget and overriding the methods that need non-default
// behavior.
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

// BaseWidget implements the mechanical parts of Widget (layout resolution,
// focus/enabled/visible state) so concrete widgets only need to implement
// drawing and event handling.
type BaseWidget struct {
	X, Y, Width, Height                      int
	PctX, PctY, PctW, PctH                   int
	IsFocusable, IsFocused, Enabled, Visible bool
	AbsX, AbsY, LastW, LastH                 int
}

// NewBaseWidget creates a BaseWidget at the given fixed position and size,
// enabled and visible by default. Percent-based layout, if wanted, is added
// afterwards via SetPercentLayout.
func NewBaseWidget(x, y, w, h int) BaseWidget {
	return BaseWidget{X: x, Y: y, Width: w, Height: h, Enabled: true, Visible: true}
}

// SetPercentLayout switches the widget to percentage-based positioning and
// sizing relative to its parent's content area. A value of 0 for any axis
// keeps the corresponding fixed X/Y/Width/Height instead.
func (b *BaseWidget) SetPercentLayout(x, y, w, h int) {
	b.PctX, b.PctY, b.PctW, b.PctH = x, y, w, h
}

// DrawRelative resolves the widget's absolute position and size (AbsX, AbsY,
// LastW, LastH) against the parent's content origin (offX, offY) and
// dimensions (pW, pH). It performs no drawing itself; concrete widgets call
// this first, then draw using the resolved fields.
func (b *BaseWidget) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	relX := b.X
	if b.PctX > 0 {
		relX = (pW * b.PctX) / 100
	}
	relY := b.Y
	if b.PctY > 0 {
		relY = (pH * b.PctY) / 100
	}

	b.AbsX = offX + relX
	// A negative X/Y is an offset from the parent's far edge rather than
	// its origin, e.g. -1 means "one column from the right".
	if relX < 0 {
		b.AbsX = offX + pW + relX
	}

	b.AbsY = offY + relY
	if relY < 0 {
		b.AbsY = offY + pH + relY
	}

	b.LastW = b.Width
	if b.PctW > 0 {
		b.LastW = (pW * b.PctW) / 100
	} else if b.Width <= 0 {
		// A zero or negative fixed Width means "stretch to fill the
		// remaining parent width", with the value acting as a right margin.
		b.LastW = pW - (b.AbsX - offX) + b.Width
	}

	b.LastH = b.Height
	if b.PctH > 0 {
		b.LastH = (pH * b.PctH) / 100
	} else if b.Height <= 0 {
		b.LastH = pH - (b.AbsY - offY) + b.Height
	}
}

// DrawOverlay draws content that must appear above sibling widgets (e.g.
// dropdown popups). The default implementation draws nothing.
func (b *BaseWidget) DrawOverlay(c *Canvas, oX, oY, pW, pH int) {}

// HandleEvent processes an input event routed to this widget. The default
// implementation ignores all events.
func (b *BaseWidget) HandleEvent(ev Event) {}

// HitTest reports whether the screen coordinates (mx, my) fall within the
// widget's last resolved bounds.
func (b *BaseWidget) HitTest(mx, my int) bool {
	return mx >= b.AbsX && mx < b.AbsX+b.LastW && my >= b.AbsY && my < b.AbsY+b.LastH
}

// SetFocus sets whether this widget currently has keyboard focus.
func (b *BaseWidget) SetFocus(f bool) { b.IsFocused = f }

// HasFocus reports whether this widget currently has keyboard focus.
func (b *BaseWidget) HasFocus() bool { return b.IsFocused }

// CanFocus reports whether this widget is eligible to receive keyboard
// focus: it must be focusable by design, enabled, and visible.
func (b *BaseWidget) CanFocus() bool { return b.IsFocusable && b.Enabled && b.Visible }

// GetChildren returns nested widgets so Window can flatten containers (e.g.
// Panel) when building the focus order and hit-testing the tree. Leaf
// widgets have none.
func (b *BaseWidget) GetChildren() []Widget { return nil }

// SetVisible sets whether this widget is drawn and eligible for focus/hit
// testing.
func (b *BaseWidget) SetVisible(v bool) { b.Visible = v }

// IsVisible reports whether this widget is currently visible.
func (b *BaseWidget) IsVisible() bool { return b.Visible }

// SetEnabled sets whether this widget accepts input. Window routes no
// events to a disabled widget, regardless of what HandleEvent does.
func (b *BaseWidget) SetEnabled(e bool) { b.Enabled = e }

// IsEnabled reports whether this widget currently accepts input.
func (b *BaseWidget) IsEnabled() bool { return b.Enabled }

// SetPosition sets the widget's fixed X/Y offset used by DrawRelative.
func (b *BaseWidget) SetPosition(x, y int) { b.X, b.Y = x, y }

// GetFixedH returns the widget's configured height, or 1 if none was set.
// Containers that stack children vertically by their natural size (e.g.
// Flex) use this for children not given a proportional weight.
func (b *BaseWidget) GetFixedH() int {
	if b.Height > 0 {
		return b.Height
	}
	return 1
}

// GetFixedW returns the widget's configured width, or 1 if none was set.
// Symmetric with GetFixedH, for containers (e.g. Flex) laying out children
// horizontally by their natural size.
func (b *BaseWidget) GetFixedW() int {
	if b.Width > 0 {
		return b.Width
	}
	return 1
}
