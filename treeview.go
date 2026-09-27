package Graphite

import (
	"math"
	"strings"
	"time"
)

// TreeNode is one entry in a TreeView's hierarchy. It's a plain data node,
// not a Widget — like ListBox.Items or TodoList.Items, a tree's rows are
// data a single TreeView widget draws and navigates, not a nested widget
// tree of their own (see docs/custom-widgets.md's GetChildren guidance).
type TreeNode struct {
	Label string

	// HasChildren marks a node that CAN have children even before they've
	// been loaded — e.g. a directory in a file manager, known to be
	// expandable without having read its contents yet. A node with
	// HasChildren false never shows an expand/collapse glyph and Expand is
	// a no-op on it, regardless of Children's current contents.
	HasChildren bool
	Expanded    bool
	Children    []*TreeNode

	loaded bool // LoadChildren has already been called for this node
	depth  int  // set by rebuildVisible; drives indentation and Left/Right
	parent *TreeNode
	isLast bool // true if this is the last child among its siblings (or the last root) — see treeBranchPrefix
}

// TreeView is a focusable, scrollable, single-selection tree list with
// expand/collapse — the structural sibling of ListBox, but over a
// hierarchy instead of a flat slice.
type TreeView struct {
	BaseWidget

	Roots []*TreeNode

	// Selected is the currently selected node's identity, not an index into
	// some flattened slice — unlike ListBox.Selected (an int), a pointer
	// can't silently end up pointing at the wrong row when Roots/Children
	// is mutated externally and shrinks; rebuildVisible re-validates it
	// against the new visible set on every structural change instead.
	Selected *TreeNode
	Scroll   int

	// LoadChildren, if set, is called at most once per node — the first
	// time it's expanded — to lazily populate Children. Left nil, the tree
	// is fully static: Children must already be populated wherever
	// HasChildren is true.
	LoadChildren func(node *TreeNode) []*TreeNode

	OnSelect      func(node *TreeNode)
	OnDoubleClick func(node *TreeNode)
	OnExpand      func(node *TreeNode)
	OnCollapse    func(node *TreeNode)

	visible []*TreeNode // flattened, expanded-aware; rebuilt on structural change

	lastClickTime time.Time
	lastClickIdx  int
}

// NewTreeView creates a TreeView at (x, y) over roots. onSelect, if
// non-nil, is called with the selected node on Enter or a mouse click on a
// row (mirroring ListBox's onSelect constructor parameter).
func NewTreeView(x, y, w, h int, roots []*TreeNode, onSelect func(node *TreeNode)) *TreeView {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = true
	t := &TreeView{BaseWidget: base, OnSelect: onSelect}
	t.SetRoots(roots)
	return t
}

// SetRoots replaces the tree's top-level nodes and reflattens the visible
// list. Selected is preserved if it's still present somewhere in the new
// tree, otherwise reset to the first visible node (or nil if there is
// none) — the same "re-validate, don't trust stale state" rule ListBox's
// own Enter handler follows for its int Selected.
func (t *TreeView) SetRoots(roots []*TreeNode) {
	t.Roots = roots
	// rebuildVisible also (re)assigns parent for every node it walks,
	// starting each root at nil.
	t.rebuildVisible()
}

// rebuildVisible walks the tree depth-first, descending into a node's
// Children only while it's Expanded, producing the flat row list
// DrawRelative/HandleEvent both work over — the same role ListBox.Items
// plays, computed instead of stored directly.
func (t *TreeView) rebuildVisible() {
	t.visible = t.visible[:0]
	// parent is assigned here, for every node this walk actually visits,
	// rather than only in Expand's lazy-load branch — a fully static tree
	// (LoadChildren nil, Children pre-populated by the caller) needs Left's
	// jump-to-parent to work too, and this covers both uniformly.
	var walk func(nodes []*TreeNode, depth int, parent *TreeNode)
	walk = func(nodes []*TreeNode, depth int, parent *TreeNode) {
		for i, n := range nodes {
			n.depth = depth
			n.parent = parent
			n.isLast = i == len(nodes)-1
			t.visible = append(t.visible, n)
			if n.Expanded && len(n.Children) > 0 {
				walk(n.Children, depth+1, n)
			}
		}
	}
	walk(t.Roots, 0, nil)

	if t.selectedIndex() < 0 {
		if len(t.visible) > 0 {
			t.Selected = t.visible[0]
		} else {
			t.Selected = nil
		}
	}
}

// selectedIndex returns Selected's position in visible, or -1 if Selected
// is nil or no longer present (see Selected's own doc comment).
func (t *TreeView) selectedIndex() int {
	if t.Selected == nil {
		return -1
	}
	for i, n := range t.visible {
		if n == t.Selected {
			return i
		}
	}
	return -1
}

// ensureVisible scrolls just enough to bring the current Selected back
// into the visible window, e.g. after Left/Right jumps to a parent/child
// outside it.
func (t *TreeView) ensureVisible() {
	idx := t.selectedIndex()
	if idx < 0 {
		return
	}
	if idx < t.Scroll {
		t.Scroll = idx
	} else if t.LastH > 0 && idx >= t.Scroll+t.LastH {
		t.Scroll = idx - t.LastH + 1
	}
}

