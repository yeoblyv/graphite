package Graphite

import (
	"strings"
	"testing"
	"time"
)

func TestNewTerminal_RunsAShellAndRendersItsOutput(t *testing.T) {
	term, err := NewTerminal(nil, 0, 0, 40, 10, "/bin/sh", []string{"-c", "echo hi"})
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	c := NewCanvas()
	c.Resize(80, 24)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		term.DrawRelative(c, 0, 0, 80, 24)
		if term.screen.Cell(0, 0).Ch == 'h' {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := term.screen.Cell(0, 0).Ch; got != 'h' {
		t.Fatalf("cell(0,0) = %q, want 'h' (from \"echo hi\")", got)
	}
	if got := term.screen.Cell(1, 0).Ch; got != 'i' {
		t.Fatalf("cell(1,0) = %q, want 'i'", got)
	}
}

func TestTerminal_WriteRawReachesTheChild(t *testing.T) {
	term, err := NewTerminal(nil, 0, 0, 40, 10, "/bin/sh", nil)
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	term.WriteRaw([]byte("echo raw-input-reached-the-shell\n"))

	deadline := time.Now().Add(2 * time.Second)
	var seen bool
	for time.Now().Before(deadline) {
		term.mu.Lock()
		var sb strings.Builder
		for y := 0; y < term.screen.rows; y++ {
			for x := 0; x < term.screen.cols; x++ {
				if ch := term.screen.Cell(x, y).Ch; ch != 0 {
					sb.WriteRune(ch)
				}
			}
		}
		term.mu.Unlock()
		if strings.Contains(sb.String(), "raw-input-reached-the-shell") {
			seen = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !seen {
		t.Fatal("shell output never showed the echoed text sent via WriteRaw")
	}
}

func TestTerminal_ExitedReportsAfterTheChildExits(t *testing.T) {
	term, err := NewTerminal(nil, 0, 0, 40, 10, "/bin/sh", []string{"-c", "exit 0"})
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if exited, _ := term.Exited(); exited {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Exited() never reported true after the child process exited")
}

func TestTerminal_ImplementsRawInputReceiver(t *testing.T) {
	term, err := NewTerminal(nil, 0, 0, 10, 5, "/bin/sh", nil)
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Close()

	var _ RawInputReceiver = term
}
