package Graphite

import "strings"

// ShowConfirm opens a modal titled title asking the user to confirm message
// with "Yes"/"No" buttons. onConfirm runs only if "Yes" is chosen; either
// button closes the modal. style colors the "Yes" button — pass BtnDanger
// for a destructive confirmation (e.g. delete), BtnDefault for a neutral one.
func ShowConfirm(app *Application, title, message string, style ButtonStyle, onConfirm func()) {
	lines := measureTextWrapped(message, 42)
	winH := lines + 6
	if winH < 8 {
		winH = 8
	}

	mod := NewWindow(50, winH, " "+title+" ")
	mod.AddWidget(NewLabel(4, 2, message))
	mod.AddWidget(NewButton(4, winH-3, "Yes", style, func() {
		app.CloseModal()
		onConfirm()
	}))
	mod.AddWidget(NewButton(14, winH-3, "No", BtnDefault, func() {
		app.CloseModal()
	}))
	app.SetModal(mod)
}

// ShowTextEditor opens a modal titled title prompting for a single line of
// free-form text via label, pre-filled with current. It is ShowValueEditor's
// non-numeric counterpart, for callers that need an arbitrary string (a file
// or directory name) rather than a bounded number.
//
// Both the "OK" button and Enter inside the field (via InputBox.OnSubmit)
// confirm; a blank (whitespace-only) value shows an inline error and leaves
// the modal open instead of calling onConfirm, since every known caller
// (naming a file or directory) requires a non-blank result. "Cancel" closes
// without calling onConfirm.
func ShowTextEditor(app *Application, title, label, current string, onConfirm func(string)) {
	mod := NewWindow(44, 9, " "+title+" ")
	mod.AddWidget(NewLabel(2, 1, label))

	input := NewInputBox(2, 3, 38, "")
	input.Value = current
	input.CursorPos = len([]rune(current))
	mod.AddWidget(input)

	errLbl := NewLabel(2, 5, "")
	mod.AddWidget(errLbl)

	confirm := func(val string) {
		if strings.TrimSpace(val) == "" {
			errLbl.SetText("Value cannot be empty.")
			return
		}
		app.CloseModal()
		onConfirm(val)
	}
	input.OnSubmit = confirm

	mod.AddWidget(NewButton(2, 7, "OK", BtnSuccess, func() {
		confirm(input.Value)
	}))
	mod.AddWidget(NewButton(14, 7, "Cancel", BtnDefault, func() {
		app.CloseModal()
	}))

	app.SetModal(mod)
}
