// Package midi is an optional companion to github.com/yeoblyv/graphite:
// it opens a real MIDI input device (via
// gitlab.com/gomidi/midi/v2 + its rtmidi driver) and forwards Note On/Off
// messages to plain callbacks, so a Graphite.PianoRoll can be played from
// physical hardware in addition to the mouse and PC keyboard it already
// supports on its own.
//
// This package requires cgo (rtmidi is a cgo binding) and, on Linux, the
// ALSA development headers (apt install libasound2-dev / dnf install
// alsa-lib-devel) — see the rtmididrv package's own README for details.
// It is kept out of the core github.com/yeoblyv/graphite module
// specifically so that consumers who don't need real MIDI hardware never
// pay that build cost.
//
// # Concurrency
//
// The rtmidi driver delivers every message on its own internal goroutine,
// not whatever goroutine called Listen. Graphite widget state (including
// Graphite.PianoRoll.NoteOn/NoteOff) is only safe to touch from the
// render loop's own goroutine — so every callback passed to Listen must
// hand off through Graphite.Application.Invoke, not call into a widget
// directly:
//
//	listener, err := midi.Listen("", app.Invoke, piano.NoteOn, piano.NoteOff)
//
// (Listen wraps this for you — see its doc comment — but if you're
// forwarding notes to your own code instead of a PianoRoll, apply the same
// rule yourself.)
package midi

import (
	"fmt"

	gomidi "gitlab.com/gomidi/midi/v2"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
)

// ListPorts returns the name of every currently available real MIDI input
// port, for presenting a choice to the user (there is no reliable way to
// guess which one they mean).
func ListPorts() []string {
	ports := gomidi.GetInPorts()
	names := make([]string, len(ports))
	for i, p := range ports {
		names[i] = p.String()
	}
	return names
}

// Listener owns one open connection to a real MIDI input device.
type Listener struct {
	stop func()
}

// Listen opens the MIDI input port whose name contains portName (matching
// is substring-based, case-insensitive — the same rule
// gitlab.com/gomidi/midi/v2's FindInPort uses) — pass "" to open whatever
// port sorts first, which is fine when only one device is connected. Every
// Note On message forwards to invoke(func(){ onNoteOn(note, velocity) })
// and every Note Off to invoke(func(){ onNoteOff(note) }); pass
// Application.Invoke as invoke so the callbacks land safely on the render
// loop's goroutine (see the package doc comment) — this is exactly what
// you want when wiring a Graphite.PianoRoll:
//
//	listener, err := midi.Listen("", app.Invoke, piano.NoteOn, piano.NoteOff)
func Listen(portName string, invoke func(func()), onNoteOn func(note, velocity uint8), onNoteOff func(note uint8)) (*Listener, error) {
	in, err := gomidi.FindInPort(portName)
	if err != nil {
		return nil, fmt.Errorf("graphite/midi: no input port matching %q (available: %v): %w", portName, ListPorts(), err)
	}

	stop, err := gomidi.ListenTo(in, func(msg gomidi.Message, _ int32) {
		var ch, key, vel uint8
		switch {
		case msg.GetNoteStart(&ch, &key, &vel):
			if onNoteOn != nil {
				invoke(func() { onNoteOn(key, vel) })
			}
		case msg.GetNoteEnd(&ch, &key):
			if onNoteOff != nil {
				invoke(func() { onNoteOff(key) })
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("graphite/midi: listen to %q: %w", portName, err)
	}
	return &Listener{stop: stop}, nil
}

// Close stops listening and releases the MIDI port. Safe to call once;
// does not close the underlying driver itself — call CloseDriver once, at
// program exit, after every Listener is closed.
func (l *Listener) Close() {
	if l.stop != nil {
		l.stop()
	}
}

// CloseDriver releases the underlying MIDI driver. Call this once, when
// your program exits, after closing every Listener — typically via
// defer midi.CloseDriver() near the top of main.
func CloseDriver() {
	gomidi.CloseDriver()
}
