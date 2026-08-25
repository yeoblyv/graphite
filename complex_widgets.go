package Graphite

import (
	"math"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// -------------------------------------------------------------------------
// TextArea (Multiline text with scrolling and full cursor support)
// -------------------------------------------------------------------------

// TextLine is one wrapped or newline-delimited line of a TextArea's text,
// with Start recording its offset (in runes) into the full text so cursor
// positions can be mapped back and forth between line-local and absolute
// coordinates.
type TextLine struct {
	Start int
	Runes []rune
}

// TextArea is a focusable, scrollable, multi-line text editor with word
// wrap and a visible cursor.
type TextArea struct {
	BaseWidget
	Text      string
	Scroll    int
	CursorPos int // Absolute rune index into Text, not a line-local offset.
}

// NewTextArea creates an empty TextArea at (x, y) with the given size.
func NewTextArea(x, y, w, h int) *TextArea {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = true
	return &TextArea{BaseWidget: base}
}

// SetText replaces the text and resets scroll and cursor to the start.
func (ta *TextArea) SetText(text string) {
	ta.Text = text
	ta.Scroll = 0
	ta.CursorPos = 0
}

// buildLines splits Text into display lines, wrapping at the widget's width
// and breaking on '\n', so DrawRelative and cursor navigation can both work
// in terms of visual lines instead of the raw rune stream.
func (ta *TextArea) buildLines() []TextLine {
	runes := []rune(ta.Text)
	mW := ta.LastW - 1
	if mW < 1 {
		mW = 1
	}

	var lines []TextLine
	var currentLine []rune
	startIdx := 0

	for i, r := range runes {
		if r == '\n' {
			lines = append(lines, TextLine{startIdx, currentLine})
			currentLine = nil
			startIdx = i + 1
		} else {
			currentLine = append(currentLine, r)
			if len(currentLine) >= mW {
				lines = append(lines, TextLine{startIdx, currentLine})
				currentLine = nil
				startIdx = i + 1
			}
		}
	}
	lines = append(lines, TextLine{startIdx, currentLine})
	return lines
}

// DrawRelative implements Widget.
func (ta *TextArea) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	ta.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	bg, fg := c.theme.BgWidget, c.theme.FgWindow
	if ta.IsFocused {
		bg, fg = RGB(45, 53, 62), c.theme.FgFocused
	}

	for iy := 0; iy < ta.LastH; iy++ {
		for ix := 0; ix < ta.LastW; ix++ {
			c.DrawCell(ta.AbsX+ix, ta.AbsY+iy, " ", bg, fg)
		}
	}

	runes := []rune(ta.Text)
	if ta.CursorPos < 0 {
		ta.CursorPos = 0
	}
	if ta.CursorPos > len(runes) {
		ta.CursorPos = len(runes)
	}

	lines := ta.buildLines()

	visY, visX := 0, 0
	if ta.IsFocused {
		for i, l := range lines {
			if ta.CursorPos >= l.Start && ta.CursorPos <= l.Start+len(l.Runes) {
				// When the cursor sits exactly on a wrap boundary, prefer
				// attributing it to the start of the next line rather than
				// the end of this one.
				if i < len(lines)-1 && ta.CursorPos == lines[i+1].Start {
					continue
				}
				visY, visX = i, ta.CursorPos-l.Start
				break
			}
		}
		if visY < ta.Scroll {
			ta.Scroll = visY
		}
		if visY >= ta.Scroll+ta.LastH {
			ta.Scroll = visY - ta.LastH + 1
		}
	}

	maxScroll := int(math.Max(0, float64(len(lines)-ta.LastH)))
	if ta.Scroll > maxScroll {
		ta.Scroll = maxScroll
	}

	for i := 0; i < ta.LastH && (i+ta.Scroll) < len(lines); i++ {
		line := lines[i+ta.Scroll]
		for j, r := range line.Runes {
			c.DrawCell(ta.AbsX+j, ta.AbsY+i, string(r), bg, fg)
		}
	}

	if ta.IsFocused {
		screenY := ta.AbsY + visY - ta.Scroll
		if screenY >= ta.AbsY && screenY < ta.AbsY+ta.LastH {
			screenX := ta.AbsX + visX
			if screenX >= ta.AbsX && screenX < ta.AbsX+ta.LastW {
				charUnderCursor := " "
				if ta.CursorPos < len(runes) && runes[ta.CursorPos] != '\n' {
					charUnderCursor = string(runes[ta.CursorPos])
				}
				// White-on-black cursor block, independent of the theme.
				c.DrawCell(screenX, screenY, charUnderCursor, RGB(255, 255, 255), RGB(0, 0, 0))
			}
		}
	}

	if len(lines) > ta.LastH {
		sH := int(math.Max(1, float64((ta.LastH*ta.LastH)/len(lines))))
		tY := 0
		if maxScroll > 0 {
			tY = (ta.Scroll * (ta.LastH - sH)) / maxScroll
		}
		for i := 0; i < ta.LastH; i++ {
			if i >= tY && i < tY+sH {
				c.DrawCell(ta.AbsX+ta.LastW-1, ta.AbsY+i, "█", bg, c.theme.Primary)
			} else {
				c.DrawCell(ta.AbsX+ta.LastW-1, ta.AbsY+i, "│", bg, c.theme.FgDisabled)
			}
		}
	}
}

