package Graphite

// controlChar handles a single C0 control character in vtGround state.
func (s *vtScreen) controlChar(b byte) {
	switch b {
	case '\n': // LF
		s.lineFeed()
	case '\r': // CR
		s.cursorX = 0
	case '\b': // BS
		if s.cursorX > 0 {
			s.cursorX--
		}
	case '\t': // HT: next multiple-of-8 stop, clamped to the last column
		next := (s.cursorX/8 + 1) * 8
		if next >= s.cols {
			next = s.cols - 1
		}
		s.cursorX = next
	case 0x07: // BEL: no visible bell to ring, silently ignored
	}
}

// printRune writes r at the cursor with the current rendition and
// advances the cursor, wrapping to the next line (scrolling if needed)
// when it runs off the right edge — matching a real terminal's
// autowrap, which every full-screen program assumes is on.
func (s *vtScreen) printRune(r rune) {
	if s.cursorX >= s.cols {
		s.cursorX = 0
		s.lineFeed()
	}
	*s.at(s.cursorX, s.cursorY) = vtCell{
		Ch: r, Fg: s.curFg, Bg: s.curBg,
		Bold: s.curBold, Underline: s.curUnderline, Reverse: s.curReverse,
	}
	s.cursorX++
}

// lineFeed moves the cursor down one row, scrolling the active screen's
// scroll region up by one line when the cursor is already at its bottom
// margin — the mechanism behind both a plain "\n" and a program
// explicitly setting a scroll region narrower than the full screen (e.g.
// a pager keeping a status line pinned at the bottom).
func (s *vtScreen) lineFeed() {
	if s.cursorY == s.scrollBottom {
		s.scrollUp(1)
		return
	}
	if s.cursorY < s.rows-1 {
		s.cursorY++
	}
}

// reverseIndex is ESC M: cursor up one row, scrolling the region down by
// one line if the cursor is already at its top margin — the mirror image
// of lineFeed, used by programs that redraw upward (e.g. `less` when you
// scroll back).
func (s *vtScreen) reverseIndex() {
	if s.cursorY == s.scrollTop {
		s.scrollDown(1)
		return
	}
	if s.cursorY > 0 {
		s.cursorY--
	}
}

// scrollUp moves the scroll region's content up by n lines, filling the
// newly exposed lines at the bottom with blank cells.
func (s *vtScreen) scrollUp(n int) {
	g := s.active()
	for y := s.scrollTop; y <= s.scrollBottom-n; y++ {
		copy(g[y*s.cols:(y+1)*s.cols], g[(y+n)*s.cols:(y+n+1)*s.cols])
	}
	for y := s.scrollBottom - n + 1; y <= s.scrollBottom; y++ {
		if y < s.scrollTop {
			continue
		}
		clearRow(g, s.cols, y)
	}
}

// scrollDown is scrollUp's mirror image, used by reverseIndex and the
// CSI 'T' scroll-down sequence.
func (s *vtScreen) scrollDown(n int) {
	g := s.active()
	for y := s.scrollBottom; y >= s.scrollTop+n; y-- {
		copy(g[y*s.cols:(y+1)*s.cols], g[(y-n)*s.cols:(y-n+1)*s.cols])
	}
	for y := s.scrollTop; y < s.scrollTop+n; y++ {
		if y > s.scrollBottom {
			continue
		}
		clearRow(g, s.cols, y)
	}
}

func clearRow(g []vtCell, cols, y int) {
	for x := 0; x < cols; x++ {
		g[y*cols+x] = vtCell{}
	}
}

// param returns CSI parameter i, or def if it wasn't supplied or was
// explicitly 0 — the VT100 convention where an omitted or zero count
// means "one" for movement/erase commands.
func (s *vtScreen) param(i, def int) int {
	if i >= len(s.csiParams) || s.csiParams[i] == 0 {
		return def
	}
	return s.csiParams[i]
}

// paramRaw is like param but returns 0 rather than def for an omitted
// argument — used where 0 and "omitted" mean different things (SGR's
// 38/48 sub-selector, DECSTBM's margins).
func (s *vtScreen) paramRaw(i, def int) int {
	if i >= len(s.csiParams) {
		return def
	}
	return s.csiParams[i]
}

