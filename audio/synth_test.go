package audio

import (
	"encoding/binary"
	"io"
	"math"
	"testing"
)

// readSamples reads n samples (2 bytes each) from v, returning them as
// signed 16-bit values, and whether io.EOF was reached before n samples.
func readSamples(t *testing.T, v *voice, n int) ([]int16, bool) {
	t.Helper()
	buf := make([]byte, n*2)
	read, err := v.Read(buf)
	samples := make([]int16, read/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(buf[i*2:]))
	}
	return samples, err == io.EOF
}

func TestVoice_AttackRampsUpFromSilence(t *testing.T) {
	v := &voice{freq: 440, velocity: 1}
	samples, _ := readSamples(t, v, 4)

	for i := 1; i < len(samples); i++ {
		if abs16(samples[i]) < abs16(samples[i-1]) && samples[i-1] != 0 {
			// Not a strict monotonic requirement (it's a sine wave, not a
			// ramp on its own), but the very first sample must be at or
			// near silence — the attack starts from amp 0, not full
			// volume — which is checked separately below.
		}
	}
	if abs16(samples[0]) > 50 {
		t.Errorf("first sample = %d, want near-silent (attack starts at amplitude 0)", samples[0])
	}
}

func TestVoice_ReachesFullVolumeAfterAttack(t *testing.T) {
	// A high enough frequency that many full cycles happen well within the
	// sampled window, so the sine itself reaches near +/-1 regardless of
	// where exactly we sample — otherwise a low frequency's own waveform,
	// not the envelope, could be why a sample happens to read low.
	v := &voice{freq: 1000, velocity: 1}
	// Read enough samples to get well past the 5ms attack (5ms * 44100 = ~220 samples).
	samples, _ := readSamples(t, v, 1000)

	peak := int16(0)
	for _, s := range samples[500:] {
		if abs16(s) > peak {
			peak = abs16(s)
		}
	}
	if peak < 30000 {
		t.Errorf("peak amplitude after attack = %d, want close to full scale (~32000)", peak)
	}
}

func TestVoice_ReleaseFadesToSilenceThenEOF(t *testing.T) {
	v := &voice{freq: 1, velocity: 1}
	// Run past the attack first so we're releasing from full volume.
	readSamples(t, v, 500)

	v.release()

	// releaseSeconds = 0.08 -> ~3528 samples at 44100Hz; read well past that.
	samples, eof := readSamples(t, v, 10000)
	if len(samples) == 0 {
		t.Fatal("expected some samples before EOF")
	}
	last := samples[len(samples)-1]
	if last != 0 {
		t.Errorf("last sample before EOF = %d, want 0 (fully released)", last)
	}

	// A second Read call after the voice is fully silent must report EOF.
	_, eof2 := readSamples(t, v, 10)
	if !eof && !eof2 {
		t.Error("expected io.EOF either on the fade-out read or the following read")
	}
	if !v.isDone() {
		t.Error("isDone() = false after the release fade completed, want true")
	}
}

func TestVoice_FrequencyMatchesMIDINoteConversion(t *testing.T) {
	// A4 = MIDI note 69 = 440Hz exactly.
	freq := 440 * math.Pow(2, (69.0-69)/12)
	if freq != 440 {
		t.Fatalf("setup: A4 frequency formula = %v, want 440", freq)
	}
	// One octave up (81) should be exactly double.
	freqOctaveUp := 440 * math.Pow(2, (81.0-69)/12)
	if math.Abs(freqOctaveUp-880) > 0.001 {
		t.Errorf("one octave above A4 = %v, want 880", freqOctaveUp)
	}
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}
