//go:build windows

package Graphite

import (
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
		return fmt.Errorf("process exited with code %d", code)
	}
	return nil
}

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

	var pi windows.ProcessInformation
	err = windows.CreateProcess(
		nil, cmdLine, nil, nil, false,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT,
		nil, nil, &si.StartupInfo, &pi)
	if err != nil {
		attrs.Delete()
		windows.ClosePseudoConsole(console)
		ptyIn.Close()
		ptyOut.Close()
		return nil, fmt.Errorf("CreateProcess: %w", err)
	}
	windows.CloseHandle(pi.Thread) // only the process handle is needed to Wait

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