// dispatchCSI runs the CSI sequence collected in s.csiParams, whose final
// byte is final.
func (s *vtScreen) dispatchCSI(final byte) {
	if s.csiPrivate {
		s.dispatchDECPrivate(final)
		return
	}
	switch final {
	case 'A': // CUU: cursor up
		s.cursorY -= s.param(0, 1)
		s.clampCursor()
	case 'B', 'e': // CUD / VPR: cursor down
		s.cursorY += s.param(0, 1)
		s.clampCursor()
	case 'C', 'a': // CUF / HPR: cursor forward
		s.cursorX += s.param(0, 1)
		s.clampCursor()
	case 'D': // CUB: cursor back
		s.cursorX -= s.param(0, 1)
		s.clampCursor()
	case 'E': // CNL: cursor next line
		s.cursorY += s.param(0, 1)
		s.cursorX = 0
		s.clampCursor()
	case 'F': // CPL: cursor previous line
		s.cursorY -= s.param(0, 1)
		s.cursorX = 0
		s.clampCursor()
	case 'G', '`': // CHA / HPA: cursor to column
		s.cursorX = s.param(0, 1) - 1
		s.clampCursor()
	case 'd': // VPA: cursor to row
		s.cursorY = s.param(0, 1) - 1
		s.clampCursor()
	case 'H', 'f': // CUP / HVP: cursor to row;col
		s.cursorY = s.param(0, 1) - 1
		s.cursorX = s.param(1, 1) - 1
		s.clampCursor()
	case 'J': // ED: erase in display
		s.eraseInDisplay(s.param(0, 0))
	case 'K': // EL: erase in line
		s.eraseInLine(s.param(0, 0))
	case 'S': // SU: scroll up
		s.scrollUp(s.param(0, 1))
	case 'T': // SD: scroll down
		s.scrollDown(s.param(0, 1))
	case 'L': // IL: insert n blank lines at the cursor
		s.insertLines(s.param(0, 1))
	case 'M': // DL: delete n lines at the cursor
		s.deleteLines(s.param(0, 1))
	case 'P': // DCH: delete n characters at the cursor
		s.deleteChars(s.param(0, 1))
	case '@': // ICH: insert n blank characters at the cursor
		s.insertChars(s.param(0, 1))
	case 'X': // ECH: erase n characters at the cursor
		s.eraseChars(s.param(0, 1))
	case 'm': // SGR: select graphic rendition
		s.selectGraphicRendition()
	case 'r': // DECSTBM: set scroll region
		top := s.paramRaw(0, 1) - 1
		bottom := s.paramRaw(1, s.rows) - 1
		if top < 0 {
			top = 0
		}
		if bottom >= s.rows {
			bottom = s.rows - 1
		}
		if top < bottom {
			s.scrollTop, s.scrollBottom = top, bottom
		} else {
			s.scrollTop, s.scrollBottom = 0, s.rows-1
		}
		s.cursorX, s.cursorY = 0, 0
	case 's': // SCP (ANSI.SYS-style save cursor; DECSTBM already handled ?-free above)
		s.savedX, s.savedY = s.cursorX, s.cursorY
	case 'u': // RCP
		s.cursorX, s.cursorY = s.savedX, s.savedY
	case 'n': // DSR: device status report — no reply channel wired up, silently ignored
	}
}

func (s *vtScreen) clampCursor() {
	if s.cursorX < 0 {
		s.cursorX = 0
	}
	if s.cursorX >= s.cols {
		s.cursorX = s.cols - 1
	}
	if s.cursorY < 0 {
		s.cursorY = 0
	}
	if s.cursorY >= s.rows {
		s.cursorY = s.rows - 1
	}
}

