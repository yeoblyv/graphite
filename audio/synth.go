// Package audio is an optional companion to github.com/yeoblyv/graphite:
// a minimal polyphonic sine-wave synthesizer that plays MIDI note numbers
// through the system's real audio output, via github.com/hajimehoshi/oto.
//
// Graphite's core widgets (Fader, PianoRoll) have no audio pipeline of
// their own by design — they're a terminal UI library, not an audio
// engine. This package exists specifically to give PianoRoll something to
// actually sound when played, without forcing every consumer of the core
// Graphite module to pull in an audio backend they may not need. Wire it
// up with:
//
//	synth, err := audio.NewSynth()
//	// ...
//	piano.OnNoteOn = synth.NoteOn
//	piano.OnNoteOff = synth.NoteOff
//	defer synth.Close()
package audio

import (
	"encoding/binary"
	"io"
	"math"
	"sync"
	"time"

	"github.com/hajimehoshi/oto/v2"
)

const sampleRate = 44100

// Synth is a minimal polyphonic sine-wave synthesizer. One Synth opens one
// real audio output stream (via one oto.Context — only one may exist per
// process, so do not call NewSynth more than once).
type Synth struct {
	ctx *oto.Context

	mu          sync.Mutex
	voices      map[uint8]*activeVoice
	closed      bool
	stopCleanup chan struct{}
}

type activeVoice struct {
	player oto.Player
	osc    *voice
}

// NewSynth opens the system's default audio output and returns a
// ready-to-use Synth.
func NewSynth() (*Synth, error) {
	ctx, ready, err := oto.NewContextWithOptions(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: 1,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return nil, err
	}
	<-ready

	s := &Synth{
		ctx:         ctx,
		voices:      make(map[uint8]*activeVoice),
		stopCleanup: make(chan struct{}),
	}
	go s.cleanupLoop()
	return s, nil
}

// NoteOn starts note (a MIDI note number, 0-127, 60 = middle C) sounding at
// the given MIDI velocity (0-127, used only to scale volume — this synth
// has no per-velocity timbre). A no-op if note is already sounding, or if
// the Synth has been closed. Signature-compatible with
// Graphite.PianoRoll.OnNoteOn — pass it directly:
// piano.OnNoteOn = synth.NoteOn.
func (s *Synth) NoteOn(note, velocity uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if _, exists := s.voices[note]; exists {
		return
	}

	freq := 440 * math.Pow(2, (float64(note)-69)/12)
	vel := float64(velocity) / 127
	if velocity == 0 {
		vel = 1
	}
	osc := &voice{freq: freq, velocity: vel}
	player := s.ctx.NewPlayer(osc)
	player.Play()
	s.voices[note] = &activeVoice{player: player, osc: osc}
}

// NoteOff releases note: it fades out over a short release envelope rather
// than stopping abruptly, then is cleaned up automatically. Signature-
// compatible with Graphite.PianoRoll.OnNoteOff — pass it directly:
// piano.OnNoteOff = synth.NoteOff.
func (s *Synth) NoteOff(note uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.voices[note]; ok {
		v.osc.release()
	}
}

// cleanupLoop closes and forgets players whose release fade has finished.
// oto does not do this on its own once a player's underlying reader
// reaches io.EOF — Close must still be called explicitly to free the
// player's resources.
func (s *Synth) cleanupLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			for note, v := range s.voices {
				if v.osc.isDone() {
					v.player.Close()
					delete(s.voices, note)
				}
			}
			s.mu.Unlock()
		case <-s.stopCleanup:
			return
		}
	}
}

// Close stops every sounding note immediately and releases the audio
// output. Call it once, when your program exits.
func (s *Synth) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.stopCleanup)
	for _, v := range s.voices {
		v.player.Close()
	}
	s.voices = nil
	return s.ctx.Suspend()
}

// voice is one active note's oscillator: a sine wave with a short linear
// attack and release envelope, read as signed 16-bit little-endian mono
// PCM samples. It implements io.Reader so it can be handed straight to
// oto.Context.NewPlayer.
type voice struct {
	freq     float64
	velocity float64

	mu        sync.Mutex
	phase     float64
	amp       float64
	releasing bool
	done      bool
}

const (
	attackSeconds  = 0.005
	releaseSeconds = 0.08
)

// release begins this voice's fade-out. Safe to call more than once.
func (v *voice) release() {
	v.mu.Lock()
	v.releasing = true
	v.mu.Unlock()
}

// isDone reports whether this voice's release fade has completed.
func (v *voice) isDone() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.done
}

// Read fills buf with signed 16-bit little-endian mono PCM samples of this
// voice's sine wave, ramping through its attack/release envelope one
// sample at a time. Once the release fade reaches silence, it zero-fills
// the remainder of buf and returns io.EOF on the following call.
func (v *voice) Read(buf []byte) (int, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.done {
		return 0, io.EOF
	}

	attackStep := 1.0 / (attackSeconds * sampleRate)
	releaseStep := 1.0 / (releaseSeconds * sampleRate)

	n := len(buf) / 2
	for i := 0; i < n; i++ {
		if v.releasing {
			v.amp -= releaseStep
			if v.amp <= 0 {
				v.amp = 0
				v.done = true
			}
		} else if v.amp < 1 {
			v.amp += attackStep
			if v.amp > 1 {
				v.amp = 1
			}
		}

		sample := math.Sin(v.phase) * v.amp * v.velocity
		v.phase += 2 * math.Pi * v.freq / sampleRate
		if v.phase > 2*math.Pi {
			v.phase -= 2 * math.Pi
		}

		s16 := int16(sample * 32000) // headroom below full scale
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s16))

		if v.done {
			for j := i + 1; j < n; j++ {
				binary.LittleEndian.PutUint16(buf[j*2:], 0)
			}
			return n * 2, nil
		}
	}
	return n * 2, nil
}
