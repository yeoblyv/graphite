package graphite

// Window is a bordered, titled container drawn centered on the canvas. It
// holds a flat list of top-level children (which may themselves be
// containers like Panel) and owns focus navigation and event routing for
// the whole subtree.
type Window struct {
	FixedW, FixedH, PctW, PctH int
	Title                      string
	Children                   []Widget
	PaddingX, PaddingY         int
}

// NewWindow creates a Window with a fixed size and title, and default
// padding around its content area.
func NewWindow(w, h int, title string) *Window {
	return &Window{
		FixedW: w, FixedH: h, Title: title,
		Children: make([]Widget, 0),
		PaddingX: 4, PaddingY: 2,
	}
}

// SetPercentSize switches the window to a size relative to the canvas
// instead of FixedW/FixedH.
func (w *Window) SetPercentSize(pw, ph int) { w.PctW, w.PctH = pw, ph }

// getFlatFocusables walks the widget tree (descending into containers via
// GetChildren) and returns every visible, focusable widget in traversal
// order, which is also Tab order.
func (w *Window) getFlatFocusables() []Widget {
	var flat []Widget
	var walk func(widgets []Widget)
	walk = func(widgets []Widget) {
		for _, child := range widgets {
			if !child.IsVisible() {
				continue
			}
			if child.CanFocus() {
				flat = append(flat, child)
			}
			if len(child.GetChildren()) > 0 {
				walk(child.GetChildren())
			}
		}
	}
	walk(w.Children)
	return flat
}

// AddWidget appends widget as a top-level child. If no widget in the window
// currently has focus, the first focusable widget (including one nested
// inside widget, if it is a container) becomes focused.
func (w *Window) AddWidget(widget Widget) {
	w.Children = append(w.Children, widget)
	flat := w.getFlatFocusables()
	hasFocus := false
	for _, f := range flat {
		if f.HasFocus() {
			hasFocus = true
			break
		}
	}
	if !hasFocus && len(flat) > 0 {
		flat[0].SetFocus(true)
	}
}

// HandleEvent routes a mouse or keyboard event to the appropriate widget:
// mouse events go to the deepest widget hit-tested under the pointer, Tab
// advances focus through the flattened focus order, and all other key
// events go to whichever widget currently has focus. Disabled widgets never
// receive an event, regardless of what their own HandleEvent does.
func (w *Window) HandleEvent(ev Event) {
	if ev.Type == EventMouseDown {
		var target Widget
		var walk func(widgets []Widget)
		walk = func(widgets []Widget) {
			for _, child := range widgets {
				if child.IsVisible() && child.HitTest(ev.MouseX, ev.MouseY) {
					target = child
					if len(child.GetChildren()) > 0 {
						walk(child.GetChildren())
					}
				}
			}
		}
		walk(w.Children)

		if target != nil && target.IsEnabled() {
			if target.CanFocus() {
				for _, f := range w.getFlatFocusables() {
					if f.HasFocus() {
						f.SetFocus(false)
					}
				}
				target.SetFocus(true)
			}
			target.HandleEvent(ev)
		}
		return
	}

	if ev.Type == EventKey {
		flat := w.getFlatFocusables()
		if ev.Key == KeyTab && len(flat) > 0 {
			idx := -1
			for i, f := range flat {
				if f.HasFocus() {
					idx = i
					f.SetFocus(false)
					break
				}
			}
			next := (idx + 1) % len(flat)
			flat[next].SetFocus(true)
		} else {
			for _, f := range flat {
				if f.HasFocus() {
					// getFlatFocusables already filters by CanFocus (which
					// implies enabled), but a custom Widget could override
					// CanFocus independently of IsEnabled, so check again.
					if f.IsEnabled() {
						f.HandleEvent(ev)
					}
					break
				}
			}
		}
	}
}

// Draw renders the window's frame, drop shadow, and title centered on c,
// then draws its children within the resulting padded content area.
func (w *Window) Draw(c *Canvas) {
	aW, aH := w.FixedW, w.FixedH
	if w.PctW > 0 {
		aW = (c.Width() * w.PctW) / 100
	}
	if w.PctH > 0 {
		aH = (c.Height() * w.PctH) / 100
	}
	if aW < 40 {
		aW = 40
	}
	if aH < 10 {
		aH = 10
	}
	aX := (c.Width() - aW) / 2
	aY := (c.Height() - aH) / 2

	for i := 2; i <= aW+1; i++ {
		c.DrawCell(aX+i, aY+aH, "░", c.GetCellBg(aX+i, aY+aH), 90)
	}
	for i := 1; i <= aH; i++ {
		c.DrawCell(aX+aW, aY+i, "░", c.GetCellBg(aX+aW, aY+i), 90)
		c.DrawCell(aX+aW+1, aY+i, "░", c.GetCellBg(aX+aW+1, aY+i), 90)
	}

	bg, fg := c.theme.BgWindow, c.theme.FgWindow
	for i := 1; i < aW-1; i++ {
		c.DrawCell(aX+i, aY, "─", bg, fg)
		c.DrawCell(aX+i, aY+aH-1, "─", bg, fg)
	}
	for i := 1; i < aH-1; i++ {
		c.DrawCell(aX, aY+i, "│", bg, fg)
		c.DrawCell(aX+aW-1, aY+i, "│", bg, fg)
	}
	c.DrawCell(aX, aY, "┌", bg, fg)
	c.DrawCell(aX+aW-1, aY, "┐", bg, fg)
	c.DrawCell(aX, aY+aH-1, "└", bg, fg)
	c.DrawCell(aX+aW-1, aY+aH-1, "┘", bg, fg)

	if w.Title != "" {
		c.DrawText(aX+2, aY, " "+w.Title+" ", bg, fg)
	}

	for iy := 1; iy < aH-1; iy++ {
		for ix := 1; ix < aW-1; ix++ {
			c.DrawCell(aX+ix, aY+iy, " ", bg, fg)
		}
	}

	cX, cY, cW, cH := aX+w.PaddingX, aY+w.PaddingY, aW-(w.PaddingX*2), aH-(w.PaddingY*2)
	for _, child := range w.Children {
		if child.IsVisible() {
			child.DrawRelative(c, cX, cY, cW, cH)
		}
	}
	for _, child := range w.Children {
		if child.IsVisible() {
			child.DrawOverlay(c, cX, cY, cW, cH)
		}
	}
}