// dispatchDECPrivate handles a CSI sequence with a '?' prefix — DEC
// private modes. Only the ones real interactive programs actually flip
// are implemented; every other private mode is silently ignored, the
// same tolerance real terminals show toward modes they don't support.
func (s *vtScreen) dispatchDECPrivate(final byte) {
	if final != 'h' && final != 'l' {
		return
	}
	set := final == 'h'
	for _, mode := range s.csiParams {
		switch mode {
		case 1: // DECCKM: application cursor keys
			s.appCursorKeys = set
		case 25: // DECTCEM: cursor visibility
			s.cursorVisible = set
		case 47, 1047, 1049: // alternate screen buffer
			if set && !s.usingAlt {
				s.usingAlt = true
				for y := 0; y < s.rows; y++ {
					clearRow(s.altGrid, s.cols, y)
				}
				s.cursorX, s.cursorY = 0, 0
			} else if !set && s.usingAlt {
				s.usingAlt = false
			}
		}
	}
}

// eraseInDisplay implements ED (mode 0 = cursor to end, 1 = start to
// cursor, 2/3 = the whole screen).
func (s *vtScreen) eraseInDisplay(mode int) {
	g := s.active()
	switch mode {
	case 0:
		s.eraseInLine(0)
		for y := s.cursorY + 1; y < s.rows; y++ {
			clearRow(g, s.cols, y)
		}
	case 1:
		s.eraseInLine(1)
		for y := 0; y < s.cursorY; y++ {
			clearRow(g, s.cols, y)
		}
	case 2, 3:
		for y := 0; y < s.rows; y++ {
			clearRow(g, s.cols, y)
		}
	}
}

// eraseInLine implements EL (mode 0 = cursor to end of line, 1 = start of
// line to cursor, 2 = the whole line).
func (s *vtScreen) eraseInLine(mode int) {
	g := s.active()
	row := s.cursorY * s.cols
	switch mode {
	case 0:
		for x := s.cursorX; x < s.cols; x++ {
			g[row+x] = vtCell{}
		}
	case 1:
		for x := 0; x <= s.cursorX && x < s.cols; x++ {
			g[row+x] = vtCell{}
		}
	case 2:
		clearRow(g, s.cols, s.cursorY)
	}
}

func (s *vtScreen) insertLines(n int) {
	if s.cursorY < s.scrollTop || s.cursorY > s.scrollBottom {
		return
	}
	top, bottom := s.scrollTop, s.scrollBottom
	s.scrollTop = s.cursorY
	s.scrollDown(n)
	s.scrollTop = top
	_ = bottom
}

func (s *vtScreen) deleteLines(n int) {
	if s.cursorY < s.scrollTop || s.cursorY > s.scrollBottom {
		return
	}
	top := s.scrollTop
	s.scrollTop = s.cursorY
	s.scrollUp(n)
	s.scrollTop = top
}

func (s *vtScreen) insertChars(n int) {
	g := s.active()
	row := s.cursorY * s.cols
	for x := s.cols - 1; x >= s.cursorX+n; x-- {
		g[row+x] = g[row+x-n]
	}
	for x := s.cursorX; x < s.cursorX+n && x < s.cols; x++ {
		g[row+x] = vtCell{}
	}
}

func (s *vtScreen) deleteChars(n int) {
	g := s.active()
	row := s.cursorY * s.cols
	for x := s.cursorX; x < s.cols-n; x++ {
		g[row+x] = g[row+x+n]
	}
	for x := s.cols - n; x < s.cols; x++ {
		if x >= s.cursorX {
			g[row+x] = vtCell{}
		}
	}
}

func (s *vtScreen) eraseChars(n int) {
	g := s.active()
	row := s.cursorY * s.cols
	for x := s.cursorX; x < s.cursorX+n && x < s.cols; x++ {
		g[row+x] = vtCell{}
	}
}

// ansiPalette is the standard 16-color ANSI palette (black/red/green/
// yellow/blue/magenta/cyan/white, then their bright counterparts) that
// SGR codes 30-37/40-47/90-97/100-107 index into — the palette most
// terminal programs assume when they ask for "red" rather than a specific
// RGB triple.
var ansiPalette = [16]Color{
	RGB(0, 0, 0), RGB(205, 49, 49), RGB(13, 188, 121), RGB(229, 229, 16),
	RGB(36, 114, 200), RGB(188, 63, 188), RGB(17, 168, 205), RGB(229, 229, 229),
	RGB(102, 102, 102), RGB(241, 76, 76), RGB(35, 209, 139), RGB(245, 245, 67),
	RGB(59, 142, 234), RGB(214, 112, 214), RGB(41, 184, 219), RGB(255, 255, 255),
}

