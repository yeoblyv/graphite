//go:build linux

package Graphite

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// unixPTY is the ptySession implementation shared in spirit by both Unix
// files (pty_linux.go, pty_darwin.go), differing only in how each obtains
// the master/slave file pair — Linux and Darwin use different ioctls for
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

// openPTYPair opens a fresh pseudo-terminal via /dev/ptmx, Linux's own
// unlock/get-number ioctl pair (TIOCSPTLCK, TIOCGPTN) plus constructing
// the conventional /dev/pts/N slave path — a different sequence from
// Darwin's grant/unlock/get-name-directly dance (see pty_darwin.go), which
// is why this isn't one shared "unix" file.
func openPTYPair() (master *os.File, slavePath string, err error) {
	master, err = os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, "", err
	}

	fd := int(master.Fd())
	// TIOCSPTLCK takes an int* the kernel reads the lock flag from
	// (get_user, not the value passed directly) — IoctlSetInt hands the
	// kernel the raw value as if it were a pointer, which faults on any
	// value other than a coincidentally-mapped address. IoctlSetPointerInt
	// is the one that actually takes its argument's address.
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		master.Close()
		return nil, "", fmt.Errorf("unlock pty: %w", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		master.Close()
		return nil, "", fmt.Errorf("get pty number: %w", err)
	}

	return master, "/dev/pts/" + strconv.Itoa(n), nil
}
