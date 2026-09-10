package Graphite

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

// eventSpy is a minimal Widget that just records every event it receives,
// used to observe Window's mouse-routing decisions directly instead of
// inferring them from a real widget's side effects.
type eventSpy struct {
	BaseWidget
	received []EventType
}

func newEventSpy(x, y, w, h int) *eventSpy {
	return &eventSpy{BaseWidget: NewBaseWidget(x, y, w, h)}
}

func (s *eventSpy) HandleEvent(ev Event) {
	s.received = append(s.received, ev.Type)
}

// Regression: once a widget is hit by EventMouseDown, it must keep
// receiving EventMouseDrag/EventMouseUp even after the pointer moves
// outside its own bounds — e.g. a fader handle dragged past the widget's
// edge — rather than Window re-hit-testing on every motion event and
// routing the drag to whatever happens to be under the pointer now.
func TestWindow_MouseCaptureFollowsDragOutsideWidgetBounds(t *testing.T) {
	win := NewWindow(40, 10, "test")

	a := newEventSpy(0, 0, 5, 5)
	b := newEventSpy(10, 0, 5, 5)
	win.AddWidget(a)
	win.AddWidget(b)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	// Press on A, then drag to a point over B, then release over B.
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: a.AbsX, MouseY: a.AbsY})
	win.HandleEvent(Event{Type: EventMouseDrag, MouseX: b.AbsX, MouseY: b.AbsY})
	win.HandleEvent(Event{Type: EventMouseUp, MouseX: b.AbsX, MouseY: b.AbsY})

	wantA := []EventType{EventMouseDown, EventMouseDrag, EventMouseUp}
	if len(a.received) != len(wantA) {
		t.Fatalf("A received %v, want %v", a.received, wantA)
	}
	for i, ev := range wantA {
		if a.received[i] != ev {
			t.Errorf("A.received[%d] = %v, want %v", i, a.received[i], ev)
		}
	}
	if len(b.received) != 0 {
		t.Errorf("B should not have received anything during A's capture, got %v", b.received)
	}

	// Capture was released on EventMouseUp, so a fresh press on B now
	// reaches B normally.
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: b.AbsX, MouseY: b.AbsY})
	if len(b.received) != 1 || b.received[0] != EventMouseDown {
		t.Errorf("B.received after a fresh press = %v, want [EventMouseDown]", b.received)
	}
}

