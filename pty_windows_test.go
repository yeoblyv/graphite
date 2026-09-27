//go:build windows

package Graphite

import "testing"

// TestWindowsExitError_ImplementsExitCodeLikeExecExitError guards the
// cross-platform Wait() contract: unixPTY.Wait (pty_linux.go/
// pty_darwin.go) returns *exec.ExitError for a non-zero exit, whose
// ExitCode() int method a caller can type-switch on — windowsExitError
// must expose the same method so that check behaves identically on every
// platform, without the caller needing an OS-specific branch.
func TestWindowsExitError_ImplementsExitCodeLikeExecExitError(t *testing.T) {
	var err error = &windowsExitError{code: 42}

	ec, ok := err.(interface{ ExitCode() int })
	if !ok {
		t.Fatalf("windowsExitError does not implement ExitCode() int")
	}
	if got := ec.ExitCode(); got != 42 {
		t.Errorf("ExitCode() = %d, want 42", got)
	}
	if err.Error() == "" {
		t.Errorf("Error() returned an empty string")
	}
}
