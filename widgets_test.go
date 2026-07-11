package graphite

import "testing"

func typeRunes(ib *InputBox, s string) {
	for _, r := range s {
		ib.HandleEvent(Event{Type: EventKey, CharCode: r})
	}
}

func TestInputBox_CursorNavigation(t *testing.T) {
	ib := NewInputBox(0, 0, 20, "")

	// Mix ASCII and Cyrillic to catch possible rune-vs-byte bugs.
	typeRunes(ib, "привetИ")
	if ib.Value != "привetИ" {
		t.Fatalf("after typing, Value = %q", ib.Value)
	}
	if ib.CursorPos != len([]rune("привetИ")) {
		t.Fatalf("CursorPos = %d, want %d", ib.CursorPos, len([]rune("привetИ")))
	}

	// Cursor starts at the end ("привetИ", position 7). Left twice moves it
	// between 'e' (idx4) and 't' (idx5) — position 5. Backspace removes the
	// rune before the cursor, i.e. 'e'.
	ib.HandleEvent(Event{Type: EventKey, Key: KeyLeft})
	ib.HandleEvent(Event{Type: EventKey, Key: KeyLeft})
	ib.HandleEvent(Event{Type: EventKey, Key: KeyBackspace})
	if ib.Value != "привtИ" {
		t.Fatalf("after backspace, Value = %q, want %q", ib.Value, "привtИ")
	}

	// Cursor is now at position 4 (between 'в' and 't'). Delete removes the
	// rune under the cursor, i.e. 't'.
	ib.HandleEvent(Event{Type: EventKey, Key: KeyDelete})
	if ib.Value != "привИ" {
		t.Fatalf("after delete, Value = %q, want %q", ib.Value, "привИ")
	}
}

func TestInputBox_CursorClampsToBounds(t *testing.T) {
	ib := NewInputBox(0, 0, 20, "")
	typeRunes(ib, "ab")

	// The cursor must not go below 0.
	for i := 0; i < 5; i++ {
		ib.HandleEvent(Event{Type: EventKey, Key: KeyLeft})
	}
	if ib.CursorPos != 0 {
		t.Fatalf("CursorPos = %d, want 0", ib.CursorPos)
	}

	// Nor past the end of the value.
	for i := 0; i < 10; i++ {
		ib.HandleEvent(Event{Type: EventKey, Key: KeyRight})
	}
	if ib.CursorPos != len([]rune(ib.Value)) {
		t.Fatalf("CursorPos = %d, want %d", ib.CursorPos, len([]rune(ib.Value)))
	}
}

// Regression: a disabled widget must not react to a mouse click. Before
// Widget.IsEnabled() and the check in Window.HandleEvent were added, this
// slipped through for every widget except Button.
func TestWindow_DisabledWidgetIgnoresMouseClick(t *testing.T) {
	win := NewWindow(40, 10, "test")

	cb := NewCheckbox(0, 0, "opt", false)
	cb.SetEnabled(false)
	win.AddWidget(cb)

	lb := NewListBox(0, 2, 10, 3, []string{"a", "b", "c"}, nil)
	lb.SetEnabled(false)
	win.AddWidget(lb)

	// Draw resolves AbsX/AbsY/LastW/LastH, which HitTest needs.
	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	if cb.Checked {
		t.Fatal("checkbox should start unchecked")
	}
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: cb.AbsX, MouseY: cb.AbsY})
	if cb.Checked {
		t.Error("disabled checkbox toggled after mouse click")
	}

	win.HandleEvent(Event{Type: EventMouseDown, MouseX: lb.AbsX, MouseY: lb.AbsY + 1})
	if lb.Selected != 0 {
		t.Errorf("disabled listbox changed selection to %d after mouse click", lb.Selected)
	}
}

func TestWindow_EnabledWidgetRespondsToMouseClick(t *testing.T) {
	win := NewWindow(40, 10, "test")
	cb := NewCheckbox(0, 0, "opt", false)
	win.AddWidget(cb)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	win.HandleEvent(Event{Type: EventMouseDown, MouseX: cb.AbsX, MouseY: cb.AbsY})
	if !cb.Checked {
		t.Error("enabled checkbox did not toggle after mouse click")
	}
}

// Regression: Items is a plain exported slice, and a caller is free to
// reassign it to a shorter slice without resetting Selected — exactly what
// components/erbe-3100-tester does between ping-test runs. Before the
// bounds guard was added, KeyEnter on a stale Selected indexed past the end
// of the new, shorter Items and panicked (a full crash of the TUI process).

func TestListBox_StaleSelectedAfterShrinkDoesNotPanic(t *testing.T) {
	lb := NewListBox(0, 0, 20, 5, []string{"a", "b", "c", "d", "e"}, func(int, string) {})
	lb.Selected = 4 // last item of the original 5

	lb.Items = []string{"x", "y"} // caller shrinks Items without touching Selected

	lb.HandleEvent(Event{Type: EventKey, Key: KeyEnter})
}

func TestTodoList_StaleSelectedAfterShrinkDoesNotPanic(t *testing.T) {
	tl := NewTodoList(0, 0, 20, 5, []string{"a", "b", "c", "d", "e"}, false)
	tl.Selected = 4

	tl.Items = []TodoItem{{Text: "x"}, {Text: "y"}}

	tl.HandleEvent(Event{Type: EventKey, Key: KeyEnter})
	tl.HandleEvent(Event{Type: EventKey, Key: KeySpace})
}