// EventMouseRightDown is hit-tested and delivered like a scroll event —
// no focus change, no mouse capture — rather than being silently dropped
// the way an undecoded SGR button used to be.
func TestWindow_RightClickIsHitTestedWithoutMovingFocusOrCapture(t *testing.T) {
	win := NewWindow(40, 10, "test")

	a := newEventSpy(0, 0, 5, 5)
	a.IsFocusable = true
	b := newEventSpy(10, 0, 5, 5)
	b.IsFocusable = true
	win.AddWidget(a)
	win.AddWidget(b)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	win.HandleEvent(Event{Type: EventMouseRightDown, MouseX: b.AbsX, MouseY: b.AbsY})

	if len(b.received) != 1 || b.received[0] != EventMouseRightDown {
		t.Fatalf("B.received = %v, want [EventMouseRightDown]", b.received)
	}
	if len(a.received) != 0 {
		t.Errorf("A should not have received anything, got %v", a.received)
	}
	if b.HasFocus() {
		t.Error("a right-click should not move focus")
	}

	// A subsequent drag/release must not go to B: a right-click must not
	// have started a mouse capture the way a left EventMouseDown does.
	win.HandleEvent(Event{Type: EventMouseDrag, MouseX: b.AbsX, MouseY: b.AbsY})
	if len(b.received) != 1 {
		t.Errorf("B received a drag after a right-click, want no capture: %v", b.received)
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

func TestButton_DefaultIdleColorIsThemeBgWidget(t *testing.T) {
	btn := NewButton(0, 0, "OK", BtnDefault, nil)
	c := NewCanvas()
	c.Resize(20, 3)
	btn.DrawRelative(c, 0, 0, 20, 3)

	theme := DefaultTheme()
	if got := c.buffer[0].BgColor; got != theme.BgWidget {
		t.Errorf("idle Button background = %v, want theme.BgWidget (%v)", got, theme.BgWidget)
	}
}

func TestButton_CustomBgColorAutoComputesContrastText(t *testing.T) {
	btn := NewButton(0, 0, "OK", BtnDefault, nil)
	btn.BgColor = Hex("#5DE4FF") // light cyan: contrast text should be black
	c := NewCanvas()
	c.Resize(20, 3)
	btn.DrawRelative(c, 0, 0, 20, 3)

	if got := c.buffer[0].BgColor; got != btn.BgColor {
		t.Errorf("Button background = %v, want the custom BgColor (%v)", got, btn.BgColor)
	}
	if got, want := c.buffer[0].FgColor, RGB(0, 0, 0); got != want {
		t.Errorf("Button foreground = %v, want auto-contrasted black (%v)", got, want)
	}
}

func TestButton_CustomBgColorDoesNotAffectFocusedOrDisabledState(t *testing.T) {
	btn := NewButton(0, 0, "OK", BtnDefault, nil)
	btn.BgColor = Hex("#5DE4FF")
	c := NewCanvas()
	c.Resize(20, 3)
	theme := DefaultTheme()

	btn.IsFocused = true
	btn.DrawRelative(c, 0, 0, 20, 3)
	if got := c.buffer[0].BgColor; got != theme.BgFocused {
		t.Errorf("focused Button background = %v, want theme.BgFocused (%v) — BgColor should not apply while focused", got, theme.BgFocused)
	}

	btn.IsFocused = false
	btn.SetEnabled(false)
	btn.DrawRelative(c, 0, 0, 20, 3)
	if got := c.buffer[0].BgColor; got != theme.BgWidget {
		t.Errorf("disabled Button background = %v, want theme.BgWidget (%v) — BgColor should not apply while disabled", got, theme.BgWidget)
	}
}

func TestNewMenuStrip_DefaultsToThemeColors(t *testing.T) {
	menu := NewMenuStrip(nil)
	theme := DefaultTheme()

	bg, fg, openBg, openFg, dropBg, dropFg := menu.resolveColors(theme)
	if bg != theme.BgWidget || fg != theme.FgWindow {
		t.Errorf("default bar colors = (%v, %v), want (BgWidget, FgWindow)", bg, fg)
	}
	if openBg != theme.Primary || openFg != theme.BgWindow {
		t.Errorf("default open-category colors = (%v, %v), want (Primary, BgWindow)", openBg, openFg)
	}
	if dropBg != theme.BgWindow || dropFg != theme.FgWindow {
		t.Errorf("default dropdown colors = (%v, %v), want (BgWindow, FgWindow)", dropBg, dropFg)
	}
}

func TestMenuStrip_CustomBgColorAutoComputesContrastText(t *testing.T) {
	menu := NewMenuStrip(nil)
	menu.BgColor = Hex("#FFD23D") // bright amber: contrast text should be black

	bg, fg, openBg, openFg, dropBg, dropFg := menu.resolveColors(DefaultTheme())
	black := RGB(0, 0, 0)
	if bg != menu.BgColor || fg != black {
		t.Errorf("bar colors = (%v, %v), want (%v, black)", bg, fg, menu.BgColor)
	}
	if openFg != black || dropFg != black {
		t.Errorf("open/dropdown fg = (%v, %v), want black for both", openFg, dropFg)
	}
	if openBg == bg {
		t.Error("the open category should be visually distinguishable from the closed bar")
	}
	if dropBg != menu.BgColor {
		t.Errorf("dropdown bg = %v, want the same custom BgColor as the bar", dropBg)
	}
}

func TestMenuStrip_CustomFgColorOverridesAutoContrast(t *testing.T) {
	menu := NewMenuStrip(nil)
	menu.BgColor = Hex("#FFD23D")
	menu.FgColor = RGB(10, 20, 30)

	_, fg, _, _, _, dropFg := menu.resolveColors(DefaultTheme())
	if fg != menu.FgColor || dropFg != menu.FgColor {
		t.Errorf("fg = (%v, %v), want explicit FgColor (%v) for both", fg, dropFg, menu.FgColor)
	}
}

func TestNewWindow_DefaultsToBorderedChrome(t *testing.T) {
	win := NewWindow(40, 10, "test")
	if win.Chrome != ChromeBordered {
		t.Errorf("NewWindow's Chrome = %v, want ChromeBordered (the zero value) so every existing window is unaffected by ChromeBorderless's addition", win.Chrome)
	}
}

func TestWindow_BorderlessFillsCanvasWithNoBorder(t *testing.T) {
	win := NewFullscreenWindow()
	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	corner := c.buffer[0]
	if corner.Symbol == "┌" {
		t.Error("a borderless window drew a border corner glyph at (0,0)")
	}
	bottomRight := c.buffer[24*80-1]
	if bottomRight.BgColor != c.theme.BgWindow {
		t.Errorf("bottom-right cell background = %v, want the window background to fill exactly to the canvas edge", bottomRight.BgColor)
	}
}

func TestWindow_BorderlessPositionsChildAtOrigin(t *testing.T) {
	win := NewFullscreenWindow()
	child := newEventSpy(0, 0, 5, 5)
	win.AddWidget(child)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	if child.AbsX != 0 || child.AbsY != 0 {
		t.Errorf("child AbsX/AbsY = %d/%d, want 0/0 (no padding, no centering) by default", child.AbsX, child.AbsY)
	}
}

func TestWindow_BorderlessRespectsPadding(t *testing.T) {
	win := NewFullscreenWindow()
	win.PaddingX, win.PaddingY = 2, 1
	child := newEventSpy(0, 0, 5, 5)
	win.AddWidget(child)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	if child.AbsX != 2 || child.AbsY != 1 {
		t.Errorf("child AbsX/AbsY = %d/%d, want 2/1 to match PaddingX/PaddingY", child.AbsX, child.AbsY)
	}
}

// Regression/contract: when two widgets both hit-test true at the same
// point — e.g. a MenuStrip's open dropdown overlapping a full-screen
// sibling drawn beneath it — Window resolves the tie in favor of whichever
// widget was added to the Window last, matching Draw's own last-drawn-on-
// top order. A consumer that wants a widget's overlay (dropdown, popup) to
// win clicks over what it visually covers must add that widget after the
// sibling(s) it can overlap.
func TestWindow_MouseDownOnOverlappingHitTestPrefersLastAddedWidget(t *testing.T) {
	win := NewWindow(40, 10, "test")

	background := newEventSpy(0, 0, 0, 0) // stretches to fill the window
	win.AddWidget(background)

	var clicked bool
	menu := NewMenuStrip([]MenuCategory{
		{Label: "File", Items: []MenuItem{{Label: "Open", Action: func() { clicked = true }}}},
	})
	win.AddWidget(menu) // added after background: must win the tie

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	// Open the dropdown with a click on the "File" header.
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: menu.AbsX + 2, MouseY: menu.AbsY})
	if menu.OpenIdx != 0 {
		t.Fatalf("OpenIdx = %d after clicking \"File\", want 0 (open)", menu.OpenIdx)
	}
	background.received = nil // discard the header click itself

	// The dropdown's first item is drawn at (menu.AbsX+2, menu.AbsY+2), a
	// point that also falls inside background's full-window bounds.
	itemX, itemY := menu.AbsX+2, menu.AbsY+2
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: itemX, MouseY: itemY})

	if !clicked {
		t.Error("clicking the dropdown item did not run its Action — the click was likely misrouted to the widget beneath it")
	}
	if len(background.received) != 0 {
		t.Errorf("background widget also received the click: %v, want none (menu should have exclusive priority while its dropdown covers this point)", background.received)
	}
}