// HandleEvent implements Widget: arrow keys move the cursor (Up/Down by
// visual line, preserving column where possible), Backspace/Delete/Enter
// edit the text, printable characters are inserted, and a mouse click moves
// the cursor to the clicked line and column.
func (ta *TextArea) HandleEvent(ev Event) {
	runes := []rune(ta.Text)
	lines := ta.buildLines()

	if ta.CursorPos < 0 {
		ta.CursorPos = 0
	}
	if ta.CursorPos > len(runes) {
		ta.CursorPos = len(runes)
	}

	if ev.Type == EventKey {
		if ev.Key == KeyLeft {
			if ta.CursorPos > 0 {
				ta.CursorPos--
			}
		} else if ev.Key == KeyRight {
			if ta.CursorPos < len(runes) {
				ta.CursorPos++
			}
		} else if ev.Key == KeyUp || ev.Key == KeyDown {
			visY, visX := 0, 0
			for i, l := range lines {
				if ta.CursorPos >= l.Start && ta.CursorPos <= l.Start+len(l.Runes) {
					if i < len(lines)-1 && ta.CursorPos == lines[i+1].Start {
						continue
					}
					visY, visX = i, ta.CursorPos-l.Start
					break
				}
			}
			if ev.Key == KeyUp && visY > 0 {
				nl := lines[visY-1]
				nX := visX
				if nX > len(nl.Runes) {
					nX = len(nl.Runes)
				}
				ta.CursorPos = nl.Start + nX
			} else if ev.Key == KeyDown && visY < len(lines)-1 {
				nl := lines[visY+1]
				nX := visX
				if nX > len(nl.Runes) {
					nX = len(nl.Runes)
				}
				ta.CursorPos = nl.Start + nX
			}
		} else if ev.Key == KeyBackspace {
			if ta.CursorPos > 0 {
				ta.Text = string(append(runes[:ta.CursorPos-1], runes[ta.CursorPos:]...))
				ta.CursorPos--
			}
		} else if ev.Key == KeyDelete {
			if ta.CursorPos < len(runes) {
				ta.Text = string(append(runes[:ta.CursorPos], runes[ta.CursorPos+1:]...))
			}
		} else if ev.Key == KeyEnter {
			head := append([]rune{}, runes[:ta.CursorPos]...)
			tail := append([]rune{}, runes[ta.CursorPos:]...)
			ta.Text = string(append(head, append([]rune{'\n'}, tail...)...))
			ta.CursorPos++
		} else if ev.Key == KeyCtrlC {
			clipboard.WriteAll(ta.Text)
		} else if ev.Key == KeyCtrlX {
			clipboard.WriteAll(ta.Text)
			ta.Text = ""
			ta.CursorPos = 0
		} else if ev.Key == KeyCtrlV {
			text, err := clipboard.ReadAll()
			if err == nil {
				// Normalize newlines
				text = strings.ReplaceAll(text, "\r\n", "\n")
				head := append([]rune{}, runes[:ta.CursorPos]...)
				tail := append([]rune{}, runes[ta.CursorPos:]...)
				pasted := []rune(text)
				ta.Text = string(append(append(head, pasted...), tail...))
				ta.CursorPos += len(pasted)
			}
		} else if ev.CharCode >= 32 {
			head := append([]rune{}, runes[:ta.CursorPos]...)
			tail := append([]rune{}, runes[ta.CursorPos:]...)
			ta.Text = string(append(head, append([]rune{ev.CharCode}, tail...)...))
			ta.CursorPos++
		}
	} else if ev.Type == EventMouseDown || ev.Type == EventMouseDrag {
		if ev.MouseX == ta.AbsX+ta.LastW-1 && len(lines) > ta.LastH {
			maxScroll := len(lines) - ta.LastH
			if maxScroll < 0 {
				maxScroll = 0
			}
			sH := int(math.Max(1, float64((ta.LastH*ta.LastH)/len(lines))))
			relY := ev.MouseY - ta.AbsY
			if relY < sH/2 {
				ta.Scroll = 0
			} else if relY >= ta.LastH-sH/2 {
				ta.Scroll = maxScroll
			} else {
				fraction := float64(relY-sH/2) / float64(ta.LastH-sH)
				ta.Scroll = int(math.Round(fraction * float64(maxScroll)))
			}
			return
		}

		if ev.Type == EventMouseDown {
			clickY := ev.MouseY - ta.AbsY + ta.Scroll
			clickX := ev.MouseX - ta.AbsX

			if clickY >= 0 && clickY < len(lines) {
				l := lines[clickY]
				if clickX > len(l.Runes) {
					clickX = len(l.Runes)
				}
				ta.CursorPos = l.Start + clickX
			} else if clickY >= len(lines) {
				ta.CursorPos = len(runes)
			}
		}
	} else if ev.Type == EventMouseScrollUp {
		if ta.Scroll > 0 {
			ta.Scroll--
		}
	} else if ev.Type == EventMouseScrollDown {
		maxScroll := len(lines) - ta.LastH
		if maxScroll < 0 {
			maxScroll = 0
		}
		if ta.Scroll < maxScroll {
			ta.Scroll++
		}
	}
}

