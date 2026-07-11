package Graphite

import "testing"

func TestTextArea_BuildLinesWrapsAndSplitsOnNewline(t *testing.T) {
	ta := NewTextArea(0, 0, 5, 3)
	ta.LastW = 5 // normally set by DrawRelative; set directly for this test.
	ta.SetText("ab\ncdefgh")

	lines := ta.buildLines()
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (\"ab\", \"cdef\", \"gh\"), lines=%+v", len(lines), lines)
	}
	if string(lines[0].Runes) != "ab" {
		t.Errorf("line 0 = %q, want %q", string(lines[0].Runes), "ab")
	}
}

// FuzzTextAreaBuildLines asserts buildLines never panics for arbitrary text
// and width — it runs on every keystroke and mouse click in a live TextArea,
// so a crash here is a crash of the whole TUI process.
func FuzzTextAreaBuildLines(f *testing.F) {
	f.Add("hello\nworld", 10)
	f.Add("", 5)
	f.Add("\n\n\n", 3)
	f.Add("no newlines at all, just one long line", 1)
	f.Add("中文混合 text", 4)

	f.Fuzz(func(t *testing.T, text string, width int) {
		if width < -10 || width > 1000 {
			t.Skip()
		}
		ta := NewTextArea(0, 0, width, 5)
		ta.LastW = width
		ta.SetText(text)
		_ = ta.buildLines()
	})
}
