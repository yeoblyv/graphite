package Graphite

import (
	"fmt"
	"sort"
	"time"
)

// PianoOrientation selects which axis a PianoRoll lays its keys along.
type PianoOrientation int

// Supported PianoRoll orientations.
const (
	// PianoHorizontal lays keys left-to-right, lowest note on the left —
	// a conventional piano keyboard.
	PianoHorizontal PianoOrientation = iota
	// PianoVertical lays keys bottom-to-top, lowest note at the bottom —
	// matching the pitch axis of a piano-roll editor or a vertically
	// mounted keyboard controller.
	PianoVertical
)

// pianoIsBlackKey reports, for a note's position within an octave (0=C
// through 11=B), whether it's a black key.
var pianoIsBlackKey = [12]bool{
	false, true, false, true, false,
	false, true, false, true, false, true, false,
}

// defaultPianoKeyMap is the standard "typing keyboard as piano" layout used
// by trackers and DAWs (Renoise, FL Studio, VCV Rack's Computer Keyboard
// module): the bottom QWERTY row plays one octave starting at semitone 0,
// the row above plays the next octave up starting at semitone 12, with the
// number row filling in that octave's black keys.
func defaultPianoKeyMap() map[rune]int {
	return map[rune]int{
		'z': 0, 's': 1, 'x': 2, 'd': 3, 'c': 4,
		'v': 5, 'g': 6, 'b': 7, 'h': 8, 'n': 9, 'j': 10, 'm': 11,
		',': 12, 'l': 13, '.': 14, ';': 15, '/': 16,

		'q': 12, '2': 13, 'w': 14, '3': 15, 'e': 16,
		'r': 17, '5': 18, 't': 19, '6': 20, 'y': 21, '7': 22, 'u': 23,
		'i': 24, '9': 25, 'o': 26, '0': 27, 'p': 28,
	}
}

// pianoKeyRect is one key's resolved on-screen rectangle, computed fresh by
// layoutKeys on every DrawRelative.
type pianoKeyRect struct {
	note    uint8
	isBlack bool
	x, y    int
	w, h    int
}

// PianoRoll is a piano-keyboard widget: it renders a standard white/black
// key pattern (horizontally or vertically), shows at least MinKeys notes
// and adds more automatically as its parent offers it more space, and can
// be played by mouse clicks, the PC keyboard, or driven programmatically —
// which is how a real MIDI input device is wired in (see the graphite/midi
// package). Sound is not part of this widget: wire OnNoteOn/OnNoteOff to a
// synth (see the graphite/audio package) or your own audio code.
type PianoRoll struct {
	BaseWidget

	Orientation PianoOrientation

	// MinKeys is the minimum number of consecutive MIDI notes (white and
	// black combined) always shown, starting at LowestNote — a floor, not
	// a cap. If the widget is offered more space than MinKeys needs, more
	// notes are added automatically, extending upward in pitch. 25, 37,
	// 49, 61, 76, and 88 are the sizes real MIDI keyboard controllers
	// ship in (2/3/4/5/6.5/7.25 octaves respectively); 25 is a sensible
	// floor matching the smallest common controller.
	MinKeys int

	// LowestNote is the MIDI note number (0-127, 60 = middle C) of the
	// leftmost (horizontal) or bottommost (vertical) key when exactly
	// MinKeys are shown.
	LowestNote uint8

	// KeyMap maps a lowercased typed rune to a semitone offset from
	// KeyboardOctaveBase. Defaults to defaultPianoKeyMap(); replace it
	// for a different layout, or set entries to remap individual keys.
	KeyMap map[rune]int
	// KeyboardOctaveBase is the MIDI note KeyMap's offset 0 corresponds
	// to. Defaults to LowestNote at construction.
	KeyboardOctaveBase uint8

	// Velocity is used for notes triggered by mouse or PC-keyboard, which
	// have no natural velocity of their own (unlike a real MIDI
	// controller, which reports how hard a key was struck).
	Velocity uint8

	// KeyReleaseTimeout is how long a PC-keyboard-triggered note keeps
	// sounding after the last repeat event for its key, before being
	// treated as released. Raw terminal input has no true key-up event —
	// only a stream of key-down/repeat bytes — so this is a heuristic,
	// not an exact measurement: the OS's keyboard repeat rate must be
	// faster than this timeout for a held key to sustain correctly.
	// Defaults to 150ms, which comfortably outlasts every common OS
	// repeat rate without noticeably delaying release on key-up.
	KeyReleaseTimeout time.Duration

	// OnNoteOn fires the instant a note starts sounding from any source
	// (mouse, keyboard, or a NoteOn call from outside, e.g. real MIDI
	// input) — not once per source, so mousing down on a key already
	// held via the keyboard does not re-trigger it.
	OnNoteOn func(note uint8, velocity uint8)
	// OnNoteOff fires the instant a note stops sounding from every
	// source that was holding it.
	OnNoteOff func(note uint8)

	mouseNote    *uint8
	keyboardHeld map[uint8]time.Time
	externalHeld map[uint8]bool

	numKeys       int
	keys          []pianoKeyRect
	effectiveSpan int
}