// -------------------------------------------------------------------------
// ListBox
// -------------------------------------------------------------------------

// ListBox is a focusable, scrollable, single-selection list.
type ListBox struct {
	BaseWidget
	Items    []string
	Selected int
	Scroll   int
	OnSelect func(int, string)

	lastClickTime time.Time
	lastClickIdx  int
	OnDoubleClick func(int, string)
}

// NewListBox creates a ListBox at (x, y) listing items. onSelect, if
// non-nil, is called with the selected index and text on Enter or a mouse
// click on a row.
func NewListBox(x, y, w, h int, items []string, onSelect func(int, string)) *ListBox {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = true
	return &ListBox{BaseWidget: base, Items: items, OnSelect: onSelect}
}

// DrawRelative implements Widget.
func (lb *ListBox) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	lb.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	bg := c.theme.BgWidget
	if lb.IsFocused {
		bg = RGB(45, 53, 62)
	}

	for iy := 0; iy < lb.LastH; iy++ {
		for ix := 0; ix < lb.LastW; ix++ {
			c.DrawCell(lb.AbsX+ix, lb.AbsY+iy, " ", bg, c.theme.FgWindow)
		}
	}

	for i := 0; i < lb.LastH && (i+lb.Scroll) < len(lb.Items); i++ {
		idx := i + lb.Scroll
		isSelected := (idx == lb.Selected)
		ibg, iffg, prefix := bg, c.theme.FgWindow, "  "
		if isSelected {
			ibg, prefix = c.theme.Primary, "► "
		}

		for ix := 0; ix < lb.LastW-1; ix++ {
			c.DrawCell(lb.AbsX+ix, lb.AbsY+i, " ", ibg, c.theme.FgWindow)
		}
		c.DrawTextBounded(lb.AbsX, lb.AbsY+i, lb.LastW, prefix+lb.Items[idx], ibg, iffg)
	}

	if len(lb.Items) > lb.LastH {
		sH := int(math.Max(1, float64((lb.LastH*lb.LastH)/len(lb.Items))))
		tY := 0
		maxScroll := len(lb.Items) - lb.LastH
		if maxScroll > 0 {
			tY = (lb.Scroll * (lb.LastH - sH)) / maxScroll
		}
		for i := 0; i < lb.LastH; i++ {
			if i >= tY && i < tY+sH {
				c.DrawCell(lb.AbsX+lb.LastW-1, lb.AbsY+i, "█", bg, c.theme.Primary)
			} else {
				c.DrawCell(lb.AbsX+lb.LastW-1, lb.AbsY+i, "│", bg, c.theme.FgDisabled)
			}
		}
	}
}

