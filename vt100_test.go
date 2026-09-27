package Graphite

import "testing"

func lineText(s *vtScreen, y int) string {
	var out []rune
	for x := 0; x < s.cols; x++ {
		c := s.Cell(x, y)
		if c.Ch == 0 {
			out = append(out, ' ')
		} else {
			out = append(out, c.Ch)
		}
	}
	return string(out)
}

func TestVTScreen_PrintsPlainText(t *testing.T) {
	s := newVTScreen(10, 3)
	s.Write([]byte("hello"))
	if got := lineText(s, 0); got != "hello     " {
		t.Errorf("line 0 = %q, want %q", got, "hello     ")
	}
	x, y := s.Cursor()
	if x != 5 || y != 0 {
		t.Errorf("cursor = (%d,%d), want (5,0)", x, y)
	}
}

func TestVTScreen_CarriageReturnAndLineFeed(t *testing.T) {
	s := newVTScreen(10, 3)
	s.Write([]byte("ab\r\ncd"))
	if got := lineText(s, 0); got[:2] != "ab" {
		t.Errorf("line 0 = %q, want to start with ab", got)
	}
	if got := lineText(s, 1); got[:2] != "cd" {
		t.Errorf("line 1 = %q, want to start with cd", got)
	}
	x, y := s.Cursor()
	if x != 2 || y != 1 {
		t.Errorf("cursor = (%d,%d), want (2,1)", x, y)
	}
}

func TestVTScreen_AutowrapAtRightMargin(t *testing.T) {
	s := newVTScreen(5, 3)
	s.Write([]byte("abcdefg"))
	if got := lineText(s, 0); got != "abcde" {
		t.Errorf("line 0 = %q, want abcde", got)
	}
	if got := lineText(s, 1); got[:2] != "fg" {
		t.Errorf("line 1 = %q, want to start with fg", got)
	}
}

func TestVTScreen_ScrollsWhenPastTheLastLine(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("one\r\ntwo\r\nthree"))
	if got := lineText(s, 0); got[:3] != "two" {
		t.Errorf("line 0 = %q, want to start with two (one scrolled off)", got)
	}
	if got := lineText(s, 1); got[:5] != "three" {
		t.Errorf("line 1 = %q, want to start with three", got)
	}
}

func TestVTScreen_CursorPositionCSI(t *testing.T) {
	s := newVTScreen(10, 5)
	s.Write([]byte("\x1b[3;5Hx"))
	x, y := s.Cursor()
	// CUP is 1-indexed row;col — row 3, col 5 lands the cursor at (4,2)
	// before printing 'x', which then advances it to (5,2).
	if x != 5 || y != 2 {
		t.Errorf("cursor after CUP+print = (%d,%d), want (5,2)", x, y)
	}
	if got := s.Cell(4, 2).Ch; got != 'x' {
		t.Errorf("cell at (4,2) = %q, want 'x'", got)
	}
}

func TestVTScreen_EraseInDisplayFromCursor(t *testing.T) {
	s := newVTScreen(5, 2)
	s.Write([]byte("abcde\r\nfghij"))
	s.Write([]byte("\x1b[1;3H\x1b[0J")) // cursor to (col3,row1), erase to end of screen
	if got := lineText(s, 0); got != "ab   " {
		t.Errorf("line 0 = %q, want ab followed by blanks", got)
	}
	if got := lineText(s, 1); got != "     " {
		t.Errorf("line 1 = %q, want all blank", got)
	}
}

func TestVTScreen_EraseInLine(t *testing.T) {
	s := newVTScreen(5, 1)
	s.Write([]byte("abcde\r\x1b[2C\x1b[K"))
	if got := lineText(s, 0); got != "ab   " {
		t.Errorf("line 0 = %q, want ab followed by blanks", got)
	}
}

func TestVTScreen_SGRForegroundColor(t *testing.T) {
	s := newVTScreen(5, 1)
	s.Write([]byte("\x1b[31mred"))
	c := s.Cell(0, 0)
	if c.Fg != ansiPalette[1] {
		t.Errorf("fg = %v, want the ANSI red %v", c.Fg, ansiPalette[1])
	}
}

func TestVTScreen_SGRResetClearsColorAndAttributes(t *testing.T) {
	s := newVTScreen(20, 1) // wide enough that nothing wraps/scrolls away
	s.Write([]byte("\x1b[1;31mred\x1b[0mplain"))
	if c := s.Cell(0, 0); c.Fg != ansiPalette[1] || !c.Bold {
		t.Errorf("'r' cell = %+v, want red+bold", c)
	}
	if c := s.Cell(3, 0); c.Fg != ColorNone || c.Bold {
		t.Errorf("'p' cell = %+v, want default color, not bold", c)
	}
}