// NewPianoRoll creates a PianoRoll at (x, y) with the given width/height
// (either may be <= 0 to stretch — see layout.md), showing at least 25
// keys starting at lowestNote.
func NewPianoRoll(x, y, w, h int, orientation PianoOrientation, lowestNote uint8) *PianoRoll {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = true
	return &PianoRoll{
		BaseWidget:         base,
		Orientation:        orientation,
		MinKeys:            25,
		LowestNote:         lowestNote,
		KeyMap:             defaultPianoKeyMap(),
		KeyboardOctaveBase: lowestNote,
		Velocity:           100,
		KeyReleaseTimeout:  150 * time.Millisecond,
		keyboardHeld:       make(map[uint8]time.Time),
		externalHeld:       make(map[uint8]bool),
	}
}

// isHeld reports whether note is currently sounding from any source.
func (p *PianoRoll) isHeld(note uint8) bool {
	if p.mouseNote != nil && *p.mouseNote == note {
		return true
	}
	if _, ok := p.keyboardHeld[note]; ok {
		return true
	}
	return p.externalHeld[note]
}

// fireNoteOn calls OnNoteOn only if note wasn't already sounding from some
// other source.
func (p *PianoRoll) fireNoteOn(note, velocity uint8) {
	if p.isHeld(note) {
		return
	}
	if p.OnNoteOn != nil {
		p.OnNoteOn(note, velocity)
	}
}

// fireNoteOffIfSilent calls OnNoteOff only once every source has released
// note.
func (p *PianoRoll) fireNoteOffIfSilent(note uint8) {
	if p.isHeld(note) {
		return
	}
	if p.OnNoteOff != nil {
		p.OnNoteOff(note)
	}
}

// NoteOn marks note as sounding from an external source (e.g. a real MIDI
// input device — see the graphite/midi package) and highlights its key.
// Safe to call for a note already sounding from another source; safe to
// call from any goroutine only via Application.Invoke, like any other
// widget mutation (see architecture.md's concurrency section) — a MIDI
// driver's callback runs on its own goroutine, not the render loop's.
func (p *PianoRoll) NoteOn(note, velocity uint8) {
	p.fireNoteOn(note, velocity)
	p.externalHeld[note] = true
}

// NoteOff releases note from the external source. See NoteOn for the
// goroutine-safety note.
func (p *PianoRoll) NoteOff(note uint8) {
	delete(p.externalHeld, note)
	p.fireNoteOffIfSilent(note)
}

// releaseStaleKeyboardNotes drops any PC-keyboard-triggered note that
// hasn't seen a repeat within KeyReleaseTimeout — the closest approximation
// to a key-up event raw terminal input can offer (see KeyReleaseTimeout's
// doc comment).
func (p *PianoRoll) releaseStaleKeyboardNotes() {
	timeout := p.KeyReleaseTimeout
	if timeout <= 0 {
		timeout = 150 * time.Millisecond
	}
	now := time.Now()
	for note, lastSeen := range p.keyboardHeld {
		if now.Sub(lastSeen) > timeout {
			delete(p.keyboardHeld, note)
			p.fireNoteOffIfSilent(note)
		}
	}
}

// countWhiteKeys counts how many of the n consecutive notes starting at
// from are white keys.
func countWhiteKeys(from uint8, n int) int {
	count := 0
	for i := 0; i < n; i++ {
		note := int(from) + i
		if note > 127 {
			break
		}
		if !pianoIsBlackKey[note%12] {
			count++
		}
	}
	return count
}

// pianoMinKeyCell is the minimum columns (horizontal) or rows (vertical) a
// white key is ever drawn at, however little space is offered — below
// this the layout stops shrinking and simply clips, the same tradeoff
// every other widget in this library makes under extreme size constraints.
const pianoMinKeyCell = 3

