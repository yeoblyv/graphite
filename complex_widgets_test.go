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

// Regression: a theme whose BgFocused/Primary is a bright, light color
// (a lime/yellow accent, not the muted blue DefaultTheme happens to use)
// used to be unreadable in a ComboBox — both its closed state (drawn with
// BgFocused when focused) and its open dropdown's selected row (drawn
// with Primary) always used theme.FgWindow for the text regardless of
// that background, so a light FgWindow tuned for a dark background read
// as near-invisible on a bright one. Both now pick their foreground via
// Color.ContrastText, the same pattern ListBox's own selected row
// already used.
func themeWithBrightAccent() Theme {
	th := DefaultTheme()
	bright := RGB(200, 255, 77) // a lime accent, like diskette's own theme
	th.BgFocused = bright
	th.Primary = bright
	return th
}

func TestComboBox_ClosedStateContrastsAgainstABrightBgFocused(t *testing.T) {
	cb := NewComboBox(0, 0, 20, []string{"Password", "Private key"}, nil)
	cb.SetFocus(true)

	c := NewCanvas()
	c.theme = themeWithBrightAccent()
	c.Resize(40, 10)
	cb.DrawRelative(c, 0, 0, 40, 10)

	bright := c.theme.BgFocused
	want := bright.ContrastText()
	if got := c.buffer[1].FgColor; got != want {
		t.Errorf("closed ComboBox text color = %v, want %v (ContrastText of BgFocused %v)", got, want, bright)
	}
	if got := c.buffer[1].BgColor; got != bright {
		t.Errorf("closed ComboBox background = %v, want theme.BgFocused %v", got, bright)
	}
}

func TestComboBox_SelectedDropdownRowContrastsAgainstABrightPrimary(t *testing.T) {
	cb := NewComboBox(0, 0, 20, []string{"Password", "Private key"}, nil)
	cb.Selected = 0
	cb.SetFocus(true) // DrawRelative force-closes IsOpen when not focused
	cb.IsOpen = true

	c := NewCanvas()
	c.theme = themeWithBrightAccent()
	c.Resize(40, 10)
	// DrawOverlay reads cb.AbsX/AbsY/LastW, which only DrawRelative sets —
	// skipping it left LastW at its zero value, so the fill loops never
	// ran and the test was reading untouched buffer cells.
	cb.DrawRelative(c, 0, 0, 40, 10)
	cb.DrawOverlay(c, 0, 0, 40, 10)

	bright := c.theme.Primary
	want := bright.ContrastText()
	// Row 0 (y=1, the first dropdown row) is the selected item.
	idx := 1*c.width + 1
	if got := c.buffer[idx].FgColor; got != want {
		t.Errorf("selected dropdown row text color = %v, want %v (ContrastText of Primary %v)", got, want, bright)
	}
	if got := c.buffer[idx].BgColor; got != bright {
		t.Errorf("selected dropdown row background = %v, want theme.Primary %v", got, bright)
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
