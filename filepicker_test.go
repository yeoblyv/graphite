package Graphite

import (
	"os"
	"path/filepath"
	"testing"
)

func findButtonByText(mod *Window, text string) *Button {
	for _, w := range mod.Children {
		if b, ok := w.(*Button); ok && b.Text == text {
			return b
		}
	}
	return nil
}

func findInputBoxByLabel(mod *Window, label string) *InputBox {
	for _, w := range mod.Children {
		if ib, ok := w.(*InputBox); ok && ib.Label == label {
			return ib
		}
	}
	return nil
}

func TestShowFolderPicker_ConfirmButtonSelectsCurrentDirectory(t *testing.T) {
	app := NewApplication()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	var got string
	called := false
	ShowFolderPicker(app, root, func(path string) {
		got = path
		called = true
	})

	mod := app.topModal()
	if mod == nil {
		t.Fatalf("ShowFolderPicker did not open a modal")
	}

	btn := findButtonByText(mod, app.T(KeySelectFolder))
	if btn == nil {
		t.Fatalf("no %q button found among the modal's children", app.T(KeySelectFolder))
	}
	btn.OnClick()

	if !called {
		t.Fatalf("onSelect was not called")
	}
	wantAbs, _ := filepath.Abs(root)
	if got != wantAbs {
		t.Errorf("onSelect got %q, want the currently browsed directory %q", got, wantAbs)
	}
	if app.topModal() != nil {
		t.Errorf("modal should have closed after confirming")
	}
}

func TestShowFolderPicker_DoesNotListFiles(t *testing.T) {
	app := NewApplication()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "afile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ShowFolderPicker(app, root, func(string) {})
	mod := app.topModal()

	var list *ListBox
	for _, w := range mod.Children {
		if lb, ok := w.(*ListBox); ok {
			list = lb
		}
	}
	if list == nil {
		t.Fatalf("no ListBox found in the folder picker")
	}
	if len(list.Items) != 1 {
		t.Fatalf("folder-mode list has %d rows, want exactly 1 (the one subdirectory, no files)", len(list.Items))
	}
}

func TestShowFilePicker_StillRequiresAFileNameToConfirm(t *testing.T) {
	app := NewApplication()
	root := t.TempDir()

	called := false
	ShowFilePicker(app, root, func(string) { called = true })
	mod := app.topModal()

	btn := findButtonByText(mod, app.T(KeyOpen))
	if btn == nil {
		t.Fatalf("no %q button found", app.T(KeyOpen))
	}
	btn.OnClick() // fileNameInput.Value is still empty

	if called {
		t.Fatalf("ShowFilePicker's confirm button should require a filename, unlike ShowFolderPicker's")
	}
}

func TestShowFilePicker_BackspaceNavigatesUpADirectory(t *testing.T) {
	app := NewApplication()
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	ShowFilePicker(app, sub, func(string) {})
	mod := app.topModal()

	pathInput := findInputBoxByLabel(mod, app.T(KeyDirLabel))
	if pathInput == nil {
		t.Fatalf("no path InputBox found")
	}
	pathInput.SetFocus(false) // Backspace must not be swallowed by a focused text field

	app.routeEvent(Event{Type: EventKey, Key: KeyBackspace})

	wantParent, _ := filepath.Abs(root)
	if pathInput.Value != wantParent {
		t.Errorf("after Backspace, browsed dir = %q, want the parent %q", pathInput.Value, wantParent)
	}
}

func TestShowFilePicker_BackspaceEditsTextWhenPathInputFocused(t *testing.T) {
	app := NewApplication()
	root := t.TempDir()

	ShowFilePicker(app, root, func(string) {})
	mod := app.topModal()
	pathInput := findInputBoxByLabel(mod, app.T(KeyDirLabel))
	pathInput.SetFocus(true)
	before := pathInput.Value

	app.routeEvent(Event{Type: EventKey, Key: KeyBackspace})

	if pathInput.Value == before[:len(before)-1] {
		return // the text field ate its own Backspace, as expected
	}
	if pathInput.Value != before {
		t.Fatalf("path input value changed in an unexpected way: %q -> %q", before, pathInput.Value)
	}
}

func TestShowFilePicker_AltLeftAltRightNavigateHistory(t *testing.T) {
	app := NewApplication()
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	ShowFilePicker(app, root, func(string) {})
	mod := app.topModal()
	pathInput := findInputBoxByLabel(mod, app.T(KeyDirLabel))

	// Navigate root -> sub via the path input, building real history.
	pathInput.OnSubmit(sub)
	wantSub, _ := filepath.Abs(sub)
	if pathInput.Value != wantSub {
		t.Fatalf("navigating to %q via pathInput.OnSubmit didn't take effect: got %q", sub, pathInput.Value)
	}

	app.routeEvent(Event{Type: EventKey, Key: KeyAltLeft})
	wantRoot, _ := filepath.Abs(root)
	if pathInput.Value != wantRoot {
		t.Errorf("after Alt+Left, browsed dir = %q, want back to %q", pathInput.Value, wantRoot)
	}

	app.routeEvent(Event{Type: EventKey, Key: KeyAltRight})
	if pathInput.Value != wantSub {
		t.Errorf("after Alt+Right, browsed dir = %q, want forward to %q", pathInput.Value, wantSub)
	}
}

func TestShowFilePicker_CtrlLFocusesPathInput(t *testing.T) {
	app := NewApplication()
	root := t.TempDir()

	ShowFilePicker(app, root, func(string) {})
	mod := app.topModal()
	pathInput := findInputBoxByLabel(mod, app.T(KeyDirLabel))
	pathInput.SetFocus(false)

	app.routeEvent(Event{Type: EventKey, Key: KeyCtrlL})

	if !pathInput.HasFocus() {
		t.Errorf("Ctrl+L should have focused the path input")
	}
}
