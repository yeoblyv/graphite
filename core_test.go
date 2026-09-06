package Graphite

import (
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestColor_ContrastText(t *testing.T) {
	tests := []struct {
		name string
		bg   Color
		want Color
	}{
		{"bright amber", Hex("#FFD23D"), RGB(0, 0, 0)},
		{"near-black", RGB(13, 13, 15), RGB(255, 255, 255)},
		{"white", RGB(255, 255, 255), RGB(0, 0, 0)},
	}
	for _, tc := range tests {
		if got := tc.bg.ContrastText(); got != tc.want {
			t.Errorf("%s: ContrastText() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

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

func TestCanvas_DrawTextAdvancesByRuneWidth(t *testing.T) {
	c := NewCanvas()
	c.Resize(10, 1)

	// "a" (width 1) followed by "中" (a CJK ideograph, width 2), followed by
	// "b" (width 1). "中" must occupy two columns: itself plus a
	// continuation placeholder that Render will skip, and "b" must land at
	// column 3, not column 2.
	c.DrawText(0, 0, "a中b", -1, -1)

	if got := c.buffer[0].Symbol; got != "a" {
		t.Fatalf("column 0 = %q, want %q", got, "a")
	}
	if got := c.buffer[1].Symbol; got != "中" {
		t.Fatalf("column 1 = %q, want %q", got, "中")
	}
	if !c.buffer[2].continuation {
		t.Fatalf("column 2 should be a continuation placeholder after a wide rune")
	}
	if got := c.buffer[3].Symbol; got != "b" {
		t.Fatalf("column 3 = %q, want %q (wide rune must push trailing text right)", got, "b")
	}
}

func TestCanvas_RenderSkipsContinuationCells(t *testing.T) {
	c := NewCanvas()
	c.Resize(10, 1)
	c.DrawText(0, 0, "中", -1, -1)

	// Render must not panic on a continuation cell, on the first (forced)
	// frame or a subsequent unforced one where nothing changed.
	c.Render()
	c.Render()
}

// FuzzSanitizeGlyph asserts the two properties that matter for the
// terminal escape-sequence injection defense: sanitizeGlyph never panics,
// and it never lets a single-rune control character (in particular ESC)
// pass through unchanged.
func FuzzSanitizeGlyph(f *testing.F) {
	f.Add("a")
	f.Add("и")
	f.Add("░")
	f.Add("\x1b")
	f.Add("\x07")
	f.Add("\x1b]0;title\x07")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		got := sanitizeGlyph(s)

		r, size := utf8.DecodeRuneInString(s)
		if size == len(s) && unicode.IsControl(r) && got == s {
			t.Fatalf("sanitizeGlyph(%q) returned the control character unchanged", s)
		}
	})
}
