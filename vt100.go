package Graphite

import "unicode/utf8"

// vtCell is one screen cell: a glyph plus the graphic rendition it was
// written with.
type vtCell struct {
	Ch                       rune
	Fg, Bg                   Color
	Bold, Underline, Reverse bool
}

// blank reports whether c is an untouched cell (never written to, as
// opposed to one explicitly cleared to a space) — used so Terminal's own
// DrawRelative can leave truly empty cells alone instead of painting over
// whatever the widget behind it drew.
func (c vtCell) blank() bool { return c.Ch == 0 }

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
// sets and a real scrollback buffer are the two known gaps; see
// terminal.go's doc comment.
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

	state      vtParserState
	csiParams  []int
	csiPrivate bool // '?' prefix seen (DEC private mode sequences)
	oscBuf     []byte
}

// newVTScreen creates a cols×rows screen, cursor at the origin, visible,
// with the default (theme-driven — ColorNone) foreground/background.
func newVTScreen(cols, rows int) *vtScreen {
	s := &vtScreen{
		cursorVisible: true,
		curFg:         ColorNone,
		curBg:         ColorNone,
		scrollBottom:  rows - 1,
	}
	s.Resize(cols, rows)
	return s
}

// Resize changes the screen's dimensions, reallocating both grids fresh —
// reflowing wrapped lines to a new width is a real terminal feature this
// implementation doesn't attempt, so a resize simply starts each screen
// blank rather than trying to preserve content that may no longer fit.
func (s *vtScreen) Resize(cols, rows int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	s.cols, s.rows = cols, rows
	s.grid = make([]vtCell, cols*rows)
	s.altGrid = make([]vtCell, cols*rows)
	s.scrollTop, s.scrollBottom = 0, rows-1
	if s.cursorX >= cols {
		s.cursorX = cols - 1
	}
	if s.cursorY >= rows {
		s.cursorY = rows - 1
	}
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
		return vtCell{}
	}
	return *s.at(x, y)
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
