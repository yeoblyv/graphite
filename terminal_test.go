package Graphite

import (
	"strings"
	"testing"
	"time"
)

func TestNewTerminal_RunsAShellAndRendersItsOutput(t *testing.T) {
	skipIfWindowsConPTYOutputIsBroken(t)
	shell, runFlag := testShell()
	term, err := NewTerminal(nil, 0, 0, 40, 10, shell, []string{runFlag, "echo hi"})
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	c := NewCanvas()
	c.Resize(80, 24)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		term.DrawRelative(c, 0, 0, 80, 24)
		if term.screen.Cell(0, 0).Ch == 'h' {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := term.screen.Cell(0, 0).Ch; got != 'h' {
		t.Fatalf("cell(0,0) = %q, want 'h' (from \"echo hi\")", got)
	}
	if got := term.screen.Cell(1, 0).Ch; got != 'i' {
		t.Fatalf("cell(1,0) = %q, want 'i'", got)
	}
}

func TestTerminal_WriteRawReachesTheChild(t *testing.T) {
	skipIfWindowsConPTYOutputIsBroken(t)
	shell, _ := testShell()
	term, err := NewTerminal(nil, 0, 0, 40, 10, shell, nil)
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	term.WriteRaw([]byte("echo raw-input-reached-the-shell\n"))

	deadline := time.Now().Add(10 * time.Second)
	var seen bool
	for time.Now().Before(deadline) {
		term.mu.Lock()
		var sb strings.Builder
		for y := 0; y < term.screen.rows; y++ {
			for x := 0; x < term.screen.cols; x++ {
				if ch := term.screen.Cell(x, y).Ch; ch != 0 {
					sb.WriteRune(ch)
				}
			}
		}
		term.mu.Unlock()
		if strings.Contains(sb.String(), "raw-input-reached-the-shell") {
			seen = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !seen {
		t.Fatal("shell output never showed the echoed text sent via WriteRaw")
	}
}

func TestTerminal_ExitedReportsAfterTheChildExits(t *testing.T) {
	// Exited() flips true only when readLoop's pty.Read finally returns
	// an error, i.e. the same broken-on-Windows EOF-on-exit signal
	// skipIfWindowsConPTYOutputIsBroken documents, even though this test
	// never inspects output text itself.
	skipIfWindowsConPTYOutputIsBroken(t)
	shell, runFlag := testShell()
	term, err := NewTerminal(nil, 0, 0, 40, 10, shell, []string{runFlag, "exit 0"})
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if exited, _ := term.Exited(); exited {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Exited() never reported true after the child process exited")
}

func TestTerminal_ImplementsRawInputReceiver(t *testing.T) {
	shell, _ := testShell()
	term, err := NewTerminal(nil, 0, 0, 10, 5, shell, nil)
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	var _ RawInputReceiver = term
}

func TestTerminal_ImplementsScrollable(t *testing.T) {
	shell, _ := testShell()
	term, err := NewTerminal(nil, 0, 0, 10, 5, shell, nil)
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	var _ Scrollable = term
}

// newIdleTerminal spawns a real pty (process creation itself works
// cross-platform, per skipIfWindowsConPTYOutputIsBroken's own doc
// comment) but never reads its output — every test using this drives
// term.screen directly instead, so it's unaffected by the Windows
// ConPTY-output gap that gates tests reading real child output.
//
// It also draws once immediately, onto the returned canvas, before
// handing back control: DrawRelative resizes term.screen from its 80×24
// placeholder down to the widget's own declared size on the first call
// (see DrawRelative), and Resize wipes the grid — so a caller writing
// test content into term.screen has to happen after this priming draw,
// not before it, or DrawRelative's own first resize would wipe it right
// back out.
func newIdleTerminal(t *testing.T) (term *Terminal, canvas *Canvas) {
	t.Helper()
	shell, runFlag := testShell()
	term, err := NewTerminal(nil, 0, 0, 10, 2, shell, []string{runFlag, testSleepOneSecond()})
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	t.Cleanup(func() { term.Close() })

	canvas = NewCanvas()
	canvas.Resize(20, 5)
	term.DrawRelative(canvas, 0, 0, 20, 5)
	return term, canvas
}

func TestTerminal_MouseWheelAdjustsScrollback(t *testing.T) {
	term, _ := newIdleTerminal(t)
	term.mu.Lock()
	term.screen.Write([]byte("one\r\ntwo\r\nthree")) // "one" scrolls off into scrollback
	term.mu.Unlock()

	term.HandleEvent(Event{Type: EventMouseScrollUp})
	term.mu.Lock()
	off := term.screen.ViewOffset()
	term.mu.Unlock()
	if off == 0 {
		t.Fatalf("ViewOffset() after a scroll-up wheel event = 0, want > 0")
	}

	term.HandleEvent(Event{Type: EventMouseScrollDown})
	term.mu.Lock()
	off = term.screen.ViewOffset()
	term.mu.Unlock()
	if off != 0 {
		t.Errorf("ViewOffset() after scrolling back down = %d, want 0", off)
	}
}

func TestTerminal_HandleScrollDeclinesDuringAltScreen(t *testing.T) {
	term, _ := newIdleTerminal(t)
	term.mu.Lock()
	term.screen.Write([]byte("one\r\ntwo\r\nthree"))
	term.screen.Write([]byte("\x1b[?1049h")) // enter alternate screen (vim, htop, less, ...)
	term.mu.Unlock()

	if term.HandleScroll(3) {
		t.Errorf("HandleScroll returned true while the alternate screen is active, want false (so the caller forwards PageUp/PageDown to the child instead)")
	}
	term.mu.Lock()
	off := term.screen.ViewOffset()
	term.mu.Unlock()
	if off != 0 {
		t.Errorf("ViewOffset() = %d after a declined HandleScroll, want unchanged (0)", off)
	}
}

func TestTerminal_CursorNotDrawnWhileScrolledUp(t *testing.T) {
	term, c := newIdleTerminal(t)
	term.SetFocus(true)
	term.mu.Lock()
	term.screen.Write([]byte("one\r\ntwo\r\nthree")) // cursor ends up on row 1 ("three")
	cx, cy := term.screen.Cursor()
	term.mu.Unlock()

	term.DrawRelative(c, 0, 0, 20, 5)
	liveCursorCell := c.buffer[cy*c.width+cx]
	liveNonCursorCell := c.buffer[(cy*c.width)+cx+1] // an adjacent, non-cursor cell for contrast

	// At the live bottom, the cursor cell is drawn with fg/bg swapped
	// relative to an ordinary cell right next to it — that's how
	// DrawRelative paints a block cursor. If this fails, the fixture
	// itself (not the scrolled case below) is wrong.
	if liveCursorCell.BgColor == liveNonCursorCell.BgColor && liveCursorCell.FgColor == liveNonCursorCell.FgColor {
		t.Fatalf("fixture check failed: cursor cell isn't visually distinct from its neighbor at ViewOffset 0")
	}

	term.HandleScroll(1)
	term.mu.Lock()
	off := term.screen.ViewOffset()
	term.mu.Unlock()
	if off == 0 {
		t.Fatalf("expected the view to be scrolled up after HandleScroll(1)")
	}
	term.DrawRelative(c, 0, 0, 20, 5)
	scrolledCursorCell := c.buffer[cy*c.width+cx]

	if scrolledCursorCell.BgColor != liveNonCursorCell.BgColor || scrolledCursorCell.FgColor != liveNonCursorCell.FgColor {
		t.Errorf("the old cursor position is still drawn inverted while scrolled up; want it rendered as plain scrollback content like its neighbor")
	}
}