// HandleEvent implements Widget: Up/Down move the selection (scrolling to
// keep it visible), Enter and a mouse click on a row both fire OnSelect.
func (lb *ListBox) HandleEvent(ev Event) {
	if ev.Type == EventKey {
		if ev.Key == KeyUp && lb.Selected > 0 {
			lb.Selected--
			if lb.Selected < lb.Scroll {
				lb.Scroll = lb.Selected
			}
		} else if ev.Key == KeyDown && lb.Selected < len(lb.Items)-1 {
			lb.Selected++
			if lb.Selected >= lb.Scroll+lb.LastH {
				lb.Scroll = lb.Selected - lb.LastH + 1
			}
		} else if ev.Key == KeyEnter && lb.Selected >= 0 && lb.Selected < len(lb.Items) {
			// Items is a plain exported slice a caller can reassign to a
			// shorter one without resetting Selected, so this bound must be
			// re-checked here rather than assumed from the Up/Down clamps.
			if lb.OnDoubleClick != nil {
				lb.OnDoubleClick(lb.Selected, lb.Items[lb.Selected])
			} else if lb.OnSelect != nil {
				lb.OnSelect(lb.Selected, lb.Items[lb.Selected])
			}
		}
	} else if ev.Type == EventMouseDown || ev.Type == EventMouseDrag {
		if ev.MouseX == lb.AbsX+lb.LastW-1 && len(lb.Items) > lb.LastH {
			maxScroll := len(lb.Items) - lb.LastH
			if maxScroll < 0 {
				maxScroll = 0
			}
			sH := int(math.Max(1, float64((lb.LastH*lb.LastH)/len(lb.Items))))
			relY := ev.MouseY - lb.AbsY
			if relY < sH/2 {
				lb.Scroll = 0
			} else if relY >= lb.LastH-sH/2 {
				lb.Scroll = maxScroll
			} else {
				fraction := float64(relY-sH/2) / float64(lb.LastH-sH)
				lb.Scroll = int(math.Round(fraction * float64(maxScroll)))
			}
			return
		}

		if ev.Type == EventMouseDown {
			clickedRow := ev.MouseY - lb.AbsY
			if clickedRow >= 0 && clickedRow < lb.LastH {
				idx := lb.Scroll + clickedRow
				if idx >= 0 && idx < len(lb.Items) {
					lb.Selected = idx
					if lb.OnSelect != nil {
						lb.OnSelect(lb.Selected, lb.Items[lb.Selected])
					}

					now := time.Now()
					if lb.OnDoubleClick != nil && lb.lastClickIdx == idx && now.Sub(lb.lastClickTime) < 500*time.Millisecond {
						lb.OnDoubleClick(lb.Selected, lb.Items[lb.Selected])
					}
					lb.lastClickIdx = idx
					lb.lastClickTime = now
				}
			}
		}
	} else if ev.Type == EventMouseScrollUp {
		if lb.Scroll > 0 {
			lb.Scroll--
		}
	} else if ev.Type == EventMouseScrollDown {
		maxScroll := len(lb.Items) - lb.LastH
		if maxScroll < 0 {
			maxScroll = 0
		}
		if lb.Scroll < maxScroll {
			lb.Scroll++
		}
	}
}

