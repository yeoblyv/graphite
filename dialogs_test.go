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