// Expand opens node, lazily populating Children via LoadChildren on its
// first expansion (LoadChildren nil, or node already loaded once, is a
// no-op there — Children is used as-is). A no-op if node is nil, already
// expanded, or HasChildren is false.
func (t *TreeView) Expand(node *TreeNode) {
	if node == nil || !node.HasChildren || node.Expanded {
		return
	}
	if !node.loaded {
		if t.LoadChildren != nil {
			node.Children = t.LoadChildren(node)
		}
		node.loaded = true
	}
	node.Expanded = true
	if t.OnExpand != nil {
		t.OnExpand(node)
	}
	t.rebuildVisible()
}

// Collapse closes node. A no-op if node is nil or already collapsed.
func (t *TreeView) Collapse(node *TreeNode) {
	if node == nil || !node.Expanded {
		return
	}
	node.Expanded = false
	if t.OnCollapse != nil {
		t.OnCollapse(node)
	}
	t.rebuildVisible()
}

// Toggle collapses an expanded node or expands a collapsed one.
func (t *TreeView) Toggle(node *TreeNode) {
	if node == nil {
		return
	}
	if node.Expanded {
		t.Collapse(node)
	} else {
		t.Expand(node)
	}
}

// treeIndentWidth is how many columns one level of treeBranchPrefix takes
// up ("│   ", "    ", "├── ", and "└── " are all 4 runes), and what
// HandleEvent uses to hit-test a click against a row's expand/collapse
// glyph rather than the label past it.
const treeIndentWidth = 4

// treeBranchPrefix builds the ├──/└──/│ connector prefix for node,
// matching the classic `tree`-command rendering: walking node's ancestor
// chain via parent, each ancestor level draws "│   " if that ancestor
// still has a later sibling (so its vertical line continues down past
// this row) or "    " if it was the last child (nothing left to connect
// to below it), and node's own immediate level draws "├── " or "└── "
// depending on whether node itself is the last child. A root-level node
// (no parent) gets no prefix at all — same as ListBox's top-level rows
// having no indentation of their own.
func treeBranchPrefix(node *TreeNode) string {
	if node.parent == nil {
		return ""
	}
	var ancestors []*TreeNode
	for p := node.parent; p != nil; p = p.parent {
		ancestors = append(ancestors, p)
	}

	// ancestors[len(ancestors)-1] is the topmost ancestor (its own parent
	// is nil) — like node itself would if it had no parent, it gets no
	// connector of its own, so it contributes no continuation column to
	// its descendants either; skip it, starting one below it instead.
	var b strings.Builder
	for i := len(ancestors) - 2; i >= 0; i-- {
		if ancestors[i].isLast {
			b.WriteString("    ")
		} else {
			b.WriteString("│   ")
		}
	}
	if node.isLast {
		b.WriteString("└── ")
	} else {
		b.WriteString("├── ")
	}
	return b.String()
}

// DrawRelative implements Widget.
func (t *TreeView) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	t.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	bg := c.theme.BgWidget
	if t.IsFocused {
		bg = RGB(45, 53, 62)
	}

	for iy := 0; iy < t.LastH; iy++ {
		for ix := 0; ix < t.LastW; ix++ {
			c.DrawCell(t.AbsX+ix, t.AbsY+iy, " ", bg, c.theme.FgWindow)
		}
	}

	for i := 0; i < t.LastH && (i+t.Scroll) < len(t.visible); i++ {
		node := t.visible[i+t.Scroll]
		isSelected := node == t.Selected
		ibg, ifg := bg, c.theme.FgWindow
		if isSelected {
			ibg, ifg = c.theme.Primary, c.theme.Primary.ContrastText()
		}

		for ix := 0; ix < t.LastW-1; ix++ {
			c.DrawCell(t.AbsX+ix, t.AbsY+i, " ", ibg, c.theme.FgWindow)
		}

		glyph := "  "
		if node.HasChildren {
			if node.Expanded {
				glyph = "▼ "
			} else {
				glyph = "▶ "
			}
		}
		prefix := treeBranchPrefix(node)
		c.DrawTextBounded(t.AbsX, t.AbsY+i, t.LastW, prefix+glyph+node.Label, ibg, ifg)
	}

	if len(t.visible) > t.LastH {
		sH := int(math.Max(1, float64((t.LastH*t.LastH)/len(t.visible))))
		tY := 0
		maxScroll := len(t.visible) - t.LastH
		if maxScroll > 0 {
			tY = (t.Scroll * (t.LastH - sH)) / maxScroll
		}
		for i := 0; i < t.LastH; i++ {
			if i >= tY && i < tY+sH {
				c.DrawCell(t.AbsX+t.LastW-1, t.AbsY+i, "█", bg, c.theme.Primary)
			} else {
				c.DrawCell(t.AbsX+t.LastW-1, t.AbsY+i, "│", bg, c.theme.FgDisabled)
			}
		}
	}
}