// ansi256Color resolves an xterm 256-color palette index: 0-15 are
// ansiPalette itself, 16-231 a 6×6×6 RGB cube, 232-255 a 24-step
// grayscale ramp — the standard xterm-256color layout.
func ansi256Color(n int) Color {
	switch {
	case n < 16:
		return ansiPalette[n]
	case n < 232:
		n -= 16
		levels := [6]uint8{0, 95, 135, 175, 215, 255}
		r, g, b := levels[n/36], levels[(n/6)%6], levels[n%6]
		return RGB(r, g, b)
	default:
		v := uint8(8 + (n-232)*10)
		return RGB(v, v, v)
	}
}

// selectGraphicRendition implements SGR: updates the current
// fg/bg/bold/underline/reverse state that every subsequently printed
// character picks up (see printRune).
func (s *vtScreen) selectGraphicRendition() {
	if len(s.csiParams) == 0 {
		s.resetRendition()
		return
	}
	for i := 0; i < len(s.csiParams); i++ {
		code := s.csiParams[i]
		switch {
		case code == 0:
			s.resetRendition()
		case code == 1:
			s.curBold = true
		case code == 4:
			s.curUnderline = true
		case code == 7:
			s.curReverse = true
		case code == 22:
			s.curBold = false
		case code == 24:
			s.curUnderline = false
		case code == 27:
			s.curReverse = false
		case code == 39:
			s.curFg = ColorNone
		case code == 49:
			s.curBg = ColorNone
		case code >= 30 && code <= 37:
			s.curFg = ansiPalette[code-30]
		case code >= 40 && code <= 47:
			s.curBg = ansiPalette[code-40]
		case code >= 90 && code <= 97:
			s.curFg = ansiPalette[8+code-90]
		case code >= 100 && code <= 107:
			s.curBg = ansiPalette[8+code-100]
		case code == 38 || code == 48:
			consumed, color := s.extendedColor(s.csiParams[i+1:])
			i += consumed
			if code == 38 {
				s.curFg = color
			} else {
				s.curBg = color
			}
		}
	}
}

func (s *vtScreen) resetRendition() {
	s.curFg, s.curBg = ColorNone, ColorNone
	s.curBold, s.curUnderline, s.curReverse = false, false, false
}

// extendedColor parses the parameters following an SGR 38/48 (256-color
// "5;n" or truecolor "2;r;g;b"), returning how many of rest it consumed
// (for the caller to skip) and the resolved color. An unrecognized or
// truncated form consumes nothing and resolves to ColorNone, leaving the
// current color unchanged in the caller's eyes.
func (s *vtScreen) extendedColor(rest []int) (consumed int, color Color) {
	if len(rest) == 0 {
		return 0, ColorNone
	}
	switch rest[0] {
	case 5:
		if len(rest) < 2 {
			return 1, ColorNone
		}
		return 2, ansi256Color(rest[1])
	case 2:
		if len(rest) < 4 {
			return len(rest), ColorNone
		}
		return 4, RGB(uint8(rest[1]), uint8(rest[2]), uint8(rest[3]))
	default:
		return 0, ColorNone
	}
}

// dispatchOSC interprets the OSC payload collected in s.oscBuf. Only
// "set title" (0 and 2) is recognized; everything else (color palette
// queries, hyperlinks, ...) is consumed without visible effect, which is
// enough to keep it from leaking into the grid as garbage text.
func (s *vtScreen) dispatchOSC() {
	buf := s.oscBuf
	i := 0
	for i < len(buf) && buf[i] != ';' {
		i++
	}
	if i >= len(buf) {
		return
	}
	switch string(buf[:i]) {
	case "0", "2":
		s.title = string(buf[i+1:])
	}
}
