package Graphite

import "testing"

// stretch returns a Panel sized 0x0, which — per BaseWidget's existing
// DrawRelative rule — adopts whatever box its parent offers instead of
// keeping a fixed natural size. Flex tests use this to observe the size it
// actually computed and passed down, as opposed to a widget (like Label)
// that always keeps its own content-driven size regardless of the box it's
// offered.
func stretch() *Panel {
	return NewPanel(0, 0, 0, 0)
}

func TestFlex_RowSplitsWeightedChildrenProportionally(t *testing.T) {
	f := NewFlex(0, 0, 0, 0, FlexRow)
	a, b := stretch(), stretch()
	f.AddChild(a, 1)
	f.AddChild(b, 2)

	f.DrawRelative(NewCanvas(), 0, 0, 90, 10)

	if a.LastW != 30 {
		t.Errorf("weight-1 child width = %d, want 30 (1/3 of 90)", a.LastW)
	}
	if b.LastW != 60 {
		t.Errorf("weight-2 child width = %d, want 60 (2/3 of 90)", b.LastW)
	}
	if a.AbsX != 0 {
		t.Errorf("first child AbsX = %d, want 0", a.AbsX)
	}
	if b.AbsX != 30 {
		t.Errorf("second child AbsX = %d, want 30 (right after the first)", b.AbsX)
	}
	if a.LastH != 10 || b.LastH != 10 {
		t.Errorf("children should stretch to the full cross-axis size (10), got a=%d b=%d", a.LastH, b.LastH)
	}
}

func TestFlex_ColumnStacksTopToBottom(t *testing.T) {
	f := NewFlex(0, 0, 0, 0, FlexColumn)
	a, b := stretch(), stretch()
	f.AddChild(a, 1)
	f.AddChild(b, 1)

	f.DrawRelative(NewCanvas(), 0, 0, 20, 10)

	if a.LastH != 5 || b.LastH != 5 {
		t.Errorf("equal weights over height 10 should split 5/5, got a=%d b=%d", a.LastH, b.LastH)
	}
	if a.AbsY != 0 || b.AbsY != 5 {
		t.Errorf("expected a at y=0 and b at y=5, got a=%d b=%d", a.AbsY, b.AbsY)
	}
	if a.LastW != 20 || b.LastW != 20 {
		t.Errorf("children should stretch to the full cross-axis size (20), got a=%d b=%d", a.LastW, b.LastW)
	}
}

func TestFlex_FixedWeightChildUsesNaturalSize(t *testing.T) {
	f := NewFlex(0, 0, 0, 0, FlexRow)
	fixed := NewButton(0, 0, "OK", BtnDefault, nil) // natural width = len("OK")+4 = 6
	grow := stretch()
	f.AddChild(fixed, 0) // weight <= 0 => natural size
	f.AddChild(grow, 1)

	f.DrawRelative(NewCanvas(), 0, 0, 50, 3)

	if fixed.LastW != fixed.GetFixedW() {
		t.Errorf("fixed child width = %d, want its natural width %d", fixed.LastW, fixed.GetFixedW())
	}
	if grow.LastW != 44 {
		t.Errorf("weighted child should take remaining space (50-6=44), got %d", grow.LastW)
	}
}

func TestFlex_InvisibleChildIsSkippedEntirely(t *testing.T) {
	f := NewFlex(0, 0, 0, 0, FlexRow)
	a, hidden, c := stretch(), stretch(), stretch()
	hidden.SetVisible(false)
	f.AddChild(a, 1)
	f.AddChild(hidden, 1)
	f.AddChild(c, 1)

	f.DrawRelative(NewCanvas(), 0, 0, 60, 3)

	// Only a and c should share the space, 30 columns each, with c
	// immediately after a — the hidden child leaves no gap.
	if a.LastW != 30 || c.LastW != 30 {
		t.Errorf("visible children should split all 60 columns evenly, got a=%d c=%d", a.LastW, c.LastW)
	}
	if c.AbsX != 30 {
		t.Errorf("second visible child AbsX = %d, want 30 (immediately after the first, hidden child skipped)", c.AbsX)
	}
}

func TestFlex_GapInsertsSpaceBetweenChildren(t *testing.T) {
	f := NewFlex(0, 0, 0, 0, FlexRow)
	f.Gap = 2
	a, b := stretch(), stretch()
	f.AddChild(a, 1)
	f.AddChild(b, 1)

	f.DrawRelative(NewCanvas(), 0, 0, 20, 3)

	if b.AbsX != a.LastW+2 {
		t.Errorf("second child AbsX = %d, want %d (first child width + 2-column gap)", b.AbsX, a.LastW+2)
	}
}

func TestFlex_GetChildrenReturnsAddedWidgetsInOrder(t *testing.T) {
	f := NewFlex(0, 0, 0, 0, FlexRow)
	a := NewLabel(0, 0, "a")
	b := NewLabel(0, 0, "b")
	f.AddChild(a, 1)
	f.AddChild(b, 1)

	got := f.GetChildren()
	if len(got) != 2 || got[0] != Widget(a) || got[1] != Widget(b) {
		t.Fatalf("GetChildren() = %v, want [a, b] in insertion order", got)
	}
}

func TestFlex_ClearRemovesAllChildrenAndLaysOutFreshOnes(t *testing.T) {
	f := NewFlex(0, 0, 0, 0, FlexRow)
	old1, old2 := stretch(), stretch()
	f.AddChild(old1, 1)
	f.AddChild(old2, 1)

	f.Clear()
	if len(f.GetChildren()) != 0 {
		t.Fatalf("GetChildren() after Clear() = %v, want empty", f.GetChildren())
	}

	fresh := stretch()
	f.AddChild(fresh, 1)
	f.DrawRelative(NewCanvas(), 0, 0, 40, 5)

	// The fresh child alone should get the full width, not share it with
	// the cleared (but not garbage-collected-from-the-struct) old children.
	if fresh.LastW != 40 {
		t.Errorf("fresh child width = %d, want 40 (sole child after Clear)", fresh.LastW)
	}
	if len(f.GetChildren()) != 1 {
		t.Errorf("GetChildren() after Clear+AddChild = %v, want exactly the fresh child", f.GetChildren())
	}
}
