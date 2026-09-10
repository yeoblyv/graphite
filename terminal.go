package Graphite

import "sync"

// Terminal is a widget that runs a shell (or any interactive program)
// attached to a real pseudo-terminal and renders its output faithfully —
// including full-screen programs like vim or htop, which need real
// cursor control and an alternate screen buffer, not just scrolling text.
//
// Getting there needs two things working together: startPTY (pty.go and
// its per-OS files) gives the child process an actual controlling
// terminal, and vtScreen (vt100.go) interprets whatever escape sequences
// it emits into a screen grid. Terminal's own job is gluing those to
// graphite — rendering the grid via DrawRelative, and forwarding
// keystrokes back to the child completely undistorted: it implements
// RawInputReceiver (see app.go) so Application.Run routes it raw bytes
// straight from the real terminal in front of the user, bypassing
// graphite's own Event-decoding entirely. Round-tripping through
// graphite's smaller KeyCode vocabulary first would lose anything that
// vocabulary doesn't cover — application-cursor-mode arrows, exotic
// modifier combinations — which is exactly the "distortion" an embedded
// terminal can't afford.
//
// Known gaps: no scrollback (only the visible grid), and DEC line-drawing
// character sets aren't translated, so a program that leans on them for
// box-drawing borders may show the raw designator characters instead.
type Terminal struct {
	BaseWidget

	pty    ptySession
	screen *vtScreen

	// mu guards screen and the two fields below: pty output arrives on
	// readLoop's own goroutine, concurrently with the render loop reading
	// screen from DrawRelative.
	mu      sync.Mutex
	exited  bool
	exitErr error

	// OnExit, if set, is called (from the main loop, via Application.Invoke
	// — never directly from readLoop's own goroutine) once the child
	// process exits, so a host program can close the tab/pane hosting it.
	OnExit func(err error)

	app *Application // needed only to hop OnExit back onto the main loop
}

// NewTerminal spawns shell (with args) attached to a pseudo-terminal at
// (x, y) sized w×h — 0 or negative for either follows BaseWidget's usual
// "stretch to fill the parent" convention, exactly as any other widget's
// constructor does. The pty/screen themselves still need a real, positive
// starting size before the first layout pass ever runs, so they start at
// a sane placeholder (80×24) that DrawRelative immediately resizes to the
// widget's actually resolved size on the very first frame. app is used
// solely to deliver OnExit safely via Application.Invoke; Terminal does
// not otherwise reach into it.
func NewTerminal(app *Application, x, y, w, h int, shell string, args []string) (*Terminal, error) {
	const placeholderCols, placeholderRows = 80, 24
	pty, err := startPTY(shell, args, placeholderCols, placeholderRows)
	if err != nil {
		return nil, err
	}

	t := &Terminal{
		BaseWidget: NewBaseWidget(x, y, w, h),
		pty:        pty,
		screen:     newVTScreen(placeholderCols, placeholderRows),
		app:        app,
	}
	t.IsFocusable = true

	go t.readLoop()
	return t, nil
}

// readLoop copies pty output into t.screen until the pty closes or the
// child exits, then records the outcome and (via Application.Invoke)
// calls OnExit on the main loop.
func (t *Terminal) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.pty.Read(buf)
		if n > 0 {
			t.mu.Lock()
			t.screen.Write(buf[:n])
			t.mu.Unlock()
		}
		if err != nil {
			waitErr := t.pty.Wait()
			t.mu.Lock()
			t.exited = true
			t.exitErr = waitErr
			t.mu.Unlock()
			if t.app != nil && t.OnExit != nil {
				t.app.Invoke(func() { t.OnExit(waitErr) })
			}
			return
		}
	}
}

// Exited reports whether the child process has exited, and its result
// (nil on a clean exit) once it has.
func (t *Terminal) Exited() (exited bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exited, t.exitErr
}

// Close releases the pty's own resources. It does not wait for or kill
// the child process — Wait (via Exited, or the pty itself) does that.
func (t *Terminal) Close() error {
	return t.pty.Close()
}

// WriteRaw implements RawInputReceiver: every byte is sent to the child
// exactly as received, with no interpretation.
func (t *Terminal) WriteRaw(p []byte) {
	t.pty.Write(p)
}

// DrawRelative implements Widget: resizes the pty/screen to match this
// frame's resolved size before rendering, so the child sees an accurate
// terminal size (e.g. after the surrounding layout reflows), then paints
// every cell and, if the child left the cursor visible, a block cursor.
func (t *Terminal) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	t.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	if t.LastW < 1 || t.LastH < 1 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.LastW != t.screen.cols || t.LastH != t.screen.rows {
		t.screen.Resize(t.LastW, t.LastH)
		t.pty.Resize(t.LastW, t.LastH)
	}

	theme := c.Theme()
	for y := 0; y < t.screen.rows; y++ {
		for x := 0; x < t.screen.cols; x++ {
			cell := t.screen.Cell(x, y)
			fg, bg := cell.Fg, cell.Bg
			if fg == ColorNone {
				fg = theme.FgWindow
			}
			if bg == ColorNone {
				bg = theme.BgWidget
			}
			if cell.Reverse {
				fg, bg = bg, fg
			}
			ch := " "
			if cell.Ch != 0 {
				ch = string(cell.Ch)
			}
			c.DrawCell(t.AbsX+x, t.AbsY+y, ch, bg, fg)
		}
	}

	if t.screen.CursorVisible() && t.IsFocused {
		cx, cy := t.screen.Cursor()
		if cx >= 0 && cx < t.screen.cols && cy >= 0 && cy < t.screen.rows {
			cell := t.screen.Cell(cx, cy)
			ch := " "
			if cell.Ch != 0 {
				ch = string(cell.Ch)
			}
			fg := cell.Bg
			if fg == ColorNone {
				fg = theme.BgWidget
			}
			bg := cell.Fg
			if bg == ColorNone {
				bg = theme.FgWindow
			}
			c.DrawCell(t.AbsX+cx, t.AbsY+cy, ch, bg, fg)
		}
	}
}

// HandleEvent implements Widget. A mouse click just focuses the terminal
// (Window's own click-to-focus handling does the rest) — no key event
// ever reaches here while focused, since Application.Run routes those
// through WriteRaw instead once this widget has focus.
func (t *Terminal) HandleEvent(ev Event) {}
