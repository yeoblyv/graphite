package Graphite

import (
	"bytes"
	"strings"
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
	canvas          *Canvas
	term            *terminal
	activeWindow    *Window
	modalStack      []*Window
	running         bool
	statusBarText   string
	idleCallback    func()
	onQuitRequested func()

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

func measureTextWrapped(text string, maxW int) int {
	lines := 0
	for _, hardLine := range strings.Split(text, "\n") {
		words := strings.Fields(hardLine)
		if len(words) == 0 {
			lines++
			continue
		}
		lineW := 0
		for _, word := range words {
			wordW := 0
			for _, r := range word {
				wordW += runeWidth(r)
			}
			if lineW+wordW > maxW {
				if lineW > 0 {
					lines++
					lineW = 0
				}
			} else if lineW > 0 {
				lineW++
			}
			for _, r := range word {
				rw := runeWidth(r)
				if lineW+rw > maxW {
					lines++
					lineW = 0
				}
				lineW += rw
			}
		}
		lines++
	}
	return lines
}

// ShowMessage opens a modal dialog with a title, a message, and a single OK
// button styled per style. It is layered on SetModal, so it stacks on top
// of any modal already open rather than replacing it.
func (app *Application) ShowMessage(title, message string, style ButtonStyle) {
	lines := measureTextWrapped(message, 42)
	winH := lines + 6
	if winH < 8 {
		winH = 8
	}

	mod := NewWindow(50, winH, " "+title+" ")
	mod.AddWidget(NewLabel(4, 2, message))
	btn := NewButton(20, winH-3, "OK", style, func() {
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
	if app.activeWindow != nil {
		app.activeWindow.ClearMouseCapture()
	}
	for _, m := range app.modalStack {
		m.ClearMouseCapture()
	}
	app.modalStack = append(app.modalStack, mod)
}

// CloseModal closes the topmost modal, revealing the previous one (if any)
// or the active window.
func (app *Application) CloseModal() {
	if len(app.modalStack) > 0 {
		app.modalStack[len(app.modalStack)-1].ClearMouseCapture()
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

// RawInputReceiver is implemented by a widget that needs terminal input
// as raw, undecoded bytes while it has focus — bypassing parseANSI's
// Event decoding entirely. Terminal (see terminal.go) is the only
// built-in widget that does: forwarding a shell's own keystrokes
// byte-for-byte is the whole point of an embedded terminal, and
// round-tripping them through graphite's own smaller KeyCode vocabulary
// first would lose anything that vocabulary doesn't happen to cover
// (application-cursor-mode arrows, exotic modifier combinations, ...).
type RawInputReceiver interface {
	Widget
	WriteRaw(p []byte)
}

// focusedRawReceiver returns the focused widget as a RawInputReceiver, if
// the currently focused widget (in the topmost modal, or the active
// window if no modal is open) both exists and implements it — unless a
// mouse gesture is still in flight in that window (see
// Window.HasMouseCapture), in which case nil is returned regardless: its
// trailing EventMouseUp needs the normal decoded routing to reach
// whatever captured the gesture, not to be redirected to a widget that
// only gained focus as a side effect of the gesture's own EventMouseDown
// (e.g. a menu click that itself creates and focuses a new Terminal tab).
func (app *Application) focusedRawReceiver() RawInputReceiver {
	win := app.topModal()
	if win == nil {
		win = app.activeWindow
	}
	if win == nil || win.HasMouseCapture() {
		return nil
	}
	raw, _ := win.focusedWidget().(RawInputReceiver)
	return raw
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

// SetOnQuitRequested overrides what Escape does when no modal is open: instead
// of quitting immediately, Run calls fn and leaves the application running.
// fn is responsible for deciding whether to quit — typically by opening a
// ShowConfirm dialog whose "Yes" button calls Quit. Passing nil restores the
// default immediate-quit behavior.
func (app *Application) SetOnQuitRequested(fn func()) {
	app.onQuitRequested = fn
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

		if raw := app.focusedRawReceiver(); raw != nil {
			if data := app.term.pollRaw(); data != nil {
				lastIn = time.Now()
				app.forwardRaw(raw, data)
			}
			continue
		}

		ev := app.term.pollEvent()
		if ev.Type != EventNone {
			lastIn = time.Now()
		}

		if time.Since(lastIn).Milliseconds() > 300 && app.idleCallback != nil {
			app.idleCallback()
		}

		app.routeEvent(ev)
	}
}

// rawDetachByte is Ctrl+\ (ASCII FS, 0x1C) — the one byte forwardRaw
// intercepts instead of passing through to a focused RawInputReceiver, so
// there's still a way to move focus off an embedded terminal that
// otherwise consumes every single keystroke. Chosen because it's a
// POSIX-conventional signal key almost nothing uses interactively inside
// a shell.
const rawDetachByte = 0x1C

// forwardRaw sends data to raw, except for rawDetachByte, which instead
// advances focus (the same as an ordinary Tab keypress) so the terminal
// doesn't have to be the last focusable widget the user can ever reach.
func (app *Application) forwardRaw(raw RawInputReceiver, data []byte) {
	if idx := bytes.IndexByte(data, rawDetachByte); idx >= 0 {
		if idx > 0 {
			raw.WriteRaw(data[:idx])
		}
		app.routeEvent(Event{Type: EventKey, Key: KeyTab})
		return
	}
	raw.WriteRaw(data)
}

// routeEvent dispatches one polled event to the topmost modal, the active
// window, or Escape's own handling, exactly as the last step of Run's loop
// body. It is split out from Run so that Escape/quit routing — including
// SetOnQuitRequested — is exercisable from a test without driving the whole
// terminal loop.
func (app *Application) routeEvent(ev Event) {
	if top := app.topModal(); top != nil {
		if ev.Type == EventKey && ev.Key == KeyEscape {
			app.CloseModal()
		} else if ev.Type != EventNone {
			top.HandleEvent(ev)
		}
		return
	}

	if ev.Type == EventKey && ev.Key == KeyEscape {
		if app.onQuitRequested != nil {
			app.onQuitRequested()
		} else {
			app.Quit()
		}
	} else if ev.Type != EventNone && app.activeWindow != nil {
		app.activeWindow.HandleEvent(ev)
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