func TestVTScreen_SGRTruecolor(t *testing.T) {
	s := newVTScreen(5, 1)
	s.Write([]byte("\x1b[38;2;10;20;30mx"))
	if got, want := s.Cell(0, 0).Fg, RGB(10, 20, 30); got != want {
		t.Errorf("fg = %v, want %v", got, want)
	}
}

func TestVTScreen_SGR256Color(t *testing.T) {
	s := newVTScreen(5, 1)
	s.Write([]byte("\x1b[38;5;196mx")) // pure red in the 256-color cube
	if got, want := s.Cell(0, 0).Fg, RGB(255, 0, 0); got != want {
		t.Errorf("fg = %v, want %v", got, want)
	}
}

func TestVTScreen_AlternateScreenIsolatesContent(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("primary"))
	s.Write([]byte("\x1b[?1049h")) // enter alternate screen
	s.Write([]byte("alt"))
	if got := lineText(s, 0); got[:3] != "alt" {
		t.Errorf("alt screen line 0 = %q, want to start with alt", got)
	}
	s.Write([]byte("\x1b[?1049l")) // leave alternate screen
	if got := lineText(s, 0); got[:7] != "primary" {
		t.Errorf("primary screen line 0 = %q, want to still say primary", got)
	}
}

func TestVTScreen_CursorVisibilityToggle(t *testing.T) {
	s := newVTScreen(5, 1)
	if !s.CursorVisible() {
		t.Fatal("cursor should start visible")
	}
	s.Write([]byte("\x1b[?25l"))
	if s.CursorVisible() {
		t.Error("cursor should be hidden after ?25l")
	}
	s.Write([]byte("\x1b[?25h"))
	if !s.CursorVisible() {
		t.Error("cursor should be visible again after ?25h")
	}
}

func TestVTScreen_ScrollRegionConfinesScrolling(t *testing.T) {
	s := newVTScreen(5, 5)
	s.Write([]byte("1\r\n2\r\n3\r\n4\r\n5")) // fill all 5 rows
	s.Write([]byte("\x1b[2;4r"))             // scroll region = rows 2-4 (1-indexed) => 0-indexed 1-3
	s.Write([]byte("\x1b[4;1H\r\n"))         // cursor to bottom of region, then scroll it
	if got := lineText(s, 0); got[:1] != "1" {
		t.Errorf("line 0 = %q, want untouched '1' (outside the scroll region)", got)
	}
	if got := lineText(s, 4); got[:1] != "5" {
		t.Errorf("line 4 = %q, want untouched '5' (outside the scroll region)", got)
	}
}

func TestVTScreen_SplitEscapeSequenceAcrossWrites(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("\x1b["))
	s.Write([]byte("31"))
	s.Write([]byte("mx"))
	if c := s.Cell(0, 0); c.Fg != ansiPalette[1] {
		t.Errorf("fg = %v, want red even though the CSI sequence arrived in 3 separate Write calls", c.Fg)
	}
}

func TestVTScreen_MultiByteUTF8Rune(t *testing.T) {
	s := newVTScreen(5, 1)
	s.Write([]byte("héllo"))
	if got := s.Cell(1, 0).Ch; got != 'é' {
		t.Errorf("cell(1,0) = %q, want 'é'", got)
	}
	x, _ := s.Cursor()
	if x != 5 {
		t.Errorf("cursor x = %d, want 5 (one column per rune, not per UTF-8 byte)", x)
	}
}

func TestVTScreen_UnrecognizedSequenceDoesNotCorruptFollowingText(t *testing.T) {
	s := newVTScreen(10, 1)
	s.Write([]byte("\x1b[999zhello")) // 'z' is not a final byte this implementation acts on
	if got := lineText(s, 0); got[:5] != "hello" {
		t.Errorf("line 0 = %q, want hello to print normally after the unknown sequence", got)
	}
}

func TestVTScreen_ResizeReallocatesBothGrids(t *testing.T) {
	s := newVTScreen(5, 5)
	s.Resize(20, 10)
	if s.cols != 20 || s.rows != 10 {
		t.Errorf("dimensions after Resize = (%d,%d), want (20,10)", s.cols, s.rows)
	}
	// Must not panic writing/reading at the new, larger bounds.
	s.Write([]byte("\x1b[10;20Hx"))
	if got := s.Cell(19, 9).Ch; got != 'x' {
		t.Errorf("cell(19,9) = %q, want 'x'", got)
	}
}

