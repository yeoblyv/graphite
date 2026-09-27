package Graphite

import (
	"sync"
	"testing"
)

// fakeRawReceiver is a minimal RawInputReceiver for exercising
// focusedRawReceiver without needing a real Terminal (which needs an
// actual pty).
type fakeRawReceiver struct {
	BaseWidget
	received []byte
}

func (f *fakeRawReceiver) WriteRaw(p []byte) { f.received = append(f.received, p...) }

// Regression: a click that both hits some widget (capturing the mouse)
// and, as a side effect of running that widget's own click handling,
// moves focus to a RawInputReceiver (e.g. a menu item that creates and
// focuses a new Terminal tab) must not have its own trailing EventMouseUp
// redirected to that receiver as raw bytes — the capture is still with
// the original widget until the up event clears it.
func TestApplication_FocusedRawReceiverSuppressedDuringMouseCapture(t *testing.T) {
	app := NewApplication()
	win := NewWindow(40, 10, "test")
	raw := &fakeRawReceiver{BaseWidget: NewBaseWidget(0, 0, 10, 1)}
	raw.IsFocusable = true
	win.AddWidget(raw)
	app.SetWindow(win)

	c := NewCanvas()
	c.Resize(80, 24)
	win.Draw(c)

	// Focus raw directly (standing in for "a click elsewhere focused it
	// as a side effect") while a capture from some other gesture is still
	// open.
	raw.SetFocus(true)
	win.mouseCapture = raw // any non-nil capture target demonstrates the guard; a different widget makes no difference here

	if got := app.focusedRawReceiver(); got != nil {
		t.Error("focusedRawReceiver returned non-nil while a mouse gesture is still in flight, want nil")
	}

	win.mouseCapture = nil
	if got := app.focusedRawReceiver(); got != raw {
		t.Error("focusedRawReceiver returned nil once the capture cleared, want the focused receiver")
	}
}