// -------------------------------------------------------------------------
// TodoList
// -------------------------------------------------------------------------

// TodoState is the completion state of a single TodoItem.
type TodoState int

// Supported todo-item states.
const (
	TodoPending TodoState = iota
	TodoRunning
	TodoDone
)

// TodoItem is one row of a TodoList: its label and current state.
type TodoItem struct {
	Text  string
	State TodoState
}

// TodoList is a scrollable checklist. In read-only mode it ignores input
// entirely and is meant to be driven programmatically via SetItemState
// (e.g. to reflect the progress of a background task).
type TodoList struct {
	BaseWidget
	Items    []TodoItem
	Selected int
	Scroll   int
	ReadOnly bool
}

// NewTodoList creates a TodoList at (x, y) from the given item labels, all
// starting in TodoPending state.
func NewTodoList(x, y, w, h int, items []string, readOnly bool) *TodoList {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = !readOnly
	todoItems := make([]TodoItem, len(items))
	for i, s := range items {
		todoItems[i] = TodoItem{Text: s, State: TodoPending}
	}
	return &TodoList{BaseWidget: base, Items: todoItems, ReadOnly: readOnly}
}

// SetItemState sets the state of the item at idx, ignoring out-of-range
// indices so callers driving this from a background goroutine's progress
// loop don't need to bounds-check.
func (tl *TodoList) SetItemState(idx int, state TodoState) {
	if idx >= 0 && idx < len(tl.Items) {
		tl.Items[idx].State = state
	}
}

// DrawRelative implements Widget.
func (tl *TodoList) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	tl.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	bg := c.theme.BgWindow
	if tl.IsFocused {
		bg = RGB(45, 53, 62)
	}

	for iy := 0; iy < tl.LastH; iy++ {
		for ix := 0; ix < tl.LastW; ix++ {
			c.DrawCell(tl.AbsX+ix, tl.AbsY+iy, " ", bg, c.theme.FgWindow)
		}
	}

	for i := 0; i < tl.LastH && (i+tl.Scroll) < len(tl.Items); i++ {
		idx := i + tl.Scroll
		isSelected := (idx == tl.Selected && !tl.ReadOnly)
		ibg, iffg := bg, c.theme.FgWindow
		if isSelected {
			ibg = c.theme.Primary
		}

		box := "[ ] "
		if tl.Items[idx].State == TodoDone {
			box, iffg = "[■] ", c.theme.Success
		} else if tl.Items[idx].State == TodoRunning {
			frames := []string{"▙", "▛", "▜", "▟"}
			ms := time.Now().UnixMilli()
			box = "[" + frames[(ms/150)%int64(len(frames))] + "] "
			iffg = c.theme.Warning
		}

		for ix := 0; ix < tl.LastW-1; ix++ {
			c.DrawCell(tl.AbsX+ix, tl.AbsY+i, " ", ibg, c.theme.FgWindow)
		}
		c.DrawTextBounded(tl.AbsX, tl.AbsY+i, tl.LastW, box+tl.Items[idx].Text, ibg, iffg)
	}

	if len(tl.Items) > tl.LastH {
		sH := int(math.Max(1, float64((tl.LastH*tl.LastH)/len(tl.Items))))
		tY := 0
		maxScroll := len(tl.Items) - tl.LastH
		if maxScroll > 0 {
			tY = (tl.Scroll * (tl.LastH - sH)) / maxScroll
		}
		for i := 0; i < tl.LastH; i++ {
			if i >= tY && i < tY+sH {
				c.DrawCell(tl.AbsX+tl.LastW-1, tl.AbsY+i, "█", bg, c.theme.Primary)
			} else {
				c.DrawCell(tl.AbsX+tl.LastW-1, tl.AbsY+i, "│", bg, c.theme.FgDisabled)
			}
		}
	}
}

