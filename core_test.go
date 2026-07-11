package Graphite

import "testing"

func TestCanvas_DrawCellSanitizesControlCharacters(t *testing.T) {
	c := NewCanvas()
	c.Resize(10, 1)

	// ESC is the vector for terminal escape-sequence injection: Render
	// writes Symbol byte-for-byte to the real terminal, so a raw ESC here
	// (e.g. from untrusted subprocess output) must never survive.
	c.DrawCell(0, 0, "\x1b", -1, -1)
	if got := c.buffer[0].Symbol; got == "\x1b" {
		t.Fatalf("DrawCell stored a raw control character: %q", got)
	}

	c.DrawCell(1, 0, "\x07", -1, -1) // BEL, terminates many OSC sequences.
	if got := c.buffer[1].Symbol; got == "\x07" {
		t.Fatalf("DrawCell stored a raw control character: %q", got)
	}
}

func TestCanvas_DrawCellKeepsPrintableGlyphs(t *testing.T) {
	c := NewCanvas()
	c.Resize(10, 1)

	for _, s := range []string{"a", "и", "░", "│", "┌"} {
		c.DrawCell(0, 0, s, -1, -1)
		if got := c.buffer[0].Symbol; got != s {
			t.Errorf("DrawCell(%q) stored %q, want unchanged", s, got)
		}
	}
}
