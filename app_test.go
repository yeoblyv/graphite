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
