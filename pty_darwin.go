//go:build darwin

package Graphite

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ioctlPtr issues an ioctl whose argument is an arbitrary pointer (unlike
// unix.IoctlSetInt/IoctlGetInt's plain-int argument) — needed for
// TIOCPTYGNAME, which fills a caller-provided byte buffer rather than
// reading or writing a single int.
func ioctlPtr(fd int, req uintptr, arg *byte) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(arg)))
	if errno != 0 {
		return errno
	}
	return nil
}

// unixPTY is the ptySession implementation shared in spirit by both Unix
// files (pty_darwin.go, pty_linux.go), differing only in how each obtains
// the master/slave file pair — Darwin and Linux use different ioctls for
// that one step (see openPTYPair in each file).
type unixPTY struct {
	master *os.File
	cmd    *exec.Cmd
}

func (p *unixPTY) Read(b []byte) (int, error)  { return p.master.Read(b) }
func (p *unixPTY) Write(b []byte) (int, error) { return p.master.Write(b) }
func (p *unixPTY) Close() error                { return p.master.Close() }
func (p *unixPTY) Wait() error                 { return p.cmd.Wait() }

func (p *unixPTY) Resize(cols, rows int) error {
	return unix.IoctlSetWinsize(int(p.master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: uint16(rows), Col: uint16(cols),
	})
}

// startPTY spawns name (with args) attached to a fresh pseudo-terminal
// sized cols×rows, with it as the child's controlling terminal — so
// programs that check isatty(0), print prompts, or install a SIGWINCH
// handler all behave exactly as they would in a real terminal.
func startPTY(name string, args []string, cols, rows int) (ptySession, error) {
	master, slavePath, err := openPTYPair()
	if err != nil {
		return nil, err
	}

	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		master.Close()
		return nil, err
	}
	defer slave.Close() // the child gets its own copy via Stdin/Stdout/Stderr below

	if err := unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: uint16(rows), Col: uint16(cols),
	}); err != nil {
		master.Close()
		return nil, err
	}

	cmd := exec.Command(name, args...)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true, // new session, so the slave can become its controlling terminal
		Setctty: true,
	}
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	if err := cmd.Start(); err != nil {
		master.Close()
		return nil, err
	}

	return &unixPTY{master: master, cmd: cmd}, nil
}

// openPTYPair opens a fresh pseudo-terminal via /dev/ptmx, Darwin's own
// three-ioctl grant/unlock/get-name dance (TIOCPTYGRANT, TIOCPTYUNLK,
// TIOCPTYGNAME) — a different sequence from Linux's TIOCGPTN/TIOCSPTLCK
// (see pty_linux.go), which is why this isn't one shared "unix" file.
func openPTYPair() (master *os.File, slavePath string, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, "", err
	}

	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		master.Close()
		return nil, "", fmt.Errorf("grant pty: %w", err)
	}
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		master.Close()
		return nil, "", fmt.Errorf("unlock pty: %w", err)
	}

	var nameBuf [1024]byte
	if err := ioctlPtr(fd, unix.TIOCPTYGNAME, &nameBuf[0]); err != nil {
		master.Close()
		return nil, "", fmt.Errorf("get pty name: %w", err)
	}
	name := string(nameBuf[:bytes.IndexByte(nameBuf[:], 0)])

	return master, name, nil
}
