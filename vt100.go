package Graphite

import "unicode/utf8"

// vtCell is one screen cell: a glyph plus the graphic rendition it was
// written with.
type vtCell struct {
	Ch                       rune
	Fg, Bg                   Color
	Bold, Underline, Reverse bool
}

// blankVTCell is the correct "erased" cell: Fg/Bg explicitly ColorNone,
// so a renderer falls back to its own default colors. Every clearing
// operation in this file and vt100_ops.go writes this instead of Go's
// zero-value vtCell{}, whose Fg/Bg would otherwise be Color(0) — solid
// black — rather than "no color set."
var blankVTCell = vtCell{Fg: ColorNone, Bg: ColorNone}

// vtParserState is which phase of an escape sequence vtScreen.Write is
// currently collecting bytes for.
type vtParserState int

const (
	vtGround vtParserState = iota
	vtEscape
	vtCSI
	vtOSC
	vtCharset // ESC ( or ESC ) seen; next byte selects a charset we ignore
)

// vtScreen is a VT100/xterm-subset terminal emulator: a grid of cells, a
// cursor, and the interpreter state to process a byte stream
// incrementally across many separate Write calls (a pty's output arrives
// in arbitrary chunks that can split an escape sequence across two
// reads). It intentionally covers the subset real-world interactive
// programs (shells, vim, htop, less, nested ssh) actually rely on rather
// than the full xterm control-sequence spec — DEC line-drawing character
// sets are the one known gap that remains; see terminal.go's doc comment.
type vtScreen struct {
	cols, rows int
	grid       []vtCell // primary screen, row-major, len == cols*rows
	altGrid    []vtCell // alternate screen (vim, htop, less, ...)
	usingAlt   bool

	cursorX, cursorY        int
	savedX, savedY          int
	cursorVisible           bool
	appCursorKeys           bool // DECCKM (?1h/l): affects how input.go encodes arrow keys
	scrollTop, scrollBottom int  // 0-indexed, inclusive DECSTBM scroll region

	curFg, curBg                      Color
	curBold, curUnderline, curReverse bool

	title string // last OSC 0/2 payload; exposed for a host that wants it, never drawn

	// scrollback holds rows scrolled off the top of the primary screen —
	// flat, complete rows (len is always a multiple of cols), oldest
	// first. Only a full-screen scroll of the primary grid pushes into it
	// (see scrollUp in vt100_ops.go): a scroll confined to a narrower
	// DECSTBM region, and anything happening on the alternate screen
	// (vim, htop, less), never does, matching what a real terminal's
	// scrollback captures.
	scrollback []vtCell
	// scrollbackLimit caps scrollback at this many rows; the oldest rows
	// are dropped once it's exceeded. Set via Terminal.SetScrollbackLimit;
	// defaults to DefaultScrollbackLines.
	scrollbackLimit int
	// viewOffset is how many rows the view is scrolled up from the live
	// bottom: 0 shows the grid exactly as Cell would (today's only
	// behavior), up to len(scrollback)/cols shows the oldest scrollback
	// row at the top. See ViewCell.
	viewOffset int

	state      vtParserState
	csiParams  []int
	csiPrivate bool // '?' prefix seen (DEC private mode sequences)
	oscBuf     []byte
}

// newVTScreen creates a cols×rows screen, cursor at the origin, visible,
// with the default (theme-driven — ColorNone) foreground/background.
func newVTScreen(cols, rows int) *vtScreen {
	s := &vtScreen{
		cursorVisible:   true,
		curFg:           ColorNone,
		curBg:           ColorNone,
		scrollBottom:    rows - 1,
		scrollbackLimit: DefaultScrollbackLines,
	}
	s.Resize(cols, rows)
	return s
}

// Resize changes the screen's dimensions, reallocating both grids fresh —
// reflowing wrapped lines to a new width is a real terminal feature this
// implementation doesn't attempt, so a resize simply starts each screen
// blank rather than trying to preserve content that may no longer fit.
// scrollback is cleared for the same reason: every row in it was captured
// at the old width, and ViewCell's indexing assumes every row is exactly
// s.cols wide.
func (s *vtScreen) Resize(cols, rows int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	s.cols, s.rows = cols, rows
	s.grid = newBlankGrid(cols, rows)
	s.altGrid = newBlankGrid(cols, rows)
	s.scrollTop, s.scrollBottom = 0, rows-1
	s.scrollback = nil
	s.viewOffset = 0
	if s.cursorX >= cols {
		s.cursorX = cols - 1
	}
	if s.cursorY >= rows {
		s.cursorY = rows - 1
	}
}

// scrollbackRows reports how many complete rows are currently in
// scrollback.
func (s *vtScreen) scrollbackRows() int {
	if s.cols == 0 {
		return 0
	}
	return len(s.scrollback) / s.cols
}

// ScrollBy moves the view delta rows toward scrollback (positive) or back
// toward the live bottom (negative), clamped to [0, scrollbackRows()].
func (s *vtScreen) ScrollBy(delta int) {
	s.viewOffset += delta
	if s.viewOffset < 0 {
		s.viewOffset = 0
	}
	if maxOffset := s.scrollbackRows(); s.viewOffset > maxOffset {
		s.viewOffset = maxOffset
	}
}

// ScrollToBottom returns the view to the live grid.
func (s *vtScreen) ScrollToBottom() { s.viewOffset = 0 }

// ViewOffset reports how many rows the view is currently scrolled up from
// the live bottom (0 = showing the live grid, same as Cell).
func (s *vtScreen) ViewOffset() int { return s.viewOffset }

