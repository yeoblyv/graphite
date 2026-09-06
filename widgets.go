package Graphite

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// -------------------------------------------------------------------------
// Label
// -------------------------------------------------------------------------

// Label draws a single line of static text.
type Label struct {
	BaseWidget
	Text string
}

// NewLabel creates a Label at (x, y) sized to fit text.
func NewLabel(x, y int, text string) *Label {
	w := len([]rune(text))
	base := NewBaseWidget(x, y, w, 1)
	return &Label{BaseWidget: base, Text: text}
}

// SetText replaces the label's text and resizes it to fit the new content.
func (l *Label) SetText(text string) {
	l.Text = text
	l.Width = len([]rune(text))
}

// DrawRelative implements Widget.
func (l *Label) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	l.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	lines := c.DrawTextWrapped(l.AbsX, l.AbsY, l.LastW, l.Text, c.theme.BgWindow, c.theme.FgWindow)
	l.LastH = lines
	if l.LastH < 1 {
		l.LastH = 1
	}
}

// -------------------------------------------------------------------------
// Spinner
// -------------------------------------------------------------------------

// Spinner draws an animated braille-style busy indicator next to a label.
type Spinner struct {
	BaseWidget
	Label string
}

// NewSpinner creates a Spinner at (x, y) sized to fit label.
func NewSpinner(x, y int, label string) *Spinner {
	w := len([]rune(label)) + 3
	base := NewBaseWidget(x, y, w, 1)
	return &Spinner{BaseWidget: base, Label: label}
}

// getFrame picks the animation frame for the current wall-clock time, so all
// spinners on screen stay in sync without needing a shared ticker.
func (s *Spinner) getFrame() string {
	frames := []string{"▙", "▛", "▜", "▟"}
	ms := time.Now().UnixMilli()
	return frames[(ms/150)%int64(len(frames))]
}

// DrawRelative implements Widget.
func (s *Spinner) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	s.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	c.DrawTextBounded(s.AbsX, s.AbsY, s.LastW, s.getFrame()+" "+s.Label, c.theme.BgWindow, c.theme.Primary)
}

// -------------------------------------------------------------------------
// Checkbox
// -------------------------------------------------------------------------

// Checkbox is a focusable boolean toggle with a label.
type Checkbox struct {
	BaseWidget
	Label    string
	Checked  bool
	OnChange func(checked bool)
}

// NewCheckbox creates a Checkbox at (x, y) with the given initial state.
func NewCheckbox(x, y int, label string, checked bool) *Checkbox {
	base := NewBaseWidget(x, y, len([]rune(label))+4, 1)
	base.IsFocusable = true
	return &Checkbox{BaseWidget: base, Label: label, Checked: checked}
}

// DrawRelative implements Widget.
func (cb *Checkbox) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	cb.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	bg, fg := c.theme.BgWindow, c.theme.FgWindow
	if cb.IsFocused {
		bg, fg = c.theme.BgFocused, c.theme.FgFocused
	}
	box := "[ ] "
	if cb.Checked {
		box = "[■] "
	}
	c.DrawTextBounded(cb.AbsX, cb.AbsY, cb.LastW, box+cb.Label, bg, fg)
}

// HandleEvent implements Widget: Space, Enter, and mouse clicks all toggle
// the checkbox.
func (cb *Checkbox) HandleEvent(ev Event) {
	if (ev.Type == EventKey && (ev.Key == KeySpace || ev.Key == KeyEnter)) || ev.Type == EventMouseDown {
		cb.Checked = !cb.Checked
		if cb.OnChange != nil {
			cb.OnChange(cb.Checked)
		}
	}
}

// -------------------------------------------------------------------------
// Button
// -------------------------------------------------------------------------

// ButtonStyle selects a Button's accent color.
type ButtonStyle int

// Supported button styles.
const (
	BtnDefault ButtonStyle = iota
	BtnSuccess
	BtnDanger
	BtnWarning
	BtnInfo
)

// Button is a focusable, clickable action with a text label.
type Button struct {
	BaseWidget
	Text    string
	Style   ButtonStyle
	OnClick func()
}

// NewButton creates a Button at (x, y) that calls onClick when activated.
func NewButton(x, y int, text string, style ButtonStyle, onClick func()) *Button {
	base := NewBaseWidget(x, y, len([]rune(text))+4, 1)
	base.IsFocusable = true
	return &Button{BaseWidget: base, Text: text, Style: style, OnClick: onClick}
}