// layoutKeys resolves how many notes fit in the space DrawRelative just
// gave this widget (at least MinKeys, more if there's room) and computes
// every key's absolute rectangle for both drawing and hit-testing.
func (p *PianoRoll) layoutKeys() {
	span := p.LastW
	if p.Orientation == PianoVertical {
		span = p.LastH
	}

	minKeys := p.MinKeys
	if minKeys < 1 {
		minKeys = 1
	}
	whiteKeysNeeded := countWhiteKeys(p.LowestNote, minKeys)
	if whiteKeysNeeded < 1 {
		whiteKeysNeeded = 1
	}

	maxWhiteKeysFit := span / pianoMinKeyCell
	numWhiteKeys := whiteKeysNeeded
	if maxWhiteKeysFit > numWhiteKeys {
		numWhiteKeys = maxWhiteKeysFit
	}

	cell := span / numWhiteKeys
	if cell < pianoMinKeyCell {
		cell = pianoMinKeyCell
	}

	// If MinKeys still doesn't fit even at the minimum cell size, the
	// layout below overflows past the offered span rather than shrinking
	// further — effectiveSpan is the larger of the two, so that overflow
	// always extends past the FAR edge (the right edge for horizontal,
	// the top edge for vertical — the direction a bigger keyboard already
	// grows in) instead of bleeding backward over whatever was drawn
	// immediately before this widget.
	p.effectiveSpan = span
	if needed := numWhiteKeys * cell; needed > p.effectiveSpan {
		p.effectiveSpan = needed
	}

	p.keys = p.keys[:0]
	whiteCount := 0
	note := int(p.LowestNote)
	var whiteCursor int
	for whiteCount < numWhiteKeys && note <= 127 {
		isBlack := pianoIsBlackKey[note%12]
		if !isBlack {
			rect := p.keyRectAt(whiteCursor, cell, false)
			rect.note = uint8(note)
			p.keys = append(p.keys, rect)
			whiteCursor++
			whiteCount++
		} else {
			// A black key sits at the boundary between the previous and
			// next white key, drawn shorter and centered on that
			// boundary, matching a real keyboard's overlap.
			rect := p.keyRectAt(whiteCursor, cell, true)
			rect.note = uint8(note)
			p.keys = append(p.keys, rect)
		}
		note++
	}
	p.numKeys = len(p.keys)

	// Black keys are drawn (and hit-tested) after white keys so they
	// correctly appear on top and win ties at the boundary they overlap.
	sort.SliceStable(p.keys, func(i, j int) bool { return !p.keys[i].isBlack && p.keys[j].isBlack })
}

// keyRectAt computes one key's absolute rectangle. whiteIndex is the
// 0-based position of the white key this key sits at or beside; cell is
// the resolved column/row size of one white key.
func (p *PianoRoll) keyRectAt(whiteIndex, cell int, isBlack bool) pianoKeyRect {
	blackLen := cell / 2
	if blackLen < 1 {
		blackLen = 1
	}

	if p.Orientation == PianoHorizontal {
		// x grows rightward from AbsX, so overflow when MinKeys doesn't
		// fit naturally extends past the right edge, never left of AbsX.
		x := p.AbsX + whiteIndex*cell
		if isBlack {
			return pianoKeyRect{isBlack: true, x: x - blackLen/2, y: p.AbsY, w: blackLen, h: (p.LastH * 3) / 5}
		}
		return pianoKeyRect{x: x, y: p.AbsY, w: cell, h: p.LastH}
	}

	// Vertical: lowest note at the bottom, increasing upward. Anchored to
	// effectiveSpan (>= LastH; only larger when MinKeys needs more room
	// than was offered) instead of LastH directly, so the topmost key
	// never starts above AbsY — any extra height MinKeys needs is added
	// below AbsY+LastH instead, extending the piano's total footprint
	// downward past its own bottom edge rather than upward past its top,
	// where it would overdraw whatever was drawn before this widget.
	y := p.AbsY + p.effectiveSpan - (whiteIndex+1)*cell
	if isBlack {
		return pianoKeyRect{isBlack: true, x: p.AbsX, y: y + cell - blackLen/2, w: (p.LastW * 3) / 5, h: blackLen}
	}
	return pianoKeyRect{x: p.AbsX, y: y, w: p.LastW, h: cell}
}

