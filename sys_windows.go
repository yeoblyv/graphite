//go:build windows

package Graphite

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVirtualTerminal turns on ANSI/VT100 escape-sequence interpretation
// for the classic Windows console host (conhost.exe, as used by a plain
// cmd.exe or PowerShell window that isn't running inside Windows Terminal).
// Unlike Windows Terminal, conhost does not enable
// ENABLE_VIRTUAL_TERMINAL_PROCESSING by default, so without this call every
// SGR/cursor escape sequence this package writes prints as literal text
// (e.g. "[38;2;255;59;48m...") instead of being interpreted as color and
// cursor control. golang.org/x/term's MakeRaw only touches stdin's input
// mode, not stdout's output mode, so this is still needed alongside it.
func enableVirtualTerminal() {
	if h := windows.Handle(os.Stdout.Fd()); h != windows.InvalidHandle {
		var mode uint32
		if windows.GetConsoleMode(h, &mode) == nil {
			_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
		}
	}
	if h := windows.Handle(os.Stdin.Fd()); h != windows.InvalidHandle {
		var mode uint32
		if windows.GetConsoleMode(h, &mode) == nil {
			_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
		}
	}
}
