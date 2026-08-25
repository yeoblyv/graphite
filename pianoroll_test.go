package Graphite

import (
	"testing"
	"time"
)

// newTestPianoRoll creates a PianoRoll and resolves its layout via a real
// DrawRelative call, the same way Window would before routing any event to
// it.
func newTestPianoRoll(orientation PianoOrientation, w, h int) *PianoRoll {
	p := NewPianoRoll(0, 0, w, h, orientation, 60) // middle C
	p.DrawRelative(NewCanvas(), 0, 0, w, h)
	return p
}

func TestPianoRoll_ShowsAtLeastMinKeys(t *testing.T) {
	p := newTestPianoRoll(PianoHorizontal, 30, 10) // deliberately too narrow to fit 25 at full width
	if p.numKeys < p.MinKeys {
		t.Fatalf("numKeys = %d, want at least MinKeys (%d) even when space is tight", p.numKeys, p.MinKeys)
	}
}

func TestPianoRoll_AddsMoreKeysWhenGivenMoreSpace(t *testing.T) {
	narrow := newTestPianoRoll(PianoHorizontal, 60, 10)
	wide := newTestPianoRoll(PianoHorizontal, 600, 10)

	if wide.numKeys <= narrow.numKeys {
		t.Fatalf("wide.numKeys = %d, narrow.numKeys = %d; want more keys with more offered width", wide.numKeys, narrow.numKeys)
	}
	if narrow.numKeys < narrow.MinKeys {
		t.Fatalf("narrow.numKeys = %d, want at least MinKeys (%d)", narrow.numKeys, narrow.MinKeys)
	}
}

func TestPianoRoll_VerticalLowestNoteAtBottom(t *testing.T) {
	p := newTestPianoRoll(PianoVertical, 10, 60)

	var lowestRect, highestRect *pianoKeyRect
	for i := range p.keys {
		k := &p.keys[i]
		if lowestRect == nil || k.note < lowestRect.note {
			lowestRect = k
		}
		if highestRect == nil || k.note > highestRect.note {
			highestRect = k
		}
	}
	if lowestRect == nil || highestRect == nil {
		t.Fatal("setup: expected at least two keys")
	}
	if lowestRect.y <= highestRect.y {
		t.Errorf("lowest note's y = %d, highest note's y = %d; want the lowest note drawn below (larger y than) the highest", lowestRect.y, highestRect.y)
	}
}

func TestPianoRoll_MouseClickPlaysAndReleasesNote(t *testing.T) {
	p := newTestPianoRoll(PianoHorizontal, 300, 10)
	var onEvents, offEvents []uint8
	p.OnNoteOn = func(note, velocity uint8) { onEvents = append(onEvents, note) }
	p.OnNoteOff = func(note uint8) { offEvents = append(offEvents, note) }

	k := p.keys[0]
	midX, midY := k.x+k.w/2, k.y+k.h/2

	p.HandleEvent(Event{Type: EventMouseDown, MouseX: midX, MouseY: midY})
	if len(onEvents) != 1 || onEvents[0] != k.note {
		t.Fatalf("after mouse down on key note=%d: onEvents = %v, want [%d]", k.note, onEvents, k.note)
	}

	p.HandleEvent(Event{Type: EventMouseUp, MouseX: midX, MouseY: midY})
	if len(offEvents) != 1 || offEvents[0] != k.note {
		t.Fatalf("after mouse up: offEvents = %v, want [%d]", offEvents, k.note)
	}
}

func TestPianoRoll_MouseDragGlissandosBetweenKeys(t *testing.T) {
	p := newTestPianoRoll(PianoHorizontal, 300, 10)
	var onEvents, offEvents []uint8
	p.OnNoteOn = func(note, velocity uint8) { onEvents = append(onEvents, note) }
	p.OnNoteOff = func(note uint8) { offEvents = append(offEvents, note) }

	if len(p.keys) < 2 {
		t.Fatal("setup: need at least two keys")
	}
	k0, k1 := p.keys[0], p.keys[1]

	p.HandleEvent(Event{Type: EventMouseDown, MouseX: k0.x + k0.w/2, MouseY: k0.y + k0.h/2})
	p.HandleEvent(Event{Type: EventMouseDrag, MouseX: k1.x + k1.w/2, MouseY: k1.y + k1.h/2})

	if len(offEvents) != 1 || offEvents[0] != k0.note {
		t.Fatalf("after dragging off key %d: offEvents = %v, want [%d] (the first key released)", k0.note, offEvents, k0.note)
	}
	if len(onEvents) != 2 || onEvents[0] != k0.note || onEvents[1] != k1.note {
		t.Fatalf("onEvents = %v, want [%d %d]", onEvents, k0.note, k1.note)
	}
}