// HandleEvent implements Widget: Up/Down move the selection, Space/Enter
// and a mouse click on a row both toggle that row between TodoPending and
// TodoDone. No-op when ReadOnly.
func (tl *TodoList) HandleEvent(ev Event) {
	if tl.ReadOnly {
		return
	}
	if ev.Type == EventKey {
		if ev.Key == KeyUp && tl.Selected > 0 {
			tl.Selected--
			if tl.Selected < tl.Scroll {
				tl.Scroll--
			}
		} else if ev.Key == KeyDown && tl.Selected < len(tl.Items)-1 {
			tl.Selected++
			if tl.Selected >= tl.Scroll+tl.LastH {
				tl.Scroll++
			}
		} else if (ev.Key == KeySpace || ev.Key == KeyEnter) && tl.Selected >= 0 && tl.Selected < len(tl.Items) {
			// Items is a plain exported slice a caller can reassign to a
			// shorter one without resetting Selected (components/erbe-3100-
			// tester does exactly this between test runs), so this bound
			// must be re-checked here rather than assumed from Up/Down.
			if tl.Items[tl.Selected].State == TodoDone {
				tl.Items[tl.Selected].State = TodoPending
			} else {
				tl.Items[tl.Selected].State = TodoDone
			}
		}
	} else if ev.Type == EventMouseDown || ev.Type == EventMouseDrag {
		if ev.MouseX == tl.AbsX+tl.LastW-1 && len(tl.Items) > tl.LastH {
			maxScroll := len(tl.Items) - tl.LastH
			if maxScroll < 0 {
				maxScroll = 0
			}
			sH := int(math.Max(1, float64((tl.LastH*tl.LastH)/len(tl.Items))))
			relY := ev.MouseY - tl.AbsY
			if relY < sH/2 {
				tl.Scroll = 0
			} else if relY >= tl.LastH-sH/2 {
				tl.Scroll = maxScroll
			} else {
				fraction := float64(relY-sH/2) / float64(tl.LastH-sH)
				tl.Scroll = int(math.Round(fraction * float64(maxScroll)))
			}
			return
		}

		if ev.Type == EventMouseDown {
			clickedRow := ev.MouseY - tl.AbsY
			if clickedRow >= 0 && clickedRow < tl.LastH {
				idx := tl.Scroll + clickedRow
				if idx >= 0 && idx < len(tl.Items) {
					tl.Selected = idx
					if tl.Items[tl.Selected].State == TodoDone {
						tl.Items[tl.Selected].State = TodoPending
					} else {
						tl.Items[tl.Selected].State = TodoDone
					}
				}
			}
		}
	} else if ev.Type == EventMouseScrollUp {
		if tl.Scroll > 0 {
			tl.Scroll--
		}
	} else if ev.Type == EventMouseScrollDown {
		maxScroll := len(tl.Items) - tl.LastH
		if maxScroll < 0 {
			maxScroll = 0
		}
		if tl.Scroll < maxScroll {
			tl.Scroll++
		}
	}
}

// -------------------------------------------------------------------------
// TabView
// -------------------------------------------------------------------------

// TabStyle selects how a TabView renders and whether it accepts focus/input.
type TabStyle int

// Supported tab-view styles.
const (
	// TabDefault is a focusable tab strip navigable with Left/Right and the
	// mouse.
	TabDefault TabStyle = iota
	// TabTimeline is a read-only progress indicator (e.g. wizard steps);
	// it never takes focus or input.
	TabTimeline
)

// Tab is one page of a TabView: its label and the widgets shown while it is
// active.
type Tab struct {
	Name    string
	Widgets []Widget
}