func TestMenuStrip_ClickingASeparatorDoesNothing(t *testing.T) {
	win := NewWindow(40, 10, "test")
	var clicked bool
	menu := NewMenuStrip([]MenuCategory{
		{Label: "File", Items: []MenuItem{
			{Separator: true},
			{Label: "Open", Action: func() { clicked = true }},
		}},
	})
	win.AddWidget(menu)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	win.HandleEvent(Event{Type: EventMouseDown, MouseX: menu.AbsX + 2, MouseY: menu.AbsY})
	// The separator is the dropdown's first row (menu.AbsY+2).
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: menu.AbsX + 2, MouseY: menu.AbsY + 2})

	if clicked {
		t.Error("clicking the separator ran an action; it should be inert")
	}
	if menu.OpenIdx != 0 {
		t.Errorf("OpenIdx = %d after clicking a separator, want 0 (dropdown stays open)", menu.OpenIdx)
	}
}

func TestMenuStrip_ClickingASubmenuItemOpensItWithoutClosingTheDropdown(t *testing.T) {
	win := NewWindow(40, 10, "test")
	menu := NewMenuStrip([]MenuCategory{
		{Label: "Tab", Items: []MenuItem{
			{Label: "Add", SubItems: []MenuItem{
				{Label: "New file list"},
				{Label: "New terminal"},
			}},
		}},
	})
	win.AddWidget(menu)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	win.HandleEvent(Event{Type: EventMouseDown, MouseX: menu.AbsX + 2, MouseY: menu.AbsY})
	if menu.OpenIdx != 0 {
		t.Fatalf("OpenIdx = %d after clicking \"Tab\", want 0", menu.OpenIdx)
	}

	// "Add" is the dropdown's first (only) item row.
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: menu.AbsX + 2, MouseY: menu.AbsY + 2})
	if menu.OpenSubIdx != 0 {
		t.Fatalf("OpenSubIdx = %d after clicking \"Add\", want 0 (its submenu open)", menu.OpenSubIdx)
	}
	if menu.OpenIdx != 0 {
		t.Error("OpenIdx was reset by opening a submenu; the parent dropdown should stay open")
	}
}