func TestLooksLikeMouseReport(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"SGR mouse down", []byte("\x1b[<0;10;5M"), true},
		{"SGR mouse up", []byte("\x1b[<0;10;5m"), true},
		{"plain letter", []byte("a"), false},
		{"an arrow key", []byte("\x1b[A"), false},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeMouseReport(tc.data); got != tc.want {
				t.Errorf("looksLikeMouseReport(%q) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

func TestApplication_InvokeDrainsOnMainGoroutine(t *testing.T) {
	app := NewApplication()

	var wg sync.WaitGroup
	results := make([]int, 0, 100)

	wg.Add(100)
	for i := 0; i < 100; i++ {
		i := i
		go func() {
			defer wg.Done()
			app.Invoke(func() {
				// Executed only from drainInvokeQueue, never concurrently,
				// so appending to the plain slice here is safe.
				results = append(results, i)
			})
		}()
	}
	wg.Wait()

	if len(results) != 0 {
		t.Fatalf("callbacks ran before drain: %d", len(results))
	}

	app.drainInvokeQueue()

	if len(results) != 100 {
		t.Fatalf("drainInvokeQueue executed %d callbacks, want 100", len(results))
	}
}

func TestApplication_InvokeCanReenterWithoutDeadlock(t *testing.T) {
	app := NewApplication()

	done := false
	app.Invoke(func() {
		app.Invoke(func() {
			done = true
		})
	})

	app.drainInvokeQueue() // runs the outer callback, which queues the inner one
	app.drainInvokeQueue() // runs the inner callback

	if !done {
		t.Fatal("nested Invoke callback never ran")
	}
}

func TestApplication_ModalStack(t *testing.T) {
	app := NewApplication()

	if app.topModal() != nil {
		t.Fatal("expected no modal initially")
	}

	first := NewWindow(40, 10, "first")
	second := NewWindow(40, 10, "second")

	app.SetModal(first)
	if app.topModal() != first {
		t.Fatal("expected first window to be on top")
	}

	app.SetModal(second)
	if app.topModal() != second {
		t.Fatal("expected second window to be on top after pushing it")
	}

	app.CloseModal()
	if app.topModal() != first {
		t.Fatal("expected first window to be on top again after closing second")
	}

	app.CloseModal()
	if app.topModal() != nil {
		t.Fatal("expected no modal after closing all")
	}

	// Closing with an empty stack must not panic.
	app.CloseModal()
}

// TestApplication_SetModalBlursTheLayerUnderneath reproduces a real bug: a
// focused InputBox draws its own cursor block unconditionally every frame
// while IsFocused is true (see widgets.go's InputBox.DrawRelative), and
// nothing about opening a new modal on top of it used to change that —
// input routing moved to the new modal, but the InputBox one layer down
// kept believing it was still focused and kept drawing its cursor, which
// could land inside the new modal's own bounds depending on layout/
// terminal width. SetModal must blur whatever was focused one layer down,
// the same way it already clears mouse capture there.
func TestApplication_SetModalBlursTheLayerUnderneath(t *testing.T) {
	app := NewApplication()

	first := NewWindow(40, 10, "first")
	input := NewInputBox(2, 2, 20, "")
	first.AddWidget(input)
	input.SetFocus(true)

	app.SetModal(first)
	if !input.HasFocus() {
		t.Fatal("test setup: expected input to be focused after opening the first modal")
	}

	second := NewWindow(40, 10, "second")
	app.SetModal(second)

	if input.HasFocus() {
		t.Error("InputBox from the first modal is still focused after a second modal opened on top of it — it will keep drawing its cursor block underneath the new modal")
	}
}

func TestCanvas_DefaultTheme(t *testing.T) {
	c := NewCanvas()
	if c.theme != DefaultTheme() {
		t.Fatalf("NewCanvas().theme = %+v, want %+v", c.theme, DefaultTheme())
	}
}

func TestCanvas_ThemeReflectsSetTheme(t *testing.T) {
	app := NewApplication()
	if got := app.canvas.Theme(); got != DefaultTheme() {
		t.Fatalf("Theme() before SetTheme = %+v, want %+v", got, DefaultTheme())
	}

	custom := Theme{Primary: 999}
	app.SetTheme(custom)
	if got := app.canvas.Theme(); got != custom {
		t.Fatalf("Theme() after SetTheme = %+v, want %+v", got, custom)
	}
}

func TestApplication_EscapeQuitsByDefault(t *testing.T) {
	app := NewApplication()

	app.routeEvent(Event{Type: EventKey, Key: KeyEscape})

	if app.running {
		t.Error("Escape with no modal and no quit hook should quit the application")
	}
}

func TestApplication_OnQuitRequestedOverridesEscape(t *testing.T) {
	app := NewApplication()
	requested := false
	app.SetOnQuitRequested(func() { requested = true })

	app.routeEvent(Event{Type: EventKey, Key: KeyEscape})

	if !requested {
		t.Error("SetOnQuitRequested hook was not called on Escape")
	}
	if !app.running {
		t.Error("Escape must not quit directly once a quit hook is set")
	}
}

func TestApplication_OnQuitRequestedDoesNotFireInsideAModal(t *testing.T) {
	app := NewApplication()
	requested := false
	app.SetOnQuitRequested(func() { requested = true })
	app.SetModal(NewWindow(40, 10, "modal"))

	app.routeEvent(Event{Type: EventKey, Key: KeyEscape})

	if requested {
		t.Error("quit hook fired for Escape while a modal was open; Escape should close the modal instead")
	}
	if app.topModal() != nil {
		t.Error("Escape did not close the open modal")
	}
}

func TestApplication_SetThemeAffectsRendering(t *testing.T) {
	app := NewApplication()
	app.canvas.Resize(20, 5)

	lbl := NewLabel(0, 0, "hi")
	lbl.DrawRelative(app.canvas, 0, 0, 20, 5)

	def := DefaultTheme()
	if got := app.canvas.buffer[0]; got.BgColor != def.BgWindow || got.FgColor != def.FgWindow {
		t.Fatalf("before SetTheme: cell = %+v, want bg=%d fg=%d", got, def.BgWindow, def.FgWindow)
	}

	custom := Theme{BgWindow: 123, FgWindow: 45}
	app.SetTheme(custom)
	lbl.DrawRelative(app.canvas, 0, 0, 20, 5)

	if got := app.canvas.buffer[0]; got.BgColor != custom.BgWindow || got.FgColor != custom.FgWindow {
		t.Fatalf("after SetTheme: cell = %+v, want bg=%d fg=%d", got, custom.BgWindow, custom.FgWindow)
	}
}

func TestScrollKeyPrefix(t *testing.T) {
	if delta, rest, ok := scrollKeyPrefix([]byte("\x1b[5~")); !ok || delta != rawPageScrollLines || len(rest) != 0 {
		t.Errorf("PageUp: delta=%d rest=%q ok=%v, want delta=%d rest=\"\" ok=true", delta, rest, ok, rawPageScrollLines)
	}
	if delta, rest, ok := scrollKeyPrefix([]byte("\x1b[6~")); !ok || delta != -rawPageScrollLines || len(rest) != 0 {
		t.Errorf("PageDown: delta=%d rest=%q ok=%v, want delta=%d rest=\"\" ok=true", delta, rest, ok, -rawPageScrollLines)
	}
	if delta, rest, ok := scrollKeyPrefix([]byte("\x1b[5~extra")); !ok || delta != rawPageScrollLines || string(rest) != "extra" {
		t.Errorf("PageUp with trailing bytes: delta=%d rest=%q ok=%v, want rest=\"extra\"", delta, rest, ok)
	}
	if _, _, ok := scrollKeyPrefix([]byte("hello")); ok {
		t.Errorf("scrollKeyPrefix matched plain text that isn't PageUp/PageDown at all")
	}
	if _, _, ok := scrollKeyPrefix([]byte("\x1b[3~")); ok {
		t.Errorf("scrollKeyPrefix matched \\x1b[3~ (Delete), not a PageUp/PageDown sequence")
	}
}

// fakeScrollable is a minimal RawInputReceiver + Scrollable double, so
// Run's PageUp/PageDown interception logic can be exercised without a
// real pty.
type fakeScrollable struct {
	BaseWidget
	written      []byte
	scrolls      []int
	acceptScroll bool
}

func (f *fakeScrollable) DrawRelative(c *Canvas, offX, offY, pW, pH int) {}
func (f *fakeScrollable) WriteRaw(p []byte)                              { f.written = append(f.written, p...) }
func (f *fakeScrollable) HandleScroll(delta int) bool {
	f.scrolls = append(f.scrolls, delta)
	return f.acceptScroll
}

// runsPageKeyInterception mirrors Run's own decision (scrollKeyPrefix,
// then HandleScroll if Scrollable, else/otherwise forwardRaw) against a
// fake receiver, since Run itself is an unbounded loop reading real
// terminal input and isn't unit-testable directly.
func runsPageKeyInterception(app *Application, raw RawInputReceiver, data []byte) {
	if delta, rest, ok := scrollKeyPrefix(data); ok {
		if scrollable, isScrollable := raw.(Scrollable); isScrollable && scrollable.HandleScroll(delta) {
			if len(rest) > 0 {
				app.forwardRaw(raw, rest)
			}
			return
		}
	}
	app.forwardRaw(raw, data)
}

func TestApplication_PageKeyInterception_AcceptedByScrollable(t *testing.T) {
	app := NewApplication()
	fs := &fakeScrollable{acceptScroll: true}

	runsPageKeyInterception(app, fs, []byte("\x1b[5~ls\n"))

	if len(fs.scrolls) != 1 || fs.scrolls[0] != rawPageScrollLines {
		t.Errorf("scrolls = %v, want a single call with delta=%d", fs.scrolls, rawPageScrollLines)
	}
	if string(fs.written) != "ls\n" {
		t.Errorf("written = %q, want only the bytes after the PageUp prefix (\"ls\\n\")", fs.written)
	}
}

func TestApplication_PageKeyInterception_DeclinedFallsThroughRaw(t *testing.T) {
	app := NewApplication()
	fs := &fakeScrollable{acceptScroll: false} // e.g. Terminal.HandleScroll during the alt screen

	runsPageKeyInterception(app, fs, []byte("\x1b[6~"))

	if len(fs.scrolls) != 1 {
		t.Fatalf("HandleScroll was not even offered the chance to accept/decline")
	}
	if string(fs.written) != "\x1b[6~" {
		t.Errorf("written = %q, want the whole original sequence forwarded unaltered when declined", fs.written)
	}
}