// DrawRelative implements Widget.
func (b *Button) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	b.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	// Every branch below sets bg explicitly, so it starts undefined; fg's
	// initial value is the one actually used in the enabled-and-unfocused
	// case.
	var bg Color
	fg := c.theme.FgWindow
	if !b.Enabled {
		bg, fg = c.theme.BgWidget, c.theme.FgDisabled
	} else if b.IsFocused {
		fg = c.theme.FgFocused
		switch b.Style {
		case BtnSuccess:
			bg = c.theme.Success
		case BtnDanger:
			bg = c.theme.Danger
		default:
			bg = c.theme.BgFocused
		}
	} else {
		if b.Style == BtnDanger {
			bg = c.theme.Danger.Darken(0.3)
		} else {
			bg = c.theme.BgWidget
		}
	}
	c.DrawTextBounded(b.AbsX, b.AbsY, b.LastW, "[ "+b.Text+" ]", bg, fg)
}

// HandleEvent implements Widget: Enter and mouse clicks both activate
// OnClick. Window.HandleEvent already withholds events from a disabled
// button, so no enabled check is needed here.
func (b *Button) HandleEvent(ev Event) {
	if (ev.Type == EventKey && ev.Key == KeyEnter) || ev.Type == EventMouseDown {
		if b.OnClick != nil {
			b.OnClick()
		}
	}
}

// -------------------------------------------------------------------------
// InputBox
// -------------------------------------------------------------------------

// InputBox is a single-line, focusable text field with a fixed label,
// horizontal scrolling, and a visible text cursor.
type InputBox struct {
	BaseWidget
	Label     string
	Value     string
	CursorPos int
	OnSubmit  func(string)
}

// NewInputBox creates an InputBox at (x, y) with the given width and label.
func NewInputBox(x, y, w int, label string) *InputBox {
	base := NewBaseWidget(x, y, w, 1)
	base.IsFocusable = true
	return &InputBox{BaseWidget: base, Label: label, CursorPos: 0}
}

// DrawRelative implements Widget.
func (ib *InputBox) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	ib.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	lblLen := len([]rune(ib.Label))
	c.DrawText(ib.AbsX, ib.AbsY, ib.Label, c.theme.BgWindow, c.theme.FgWindow)

	inX := ib.AbsX + lblLen
	inW := ib.LastW - lblLen
	if inW < 3 {
		return
	}

	c.DrawCell(inX, ib.AbsY, "[", c.theme.BgWindow, c.theme.FgWindow)
	c.DrawCell(inX+inW-1, ib.AbsY, "]", c.theme.BgWindow, c.theme.FgWindow)

	bg, fg := c.theme.BgWidget, c.theme.FgWindow
	if ib.IsFocused {
		bg, fg = RGB(45, 53, 62), c.theme.FgWindow
	}

	for i := 1; i < inW-1; i++ {
		c.DrawCell(inX+i, ib.AbsY, " ", bg, fg)
	}

	runes := []rune(ib.Value)
	maxVis := inW - 2

	// Scroll just enough to keep the cursor inside the visible field.
	scroll := 0
	if ib.CursorPos >= maxVis {
		scroll = ib.CursorPos - maxVis + 1
	}

	startIdx := scroll
	if startIdx > len(runes) {
		startIdx = len(runes)
	}
	endIdx := scroll + maxVis
	if endIdx > len(runes) {
		endIdx = len(runes)
	}

	visRunes := runes[startIdx:endIdx]
	c.DrawText(inX+1, ib.AbsY, string(visRunes), bg, fg)

	if ib.IsFocused {
		cursorScreenX := inX + 1 + ib.CursorPos - scroll
		if cursorScreenX >= inX+1 && cursorScreenX < inX+inW-1 {
			charUnderCursor := " "
			if ib.CursorPos < len(runes) {
				charUnderCursor = string(runes[ib.CursorPos])
			}
			// White-on-black cursor block, independent of the theme.
			c.DrawCell(cursorScreenX, ib.AbsY, charUnderCursor, RGB(255, 255, 255), RGB(0, 0, 0))
		}
	}
}

