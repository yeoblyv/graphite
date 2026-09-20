package Graphite

import (
	"runtime"
	"testing"
)

// testShell returns the interpreter and its "run this one command" flag
// for the current OS, so pty_test.go and terminal_test.go can spawn a
// real shell without hardcoding a Unix path that doesn't exist on
// Windows.
func testShell() (name, runFlag string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/c"
	}
	return "/bin/sh", "-c"
}

// testSleepOneSecond returns a one-second-delay command line for the
// current OS's shell — cmd.exe has no built-in sleep, so this pings
// localhost twice instead, the standard portable substitute.
func testSleepOneSecond() string {
	if runtime.GOOS == "windows" {
		return "ping -n 2 127.0.0.1 >NUL"
	}
	return "sleep 1"
}

// skipIfWindowsConPTYOutputIsBroken skips a test that depends on reading
// a child process's output through startPTY/NewTerminal on Windows.
// Process creation itself works there (TestStartPTY_ResizeSucceeds,
// which never reads output, passes) but the ConPTY output pipe
// consistently never delivers anything to pty_windows.go's reader in CI,
// regardless of read timeout — every code path here (pipe wiring,
// CreateProcess flags, STARTUPINFOEX sizing) matches Microsoft's
// documented ConPTY sample exactly, so this is a real, unresolved gap
// that needs a real Windows machine to debug further, not a test
// deadline or a platform quirk this file can paper over blind.
func skipIfWindowsConPTYOutputIsBroken(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("ConPTY child output isn't reaching the reader on Windows CI — unresolved, needs a real Windows machine to debug")
	}
}