// DrawRelative implements Widget.
func (p *PianoRoll) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	p.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	p.releaseStaleKeyboardNotes()
	p.layoutKeys()

	whiteBg, blackBg := RGB(225, 225, 225), RGB(20, 20, 20)
	whiteFg, blackFg := RGB(20, 20, 20), RGB(225, 225, 225)
	border := RGB(90, 90, 90)

	for _, k := range p.keys {
		bg, fg := whiteBg, whiteFg
		if k.isBlack {
			bg, fg = blackBg, blackFg
		}
		if p.isHeld(k.note) {
			bg = c.theme.Primary
			if k.isBlack {
				bg = c.theme.Primary.Darken(0.35)
			}
		}
		for iy := 0; iy < k.h; iy++ {
			for ix := 0; ix < k.w; ix++ {
				ch := " "
				if !k.isBlack && ix == 0 && p.Orientation == PianoHorizontal {
					ch = "│"
				}
				if !k.isBlack && iy == 0 && p.Orientation == PianoVertical {
					ch = "─"
				}
				cellFg := fg
				if ch != " " {
					cellFg = border
				}
				c.DrawCell(k.x+ix, k.y+iy, ch, bg, cellFg)
			}
		}
	}
}

// noteRuneKey normalizes ev's printable character to the lowercase rune
// KeyMap is keyed on. Returns 0, false for non-printable events.
func noteRuneKey(ev Event) (rune, bool) {
	if ev.Type != EventKey || ev.CharCode == 0 {
		return 0, false
	}
	r := ev.CharCode
	if r >= 'A' && r <= 'Z' {
		r += 'a' - 'A'
	}
	return r, true
}

// hitTestKey returns the topmost (last-drawn, i.e. black-key-priority) key
// rect under (mx, my), or nil.
func (p *PianoRoll) hitTestKey(mx, my int) *pianoKeyRect {
	for i := len(p.keys) - 1; i >= 0; i-- {
		k := &p.keys[i]
		if mx >= k.x && mx < k.x+k.w && my >= k.y && my < k.y+k.h {
			return k
		}
	}
	return nil
}

// HandleEvent implements Widget: a mouse press/drag plays whichever key is
// under the pointer (dragging across keys glissandos, releasing the
// previous key first), a mapped keyboard character plays its note for as
// long as it keeps repeating (see KeyReleaseTimeout), and Left/Right nudge
// LowestNote by one semitone, transposing the whole keyboard.
func (p *PianoRoll) HandleEvent(ev Event) {
	switch ev.Type {
	case EventMouseDown, EventMouseDrag:
		k := p.hitTestKey(ev.MouseX, ev.MouseY)
		if k == nil {
			if ev.Type == EventMouseDown && p.mouseNote != nil {
				prev := *p.mouseNote
				p.mouseNote = nil
				p.fireNoteOffIfSilent(prev)
			}
			return
		}
		if p.mouseNote != nil && *p.mouseNote == k.note {
			return
		}
		if p.mouseNote != nil {
			prev := *p.mouseNote
			p.mouseNote = nil
			p.fireNoteOffIfSilent(prev)
		}
		note := k.note
		p.fireNoteOn(note, p.velocityOrDefault())
		p.mouseNote = &note
	case EventMouseUp:
		if p.mouseNote != nil {
			prev := *p.mouseNote
			p.mouseNote = nil
			p.fireNoteOffIfSilent(prev)
		}
	case EventKey:
		switch ev.Key {
		case KeyLeft:
			if p.LowestNote > 0 {
				p.LowestNote--
			}
			return
		case KeyRight:
			if p.LowestNote < 127 {
				p.LowestNote++
			}
			return
		}
		r, ok := noteRuneKey(ev)
		if !ok {
			return
		}
		offset, ok := p.KeyMap[r]
		if !ok {
			return
		}
		note := int(p.KeyboardOctaveBase) + offset
		if note < 0 || note > 127 {
			return
		}
		p.fireNoteOn(uint8(note), p.velocityOrDefault())
		p.keyboardHeld[uint8(note)] = time.Now()
	}
}

func (p *PianoRoll) velocityOrDefault() uint8 {
	if p.Velocity == 0 {
		return 100
	}
	return p.Velocity
}

// noteNames are the 12 pitch-class names, sharps preferred over flats —
// used by NoteName.
var noteNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// NoteName formats a MIDI note number (0-127) as a pitch class plus octave
// number, e.g. NoteName(60) == "C4", using the widely used convention
// where note 60 (middle C) is octave 4.
func NoteName(note uint8) string {
	octave := int(note)/12 - 1
	return fmt.Sprintf("%s%d", noteNames[note%12], octave)
}