// HandleEvent implements Widget: arrow keys move the cursor, Backspace and
// Delete remove the rune behind/under it, and any other printable character
// is inserted at the cursor. A mouse click moves the cursor to the clicked
// column, accounting for horizontal scroll.
func (ib *InputBox) HandleEvent(ev Event) {
	runes := []rune(ib.Value)
	if ev.Type == EventKey {
		if ev.Key == KeyLeft && ib.CursorPos > 0 {
			ib.CursorPos--
		} else if ev.Key == KeyRight && ib.CursorPos < len(runes) {
			ib.CursorPos++
		} else if ev.Key == KeyBackspace && ib.CursorPos > 0 {
			ib.Value = string(append(runes[:ib.CursorPos-1], runes[ib.CursorPos:]...))
			ib.CursorPos--
		} else if ev.Key == KeyDelete && ib.CursorPos < len(runes) {
			ib.Value = string(append(runes[:ib.CursorPos], runes[ib.CursorPos+1:]...))
		} else if ev.Key == KeyCtrlC {
			clipboard.WriteAll(ib.Value)
		} else if ev.Key == KeyCtrlX {
			clipboard.WriteAll(ib.Value)
			ib.Value = ""
			ib.CursorPos = 0
		} else if ev.Key == KeyCtrlV {
			text, err := clipboard.ReadAll()
			if err == nil {
				// Remove newlines since InputBox is single-line
				text = strings.ReplaceAll(text, "\n", "")
				text = strings.ReplaceAll(text, "\r", "")
				head := append([]rune{}, runes[:ib.CursorPos]...)
				tail := append([]rune{}, runes[ib.CursorPos:]...)
				pasted := []rune(text)
				ib.Value = string(append(append(head, pasted...), tail...))
				ib.CursorPos += len(pasted)
			}
		} else if ev.Key == KeyEnter {
			if ib.OnSubmit != nil {
				ib.OnSubmit(ib.Value)
			}
		} else if ev.CharCode >= 32 {
			head := append([]rune{}, runes[:ib.CursorPos]...)
			tail := append([]rune{}, runes[ib.CursorPos:]...)
			head = append(head, ev.CharCode)
			ib.Value = string(append(head, tail...))
			ib.CursorPos++
		}
	} else if ev.Type == EventMouseDown {
		relX := ev.MouseX - (ib.AbsX + len([]rune(ib.Label)) + 1)
		if relX >= 0 {
			maxVis := ib.LastW - len([]rune(ib.Label)) - 2
			scroll := 0
			if ib.CursorPos >= maxVis {
				scroll = ib.CursorPos - maxVis + 1
			}

			newPos := scroll + relX
			if newPos > len(runes) {
				ib.CursorPos = len(runes)
			} else {
				ib.CursorPos = newPos
			}
		}
	}
}

// -------------------------------------------------------------------------
// ProgressBar
// -------------------------------------------------------------------------

// ProgressBar draws a labeled, filled bar showing completion from 0 to 100.
type ProgressBar struct {
	BaseWidget
	Label    string
	Progress float32
}

// NewProgressBar creates a ProgressBar at (x, y) starting at 0% progress.
func NewProgressBar(x, y, w int, label string) *ProgressBar {
	base := NewBaseWidget(x, y, w, 1)
	return &ProgressBar{BaseWidget: base, Label: label, Progress: 0.0}
}

// SetProgress sets the completion percentage, clamped to [0, 100].
func (pb *ProgressBar) SetProgress(p float32) {
	pb.Progress = float32(math.Min(math.Max(float64(p), 0), 100))
}

// DrawRelative implements Widget.
func (pb *ProgressBar) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	pb.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	c.DrawText(pb.AbsX, pb.AbsY, pb.Label, c.theme.BgWindow, c.theme.FgWindow)
	lblLen := len([]rune(pb.Label))
	bX, bW := pb.AbsX+lblLen+1, pb.LastW-lblLen-9
	if bW < 5 {
		return
	}
	c.DrawCell(bX, pb.AbsY, "[", c.theme.BgWindow, c.theme.FgWindow)
	c.DrawCell(bX+bW-1, pb.AbsY, "]", c.theme.BgWindow, c.theme.FgWindow)

	fW := bW - 2
	filled := int((pb.Progress * float32(fW)) / 100.0)
	for i := 0; i < fW; i++ {
		if i < filled {
			c.DrawCell(bX+1+i, pb.AbsY, "█", c.theme.BgWindow, c.theme.Primary)
		} else {
			c.DrawCell(bX+1+i, pb.AbsY, "░", c.theme.BgWindow, c.theme.Disabled)
		}
	}
	c.DrawText(bX+bW, pb.AbsY, fmt.Sprintf(" %.1f%%", pb.Progress), c.theme.BgWindow, c.theme.FgWindow)
}

