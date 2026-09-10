package Graphite

import "io"

// ptySession is a running shell (or other interactive program) attached to
// a pseudo-terminal: writes go to its input exactly as if typed at a real
// terminal, and reads return exactly what it would have printed to one —
// including every escape sequence it emits, for Terminal's own VT100
// interpreter to consume. Implemented per-OS (pty_unix.go's startPTY for
// Linux/macOS via a real PTY device, pty_windows.go's via ConPTY), since
// the two platforms have no common kernel-level concept to share here.
type ptySession interface {
	io.ReadWriter

	// Resize tells the child process its terminal is now cols columns by
	// rows rows (SIGWINCH on Unix; ConPTY's own resize call on Windows).
	Resize(cols, rows int) error

	// Close releases the pty's own resources (the master side on Unix, the
	// pseudoconsole handle and pipes on Windows). It does not itself wait
	// for or kill the child process.
	Close() error

	// Wait blocks until the child process exits, returning its own error
	// (e.g. *exec.ExitError) if it exited non-zero.
	Wait() error
}
