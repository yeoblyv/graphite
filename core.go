package graphite

import (
	"fmt"
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

// Theme is the color palette a Canvas renders widgets with. Colors are raw
// ANSI SGR codes (e.g. 30-47), matching what Canvas.Render writes directly
// to the terminal.
type Theme struct {
	BgScreen   int
	BgWindow   int
	FgWindow   int
	BgWidget   int
	BgFocused  int
	FgFocused  int
	Primary    int
	Success    int
	Danger     int
	Warning    int
	Disabled   int
	FgDisabled int
}

// DefaultTheme returns the built-in color palette used by a new Canvas until
// Application.SetTheme overrides it.
func DefaultTheme() Theme {
	return Theme{
		BgScreen: 44, BgWindow: 40, FgWindow: 37, BgWidget: 100,
		BgFocused: 47, FgFocused: 30, Primary: 46, Success: 42,
		Danger: 41, Warning: 43, Disabled: 40, FgDisabled: 90,
	}
}

// Cell is a single terminal character position: its glyph plus foreground
// and background color.
type Cell struct {
	Symbol  string
	BgColor int
	FgColor int
}

// NotEqual reports whether c would render differently from other.
func (c Cell) NotEqual(other Cell) bool {
	return c.Symbol != other.Symbol || c.BgColor != other.BgColor || c.FgColor != other.FgColor
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
		c.buffer[i] = Cell{Symbol: " ", BgColor: c.theme.BgScreen, FgColor: 37}
		c.frontBuffer[i] = Cell{Symbol: "", BgColor: -1, FgColor: -1}
	}
	c.forceRedraw = true
}

// Clear resets every cell in the back buffer to the screen background,
// ready for the next frame's widgets to draw over it.
func (c *Canvas) Clear() {
	for i := range c.buffer {
		c.buffer[i] = Cell{Symbol: " ", BgColor: c.theme.BgScreen, FgColor: 37}
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
// Passing -1 for bg or fg leaves that color unchanged.
func (c *Canvas) DrawCell(x, y int, s string, bg, fg int) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		idx := y*c.width + x
		c.buffer[idx].Symbol = sanitizeGlyph(s)
		if bg != -1 {
			c.buffer[idx].BgColor = bg
		}
		if fg != -1 {
			c.buffer[idx].FgColor = fg
		}
	}
}

// DrawText writes text starting at (x, y), one rune per cell.
func (c *Canvas) DrawText(x, y int, text string, bg, fg int) {
	for i, r := range []rune(text) {
		c.DrawCell(x+i, y, string(r), bg, fg)
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
	cBg, cFg, lX, lY := -1, -1, -2, -2
	for y := 0; y < c.height; y++ {
		for x := 0; x < c.width; x++ {
			idx := y*c.width + x
			if c.buffer[idx].NotEqual(c.frontBuffer[idx]) || c.forceRedraw {
				// Only emit a cursor-move sequence when the previous write
				// didn't already leave the cursor at this position.
				if x != lX+1 || y != lY {
					out.WriteString(fmt.Sprintf("\033[%d;%dH", y+1, x+1))
				}
				if c.buffer[idx].BgColor != cBg || c.buffer[idx].FgColor != cFg {
					out.WriteString(fmt.Sprintf("\033[%d;%dm", c.buffer[idx].FgColor, c.buffer[idx].BgColor))
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
func (c *Canvas) GetCellBg(x, y int) int {
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
// Containers that stack children vertically (e.g. Panel) use this to lay
// out widgets whose Height is left at its zero value.
func (b *BaseWidget) GetFixedH() int {
	if b.Height > 0 {
		return b.Height
	}
	return 1
}