func TestMenuStrip_ClickingASubmenuLeafRunsItAndClosesEverything(t *testing.T) {
	win := NewWindow(40, 10, "test")
	var picked string
	menu := NewMenuStrip([]MenuCategory{
		{Label: "Tab", Items: []MenuItem{
			{Label: "Add", SubItems: []MenuItem{
				{Label: "New file list", Action: func() { picked = "file list" }},
				{Label: "New terminal", Action: func() { picked = "terminal" }},
			}},
		}},
	})
	win.AddWidget(menu)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	win.HandleEvent(Event{Type: EventMouseDown, MouseX: menu.AbsX + 2, MouseY: menu.AbsY})
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: menu.AbsX + 2, MouseY: menu.AbsY + 2}) // open "Add"'s submenu

	subX, subY, _, _ := menu.submenuGeometry(0, 0)
	// The submenu's second row is "New terminal".
	win.HandleEvent(Event{Type: EventMouseDown, MouseX: subX + 1, MouseY: subY + 2})

	if picked != "terminal" {
		t.Errorf("picked = %q, want \"terminal\"", picked)
	}
	if menu.OpenIdx != -1 || menu.OpenSubIdx != -1 {
		t.Errorf("OpenIdx/OpenSubIdx = %d/%d after picking a submenu leaf, want both closed (-1)", menu.OpenIdx, menu.OpenSubIdx)
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

// TestListBox_SelectedRowTextIsReadableAgainstItsHighlight guards against a
// real regression: the selected row's background switches to theme.Primary
// (a bright, saturated accent in some themes, e.g. Diskette's lime), but its
// foreground stayed the fixed theme.FgWindow regardless — light-on-light
// text a caller with a light Primary and light FgWindow can't read at all.
func TestListBox_SelectedRowTextIsReadableAgainstItsHighlight(t *testing.T) {
	lb := NewListBox(0, 0, 20, 3, []string{"alpha", "beta"}, nil)
	lb.Selected = 0

	theme := DefaultTheme()
	theme.Primary = RGB(255, 255, 255) // a bright Primary a fixed light FgWindow can't contrast against
	theme.FgWindow = RGB(240, 240, 240)

	c := NewCanvas()
	c.Resize(20, 3)
	c.theme = theme
	lb.DrawRelative(c, 0, 0, 20, 3)

	got := c.buffer[0].FgColor
	want := theme.Primary.ContrastText()
	if got != want {
		t.Errorf("selected row foreground = %v, want %v (contrast-computed against the highlight)", got, want)
	}
}

func TestTodoList_StaleSelectedAfterShrinkDoesNotPanic(t *testing.T) {
	tl := NewTodoList(0, 0, 20, 5, []string{"a", "b", "c", "d", "e"}, false)
	tl.Selected = 4

	tl.Items = []TodoItem{{Text: "x"}, {Text: "y"}}

	tl.HandleEvent(Event{Type: EventKey, Key: KeyEnter})
	tl.HandleEvent(Event{Type: EventKey, Key: KeySpace})
}
