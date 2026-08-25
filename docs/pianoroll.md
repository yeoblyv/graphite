# PianoRoll

`PianoRoll` is a piano-keyboard widget: a standard white/black key layout,
horizontal or vertical, that shows at least a floor number of keys and
grows automatically as its parent offers it more space. It can be played
by mouse, the PC keyboard, or driven programmatically — which is how real
MIDI hardware input is wired in. This document covers the widget itself;
its two optional companion packages — [`graphite/audio`](#playing-real-sound-the-graphiteaudio-package)
for actual sound and [`graphite/midi`](#real-midi-hardware-the-graphitemidi-package)
for real MIDI devices — are covered at the end.

## Quickstart

```go
piano := Graphite.NewPianoRoll(0, 2, 0, 10, Graphite.PianoHorizontal, 48) // C3
piano.OnNoteOn = func(note, velocity uint8) {
	fmt.Println("note on:", Graphite.NoteName(note), velocity)
}
piano.OnNoteOff = func(note uint8) {
	fmt.Println("note off:", Graphite.NoteName(note))
}
win.AddWidget(piano)
```

Click a key, or (once focused) type on the PC keyboard: the bottom
QWERTY row (`Z X C V B N M , . /`) plays one octave, the row above (`Q W E
R T Y U I O P` plus the number row for its black keys) plays the next
octave up. `Left`/`Right` transpose the whole keyboard by a semitone.

## Orientation

```go
type PianoOrientation int

const (
	PianoHorizontal PianoOrientation = iota // keys left-to-right, lowest note on the left
	PianoVertical                           // keys bottom-to-top, lowest note at the bottom
)
```

`PianoVertical` puts the lowest note at the *bottom* and increasing pitch
upward — matching a piano-roll editor's pitch axis (and `Fader`'s own
`rowForPercent`, where 100% is the top row), not top-to-bottom reading
order.

## Adaptive key count

```go
type PianoRoll struct {
	// ...
	MinKeys    int   // floor; default 25
	LowestNote uint8 // MIDI note of the leftmost/bottommost key at MinKeys
	// ...
}
```

`MinKeys` is a **floor**, not a cap: however little space the widget is
offered, it always shows at least this many consecutive MIDI notes
(white and black combined) starting at `LowestNote`, shrinking each key
down to a 3-column/row minimum if needed. If more space is available than
`MinKeys` needs, `PianoRoll` automatically shows more notes, extending
upward in pitch — resize the terminal (or the `Flex`/`Panel` this widget
sits in) and the keyboard grows with it, with no code of your own needed
to react to the resize.

25, 37, 49, 61, 76, and 88 keys are the sizes real MIDI keyboard
controllers ship in (2 through 7¼ octaves); 25 — the default — matches
the smallest common controller (e.g. an AKAI MPK Mini). Set `MinKeys` to
one of the others if you want a taller floor, or lower for a more compact
minimum.

### If the offered space is smaller than MinKeys needs

The widget doesn't shrink below a 3-column/row-per-key minimum, so it can
end up needing more room than its parent actually gave it. When that
happens, the extra keys overflow past the widget's own far edge — to the
*right* for `PianoHorizontal`, and *downward past the bottom* for
`PianoVertical` — rather than growing backward over whatever was drawn
immediately before this widget in the window. This matters specifically
for `PianoVertical`: since its natural anchor is the bottom edge (lowest
note at the bottom), overflow could otherwise extend upward past the
widget's own top edge and paint over sibling widgets positioned above it
— `PianoRoll` deliberately anchors its topmost key at `AbsY` and lets any
overflow grow downward instead, specifically to avoid that. If you see a
`PianoVertical` genuinely clipping keys, the fix is to give it more
height (or lower `MinKeys`), not to worry about corrupted neighbors.

## Playing by mouse

Click a key to play it; releasing stops it. Dragging across keys while
held glissandos — releasing the previous key and starting the new one —
the same way a finger sliding across a real keyboard would. This is
ordinary click/drag handling, backed by `Window`'s implicit mouse capture
(see [architecture.md](architecture.md#mouse-routing-and-implicit-capture)),
nothing PianoRoll-specific to configure.

## Playing by PC keyboard

```go
KeyMap             map[rune]int // lowercased typed rune -> semitone offset
KeyboardOctaveBase uint8        // MIDI note KeyMap's offset 0 means
```

The default `KeyMap` is the widely used "typing keyboard as piano"
scheme from trackers and DAWs (Renoise, FL Studio, VCV Rack's Computer
Keyboard module):

```
Z S X D C V G B H N J M , L . ; /   (one octave, offsets 0-16)
Q 2 W 3 E R 5 T 6 Y 7 U I 9 O 0 P   (next octave up, offsets 12-28)
```

`KeyboardOctaveBase` (defaulting to `LowestNote` at construction) is the
MIDI note offset `0` in `KeyMap` corresponds to — set it independently of
`LowestNote` if you want the playable keyboard range and the *visible*
key range to differ (e.g. a wide visible keyboard where only the middle
portion is reachable from the PC keyboard's fixed two-octave span).
Replace `KeyMap` entirely, or edit individual entries, to use a different
layout.

### The one real limitation: no true key-up event

Raw terminal input has no key-release notification — only a stream of
key-down/repeat bytes as long as a key is held (relying on the OS's own
keyboard auto-repeat). `PianoRoll` approximates "held" by refreshing a
per-note timestamp on every repeat and releasing the note once
`KeyReleaseTimeout` (default 150ms) passes with no repeat seen:

```go
KeyReleaseTimeout time.Duration // default 150ms
```

This works correctly as long as the OS's keyboard repeat rate is faster
than `KeyReleaseTimeout` — true for every common OS default. If a user has
configured an unusually slow repeat rate, raise `KeyReleaseTimeout` to
compensate; there is no way to get a truly exact key-up event over raw
terminal input, so this remains a heuristic, not a measurement. Mouse
input and external `NoteOn`/`NoteOff` calls (real MIDI, see below) don't
have this problem — they have genuine press/release or on/off events.

## Driving it programmatically (and real MIDI input)

```go
func (p *PianoRoll) NoteOn(note, velocity uint8)
func (p *PianoRoll) NoteOff(note uint8)
```

Call these directly to highlight a key and fire the same `OnNoteOn`/
`OnNoteOff` callbacks as a mouse click or keyboard press — this is how
`graphite/midi` (below) feeds real hardware input into a `PianoRoll`:
mouse, keyboard, and external sources all converge into one "is this note
currently sounding" state machine, so a note held by two sources at once
(say, a mouse click on a note the MIDI controller is also holding) fires
`OnNoteOn`/`OnNoteOff` exactly once each, not once per source.

**Concurrency note**: like any widget state, `NoteOn`/`NoteOff` are only
safe to call from the render loop's own goroutine — a real MIDI driver
delivers messages on its own goroutine, so route through
`Application.Invoke` (see [architecture.md](architecture.md#concurrency-and-invoke)).
`graphite/midi`'s `Listen` does this for you automatically.

## `OnNoteOn`/`OnNoteOff`: wiring up sound

```go
OnNoteOn  func(note uint8, velocity uint8)
OnNoteOff func(note uint8)
```

`PianoRoll` itself makes no sound — Graphite has no audio pipeline (see
`Fader`'s own doc comment for the same point). Wire these to the
`graphite/audio` package's `Synth` for real playback:

```go
synth, err := audio.NewSynth()
if err != nil {
	// ...
}
defer synth.Close()

piano.OnNoteOn = synth.NoteOn
piano.OnNoteOff = synth.NoteOff
```

Or wire them to your own logic — logging, a custom synth, forwarding to
an external MIDI output, anything with a matching signature.

## `NoteName`

```go
func NoteName(note uint8) string
```

Formats a MIDI note number as a pitch class plus octave, e.g.
`NoteName(60) == "C4"`, using the widely used convention where note 60
(middle C) is octave 4. Handy for status text, logging, or labeling —
`showcase`'s Piano tab uses it for exactly that.

## Transposing

`Left`/`Right` (while the widget has focus) nudge `LowestNote` by one
semitone, transposing the whole visible keyboard — useful for reaching
notes outside a narrow widget's visible range without needing a wider
terminal.

---

## Playing real sound: the `graphite/audio` package

```go
import "github.com/yeoblyv/graphite/audio"

synth, err := audio.NewSynth()
if err != nil {
	// ...
}
defer synth.Close()

piano.OnNoteOn = synth.NoteOn
piano.OnNoteOff = synth.NoteOff
```

`audio.Synth` is a minimal polyphonic sine-wave synthesizer built on
[oto](https://github.com/hajimehoshi/oto) that plays MIDI note numbers
through the system's real audio output — a short linear attack (5ms) and
release (80ms) envelope per note, scaled by MIDI velocity. It is a
**separate Go module** (`github.com/yeoblyv/graphite/audio`, its own
`go.mod`), not a subpackage of the core `graphite` module, specifically so
that programs using `PianoRoll` (or anything else in Graphite) without
needing real audio playback never pull in an audio backend they don't
want.

`oto` needs no cgo on Windows or macOS; on Linux it needs the ALSA
development headers (`apt install libasound2-dev` / `dnf install
alsa-lib-devel`).

Only one `Synth` (one `oto.Context`) may exist per process — construct it
once, near the top of `main`, and `defer synth.Close()`.

## Real MIDI hardware: the `graphite/midi` package

```go
import "github.com/yeoblyv/graphite/midi"

listener, err := midi.Listen("", app.Invoke, piano.NoteOn, piano.NoteOff)
if err != nil {
	// ...
}
defer listener.Close()
defer midi.CloseDriver()
```

`midi.Listen` opens a real MIDI input port (matched by substring against
its name — pass `""` to open whatever sorts first, fine with only one
device connected; use `midi.ListPorts()` to list what's available and let
the user choose) and forwards every Note On/Off message it receives to
the callbacks you pass, wrapped in `invoke` (pass `Application.Invoke` —
see the concurrency note above and in
[architecture.md](architecture.md#concurrency-and-invoke): the driver
delivers messages on its own goroutine, not the render loop's, so this
hand-off is required, not optional).

This package is **also a separate Go module**
(`github.com/yeoblyv/graphite/midi`), for the same reason as
`graphite/audio`: it wraps
[gitlab.com/gomidi/midi/v2](https://gitlab.com/gomidi/midi/v2)'s `rtmidi`
driver, which requires cgo everywhere (plus, on Linux, the same ALSA
headers `graphite/audio` needs) — kept out of the core module so it's
opt-in, not a tax on every consumer.

```go
piano, synth := setUpPianoAndSynth() // your own code
listener, err := midi.Listen("", app.Invoke, piano.NoteOn, piano.NoteOff)
if err != nil {
	app.ShowMessage(" MIDI ", err.Error(), Graphite.BtnDanger)
} else {
	defer listener.Close()
}
defer midi.CloseDriver()

piano.OnNoteOn = synth.NoteOn // still wire the widget to the synth for sound
piano.OnNoteOff = synth.NoteOff
```

Since `PianoRoll.NoteOn`/`NoteOff` fire the same `OnNoteOn`/`OnNoteOff`
callbacks regardless of source, a single `synth.NoteOn`/`NoteOff` pair
wired to the widget picks up mouse, keyboard, *and* real MIDI input
uniformly — you don't need to wire the synth to the MIDI listener
directly.
