package Graphite

import "testing"

// buttons returns every *Button among mod's top-level children, in order,
// so a test can find "OK"/"Cancel"/"Yes"/"No" without hardcoding indices.
func buttons(mod *Window) []*Button {
	var out []*Button
	for _, w := range mod.Children {
		if b, ok := w.(*Button); ok {
			out = append(out, b)
		}
	}
	return out
}

// clickButtonByMouse simulates a real mouse click on btn by going through
// the actual Draw -> HitTest -> HandleEvent pipeline, instead of calling
// OnClick directly. This is the only way to catch a button whose resolved
// LastW/LastH ends up 0 — perfectly focusable and Enter-triggerable, but
// never clickable, since HitTest requires a positive width and height.
// mod must already be on app's modal stack.
func clickButtonByMouse(t *testing.T, mod *Window, btn *Button) {
	t.Helper()
	c := NewCanvas()
	c.Resize(213, 54) // a realistic terminal size, not the library's 80x24 fallback
	mod.Draw(c)
	if btn.LastW <= 0 || btn.LastH <= 0 {
		t.Fatalf("button %q resolved to LastW=%d LastH=%d — HitTest can never match it, so a mouse click can never reach it", btn.Text, btn.LastW, btn.LastH)
	}
	mod.HandleEvent(Event{Type: EventMouseDown, MouseX: btn.AbsX + 1, MouseY: btn.AbsY})
}

// Regression: ShowConfirm originally placed its buttons at a fixed
// "winH-3" row, computed from the modal's own requested height rather
// than the content area PaddingY leaves available. BaseWidget's
// parent-bounds clamp silently capped their LastH at 0 whenever that
// arithmetic put them past the actual content bounds — Enter still
// triggered them (keyboard routing goes by focus, not HitTest), but a
// mouse click never could.
func TestShowConfirm_NoButtonIsActuallyClickable(t *testing.T) {
	app := NewApplication()
	called := false

	ShowConfirm(app, "Delete", "Delete this file?", BtnDanger, func() { called = true })
	mod := app.topModal()
	clickButtonByMouse(t, mod, buttons(mod)[1]) // No

	if called {
		t.Error("clicking No by mouse called onConfirm")
	}
	if app.topModal() != nil {
		t.Error("modal still open after clicking No by mouse")
	}
}

func TestShowConfirm_YesButtonIsActuallyClickable(t *testing.T) {
	app := NewApplication()
	called := false

	ShowConfirm(app, "Delete", "Delete this file?", BtnDanger, func() { called = true })
	mod := app.topModal()
	clickButtonByMouse(t, mod, buttons(mod)[0]) // Yes

	if !called {
		t.Error("clicking Yes by mouse did not call onConfirm")
	}
}

func TestShowConfirm_YesCallsOnConfirmAndCloses(t *testing.T) {
	app := NewApplication()
	called := false

	ShowConfirm(app, "Delete", "Delete this file?", BtnDanger, func() { called = true })

	mod := app.topModal()
	if mod == nil {
		t.Fatal("ShowConfirm did not open a modal")
	}

	btns := buttons(mod)
	if len(btns) != 2 {
		t.Fatalf("got %d buttons, want 2 (Yes, No)", len(btns))
	}
	if btns[0].Text != "Yes" || btns[1].Text != "No" {
		t.Fatalf("button labels = %q, %q, want Yes, No", btns[0].Text, btns[1].Text)
	}

	btns[0].OnClick()

	if !called {
		t.Error("onConfirm was not called after Yes")
	}
	if app.topModal() != nil {
		t.Error("modal still open after Yes")
	}
}

func TestShowConfirm_NoClosesWithoutCalling(t *testing.T) {
	app := NewApplication()
	called := false

	ShowConfirm(app, "Delete", "Delete this file?", BtnDanger, func() { called = true })
	buttons(app.topModal())[1].OnClick() // No

	if called {
		t.Error("onConfirm was called after No")
	}
	if app.topModal() != nil {
		t.Error("modal still open after No")
	}
}

// Regression: ShowTextEditor mirrored ShowValueEditor's undersized modal
// height (see fader_test.go's TestShowValueEditor_OKButtonIsActuallyClickable
// for the fuller explanation) — its OK/Cancel row sat past the actual
// content bounds, so BaseWidget's parent-bounds clamp capped their LastH
// at 0.
func TestShowTextEditor_OKButtonIsActuallyClickable(t *testing.T) {
	app := NewApplication()
	var got string

	ShowTextEditor(app, "Rename", "New name:", "old.txt", func(v string) { got = v })
	mod := app.topModal()
	clickButtonByMouse(t, mod, buttons(mod)[0]) // OK, with the prefilled value untouched

	if got != "old.txt" {
		t.Errorf("clicking OK by mouse got %q, want %q", got, "old.txt")
	}
}

func TestShowTextEditor_CancelButtonIsActuallyClickable(t *testing.T) {
	app := NewApplication()
	called := false

	ShowTextEditor(app, "Rename", "New name:", "old.txt", func(string) { called = true })
	mod := app.topModal()
	clickButtonByMouse(t, mod, buttons(mod)[1]) // Cancel

	if called {
		t.Error("clicking Cancel by mouse called onConfirm")
	}
	if app.topModal() != nil {
		t.Error("modal still open after clicking Cancel by mouse")
	}
}

func TestShowTextEditor_PrefillsAndConfirms(t *testing.T) {
	app := NewApplication()
	var got string

	ShowTextEditor(app, "Rename", "New name:", "old.txt", func(v string) { got = v })

	mod := app.topModal()
	var input *InputBox
	for _, w := range mod.Children {
		if ib, ok := w.(*InputBox); ok {
			input = ib
		}
	}
	if input == nil {
		t.Fatal("ShowTextEditor did not add an InputBox")
	}
	if input.Value != "old.txt" {
		t.Fatalf("input prefilled with %q, want %q", input.Value, "old.txt")
	}

	input.Value = "new.txt"
	buttons(mod)[0].OnClick() // OK

	if got != "new.txt" {
		t.Errorf("onConfirm got %q, want %q", got, "new.txt")
	}
	if app.topModal() != nil {
		t.Error("modal still open after OK")
	}
}

func TestShowTextEditor_BlankValueStaysOpen(t *testing.T) {
	app := NewApplication()
	called := false

	ShowTextEditor(app, "New folder", "Name:", "", func(string) { called = true })

	mod := app.topModal()
	buttons(mod)[0].OnClick() // OK with the still-blank prefilled value

	if called {
		t.Error("onConfirm was called with a blank value")
	}
	if app.topModal() != mod {
		t.Error("modal was closed despite a blank, invalid value")
	}
}

func TestShowTextEditor_CancelClosesWithoutCalling(t *testing.T) {
	app := NewApplication()
	called := false

	ShowTextEditor(app, "Rename", "New name:", "old.txt", func(string) { called = true })
	buttons(app.topModal())[1].OnClick() // Cancel

	if called {
		t.Error("onConfirm was called after Cancel")
	}
	if app.topModal() != nil {
		t.Error("modal still open after Cancel")
	}
}
