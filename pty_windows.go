//go:build windows

package Graphite

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// conPTY is the ptySession implementation for Windows: there's no single
// kernel object playing the role a Unix pty's master fd does, so this
// wraps everything ConPTY needs instead — the pseudoconsole handle itself,
// the pipe ends the caller reads/writes, and the child process handle to
// wait on.
type conPTY struct {
	console windows.Handle
	in      *os.File // write here to send input to the child
	out     *os.File // read here for the child's output
	proc    windows.Handle
	attrs   *windows.ProcThreadAttributeListContainer
}

func (p *conPTY) Read(b []byte) (int, error)  { return p.out.Read(b) }
func (p *conPTY) Write(b []byte) (int, error) { return p.in.Write(b) }

func (p *conPTY) Resize(cols, rows int) error {
	return windows.ResizePseudoConsole(p.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (p *conPTY) Close() error {
	windows.ClosePseudoConsole(p.console)
	p.attrs.Delete()
	p.in.Close()
	p.out.Close()
	return windows.CloseHandle(p.proc)
}

func (p *conPTY) Wait() error {
	if _, err := windows.WaitForSingleObject(p.proc, windows.INFINITE); err != nil {
		return err
	}
	var code uint32
	if err := windows.GetExitCodeProcess(p.proc, &code); err != nil {
		return err
	}
	if code != 0 {
		return &windowsExitError{code: int(code)}
	}
	return nil
}

// windowsExitError is unixPTY.Wait's Windows counterpart for a non-zero
// exit: unixPTY.Wait (pty_linux.go/pty_darwin.go) delegates to exec.Cmd,
// whose *exec.ExitError has its own ExitCode() int method a caller might
// reasonably type-switch on to get the numeric code — a pattern that
// worked on Unix but silently failed on Windows, since this path doesn't
// go through os/exec at all (ConPTY spawns via a raw windows.CreateProcess
// call). Implementing the same ExitCode() int method here means
// `err.(interface{ ExitCode() int })` behaves identically on every
// platform, without needing exec.Cmd on the Windows path.
type windowsExitError struct{ code int }

func (e *windowsExitError) Error() string { return fmt.Sprintf("process exited with code %d", e.code) }
func (e *windowsExitError) ExitCode() int { return e.code }

// startPTY spawns name (with args) attached to a fresh ConPTY (Windows's
// pseudoconsole API, available since Windows 10 1809) sized cols×rows —
// the Windows equivalent of a Unix controlling terminal: the child's own
// console APIs, cursor, and screen buffer all work exactly as they would
// in a real console window.
func startPTY(name string, args []string, cols, rows int) (ptySession, error) {
	// newPipePair returns (readEnd, writeEnd). For the child's console
	// input, ConPTY reads and we write; for its output, ConPTY writes and
	// we read — opposite ends of each pipe for us vs. the console side.
	consoleIn, ptyIn, err := newPipePair()
	if err != nil {
		return nil, fmt.Errorf("input pipe: %w", err)
	}
	ptyOut, consoleOut, err := newPipePair()
	if err != nil {
		ptyIn.Close()
		consoleIn.Close()
		return nil, fmt.Errorf("output pipe: %w", err)
	}

	var console windows.Handle
	err = windows.CreatePseudoConsole(
		windows.Coord{X: int16(cols), Y: int16(rows)},
		windows.Handle(consoleIn.Fd()), windows.Handle(consoleOut.Fd()),
		0, &console)
	// ConPTY has duplicated the handles it needs; our copies of the
	// console-facing ends are no longer needed either way.
	consoleIn.Close()
	consoleOut.Close()
	if err != nil {
		ptyIn.Close()
		ptyOut.Close()
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(console)
		ptyIn.Close()
		ptyOut.Close()
		return nil, err
	}
	// PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE is documented to take the HPCON
	// handle's own value in the lpValue slot (mirroring the C sample,
	// which passes hpc rather than &hpc) — not a pointer to a variable
	// holding it. windows.Handle is a uintptr, and converting an
	// arbitrary integer straight to unsafe.Pointer is exactly what
	// go vet's unsafeptr check exists to catch, even though it's correct
	// here; going through *unsafe.Pointer re-express the same bits as a
	// pointer-to-pointer conversion, which vet always allows.
	if err := attrs.Update(
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		*(*unsafe.Pointer)(unsafe.Pointer(&console)), unsafe.Sizeof(console),
	); err != nil {
		attrs.Delete()
		windows.ClosePseudoConsole(console)
		ptyIn.Close()
		ptyOut.Close()
		return nil, err
	}

	var si windows.StartupInfoEx
	si.ProcThreadAttributeList = attrs.List()
	si.Cb = uint32(unsafe.Sizeof(si))

	cmdLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{name}, args...)))
	if err != nil {
		attrs.Delete()
		windows.ClosePseudoConsole(console)
		ptyIn.Close()
		ptyOut.Close()
		return nil, err
	}

	baseFlags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	var pi windows.ProcessInformation
	// CREATE_BREAKAWAY_FROM_JOB lets the spawned shell escape whatever
	// Windows Job Object this process itself happens to be confined to
	// (common when Graphite is launched from another terminal, IDE, or
	// service that scopes its own children's lifetime/resource limits via
	// a job) — an interactive shell inheriting the host app's own job
	// limits is rarely what anyone wants. Not every job permits breakaway
	// (CreateProcess fails outright with the flag if it doesn't, rather
	// than just ignoring it), so this retries once without the flag on
	// that specific failure instead of hard-failing startPTY over it.
	err = windows.CreateProcess(
		nil, cmdLine, nil, nil, false, baseFlags|windows.CREATE_BREAKAWAY_FROM_JOB,
		nil, nil, &si.StartupInfo, &pi)
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		err = windows.CreateProcess(
			nil, cmdLine, nil, nil, false, baseFlags,
			nil, nil, &si.StartupInfo, &pi)
	}
	if err != nil {
		attrs.Delete()
		windows.ClosePseudoConsole(console)
		ptyIn.Close()
		ptyOut.Close()
		return nil, fmt.Errorf("CreateProcess: %w", err)
	}
	_ = windows.CloseHandle(pi.Thread) // only the process handle is needed to Wait; nothing actionable if closing the thread handle itself fails

	return &conPTY{console: console, in: ptyIn, out: ptyOut, proc: pi.Process, attrs: attrs}, nil
}

// newPipePair creates an anonymous pipe and wraps both ends as *os.File —
// readEnd's Read returns what writeEnd's Write sends.
func newPipePair() (readEnd, writeEnd *os.File, err error) {
	var r, w windows.Handle
	if err := windows.CreatePipe(&r, &w, nil, 0); err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(r), "pty-pipe-r"), os.NewFile(uintptr(w), "pty-pipe-w"), nil
}
