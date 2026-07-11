package graphite

import (
	"sync"
	"testing"
)

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
