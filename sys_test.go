package Graphite

import "testing"

func TestParseANSI(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want Event
	}{
		{"tab", []byte{9}, Event{Type: EventKey, Key: KeyTab}},
		{"enter", []byte{13}, Event{Type: EventKey, Key: KeyEnter}},
		{"escape", []byte{27}, Event{Type: EventKey, Key: KeyEscape}},
		{"space", []byte{32}, Event{Type: EventKey, Key: KeySpace, CharCode: ' '}},
		{"backspace", []byte{127}, Event{Type: EventKey, Key: KeyBackspace}},
		{"plain char", []byte{'a'}, Event{Type: EventKey, CharCode: 'a'}},
		{"arrow up", []byte{27, '[', 'A'}, Event{Type: EventKey, Key: KeyUp}},
		{"arrow down", []byte{27, '[', 'B'}, Event{Type: EventKey, Key: KeyDown}},
		{"arrow right", []byte{27, '[', 'C'}, Event{Type: EventKey, Key: KeyRight}},
		{"arrow left", []byte{27, '[', 'D'}, Event{Type: EventKey, Key: KeyLeft}},
		{"insert", []byte("\033[2~"), Event{Type: EventKey, Key: KeyInsert}},
		{"delete", []byte{27, '[', '3', '~'}, Event{Type: EventKey, Key: KeyDelete}},
		{"f1 (SS3)", []byte{27, 'O', 'P'}, Event{Type: EventKey, Key: KeyF1}},
		{"f2 (SS3)", []byte{27, 'O', 'Q'}, Event{Type: EventKey, Key: KeyF2}},
		{"f3 (SS3)", []byte{27, 'O', 'R'}, Event{Type: EventKey, Key: KeyF3}},
		{"f4 (SS3)", []byte{27, 'O', 'S'}, Event{Type: EventKey, Key: KeyF4}},
		{"f1 (CSI-tilde)", []byte("\033[11~"), Event{Type: EventKey, Key: KeyF1}},
		{"f5", []byte("\033[15~"), Event{Type: EventKey, Key: KeyF5}},
		{"f6", []byte("\033[17~"), Event{Type: EventKey, Key: KeyF6}},
		{"f7", []byte("\033[18~"), Event{Type: EventKey, Key: KeyF7}},
		{"f8", []byte("\033[19~"), Event{Type: EventKey, Key: KeyF8}},
		{"f9", []byte("\033[20~"), Event{Type: EventKey, Key: KeyF9}},
		{"f10", []byte("\033[21~"), Event{Type: EventKey, Key: KeyF10}},
		{"f11", []byte("\033[23~"), Event{Type: EventKey, Key: KeyF11}},
		{"f12", []byte("\033[24~"), Event{Type: EventKey, Key: KeyF12}},
		{
			"sgr mouse down",
			[]byte("\033[<0;10;5M"),
			Event{Type: EventMouseDown, MouseX: 9, MouseY: 4},
		},
		{
			"sgr mouse right down",
			[]byte("\033[<2;10;5M"),
			Event{Type: EventMouseRightDown, MouseX: 9, MouseY: 4},
		},
		{
			"sgr mouse drag",
			[]byte("\033[<32;12;7M"),
			Event{Type: EventMouseDrag, MouseX: 11, MouseY: 6},
		},
		{
			"sgr mouse up",
			[]byte("\033[<0;10;5m"),
			Event{Type: EventMouseUp, MouseX: 9, MouseY: 4},
		},
		{"empty", []byte{}, Event{Type: EventNone}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseANSI(tc.in)
			if len(got) == 0 {
				if tc.want.Type != EventNone {
					t.Errorf("parseANSI(%v) = [], want %+v", tc.in, tc.want)
				}
			} else {
				if got[0] != tc.want {
					t.Errorf("parseANSI(%v) = %+v, want %+v", tc.in, got[0], tc.want)
				}
			}
		})
	}
}

func TestParseANSI_MouseReleaseIgnoresButtonNumber(t *testing.T) {
	// A release ends mouse capture regardless of which button was let go
	// (see Window.mouseCapture), so any btn value with an "m" suffix must
	// decode to EventMouseUp, not just btn=0.
	got := parseANSI([]byte("\033[<2;10;5m"))
	if len(got) == 0 || got[0].Type != EventMouseUp {
		t.Errorf("parseANSI(btn=2 release) = %+v, want Type=EventMouseUp", got)
	}
}

func TestParseANSI_UnassignedCSITildeCodeProducesNoEvent(t *testing.T) {
	// 16 and 22 are intentionally unmapped VT220 gaps; the whole escape
	// sequence must still be consumed silently rather than leaking its
	// digits/tilde through as literal typed characters.
	got := parseANSI([]byte("\033[16~a"))
	if len(got) != 1 || got[0].CharCode != 'a' {
		t.Errorf("parseANSI(unassigned CSI-tilde + 'a') = %+v, want exactly one CharCode='a' event", got)
	}
}

func TestParseANSI_MultiByteUnicode(t *testing.T) {
	// 'ю' encoded as UTF-8 (2 bytes), no leading ESC.
	got := parseANSI([]byte("ю"))
	if len(got) == 0 || got[0].Type != EventKey || got[0].CharCode != 'ю' {
		t.Errorf("expected charcode 'ю', got %+v", got)
	}
}

// FuzzParseANSI feeds parseANSI arbitrary byte sequences — the terminal
// reader hands it raw, attacker-influenceable stdin bytes in production, so
// the only real contract to fuzz for is "never panics", not any specific
// decoded Event.
func FuzzParseANSI(f *testing.F) {
	f.Add([]byte{9})
	f.Add([]byte{13})
	f.Add([]byte{27})
	f.Add([]byte{127})
	f.Add([]byte{27, '[', 'A'})
	f.Add([]byte{27, '[', '3', '~'})
	f.Add([]byte{27, 'O', 'P'})
	f.Add([]byte("\033[21~"))
	f.Add([]byte("\033[16~"))
	f.Add([]byte{27, 'O'})
	f.Add([]byte("\033[<0;10;5M"))
	f.Add([]byte("\033[<32;10;5M"))
	f.Add([]byte("\033[<0;10;5m"))
	f.Add([]byte("ю"))
	f.Add([]byte{})
	f.Add([]byte{27, '['})
	f.Add([]byte{27, '[', '<'})

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = parseANSI(data)
	})
}