// TabView switches between named pages of widgets, showing exactly one at
// a time.
type TabView struct {
	BaseWidget
	Tabs   []Tab
	Active int
	Style  TabStyle
}

// NewTabView creates an empty TabView at (x, y) with the given width and
// style.
func NewTabView(x, y, w int, style TabStyle) *TabView {
	base := NewBaseWidget(x, y, w, 1)
	base.IsFocusable = (style == TabDefault)
	return &TabView{BaseWidget: base, Style: style, Tabs: make([]Tab, 0)}
}

// AddTab appends a new page and refreshes widget visibility so only the
// active tab's widgets are shown.
func (tv *TabView) AddTab(name string, widgets []Widget) {
	tv.Tabs = append(tv.Tabs, Tab{Name: name, Widgets: widgets})
	tv.UpdateVisibility()
}

// UpdateVisibility shows the active tab's widgets and hides every other
// tab's. Call this after changing Active directly.
func (tv *TabView) UpdateVisibility() {
	for i, tab := range tv.Tabs {
		for _, w := range tab.Widgets {
			w.SetVisible(i == tv.Active)
		}
	}
}

// DrawRelative implements Widget.
func (tv *TabView) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	tv.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	curX := tv.AbsX

	for i, tab := range tv.Tabs {
		isActive := (i == tv.Active)
		bg, fg := c.theme.BgWidget, c.theme.FgWindow

		if tv.Style == TabTimeline {
			bg = c.theme.BgWindow
			if i < tv.Active {
				fg = c.theme.Success
			} else if isActive {
				bg, fg = c.theme.Primary, c.theme.FgWindow
			}
		} else {
			if isActive {
				bg, fg = c.theme.BgFocused, c.theme.FgFocused
				if tv.IsFocused {
					bg, fg = c.theme.Primary, c.theme.FgWindow
				}
			}
		}

		label := " " + tab.Name + " "
		c.DrawText(curX, tv.AbsY, label, bg, fg)
		curX += len([]rune(label))

		if tv.Style == TabTimeline && i < len(tv.Tabs)-1 {
			c.DrawText(curX, tv.AbsY, " ➔ ", c.theme.BgWindow, c.theme.FgDisabled)
			curX += 3
		} else if tv.Style == TabDefault {
			curX += 1
		}
	}
}

// HandleEvent implements Widget: Left/Right switch the active tab. No-op
// for TabTimeline, which is display-only.
func (tv *TabView) HandleEvent(ev Event) {
	if tv.Style == TabTimeline {
		return
	}
	if ev.Type == EventKey {
		if ev.Key == KeyLeft && tv.Active > 0 {
			tv.Active--
			tv.UpdateVisibility()
		} else if ev.Key == KeyRight && tv.Active < len(tv.Tabs)-1 {
			tv.Active++
			tv.UpdateVisibility()
		}
	} else if ev.Type == EventMouseDown {
		curX := tv.AbsX
		for i, tab := range tv.Tabs {
			labelW := len([]rune(" " + tab.Name + " "))
			if ev.MouseX >= curX && ev.MouseX < curX+labelW && ev.MouseY == tv.AbsY {
				tv.Active = i
				tv.UpdateVisibility()
				return
			}
			curX += labelW
			if tv.Style == TabDefault {
				curX += 1
			}
		}
	}
}

// -------------------------------------------------------------------------
// ComboBox
// -------------------------------------------------------------------------

// ComboBox is a focusable dropdown menu.
type ComboBox struct {
	BaseWidget
	Items    []string
	Selected int
	IsOpen   bool
	OnSelect func(idx int, item string)
}

// NewComboBox creates a ComboBox at (x, y) with the given fixed width.
func NewComboBox(x, y, w int, items []string, onSelect func(int, string)) *ComboBox {
	base := NewBaseWidget(x, y, w, 1)
	base.IsFocusable = true
	return &ComboBox{BaseWidget: base, Items: items, OnSelect: onSelect}
}