func viewLineText(s *vtScreen, y int) string {
	var out []rune
	for x := 0; x < s.cols; x++ {
		c := s.ViewCell(x, y)
		if c.Ch == 0 {
			out = append(out, ' ')
		} else {
			out = append(out, c.Ch)
		}
	}
	return string(out)
}

func TestVTScreen_ScrollingPastTheTopCapturesScrollback(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("one\r\ntwo\r\nthree")) // "one" scrolls off
	if got := s.scrollbackRows(); got != 1 {
		t.Fatalf("scrollbackRows() = %d, want 1", got)
	}
	if got := s.scrollback[0]; got.Ch != 'o' {
		t.Errorf("scrollback[0].Ch = %q, want 'o' (start of \"one\")", got.Ch)
	}
}

func TestVTScreen_ViewCellAtZeroOffsetMatchesCell(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("one\r\ntwo\r\nthree"))
	for y := 0; y < s.rows; y++ {
		for x := 0; x < s.cols; x++ {
			if got, want := s.ViewCell(x, y), s.Cell(x, y); got != want {
				t.Fatalf("ViewCell(%d,%d) = %+v, want %+v (Cell at viewOffset 0)", x, y, got, want)
			}
		}
	}
}

func TestVTScreen_ScrollByRevealsScrollbackThenClampsAtTop(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("one\r\ntwo\r\nthree")) // scrollback: ["one"], grid: ["two", "three"]

	s.ScrollBy(1)
	if got := viewLineText(s, 0); got[:3] != "one" {
		t.Errorf("scrolled up 1: line 0 = %q, want to start with \"one\"", got)
	}
	if got := viewLineText(s, 1); got[:3] != "two" {
		t.Errorf("scrolled up 1: line 1 = %q, want to start with \"two\"", got)
	}

	s.ScrollBy(10) // far past the top
	if got := s.ViewOffset(); got != 1 {
		t.Errorf("ViewOffset() after over-scrolling = %d, want clamped to 1 (only 1 scrollback row exists)", got)
	}

	s.ScrollToBottom()
	if got := viewLineText(s, 0); got[:3] != "two" {
		t.Errorf("after ScrollToBottom: line 0 = %q, want the live grid's \"two\" again", got)
	}
}

func TestVTScreen_AlternateScreenScrollDoesNotCaptureScrollback(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("\x1b[?1049h"))         // enter alternate screen
	s.Write([]byte("one\r\ntwo\r\nthree")) // would scroll "one" off, on the primary screen
	if got := s.scrollbackRows(); got != 0 {
		t.Errorf("scrollbackRows() after an alt-screen scroll = %d, want 0", got)
	}
}

func TestVTScreen_NarrowScrollRegionDoesNotCaptureScrollback(t *testing.T) {
	s := newVTScreen(5, 5)
	s.Write([]byte("1\r\n2\r\n3\r\n4\r\n5")) // fill all 5 rows
	s.Write([]byte("\x1b[2;4r"))             // scroll region = rows 2-4 (1-indexed)
	s.Write([]byte("\x1b[4;1H\r\n"))         // scroll within that narrower region only
	if got := s.scrollbackRows(); got != 0 {
		t.Errorf("scrollbackRows() after a confined-region scroll = %d, want 0 (only a full-screen scroll captures)", got)
	}
}

func TestVTScreen_ScrollbackTrimsToLimitFromTheFront(t *testing.T) {
	s := newVTScreen(10, 1) // 1 row: every line feed scrolls the whole screen
	s.scrollbackLimit = 3
	for i := 0; i < 5; i++ {
		s.Write([]byte{byte('a' + i), '\r', '\n'})
	}
	if got := s.scrollbackRows(); got != 3 {
		t.Fatalf("scrollbackRows() = %d, want capped at the limit of 3", got)
	}
	// The oldest rows ('a', 'b') should have been dropped, keeping 'c','d','e'.
	if got := s.scrollback[0].Ch; got != 'c' {
		t.Errorf("oldest remaining scrollback row starts with %q, want 'c' (the 3 most recent survive)", got)
	}
}

func TestVTScreen_ResizeClearsScrollback(t *testing.T) {
	s := newVTScreen(10, 2)
	s.Write([]byte("one\r\ntwo\r\nthree"))
	s.ScrollBy(1)

	s.Resize(20, 3)

	if got := s.scrollbackRows(); got != 0 {
		t.Errorf("scrollbackRows() after Resize = %d, want 0 (old rows are the wrong width now)", got)
	}
	if got := s.ViewOffset(); got != 0 {
		t.Errorf("ViewOffset() after Resize = %d, want 0", got)
	}
}