func TestPianoRoll_KeyboardMappingPlaysMappedNote(t *testing.T) {
	p := newTestPianoRoll(PianoHorizontal, 300, 10)
	p.KeyboardOctaveBase = 60
	var onEvents []uint8
	p.OnNoteOn = func(note, velocity uint8) { onEvents = append(onEvents, note) }

	p.HandleEvent(Event{Type: EventKey, CharCode: 'z'}) // offset 0 in defaultPianoKeyMap
	if len(onEvents) != 1 || onEvents[0] != 60 {
		t.Fatalf("after 'z': onEvents = %v, want [60]", onEvents)
	}
}

func TestPianoRoll_KeyboardRepeatSustainsThenTimesOut(t *testing.T) {
	p := newTestPianoRoll(PianoHorizontal, 300, 10)
	p.KeyboardOctaveBase = 60
	p.KeyReleaseTimeout = 50 * time.Millisecond
	var offEvents []uint8
	p.OnNoteOff = func(note uint8) { offEvents = append(offEvents, note) }

	p.HandleEvent(Event{Type: EventKey, CharCode: 'z'})
	// Simulate the OS repeat rate refreshing the hold before the timeout.
	p.keyboardHeld[60] = time.Now()
	p.releaseStaleKeyboardNotes()
	if len(offEvents) != 0 {
		t.Fatalf("offEvents = %v after a fresh repeat, want none (still sustaining)", offEvents)
	}

	// Now let the "last seen" timestamp actually go stale.
	p.keyboardHeld[60] = time.Now().Add(-100 * time.Millisecond)
	p.releaseStaleKeyboardNotes()
	if len(offEvents) != 1 || offEvents[0] != 60 {
		t.Fatalf("offEvents = %v after the repeat stopped, want [60]", offEvents)
	}
}

func TestPianoRoll_ExternalNoteOnDoesNotDoubleFireWhileKeyboardHeld(t *testing.T) {
	p := newTestPianoRoll(PianoHorizontal, 300, 10)
	p.KeyboardOctaveBase = 60
	onCount, offCount := 0, 0
	p.OnNoteOn = func(note, velocity uint8) { onCount++ }
	p.OnNoteOff = func(note uint8) { offCount++ }

	p.HandleEvent(Event{Type: EventKey, CharCode: 'z'}) // note 60, keyboard-held
	p.NoteOn(60, 100)                                   // e.g. a MIDI device playing the same note
	if onCount != 1 {
		t.Fatalf("onCount = %d, want 1 (NoteOn while already keyboard-held must not re-fire)", onCount)
	}

	p.NoteOff(60) // MIDI releases, but keyboard is still holding it
	if offCount != 0 {
		t.Fatalf("offCount = %d, want 0 (still held by the keyboard)", offCount)
	}

	p.keyboardHeld = map[uint8]time.Time{} // simulate the keyboard also releasing
	p.fireNoteOffIfSilent(60)
	if offCount != 1 {
		t.Fatalf("offCount = %d, want 1 once every source has released", offCount)
	}
}

func TestPianoRoll_LeftRightTransposes(t *testing.T) {
	p := newTestPianoRoll(PianoHorizontal, 300, 10)
	p.LowestNote = 60

	p.HandleEvent(Event{Type: EventKey, Key: KeyRight})
	if p.LowestNote != 61 {
		t.Errorf("after KeyRight: LowestNote = %d, want 61", p.LowestNote)
	}
	p.HandleEvent(Event{Type: EventKey, Key: KeyLeft})
	p.HandleEvent(Event{Type: EventKey, Key: KeyLeft})
	if p.LowestNote != 59 {
		t.Errorf("after two KeyLeft: LowestNote = %d, want 59", p.LowestNote)
	}
}

func TestNoteName(t *testing.T) {
	cases := map[uint8]string{
		60:  "C4", // middle C
		69:  "A4", // concert pitch A
		61:  "C#4",
		0:   "C-1",
		127: "G9",
	}
	for note, want := range cases {
		if got := NoteName(note); got != want {
			t.Errorf("NoteName(%d) = %q, want %q", note, got, want)
		}
	}
}

func TestCountWhiteKeys(t *testing.T) {
	// C through B (one full octave, note 60-71): 7 white keys.
	if got := countWhiteKeys(60, 12); got != 7 {
		t.Errorf("countWhiteKeys(60, 12) = %d, want 7", got)
	}
	// Just C: 1 white key.
	if got := countWhiteKeys(60, 1); got != 1 {
		t.Errorf("countWhiteKeys(60, 1) = %d, want 1", got)
	}
	// Just C#: 0 white keys.
	if got := countWhiteKeys(61, 1); got != 0 {
		t.Errorf("countWhiteKeys(61, 1) = %d, want 0", got)
	}
}