// ViewCell is Cell's scrollback-aware counterpart: at viewOffset 0 it
// returns exactly what Cell(x, y) would; scrolled up, row y instead reads
// through scrollback, with the oldest captured row appearing at the top
// once the view is scrolled all the way back.
func (s *vtScreen) ViewCell(x, y int) vtCell {
	if s.viewOffset == 0 {
		return s.Cell(x, y)
	}
	if x < 0 || x >= s.cols || y < 0 || y >= s.rows {
		return blankVTCell
	}
	rows := s.scrollbackRows()
	rowFromTop := rows - s.viewOffset + y
	if rowFromTop < 0 {
		return blankVTCell
	}
	if rowFromTop < rows {
		return s.scrollback[rowFromTop*s.cols+x]
	}
	return s.Cell(x, rowFromTop-rows)
}

// active returns whichever grid (primary or alternate) is currently
// visible, so every drawing/scrolling helper works the same regardless of
// which screen is active.
func (s *vtScreen) active() []vtCell {
	if s.usingAlt {
		return s.altGrid
	}
	return s.grid
}

func (s *vtScreen) at(x, y int) *vtCell {
	g := s.active()
	return &g[y*s.cols+x]
}

// Cell returns the cell at (x, y) for a caller (Terminal.DrawRelative)
// that only wants to read the screen, not mutate it. Out-of-range
// coordinates return a blank cell rather than panicking.
func (s *vtScreen) Cell(x, y int) vtCell {
	if x < 0 || x >= s.cols || y < 0 || y >= s.rows {
		return blankVTCell
	}
	return *s.at(x, y)
}

// newBlankGrid allocates a cols*rows grid pre-filled with blankVTCell
// (Go's own zero-fill from make() would leave every cell's Fg/Bg at
// Color(0) — solid black — instead).
func newBlankGrid(cols, rows int) []vtCell {
	g := make([]vtCell, cols*rows)
	for i := range g {
		g[i] = blankVTCell
	}
	return g
}

// CursorVisible reports whether the cursor should be drawn — false while
// a DECTCEM ?25l is in effect (most full-screen programs hide it while
// they own the whole display and draw their own).
func (s *vtScreen) CursorVisible() bool { return s.cursorVisible }

// Cursor returns the cursor's current position.
func (s *vtScreen) Cursor() (x, y int) { return s.cursorX, s.cursorY }

// Write feeds pty output through the parser, updating the screen. It
// never returns an error: an unrecognized or malformed sequence is
// dropped rather than treated as fatal, the same tolerance every real
// terminal emulator applies (a program's output is not a trusted,
// well-formed protocol).
func (s *vtScreen) Write(p []byte) (int, error) {
	i := 0
	for i < len(p) {
		b := p[i]
		switch s.state {
		case vtGround:
			if b == 0x1B {
				s.state = vtEscape
				i++
				continue
			}
			if b < 0x20 || b == 0x7F {
				s.controlChar(b)
				i++
				continue
			}
			// A printable character: decode one full UTF-8 rune (pty
			// output is UTF-8, matching graphite's own terminal-input
			// assumption elsewhere) rather than treating each byte of a
			// multi-byte rune as its own cell.
			r, size := utf8.DecodeRune(p[i:])
			s.printRune(r)
			i += size

		case vtEscape:
			i++
			switch b {
			case '[':
				s.state = vtCSI
				// Start with one zero-valued param already in place —
				// digits accumulate into whatever the last slot is, and
				// ';' appends a fresh one, so there's always exactly one
				// "current" slot to write into (see the vtCSI case
				// below); never appending from two different triggers
				// keeps a run of digits from producing a stray extra
				// param between two real ones.
				s.csiParams = append(s.csiParams[:0], 0)
				s.csiPrivate = false
			case ']':
				s.state = vtOSC
				s.oscBuf = s.oscBuf[:0]
			case '7':
				s.savedX, s.savedY = s.cursorX, s.cursorY
				s.state = vtGround
			case '8':
				s.cursorX, s.cursorY = s.savedX, s.savedY
				s.state = vtGround
			case '(', ')':
				s.state = vtCharset
			case 'M': // reverse index: cursor up, scrolling down if at the top margin
				s.reverseIndex()
				s.state = vtGround
			default:
				s.state = vtGround // unsupported single-char escape: consumed, ignored
			}

		case vtCharset:
			i++
			s.state = vtGround // the charset designator byte itself; charset switching unsupported

		case vtCSI:
			i++
			if b >= '0' && b <= '9' {
				last := len(s.csiParams) - 1
				s.csiParams[last] = s.csiParams[last]*10 + int(b-'0')
				continue
			}
			if b == ';' {
				s.csiParams = append(s.csiParams, 0)
				continue
			}
			if b == '?' {
				s.csiPrivate = true
				continue
			}
			if (b >= 0x40 && b <= 0x7E) || b == '~' {
				// A trailing '~' (used by some DEC private sequences) is
				// treated as its own final byte alongside the standard
				// 0x40-0x7E final-byte range.
				s.dispatchCSI(b)
				s.state = vtGround
				continue
			}
			// Any other byte (an intermediate like ' ' or '$') is
			// consumed and ignored — this implementation doesn't act on
			// intermediates, only the small set of final bytes it knows.

		case vtOSC:
			i++
			if b == 0x07 { // BEL terminator
				s.dispatchOSC()
				s.state = vtGround
				continue
			}
			if b == 0x1B && i < len(p) && p[i] == '\\' { // ST terminator (ESC \)
				i++
				s.dispatchOSC()
				s.state = vtGround
				continue
			}
			s.oscBuf = append(s.oscBuf, b)
		}
	}
	return len(p), nil
}
