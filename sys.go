package Graphite

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	eventQueue    []Event
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
	enableVirtualTerminal()
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

// pollRaw waits briefly for the next chunk of raw input, returning nil if
// nothing arrives within the timeout. Used instead of pollEvent while a
// RawInputReceiver has focus (see Application.focusedRawReceiver), so its
// bytes reach the receiver completely undecoded rather than round-
// tripping through parseANSI's Event vocabulary first. A handful of
// events already sitting in eventQueue from before focus moved to a raw
// receiver are simply left queued (and delivered once decoded input
// routing resumes) — an exceedingly rare mid-batch race in practice,
// since one input read almost always corresponds to one discrete
// keystroke.
func (t *terminal) pollRaw() []byte {
	select {
	case buf := <-t.input:
		return buf
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

// pollEvent waits briefly for the next input event, returning EventNone if
// nothing arrives within the timeout so the render loop keeps ticking (and
// can service idle callbacks, animations, etc.) even without input.
func (t *terminal) pollEvent() Event {
	if len(t.eventQueue) > 0 {
		ev := t.eventQueue[0]
		t.eventQueue = t.eventQueue[1:]
		return ev
	}
	select {
	case buf := <-t.input:
		evs := parseANSI(buf)
		if len(evs) > 0 {
			ev := evs[0]
			t.eventQueue = append(t.eventQueue, evs[1:]...)
			return ev
		}
		return Event{Type: EventNone}
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

// ss3Key maps the third byte of an SS3 escape sequence ("\x1bO" + letter) to
// the key it represents. xterm and its descendants (macOS Terminal.app,
// iTerm2, most Linux emulators) send F1-F4 this way.
var ss3Key = map[byte]KeyCode{
	'P': KeyF1,
	'Q': KeyF2,
	'R': KeyF3,
	'S': KeyF4,
}

// csiTildeKey maps the numeric parameter of a CSI-tilde escape sequence
// ("\x1b[" + digits + "~") to the key it represents. This is the portable
// encoding for Insert, Delete, and F5-F12 (and an alternate encoding some
// terminals also use for F1-F4) across every terminal Graphite targets,
// including Windows Terminal and conhost.exe once virtual-terminal input
// processing is enabled (see docs/windows-terminal.md). 16 and 22 have no
// assigned key by long-standing VT220 convention and are intentionally
// absent.
var csiTildeKey = map[int]KeyCode{
	2:  KeyInsert,
	3:  KeyDelete,
	11: KeyF1,
	12: KeyF2,
	13: KeyF3,
	14: KeyF4,
	15: KeyF5,
	17: KeyF6,
	18: KeyF7,
	19: KeyF8,
	20: KeyF9,
	21: KeyF10,
	23: KeyF11,
	24: KeyF12,
}

// parseANSI decodes raw reads from stdin into a slice of Graphite Events.
// It recognizes plain ASCII keys, common ANSI escape sequences (arrows,
// Delete, F1-F12), SGR mouse press/drag/release reports, and falls back to
// treating any other multi-byte sequence as decoded UTF-8 runes.
func parseANSI(buf []byte) []Event {
	if len(buf) == 0 {
		return nil
	}

	var events []Event

	for len(buf) > 0 {
		if buf[0] == 27 {
			if len(buf) >= 3 && buf[1] == 'O' {
				if key, ok := ss3Key[buf[2]]; ok {
					events = append(events, Event{Type: EventKey, Key: key})
					buf = buf[3:]
					continue
				}
			}

			if len(buf) >= 3 && buf[1] == '[' {
				matched := false
				if buf[2] == 'A' {
					events = append(events, Event{Type: EventKey, Key: KeyUp})
					buf = buf[3:]
					matched = true
				}
				if !matched && buf[2] == 'B' {
					events = append(events, Event{Type: EventKey, Key: KeyDown})
					buf = buf[3:]
					matched = true
				}
				if !matched && buf[2] == 'C' {
					events = append(events, Event{Type: EventKey, Key: KeyRight})
					buf = buf[3:]
					matched = true
				}
				if !matched && buf[2] == 'D' {
					events = append(events, Event{Type: EventKey, Key: KeyLeft})
					buf = buf[3:]
					matched = true
				}

				if matched {
					continue
				}

				if j := 2; j < len(buf) && buf[j] >= '0' && buf[j] <= '9' {
					for j < len(buf) && buf[j] >= '0' && buf[j] <= '9' {
						j++
					}
					if j < len(buf) && buf[j] == '~' {
						num, _ := strconv.Atoi(string(buf[2:j]))
						if key, ok := csiTildeKey[num]; ok {
							events = append(events, Event{Type: EventKey, Key: key})
						}
						buf = buf[j+1:]
						continue
					}
				}

				if buf[2] == '<' {
					endIdx := 3
					for endIdx < len(buf) && buf[endIdx] != 'M' && buf[endIdx] != 'm' {
						endIdx++
					}
					if endIdx < len(buf) {
						seq := string(buf[3:endIdx])
						isPress := buf[endIdx] == 'M'
						parts := strings.Split(seq, ";")

						if len(parts) == 3 {
							btn, _ := strconv.Atoi(parts[0])
							x, _ := strconv.Atoi(parts[1])
							y, _ := strconv.Atoi(parts[2])
							mx, my := x-1, y-1

							if !isPress {
								events = append(events, Event{Type: EventMouseUp, MouseX: mx, MouseY: my})
							} else {
								switch btn {
								case 0:
									events = append(events, Event{Type: EventMouseDown, MouseX: mx, MouseY: my})
								case 2:
									events = append(events, Event{Type: EventMouseRightDown, MouseX: mx, MouseY: my})
								case 32:
									events = append(events, Event{Type: EventMouseDrag, MouseX: mx, MouseY: my})
								case 64:
									events = append(events, Event{Type: EventMouseScrollUp, MouseX: mx, MouseY: my})
								case 65:
									events = append(events, Event{Type: EventMouseScrollDown, MouseX: mx, MouseY: my})
								}
							}
						}
						buf = buf[endIdx+1:]
						continue
					}
				}
			}

			events = append(events, Event{Type: EventKey, Key: KeyEscape})
			buf = buf[1:]
		} else {
			b := buf[0]
			switch b {
			case 3:
				events = append(events, Event{Type: EventKey, Key: KeyCtrlC})
			case 9:
				events = append(events, Event{Type: EventKey, Key: KeyTab})
			case 13:
				events = append(events, Event{Type: EventKey, Key: KeyEnter})
			case 22:
				events = append(events, Event{Type: EventKey, Key: KeyCtrlV})
			case 24:
				events = append(events, Event{Type: EventKey, Key: KeyCtrlX})
			case 32:
				events = append(events, Event{Type: EventKey, Key: KeySpace, CharCode: rune(b)})
			case 127:
				events = append(events, Event{Type: EventKey, Key: KeyBackspace})
			default:
				if b >= 32 {
					r, size := utf8.DecodeRune(buf)
					events = append(events, Event{Type: EventKey, CharCode: r})
					buf = buf[size:]
					continue
				}
			}
			buf = buf[1:]
		}
	}

	return events
}
