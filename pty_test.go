package Graphite

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStartPTY_RunsACommandAndReturnsItsOutput(t *testing.T) {
	shell, runFlag := testShell()
	pty, err := startPTY(shell, []string{runFlag, "echo hello-pty"}, 80, 24)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer pty.Close()

	buf := make([]byte, 4096)
	var got strings.Builder
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		n, err := pty.Read(buf)
		if n > 0 {
			got.Write(buf[:n])
		}
		if strings.Contains(got.String(), "hello-pty") {
			break
		}
		if err != nil {
			break
		}
	}

	if !strings.Contains(got.String(), "hello-pty") {
		t.Fatalf("pty output = %q, want it to contain \"hello-pty\"", got.String())
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

	buf := make([]byte, 4096)
	var got strings.Builder
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		n, err := pty.Read(buf)
		if n > 0 {
			got.Write(buf[:n])
		}
		if err != nil {
			break
		}
		if strings.Contains(got.String(), "/dev/") {
			break
		}
	}

	if !strings.Contains(got.String(), "/dev/") {
		t.Fatalf("`tty` output = %q, want a /dev/... path (i.e. a real controlling terminal)", got.String())
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
