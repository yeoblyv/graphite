package Graphite

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

// =========================================================================
// SYSTEM TERMINAL & INPUT HANDLING
// =========================================================================

// terminal owns the raw-mode terminal state and the background stdin-reader
// goroutine for one Application. Keeping this state on a struct (rather
// than at package scope) means each Application instance is self-contained
// and tests can construct one without side effects on any other instance.
type terminal struct {
	oldState      *term.State
	input         chan []byte
	readerRunning bool
}

// newTerminal creates a terminal with its input channel ready to receive.
func newTerminal() *terminal {
	return &terminal{input: make(chan []byte, 100)}
}

// init switches the terminal into raw mode, enters the alternate screen
// buffer, hides the cursor, and enables SGR button-event mouse reporting
// (clicks, releases, and motion while a button is held — not idle
// movement, which would be reported for every pixel the pointer crosses).
// It starts the background input reader on first call and is safe to call
// again after restore (e.g. across a Suspend/Resume cycle).
func (t *terminal) init() {
	fmt.Print("\033[?1049h\033[?25l\033[?1002h\033[?1015h\033[?1006h")

	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		t.oldState = state
	}

	if !t.readerRunning {
		t.readerRunning = true
		go t.readInput()
	}
}

// readInput continuously reads raw bytes from stdin and forwards them to
// the input channel for pollEvent to consume. It runs for the lifetime of
// the process once started: reading from stdin has no reliable
// cross-platform cancellation, so restore does not attempt to stop it.
func (t *terminal) readInput() {
	reader := bufio.NewReader(os.Stdin)
	for {
		buf := make([]byte, 128)
		n, err := reader.Read(buf)
		if n > 0 && err == nil {
			t.input <- buf[:n]
		}
	}
}

// restore resets the terminal back to its original (cooked) mode, disables
// mouse reporting, shows the cursor, and exits the alternate screen buffer.
func (t *terminal) restore() {
	if t.oldState != nil {
		_ = term.Restore(int(os.Stdin.Fd()), t.oldState)
	}
	fmt.Print("\033[?1006l\033[?1015l\033[?1002l\033[?25h\033[?1049l")
}

// pollEvent waits briefly for the next input event, returning EventNone if
// nothing arrives within the timeout so the render loop keeps ticking (and
// can service idle callbacks, animations, etc.) even without input.
func (t *terminal) pollEvent() Event {
	select {
	case buf := <-t.input:
		return parseANSI(buf)
	case <-time.After(10 * time.Millisecond):
		return Event{Type: EventNone}
	}
}

// GetTerminalSize returns the current width and height of the terminal, or
// an 80x24 fallback if the size cannot be determined (e.g. stdout is not a
// terminal).
func GetTerminalSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80, 24
	}
	return w, h
}

// parseANSI decodes one raw read from stdin into a single Graphite Event.
// It recognizes plain ASCII keys, common ANSI escape sequences (arrows,
// Delete), SGR mouse press/drag/release reports, and falls back to treating
// any other multi-byte sequence as a single decoded UTF-8 rune.
func parseANSI(buf []byte) Event {
	if len(buf) == 0 {
		return Event{Type: EventNone}
	}

	if len(buf) == 1 {
		b := buf[0]
		switch b {
		case 9:
			return Event{Type: EventKey, Key: KeyTab}
		case 13:
			return Event{Type: EventKey, Key: KeyEnter}
		case 27:
			return Event{Type: EventKey, Key: KeyEscape}
		case 32:
			return Event{Type: EventKey, Key: KeySpace, CharCode: rune(b)}
		case 127:
			return Event{Type: EventKey, Key: KeyBackspace}
		default:
			if b >= 32 {
				return Event{Type: EventKey, CharCode: rune(b)}
			}
		}
	}

	if len(buf) >= 3 && buf[0] == 27 && buf[1] == '[' {
		if len(buf) == 3 {
			switch buf[2] {
			case 'A':
				return Event{Type: EventKey, Key: KeyUp}
			case 'B':
				return Event{Type: EventKey, Key: KeyDown}
			case 'C':
				return Event{Type: EventKey, Key: KeyRight}
			case 'D':
				return Event{Type: EventKey, Key: KeyLeft}
			}
		}

		if len(buf) >= 4 && buf[2] == '3' && buf[3] == '~' {
			return Event{Type: EventKey, Key: KeyDelete}
		}

		// SGR mouse report: \033[<Btn;X;Y M (press/drag) or m (release).
		if buf[2] == '<' {
			seq := string(buf[3:])
			isPress := strings.HasSuffix(seq, "M")
			seq = strings.TrimRight(seq, "Mm")
			parts := strings.Split(seq, ";")

			if len(parts) == 3 {
				btn, _ := strconv.Atoi(parts[0])
				x, _ := strconv.Atoi(parts[1])
				y, _ := strconv.Atoi(parts[2])
				mx, my := x-1, y-1 // Convert from 1-based to 0-based.

				if !isPress {
					// A release ends whatever widget captured the mouse on
					// the preceding press, regardless of which button — see
					// Window's mouse capture.
					return Event{Type: EventMouseUp, MouseX: mx, MouseY: my}
				}
				// SGR reports the left button as 0 for a fresh press, or 32
				// (button bit unchanged, motion bit set) while dragging.
				switch btn {
				case 0:
					return Event{Type: EventMouseDown, MouseX: mx, MouseY: my}
				case 32:
					return Event{Type: EventMouseDrag, MouseX: mx, MouseY: my}
				}
			}
		}
	}

	if len(buf) > 1 && buf[0] != 27 {
		runes := []rune(string(buf))
		if len(runes) > 0 {
			return Event{Type: EventKey, CharCode: runes[0]}
		}
	}

	return Event{Type: EventNone}
}
