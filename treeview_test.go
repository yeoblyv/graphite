package Graphite

import (
	"strings"
	"testing"
)

func drawTreeView(t *TreeView, w, h int) *Canvas {
	c := NewCanvas()
	c.Resize(w, h)
	t.DrawRelative(c, 0, 0, w, h)
	return c
}

// canvasRowText concatenates row y's cell symbols, trimmed of trailing
// blanks, for asserting on exactly what a row rendered.
func canvasRowText(c *Canvas, y int) string {
	var b strings.Builder
	for x := 0; x < c.width; x++ {
		b.WriteString(c.buffer[y*c.width+x].Symbol)
	}
	return strings.TrimRight(b.String(), " ")
}

// TestTreeView_DrawsClassicBranchConnectors builds exactly the tree from
// the user's own example:
//
//	root/
//	├── folder_1/
//	│   ├── file_1.1.txt
//	│   └── file_1.2.txt
//	├── folder_2/
//	│   ├── subfolder_2.1/
//	│   │   └── file_2.1.1.docx
//	│   └── file_2.2.txt
//	└── file_root.txt
//
// and asserts the rendered rows match it rune-for-rune (past each row's
// own expand/collapse glyph, which the plain `tree` command doesn't
// draw but this interactive widget needs).
func TestTreeView_DrawsClassicBranchConnectors(t *testing.T) {
	file11 := &TreeNode{Label: "file_1.1.txt"}
	file12 := &TreeNode{Label: "file_1.2.txt"}
	folder1 := &TreeNode{Label: "folder_1/", HasChildren: true, Expanded: true, loaded: true, Children: []*TreeNode{file11, file12}}

	file211 := &TreeNode{Label: "file_2.1.1.docx"}
	subfolder21 := &TreeNode{Label: "subfolder_2.1/", HasChildren: true, Expanded: true, loaded: true, Children: []*TreeNode{file211}}
	file22 := &TreeNode{Label: "file_2.2.txt"}
	folder2 := &TreeNode{Label: "folder_2/", HasChildren: true, Expanded: true, loaded: true, Children: []*TreeNode{subfolder21, file22}}

	fileRoot := &TreeNode{Label: "file_root.txt"}

	root := &TreeNode{Label: "root/", HasChildren: true, Expanded: true, loaded: true, Children: []*TreeNode{folder1, folder2, fileRoot}}

	tv := NewTreeView(0, 0, 40, 10, []*TreeNode{root}, nil)
	tv.rebuildVisible() // Expanded/Children were set directly on the literals above, not via Expand

	c := drawTreeView(tv, 40, 10)

	want := []string{
		"▼ root/",
		"├── ▼ folder_1/",
		"│   ├──   file_1.1.txt",
		"│   └──   file_1.2.txt",
		"├── ▼ folder_2/",
		"│   ├── ▼ subfolder_2.1/",
		"│   │   └──   file_2.1.1.docx",
		"│   └──   file_2.2.txt",
		"└──   file_root.txt",
	}
	for i, w := range want {
		if got := canvasRowText(c, i); got != w {
			t.Errorf("row %d = %q, want %q", i, got, w)
		}
	}
}

func TestTreeView_InitialSelectionIsFirstRoot(t *testing.T) {
	roots := []*TreeNode{{Label: "a"}, {Label: "b"}}
	tv := NewTreeView(0, 0, 20, 5, roots, nil)
	if tv.Selected != roots[0] {
		t.Fatalf("Selected = %v, want the first root", tv.Selected)
	}
}

func TestTreeView_ExpandLazilyLoadsChildrenOnce(t *testing.T) {
	root := &TreeNode{Label: "dir", HasChildren: true}
	calls := 0
	roots := []*TreeNode{root}
	tv := NewTreeView(0, 0, 20, 5, roots, nil)
	tv.LoadChildren = func(node *TreeNode) []*TreeNode {
		calls++
		return []*TreeNode{{Label: "child1"}, {Label: "child2"}}
	}

	tv.Expand(root)
	if calls != 1 {
		t.Fatalf("LoadChildren called %d times on first Expand, want 1", calls)
	}
	if len(root.Children) != 2 {
		t.Fatalf("root.Children = %d entries, want 2", len(root.Children))
	}
	if len(tv.visible) != 3 {
		t.Fatalf("visible = %d rows after expand, want 3 (root + 2 children)", len(tv.visible))
	}

	tv.Collapse(root)
	tv.Expand(root)
	if calls != 1 {
		t.Fatalf("LoadChildren called %d times across expand/collapse/expand, want 1 (loaded once, cached)", calls)
	}
	if len(tv.visible) != 3 {
		t.Fatalf("visible = %d rows after re-expand, want 3", len(tv.visible))
	}
}

func TestTreeView_CollapseHidesDescendantsFromVisible(t *testing.T) {
	child := &TreeNode{Label: "child"}
	root := &TreeNode{Label: "root", HasChildren: true, Children: []*TreeNode{child}, loaded: true}
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{root}, nil)

	tv.Expand(root)
	if len(tv.visible) != 2 {
		t.Fatalf("visible = %d after expand, want 2", len(tv.visible))
	}

	tv.Collapse(root)
	if len(tv.visible) != 1 {
		t.Fatalf("visible = %d after collapse, want 1 (child hidden)", len(tv.visible))
	}
}