// -------------------------------------------------------------------------
// Panel — layout container for responsive columns/blocks
// -------------------------------------------------------------------------

// Panel groups child widgets under a shared position and size (typically
// percentage-based) without being focusable itself; only its children are.
type Panel struct {
	BaseWidget
	Children []Widget
	FocusIdx int
}

// NewPanel creates an empty Panel at (x, y) with the given size.
func NewPanel(x, y, w, h int) *Panel {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = false
	return &Panel{BaseWidget: base, Children: make([]Widget, 0)}
}

// AddWidget appends a child widget to the panel.
func (p *Panel) AddWidget(w Widget) {
	p.Children = append(p.Children, w)
}

// GetChildren implements Widget, letting Window descend into the panel when
// building the focus order and hit-testing the tree.
func (p *Panel) GetChildren() []Widget {
	return p.Children
}

// DrawRelative implements Widget.
func (p *Panel) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	p.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	for _, child := range p.Children {
		if child.IsVisible() {
			child.DrawRelative(c, p.AbsX, p.AbsY, p.LastW, p.LastH)
		}
	}
}

// DrawOverlay implements Widget.
func (p *Panel) DrawOverlay(c *Canvas, offX, offY, pW, pH int) {
	for _, child := range p.Children {
		if child.IsVisible() {
			child.DrawOverlay(c, p.AbsX, p.AbsY, p.LastW, p.LastH)
		}
	}
}

// -------------------------------------------------------------------------
// MenuStrip
// -------------------------------------------------------------------------

type MenuItem struct {
	Label  string
	Action func()
}

type MenuCategory struct {
	Label string
	Items []MenuItem
}

// MenuStrip is a top-level horizontal bar containing clickable categories
// that open dropdowns.
type MenuStrip struct {
	BaseWidget
	Categories []MenuCategory
	OpenIdx    int
	// BgColor overrides the strip's (and its open dropdown's) background;
	// ColorNone (the default set by NewMenuStrip) uses the theme's
	// BgWidget/BgWindow instead, matching the strip's original
	// appearance. FgColor overrides the text color; ColorNone auto-picks
	// black or white for contrast against BgColor (via Color.ContrastText)
	// once BgColor is itself set, or falls back to the theme's FgWindow
	// when neither is set.
	BgColor Color
	FgColor Color
}

// NewMenuStrip creates a MenuStrip spanning the full width of whatever
// contains it, with BgColor/FgColor left at their default (ColorNone,
// meaning "use the theme").
func NewMenuStrip(categories []MenuCategory) *MenuStrip {
	base := NewBaseWidget(0, 0, 0, 1)
	base.SetPercentLayout(0, 0, 100, 0)
	base.IsFocusable = true
	return &MenuStrip{BaseWidget: base, Categories: categories, OpenIdx: -1, BgColor: ColorNone, FgColor: ColorNone}
}

// resolveColors returns (bar bg, bar fg, open-category bg, open-category
// fg, dropdown bg, dropdown fg). With BgColor unset, this reproduces the
// strip's original theme-driven appearance exactly (bar reads BgWidget/
// FgWindow, the open category highlights with Primary/BgWindow, the
// dropdown reads BgWindow/FgWindow) — every existing MenuStrip is
// unaffected by this method's addition. With BgColor set, the whole strip
// and its dropdown share one flat color (the open category darkened
// slightly to still show which one is open), with FgColor or an
// auto-computed contrast color for all of the text.
func (m *MenuStrip) resolveColors(theme Theme) (barBg, barFg, openBg, openFg, dropBg, dropFg Color) {
	if m.BgColor == ColorNone {
		return theme.BgWidget, theme.FgWindow, theme.Primary, theme.BgWindow, theme.BgWindow, theme.FgWindow
	}
	fg := m.FgColor
	if fg == ColorNone {
		fg = m.BgColor.ContrastText()
	}
	return m.BgColor, fg, m.BgColor.Darken(0.2), fg, m.BgColor, fg
}