// HandleEvent implements Widget: Up/Down/Home/End/PageUp/PageDown move the
// selection (scrolling to keep it visible); Left collapses the selected
// node, or moves to its parent if it's already collapsed (or a leaf);
// Right expands it, or moves to its first child if it's already expanded;
// Enter and a mouse click on a row fire OnSelect/OnDoubleClick; a click on
// a row's own glyph toggles expand/collapse instead of selecting.
func (t *TreeView) HandleEvent(ev Event) {
	if ev.Type == EventKey {
		idx := t.selectedIndex()
		switch ev.Key {
		case KeyUp:
			if idx > 0 {
				t.Selected = t.visible[idx-1]
				if idx-1 < t.Scroll {
					t.Scroll = idx - 1
				}
			}
		case KeyDown:
			if idx >= 0 && idx < len(t.visible)-1 {
				t.Selected = t.visible[idx+1]
				if idx+1 >= t.Scroll+t.LastH {
					t.Scroll = idx + 1 - t.LastH + 1
				}
			}
		case KeyHome:
			if len(t.visible) > 0 {
				t.Selected = t.visible[0]
				t.Scroll = 0
			}
		case KeyEnd:
			if len(t.visible) > 0 {
				t.Selected = t.visible[len(t.visible)-1]
				if len(t.visible)-1 >= t.LastH {
					t.Scroll = len(t.visible) - t.LastH
				}
			}
		case KeyPageUp:
			if idx >= 0 {
				ni := idx - t.LastH
				if ni < 0 {
					ni = 0
				}
				t.Selected = t.visible[ni]
				if ni < t.Scroll {
					t.Scroll = ni
				}
			}
		case KeyPageDown:
			if idx >= 0 && len(t.visible) > 0 {
				ni := idx + t.LastH
				if ni >= len(t.visible) {
					ni = len(t.visible) - 1
				}
				t.Selected = t.visible[ni]
				if ni >= t.Scroll+t.LastH {
					t.Scroll = ni - t.LastH + 1
				}
			}
		case KeyRight:
			if t.Selected != nil {
				if t.Selected.HasChildren && !t.Selected.Expanded {
					t.Expand(t.Selected)
				} else if t.Selected.Expanded && len(t.Selected.Children) > 0 {
					t.Selected = t.Selected.Children[0]
					t.ensureVisible()
				}
			}
		case KeyLeft:
			if t.Selected != nil {
				if t.Selected.Expanded {
					t.Collapse(t.Selected)
				} else if t.Selected.parent != nil {
					t.Selected = t.Selected.parent
					t.ensureVisible()
				}
			}
		case KeyEnter:
			// Re-validated via selectedIndex rather than trusting Selected
			// is still live — the same defensive check ListBox's Enter
			// handler applies to its own Selected (see
			// TestListBox_StaleSelectedAfterShrinkDoesNotPanic).
			if t.Selected != nil && t.selectedIndex() >= 0 {
				if t.OnDoubleClick != nil {
					t.OnDoubleClick(t.Selected)
				} else if t.OnSelect != nil {
					t.OnSelect(t.Selected)
				}
			}
		}
	} else if ev.Type == EventMouseDown || ev.Type == EventMouseDrag {
		if ev.MouseX == t.AbsX+t.LastW-1 && len(t.visible) > t.LastH {
			maxScroll := len(t.visible) - t.LastH
			if maxScroll < 0 {
				maxScroll = 0
			}
			sH := int(math.Max(1, float64((t.LastH*t.LastH)/len(t.visible))))
			relY := ev.MouseY - t.AbsY
			if relY < sH/2 {
				t.Scroll = 0
			} else if relY >= t.LastH-sH/2 {
				t.Scroll = maxScroll
			} else {
				fraction := float64(relY-sH/2) / float64(t.LastH-sH)
				t.Scroll = int(math.Round(fraction * float64(maxScroll)))
			}
			return
		}

		if ev.Type == EventMouseDown {
			clickedRow := ev.MouseY - t.AbsY
			if clickedRow >= 0 && clickedRow < t.LastH {
				idx := t.Scroll + clickedRow
				if idx >= 0 && idx < len(t.visible) {
					node := t.visible[idx]
					t.Selected = node

					glyphStart := t.AbsX + node.depth*treeIndentWidth
					if node.HasChildren && ev.MouseX >= glyphStart && ev.MouseX < glyphStart+treeIndentWidth {
						t.Toggle(node)
						return
					}

					if t.OnSelect != nil {
						t.OnSelect(node)
					}

					now := time.Now()
					if t.OnDoubleClick != nil && t.lastClickIdx == idx && now.Sub(t.lastClickTime) < 500*time.Millisecond {
						t.OnDoubleClick(node)
					}
					t.lastClickIdx = idx
					t.lastClickTime = now
				}
			}
		}
	} else if ev.Type == EventMouseScrollUp {
		if t.Scroll > 0 {
			t.Scroll--
		}
	} else if ev.Type == EventMouseScrollDown {
		maxScroll := len(t.visible) - t.LastH
		if maxScroll < 0 {
			maxScroll = 0
		}
		if t.Scroll < maxScroll {
			t.Scroll++
		}
	}
}
