package graphite

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
		{"delete", []byte{27, '[', '3', '~'}, Event{Type: EventKey, Key: KeyDelete}},
		{
			"sgr mouse down",
			[]byte("\033[<0;10;5M"),
			Event{Type: EventMouseDown, MouseX: 9, MouseY: 4},
		},
		{"empty", []byte{}, Event{Type: EventNone}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseANSI(tc.in)
			if got != tc.want {
				t.Errorf("parseANSI(%v) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseANSI_MouseRelease(t *testing.T) {
	// "m" suffix means release, not a press — should be ignored.
	got := parseANSI([]byte("\033[<0;10;5m"))
	if got.Type != EventNone {
		t.Errorf("expected release to be ignored, got %+v", got)
	}
}

func TestParseANSI_MultiByteUnicode(t *testing.T) {
	// 'ю' encoded as UTF-8 (2 bytes), no leading ESC.
	got := parseANSI([]byte("ю"))
	if got.Type != EventKey || got.CharCode != 'ю' {
		t.Errorf("expected charcode 'ю', got %+v", got)
	}
}