// HitTest overrides BaseWidget.HitTest to capture all clicks while a menu is open.
func (m *MenuStrip) HitTest(mx, my int) bool {
	if m.OpenIdx >= 0 {
		return true
	}
	return m.BaseWidget.HitTest(mx, my)
}

// DrawRelative implements Widget.
func (m *MenuStrip) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	m.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	bg, fg, openBg, openFg, _, _ := m.resolveColors(c.theme)
	for i := 0; i < m.LastW; i++ {
		c.DrawCell(m.AbsX+i, m.AbsY, " ", bg, fg)
	}

	cursorX := m.AbsX + 1
	for i, cat := range m.Categories {
		lbl := " " + cat.Label + " "
		itemBg, itemFg := bg, fg
		if i == m.OpenIdx {
			itemBg, itemFg = openBg, openFg
		}
		c.DrawText(cursorX, m.AbsY, lbl, itemBg, itemFg)
		cursorX += len([]rune(lbl))
	}
}

// DrawOverlay implements Widget.
func (m *MenuStrip) DrawOverlay(c *Canvas, offX, offY, pW, pH int) {
	if m.OpenIdx < 0 || m.OpenIdx >= len(m.Categories) {
		return
	}

	cat := m.Categories[m.OpenIdx]

	cursorX := m.AbsX + 1
	for i := 0; i < m.OpenIdx; i++ {
		cursorX += len([]rune(" " + m.Categories[i].Label + " "))
	}

	dropX := cursorX
	dropY := m.AbsY + 1
	dropW := 15
	for _, item := range cat.Items {
		w := len([]rune(item.Label)) + 4
		if w > dropW {
			dropW = w
		}
	}
	dropH := len(cat.Items) + 2

	for i := 1; i <= dropW; i++ {
		c.DrawCell(dropX+i, dropY+dropH-1, "░", c.GetCellBg(dropX+i, dropY+dropH-1), c.theme.Disabled)
	}
	for i := 0; i < dropH-1; i++ {
		c.DrawCell(dropX+dropW, dropY+i+1, "░", c.GetCellBg(dropX+dropW, dropY+i+1), c.theme.Disabled)
	}

	_, _, _, _, bg, fg := m.resolveColors(c.theme)
	for iy := 0; iy < dropH-1; iy++ {
		for ix := 0; ix < dropW; ix++ {
			c.DrawCell(dropX+ix, dropY+iy, " ", bg, fg)
		}
	}

	for i, item := range cat.Items {
		c.DrawText(dropX+2, dropY+1+i, item.Label, bg, fg)
	}
}

// HandleEvent implements Widget: clicking a category toggles its dropdown
// open/closed, and clicking an item in an open dropdown runs its Action
// and closes the dropdown.
func (m *MenuStrip) HandleEvent(ev Event) {
	if ev.Type == EventMouseDown {
		if ev.MouseY == m.AbsY {
			cursorX := m.AbsX + 1
			for i, cat := range m.Categories {
				lblLen := len([]rune(" " + cat.Label + " "))
				if ev.MouseX >= cursorX && ev.MouseX < cursorX+lblLen {
					if m.OpenIdx == i {
						m.OpenIdx = -1
					} else {
						m.OpenIdx = i
					}
					return
				}
				cursorX += lblLen
			}
			m.OpenIdx = -1
		} else if m.OpenIdx >= 0 {
			cat := m.Categories[m.OpenIdx]

			cursorX := m.AbsX + 1
			for i := 0; i < m.OpenIdx; i++ {
				cursorX += len([]rune(" " + m.Categories[i].Label + " "))
			}

			dropX := cursorX
			dropY := m.AbsY + 1
			dropW := 15
			for _, item := range cat.Items {
				w := len([]rune(item.Label)) + 4
				if w > dropW {
					dropW = w
				}
			}
			dropH := len(cat.Items)

			if ev.MouseX >= dropX && ev.MouseX < dropX+dropW && ev.MouseY > dropY && ev.MouseY <= dropY+dropH {
				itemIdx := ev.MouseY - dropY - 1
				if itemIdx >= 0 && itemIdx < len(cat.Items) {
					if cat.Items[itemIdx].Action != nil {
						cat.Items[itemIdx].Action()
					}
				}
			}
			m.OpenIdx = -1
		}
	}
}