func TestTreeView_StaleSelectedAfterCollapseDoesNotPanic(t *testing.T) {
	// Mirrors TestListBox_StaleSelectedAfterShrinkDoesNotPanic: Selected
	// pointed at a node that collapsing just removed from visible, so
	// rebuildVisible must fall back to something safe rather than leaving
	// a dangling Selected that HandleEvent's Enter later dereferences.
	child := &TreeNode{Label: "child"}
	root := &TreeNode{Label: "root", HasChildren: true, Children: []*TreeNode{child}, loaded: true}
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{root}, nil)
	tv.Expand(root)
	tv.Selected = child

	tv.Collapse(root)
	if tv.Selected != root {
		t.Fatalf("Selected after collapsing the selected child's parent = %v, want it to fall back to the first visible node (root)", tv.Selected)
	}

	tv.HandleEvent(Event{Type: EventKey, Key: KeyEnter}) // must not panic
}

func TestTreeView_RightArrowExpandsThenDrillsIn(t *testing.T) {
	child := &TreeNode{Label: "child"}
	root := &TreeNode{Label: "root", HasChildren: true, Children: []*TreeNode{child}, loaded: true}
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{root}, nil)

	tv.HandleEvent(Event{Type: EventKey, Key: KeyRight})
	if !root.Expanded {
		t.Fatalf("first Right should expand the collapsed root")
	}
	if tv.Selected != root {
		t.Fatalf("first Right should not move selection off root, got %v", tv.Selected)
	}

	tv.HandleEvent(Event{Type: EventKey, Key: KeyRight})
	if tv.Selected != child {
		t.Fatalf("second Right (already expanded) should drill into the first child, got %v", tv.Selected)
	}
}

func TestTreeView_LeftArrowCollapsesThenJumpsToParent(t *testing.T) {
	child := &TreeNode{Label: "child"}
	root := &TreeNode{Label: "root", HasChildren: true, Children: []*TreeNode{child}, loaded: true}
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{root}, nil)
	tv.Expand(root)
	tv.Selected = child

	tv.HandleEvent(Event{Type: EventKey, Key: KeyLeft})
	if !root.Expanded {
		t.Fatalf("Left on a leaf should not collapse its parent")
	}
	if tv.Selected != root {
		t.Fatalf("Left on a leaf should jump selection to its parent, got %v", tv.Selected)
	}

	tv.HandleEvent(Event{Type: EventKey, Key: KeyLeft})
	if root.Expanded {
		t.Fatalf("Left on an expanded node should collapse it")
	}
	if tv.Selected != root {
		t.Fatalf("Left that collapses should leave selection on the node itself, got %v", tv.Selected)
	}
}

func TestTreeView_UpDownNavigateFlattenedVisibleList(t *testing.T) {
	child := &TreeNode{Label: "child"}
	root := &TreeNode{Label: "root", HasChildren: true, Children: []*TreeNode{child}, loaded: true, Expanded: true}
	sibling := &TreeNode{Label: "sibling"}
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{root, sibling}, nil)
	tv.rebuildVisible() // Expanded was set directly on the literal above, not via Expand

	if tv.Selected != root {
		t.Fatalf("Selected = %v, want root first", tv.Selected)
	}
	tv.HandleEvent(Event{Type: EventKey, Key: KeyDown})
	if tv.Selected != child {
		t.Fatalf("after one Down, Selected = %v, want child", tv.Selected)
	}
	tv.HandleEvent(Event{Type: EventKey, Key: KeyDown})
	if tv.Selected != sibling {
		t.Fatalf("after two Down, Selected = %v, want sibling", tv.Selected)
	}
	tv.HandleEvent(Event{Type: EventKey, Key: KeyUp})
	if tv.Selected != child {
		t.Fatalf("after Up from sibling, Selected = %v, want child", tv.Selected)
	}
}

func TestTreeView_EnterFiresOnSelectOrOnDoubleClick(t *testing.T) {
	root := &TreeNode{Label: "root"}
	var selected *TreeNode
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{root}, func(n *TreeNode) { selected = n })

	tv.HandleEvent(Event{Type: EventKey, Key: KeyEnter})
	if selected != root {
		t.Fatalf("OnSelect not fired by Enter")
	}

	var doubled *TreeNode
	tv.OnDoubleClick = func(n *TreeNode) { doubled = n }
	selected = nil
	tv.HandleEvent(Event{Type: EventKey, Key: KeyEnter})
	if doubled != root {
		t.Fatalf("OnDoubleClick not fired by Enter once set")
	}
	if selected != nil {
		t.Fatalf("OnSelect should not also fire once OnDoubleClick is set (mirrors ListBox)")
	}
}

func TestTreeView_ClickOnGlyphTogglesWithoutFiringOnSelect(t *testing.T) {
	root := &TreeNode{Label: "root", HasChildren: true}
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{root}, nil)
	tv.LoadChildren = func(*TreeNode) []*TreeNode { return []*TreeNode{{Label: "child"}} }
	drawTreeView(tv, 20, 5) // resolves AbsX/AbsY/LastW/LastH

	called := false
	tv.OnSelect = func(*TreeNode) { called = true }
	tv.HandleEvent(Event{Type: EventMouseDown, MouseX: tv.AbsX, MouseY: tv.AbsY})

	if !root.Expanded {
		t.Fatalf("clicking the glyph column should have expanded root")
	}
	if called {
		t.Fatalf("clicking the glyph column should toggle, not also fire OnSelect")
	}
}

func TestTreeView_SetRootsResetsStaleSelection(t *testing.T) {
	oldRoot := &TreeNode{Label: "old"}
	tv := NewTreeView(0, 0, 20, 5, []*TreeNode{oldRoot}, nil)

	newRoot := &TreeNode{Label: "new"}
	tv.SetRoots([]*TreeNode{newRoot})

	if tv.Selected != newRoot {
		t.Fatalf("after SetRoots dropped the old tree, Selected = %v, want the new first root", tv.Selected)
	}
}