// DrawRelative draws the closed state of the ComboBox.
func (cb *ComboBox) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	cb.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	// If we lost focus, close the dropdown automatically
	if !cb.IsFocused {
		cb.IsOpen = false
	}

	bg, fg := c.theme.BgWidget, c.theme.FgWindow
	if cb.IsFocused {
		bg = c.theme.BgFocused
	}

	for ix := 0; ix < cb.LastW; ix++ {
		c.DrawCell(cb.AbsX+ix, cb.AbsY, " ", bg, fg)
	}

	text := ""
	if cb.Selected >= 0 && cb.Selected < len(cb.Items) {
		text = cb.Items[cb.Selected]
	}

	// Draw the selected text and a dropdown arrow
	c.DrawTextBounded(cb.AbsX+1, cb.AbsY, cb.LastW-2, text, bg, fg)
	c.DrawCell(cb.AbsX+cb.LastW-1, cb.AbsY, "▼", bg, fg)
}

// DrawOverlay draws the expanded dropdown list if IsOpen is true.
func (cb *ComboBox) DrawOverlay(c *Canvas, offX, offY, pW, pH int) {
	if !cb.IsOpen || len(cb.Items) == 0 {
		return
	}

	h := len(cb.Items)
	if h > 5 {
		h = 5
	}

	// Ensure the overlay doesn't exceed screen bottom
	// If it does, we could draw it going up, but for now just clip/draw down.
	// We draw it starting at AbsY + 1
	bg, fg := c.theme.BgWidget, c.theme.FgWindow

	for iy := 0; iy < h; iy++ {
		for ix := 0; ix < cb.LastW; ix++ {
			c.DrawCell(cb.AbsX+ix, cb.AbsY+1+iy, " ", bg, fg)
		}

		idx := iy // Note: no scrolling implemented yet for >5 items, just show first 5
		if idx < len(cb.Items) {
			ibg := bg
			if ix := cb.Selected; ix == idx {
				ibg = c.theme.Primary
				for ix2 := 0; ix2 < cb.LastW; ix2++ {
					c.DrawCell(cb.AbsX+ix2, cb.AbsY+1+iy, " ", ibg, fg)
				}
			}
			c.DrawTextBounded(cb.AbsX+1, cb.AbsY+1+iy, cb.LastW-2, cb.Items[idx], ibg, fg)
		}
	}
}

// HitTest overrides BaseWidget.HitTest to expand the hit area when open.
func (cb *ComboBox) HitTest(mx, my int) bool {
	if cb.IsOpen {
		h := len(cb.Items)
		if h > 5 {
			h = 5
		}
		return mx >= cb.AbsX && mx < cb.AbsX+cb.LastW && my >= cb.AbsY && my <= cb.AbsY+h
	}
	return cb.BaseWidget.HitTest(mx, my)
}

// HandleEvent processes input.
func (cb *ComboBox) HandleEvent(ev Event) {
	if ev.Type == EventMouseDown {
		if !cb.IsOpen {
			cb.IsOpen = true
		} else {
			// Clicked somewhere in the overlay or on the main widget
			if ev.MouseY > cb.AbsY {
				idx := ev.MouseY - cb.AbsY - 1
				if idx >= 0 && idx < len(cb.Items) {
					cb.Selected = idx
					if cb.OnSelect != nil {
						cb.OnSelect(cb.Selected, cb.Items[cb.Selected])
					}
				}
			}
			cb.IsOpen = false
		}
	} else if ev.Type == EventKey {
		if ev.Key == KeyEnter {
			cb.IsOpen = !cb.IsOpen
		} else if ev.Key == KeyUp && cb.Selected > 0 {
			cb.Selected--
			if cb.OnSelect != nil {
				cb.OnSelect(cb.Selected, cb.Items[cb.Selected])
			}
		} else if ev.Key == KeyDown && cb.Selected < len(cb.Items)-1 {
			cb.Selected++
			if cb.OnSelect != nil {
				cb.OnSelect(cb.Selected, cb.Items[cb.Selected])
			}
		}
	}
}
