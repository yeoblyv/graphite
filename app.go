package Graphite

import (
	"sync"
	"time"
)

// =========================================================================
// APPLICATION ABSTRACTION
// =========================================================================

// Application owns the terminal, the canvas, the active window and modal
// stack, and drives the main render/input loop via Run. It is the single
// composition root a program constructs: all state that would otherwise
// need to be global (terminal mode, color theme) lives on this struct
// instead.
type Application struct {
	canvas        *Canvas
	term          *terminal
	activeWindow  *Window
	modalStack    []*Window
	running       bool
	statusBarText string
	idleCallback  func()

	invokeMu    sync.Mutex
	invokeQueue []func()
}

// NewApplication creates an Application with an empty canvas using
// DefaultTheme. Call SetTheme afterwards to customize colors.
func NewApplication() *Application {
	return &Application{
		canvas:  NewCanvas(),
		term:    newTerminal(),
		running: true,
	}
}

// SetTheme replaces the color palette used to render every widget and
// forces a full redraw so the change is visible on the next frame.
func (app *Application) SetTheme(t Theme) {
	app.canvas.theme = t
	app.canvas.forceRedraw = true
}

// ShowMessage opens a modal dialog with a title, a message, and a single OK
// button styled per style. It is layered on SetModal, so it stacks on top
// of any modal already open rather than replacing it.
func (app *Application) ShowMessage(title, message string, style ButtonStyle) {
	mod := NewWindow(50, 8, " "+title+" ")
	mod.AddWidget(NewLabel(4, 2, message))
	btn := NewButton(20, 5, "OK", style, func() {
		app.CloseModal()
	})
	mod.AddWidget(btn)
	app.SetModal(mod)
}

// SetWindow sets the non-modal window drawn behind any open modals.
func (app *Application) SetWindow(win *Window) {
	app.activeWindow = win
}

// SetModal opens mod on top of the current modal stack, leaving any
// already-open modal in place beneath it.
func (app *Application) SetModal(mod *Window) {
	app.modalStack = append(app.modalStack, mod)
}

// CloseModal closes the topmost modal, revealing the previous one (if any)
// or the active window.
func (app *Application) CloseModal() {
	if len(app.modalStack) > 0 {
		app.modalStack = app.modalStack[:len(app.modalStack)-1]
	}
}

// topModal returns the modal currently receiving input, or nil if none is
// open.
func (app *Application) topModal() *Window {
	if len(app.modalStack) == 0 {
		return nil
	}
	return app.modalStack[len(app.modalStack)-1]
}

// Invoke queues fn to run on the main loop just before the next frame is
// drawn. This is the only safe way to touch widget state from a background
// goroutine: mutating a widget's fields directly from another goroutine
// races with Run's render loop reading those same fields.
func (app *Application) Invoke(fn func()) {
	app.invokeMu.Lock()
	app.invokeQueue = append(app.invokeQueue, fn)
	app.invokeMu.Unlock()
}

// drainInvokeQueue runs every callback queued via Invoke since the last
// call. The queue is swapped out and unlocked before any callback runs, so
// a callback is free to call Invoke itself without deadlocking.
func (app *Application) drainInvokeQueue() {
	app.invokeMu.Lock()
	queue := app.invokeQueue
	app.invokeQueue = nil
	app.invokeMu.Unlock()

	for _, fn := range queue {
		fn()
	}
}

// SetStatus sets the text shown in the status bar along the bottom row.
func (app *Application) SetStatus(status string) {
	app.statusBarText = status
}

// SetIdleCallback sets a function invoked repeatedly once 300ms have
// elapsed with no input, useful for polling background state (see
// components/erbe-3100-tester for an example).
func (app *Application) SetIdleCallback(cb func()) {
	app.idleCallback = cb
}

// Quit stops Run after the current frame.
func (app *Application) Quit() {
	app.running = false
}

// Run initializes the terminal and drives the main loop: resize, drain
// queued Invoke callbacks, draw, render, poll for input, and route the
// resulting event, until Quit is called or the process is torn down. The
// terminal is always restored on return, including on panic.
func (app *Application) Run() {
	app.term.init()
	defer app.term.restore()

	lastIn := time.Now()

	for app.running {
		app.drainInvokeQueue()

		tW, tH := GetTerminalSize()
		app.canvas.Resize(tW, tH)

		app.canvas.Clear()

		if app.activeWindow != nil {
			app.activeWindow.Draw(app.canvas)
		}
		for _, mod := range app.modalStack {
			mod.Draw(app.canvas)
		}

		if app.statusBarText != "" && tH > 0 {
			for i := 0; i < tW; i++ {
				app.canvas.DrawCell(i, tH-1, " ", app.canvas.theme.BgFocused, app.canvas.theme.FgFocused)
			}
			app.canvas.DrawText(2, tH-1, app.statusBarText, app.canvas.theme.BgFocused, app.canvas.theme.FgFocused)
		}

		app.canvas.Render()

		ev := app.term.pollEvent()
		if ev.Type != EventNone {
			lastIn = time.Now()
		}

		if time.Since(lastIn).Milliseconds() > 300 && app.idleCallback != nil {
			app.idleCallback()
		}

		if top := app.topModal(); top != nil {
			if ev.Type == EventKey && ev.Key == KeyEscape {
				app.CloseModal()
			} else if ev.Type != EventNone {
				top.HandleEvent(ev)
			}
		} else {
			if ev.Type == EventKey && ev.Key == KeyEscape {
				app.Quit()
			} else if ev.Type != EventNone && app.activeWindow != nil {
				app.activeWindow.HandleEvent(ev)
			}
		}
	}
}

// =========================================================================
// TERMINAL SUSPEND/RESUME HANDLERS
// =========================================================================

// Suspend restores the terminal to normal (cooked) mode, e.g. before
// shelling out to an interactive subprocess.
func (app *Application) Suspend() {
	app.term.restore()
}

// Resume switches the terminal back into raw/TUI mode and forces a full
// redraw, e.g. after an interactive subprocess launched via Suspend exits.
func (app *Application) Resume() {
	app.term.init()
	app.canvas.forceRedraw = true
	app.canvas.Clear()
}
