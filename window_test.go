package Graphite

import "testing"

func TestWindow_ShiftTabReversesFocusOrder(t *testing.T) {
	w := NewWindow(40, 10, "")
	a := NewButton(0, 0, "a", BtnDefault, nil)
	b := NewButton(0, 1, "b", BtnDefault, nil)
	c := NewButton(0, 2, "c", BtnDefault, nil)
	w.AddWidget(a)
	w.AddWidget(b)
	w.AddWidget(c)
	// AddWidget focused a automatically (first focusable added).

	w.HandleEvent(Event{Type: EventKey, Key: KeyBackTab})
	if !c.HasFocus() {
		t.Fatalf("Shift+Tab from the first widget should wrap to the last one")
	}

	w.HandleEvent(Event{Type: EventKey, Key: KeyBackTab})
	if !b.HasFocus() {
		t.Fatalf("Shift+Tab should move focus backward through Tab order")
	}
}

func TestWindow_PreDispatchShortCircuitsNormalRouting(t *testing.T) {
	w := NewWindow(40, 10, "")
	btn := NewButton(0, 0, "a", BtnDefault, nil)
	w.AddWidget(btn)

	var seen Event
	called := false
	w.PreDispatch = func(ev Event) bool {
		seen = ev
		called = true
		return true // claim every event, so normal dispatch (e.g. Tab) never runs
	}

	w.HandleEvent(Event{Type: EventKey, Key: KeyTab})
	if !called {
		t.Fatalf("PreDispatch was not invoked")
	}
	if seen.Key != KeyTab {
		t.Fatalf("PreDispatch saw Key=%v, want KeyTab", seen.Key)
	}
	if !btn.HasFocus() {
		t.Fatalf("PreDispatch returning true should have stopped Tab from moving focus off the only widget")
	}
}

func TestWindow_PreDispatchFallsThroughWhenUnclaimed(t *testing.T) {
	w := NewWindow(40, 10, "")
	a := NewButton(0, 0, "a", BtnDefault, nil)
	b := NewButton(0, 1, "b", BtnDefault, nil)
	w.AddWidget(a)
	w.AddWidget(b)

	w.PreDispatch = func(ev Event) bool { return false } // never claims anything

	w.HandleEvent(Event{Type: EventKey, Key: KeyTab})
	if !b.HasFocus() {
		t.Fatalf("PreDispatch returning false should let normal Tab handling run")
	}
}
