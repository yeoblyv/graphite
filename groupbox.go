package Graphite

// GroupBox is a Panel that draws a border and a label around its children.
// Children should be positioned at least at X=1, Y=1 (or Y=2 if they shouldn't overlap the top border).
type GroupBox struct {
	BaseWidget
	Label    string
	Children []Widget
}

// NewGroupBox creates an empty GroupBox at (x, y) with the given size and label.
func NewGroupBox(x, y, w, h int, label string) *GroupBox {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = false
	return &GroupBox{BaseWidget: base, Label: label, Children: make([]Widget, 0)}
}

// AddWidget appends a child widget to the groupbox.
func (gb *GroupBox) AddWidget(w Widget) {
	gb.Children = append(gb.Children, w)
}

// GetChildren implements Widget.
func (gb *GroupBox) GetChildren() []Widget {
	return gb.Children
}

// DrawRelative implements Widget.
func (gb *GroupBox) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	gb.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	bg := c.theme.BgWindow
	fg := c.theme.FgDisabled

	if gb.LastW > 2 && gb.LastH > 2 {
		// Draw corners
		c.DrawCell(gb.AbsX, gb.AbsY, "┌", bg, fg)
		c.DrawCell(gb.AbsX+gb.LastW-1, gb.AbsY, "┐", bg, fg)
		c.DrawCell(gb.AbsX, gb.AbsY+gb.LastH-1, "└", bg, fg)
		c.DrawCell(gb.AbsX+gb.LastW-1, gb.AbsY+gb.LastH-1, "┘", bg, fg)

		// Draw top and bottom borders
		for i := 1; i < gb.LastW-1; i++ {
			c.DrawCell(gb.AbsX+i, gb.AbsY, "─", bg, fg)
			c.DrawCell(gb.AbsX+i, gb.AbsY+gb.LastH-1, "─", bg, fg)
		}

		// Draw left and right borders
		for i := 1; i < gb.LastH-1; i++ {
			c.DrawCell(gb.AbsX, gb.AbsY+i, "│", bg, fg)
			c.DrawCell(gb.AbsX+gb.LastW-1, gb.AbsY+i, "│", bg, fg)
		}

		// Draw label
		if gb.Label != "" {
			lbl := " " + gb.Label + " "
			if len([]rune(lbl)) > gb.LastW-2 {
				lbl = string([]rune(lbl)[:gb.LastW-2])
			}
			c.DrawText(gb.AbsX+2, gb.AbsY, lbl, bg, c.theme.FgWindow)
		}
	}

	// Draw children
	for _, child := range gb.Children {
		if child.IsVisible() {
			// Offset by 2 horizontally and 2 vertically to provide padding from the label and borders.
			child.DrawRelative(c, gb.AbsX+2, gb.AbsY+2, gb.LastW-4, gb.LastH-3)
		}
	}
}

// DrawOverlay implements Widget.
func (gb *GroupBox) DrawOverlay(c *Canvas, offX, offY, pW, pH int) {
	for _, child := range gb.Children {
		if child.IsVisible() {
			child.DrawOverlay(c, gb.AbsX+2, gb.AbsY+2, gb.LastW-4, gb.LastH-3)
		}
	}
}
