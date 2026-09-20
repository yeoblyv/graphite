package Graphite

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// readUntil reads from pty in a background goroutine — which the caller
// never waits on — until want appears in the accumulated output or the
// pty returns an error, then sends the accumulated text once. Unlike a
// synchronous read loop gated by a deadline check between calls, this
// survives a Read that blocks past the deadline: on Windows, ConPTY's
// output pipe doesn't signal EOF when the child exits, only when the
// pseudoconsole itself closes (see pty.Close()), so a bare blocking Read
// after the child's done can hang indefinitely — exactly what
// Terminal.readLoop() already sidesteps by running in its own goroutine
// the caller doesn't join either.
func readUntil(pty ptySession, want string, timeout time.Duration) (got string, timedOut bool) {
	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		var sb strings.Builder
		for {
			n, err := pty.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
				if strings.Contains(sb.String(), want) {
					done <- sb.String()
					return
				}
			}
			if err != nil {
				done <- sb.String()
				return
			}
		}
	}()

	select {
	case out := <-done:
		return out, false
	case <-time.After(timeout):
		return "", true
	}
}

func TestStartPTY_RunsACommandAndReturnsItsOutput(t *testing.T) {
	shell, runFlag := testShell()
	pty, err := startPTY(shell, []string{runFlag, "echo hello-pty"}, 80, 24)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer pty.Close()

	got, timedOut := readUntil(pty, "hello-pty", 10*time.Second)
	if timedOut {
		t.Fatal("timed out waiting for \"hello-pty\" in pty output")
	}
	if !strings.Contains(got, "hello-pty") {
		t.Fatalf("pty output = %q, want it to contain \"hello-pty\"", got)
	}
}

func TestStartPTY_ChildSeesAControllingTerminal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no POSIX controlling-terminal/tty concept under ConPTY")
	}
	// A shell run without a controlling terminal can't do this; `tty`
	// prints the device path (e.g. /dev/ttys003 or /dev/pts/4) only when
	// stdin actually is one.
	pty, err := startPTY("/bin/sh", []string{"-c", "tty"}, 80, 24)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer pty.Close()

	got, timedOut := readUntil(pty, "/dev/", 10*time.Second)
	if timedOut {
		t.Fatal("timed out waiting for a /dev/... path in `tty` output")
	}
	if !strings.Contains(got, "/dev/") {
		t.Fatalf("`tty` output = %q, want a /dev/... path (i.e. a real controlling terminal)", got)
	}
}

func TestStartPTY_ResizeSucceeds(t *testing.T) {
	shell, runFlag := testShell()
	pty, err := startPTY(shell, []string{runFlag, testSleepOneSecond()}, 80, 24)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer pty.Close()

	if err := pty.Resize(120, 40); err != nil {
		t.Errorf("Resize: %v", err)
	}
}
