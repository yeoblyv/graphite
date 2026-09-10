package Graphite

// WindowChrome selects how much decoration Window.Draw paints around a
// window's content.
type WindowChrome int

// Supported chrome modes.
const (
	// ChromeBordered draws a titled border and drop shadow, centered on
	// the canvas at the window's fixed/percentage size. Window's original
	// behavior, and still the default (the zero value), so every existing
	// window is completely unaffected by ChromeBorderless's addition.
	ChromeBordered WindowChrome = iota
	// ChromeBorderless fills the entire canvas with no border, shadow, or
	// title bar — for a full-screen application's main window (e.g. a
	// commander-style file manager) rather than a floating dialog.
	// FixedW/FixedH/PctW/PctH are ignored in this mode: the window is
	// always exactly the canvas size.
	ChromeBorderless
)

// Window is a titled container holding a flat list of top-level children
// (which may themselves be containers like Panel) and owning focus
// navigation and event routing for the whole subtree. By default
// (ChromeBordered) it draws a border, drop shadow, and title, centered on
// the canvas at a fixed or percentage size — see ChromeBorderless for a
// full-screen alternative.
type Window struct {
	FixedW, FixedH, PctW, PctH int
	Title                      string
	Children                   []Widget
	PaddingX, PaddingY         int
	Chrome                     WindowChrome

	// mouseCapture is the widget that was hit by the most recent
	// EventMouseDown. Until the matching EventMouseUp, drag and release
	// events go straight to it — bypassing hit-testing — so a drag that
	// moves the pointer outside the widget's bounds (e.g. a fader handle
	// dragged past the widget's edge) still reaches it. This mirrors the
	// implicit mouse capture every desktop GUI toolkit does.
	mouseCapture Widget
}

// ClearMouseCapture forcefully releases any active mouse capture in this
// window, preventing mouse up/drag events from being routed to the widget
// that triggered a modal open.
func (w *Window) ClearMouseCapture() {
	w.mouseCapture = nil
}

// HasMouseCapture reports whether a mouse gesture is still in flight —
// EventMouseDown has hit some widget but the matching EventMouseUp
// hasn't arrived yet. Application.Run checks this before switching a
// focused RawInputReceiver's raw-passthrough on: if a gesture that
// started before focus changed (e.g. a click on a menu item that itself
// creates and focuses a new Terminal tab) is still open, its trailing
// EventMouseUp needs to reach the widget that captured it — normally
// decoded and routed — rather than being redirected to the newly
// focused widget as raw bytes.
func (w *Window) HasMouseCapture() bool {
	return w.mouseCapture != nil
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

// NewFullscreenWindow creates a Window with ChromeBorderless chrome and no
// padding — it fills the canvas exactly, edge to edge, with no border,
// shadow, or title bar. Call SetPercentLayout/SetPosition on individual
// children (or give the window PaddingX/PaddingY) for breathing room; the
// window itself adds none by default, unlike NewWindow's 4/2.
func NewFullscreenWindow() *Window {
	return &Window{Chrome: ChromeBorderless, Children: make([]Widget, 0)}
}

// focusedWidget returns whichever widget in this window currently has
// focus, or nil if none does — used to find a RawInputReceiver (see
// terminal.go) to route undecoded input bytes to.
func (w *Window) focusedWidget() Widget {
	for _, f := range w.getFlatFocusables() {
		if f.HasFocus() {
			return f
		}
	}
	return nil
}

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
// a mouse press goes to the deepest widget hit-tested under the pointer and
// captures the mouse, so the resulting drag/release events go straight to
// that same widget regardless of where the pointer moves next; a scroll or
// right-click event is hit-tested and delivered the same way but without
// moving focus or starting a capture, since neither has a drag/release to
// capture for; Tab advances focus through the flattened focus order; all
// other key events go to whichever widget currently has focus. Disabled
// widgets never receive an event, regardless of what their own HandleEvent
// does.
func (w *Window) HandleEvent(ev Event) {
	if ev.Type == EventMouseDrag || ev.Type == EventMouseUp {
		if w.mouseCapture != nil && w.mouseCapture.IsEnabled() {
			w.mouseCapture.HandleEvent(ev)
		}
		if ev.Type == EventMouseUp {
			w.mouseCapture = nil
		}
		return
	}

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

		w.mouseCapture = nil
		if target != nil && target.IsEnabled() {
			if target.CanFocus() {
				for _, f := range w.getFlatFocusables() {
					if f.HasFocus() {
						f.SetFocus(false)
					}
				}
				target.SetFocus(true)
			}
			w.mouseCapture = target
			target.HandleEvent(ev)
		}
		return
	}

	if ev.Type == EventMouseScrollUp || ev.Type == EventMouseScrollDown || ev.Type == EventMouseRightDown {
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

// Draw renders the window, then its children within the resulting padded
// content area. With ChromeBorderless (see NewFullscreenWindow), that's
// the whole canvas with no decoration; the default ChromeBordered instead
// draws a frame, drop shadow, and title centered on c at a fixed or
// percentage size.
func (w *Window) Draw(c *Canvas) {
	if w.Chrome == ChromeBorderless {
		w.drawBorderless(c)
		return
	}

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
		c.DrawCell(aX+i, aY+aH, "░", c.GetCellBg(aX+i, aY+aH), c.theme.Disabled)
	}
	for i := 1; i <= aH; i++ {
		c.DrawCell(aX+aW, aY+i, "░", c.GetCellBg(aX+aW, aY+i), c.theme.Disabled)
		c.DrawCell(aX+aW+1, aY+i, "░", c.GetCellBg(aX+aW+1, aY+i), c.theme.Disabled)
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

// drawBorderless implements Draw for ChromeBorderless: no frame, shadow,
// or title — just the theme's window background filling the canvas
// exactly, with children laid out inside PaddingX/PaddingY (0 by default,
// per NewFullscreenWindow).
func (w *Window) drawBorderless(c *Canvas) {
	aW, aH := c.Width(), c.Height()
	bg, fg := c.theme.BgWindow, c.theme.FgWindow
	for iy := 0; iy < aH; iy++ {
		for ix := 0; ix < aW; ix++ {
			c.DrawCell(ix, iy, " ", bg, fg)
		}
	}

	cX, cY, cW, cH := w.PaddingX, w.PaddingY, aW-(w.PaddingX*2), aH-(w.PaddingY*2)
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
