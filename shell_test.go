package Graphite

import "runtime"

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
