package Graphite

// FlexDirection selects the main axis a Flex container lays its children
// out along.
type FlexDirection int

// Supported flex directions.
const (
	// FlexRow arranges children left-to-right; each spans the container's
	// full height.
	FlexRow FlexDirection = iota
	// FlexColumn arranges children top-to-bottom; each spans the
	// container's full width.
	FlexColumn
)

// flexChild pairs a child widget with its share of the container's main
// axis.
type flexChild struct {
	Widget Widget
	Weight int
}

// Flex is a layout container that distributes space among its children
// along one axis, CSS-flexbox-style, instead of requiring the caller to
// compute fixed pixel or percentage offsets by hand. A child with Weight
// <= 0 gets its own natural size (GetFixedW for FlexRow, GetFixedH for
// FlexColumn); children with a positive weight split whatever space is
// left over, proportional to their weight relative to the other weighted
// children. The cross axis always stretches a child to the container's
// full size, matching Panel's existing behavior.
type Flex struct {
	BaseWidget
	Direction FlexDirection
	// Gap is the number of columns/rows of empty space inserted between
	// consecutive visible children.
	Gap      int
	children []flexChild
}

// NewFlex creates an empty Flex container at (x, y) with the given size and
// direction.
func NewFlex(x, y, w, h int, dir FlexDirection) *Flex {
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = false
	return &Flex{BaseWidget: base, Direction: dir}
}

// AddChild appends a child widget. weight <= 0 sizes it to its own natural
// size along the main axis; a positive weight instead claims that
// proportion of the space remaining after every fixed-size sibling.
//
// The weighted share is the box Flex offers the child via DrawRelative —
// whether the child actually fills it follows the same rule every widget
// already follows: a widget with a positive fixed Width/Height keeps that
// size regardless of the offered box (e.g. Label sizes itself to its text),
// while one left at Width/Height <= 0, or using SetPercentLayout, stretches
// to fill it. Use Panel (or a nested Flex) as the weighted child when the
// content itself should grow to fill the allotted space.
func (f *Flex) AddChild(w Widget, weight int) {
	f.children = append(f.children, flexChild{Widget: w, Weight: weight})
}

// GetChildren implements Widget, letting Window descend into the container
// when building the focus order and hit-testing the tree.
func (f *Flex) GetChildren() []Widget {
	widgets := make([]Widget, len(f.children))
	for i, ch := range f.children {
		widgets[i] = ch.Widget
	}
	return widgets
}

// mainAxisLength returns the container's resolved size along its main axis.
func (f *Flex) mainAxisLength() int {
	if f.Direction == FlexRow {
		return f.LastW
	}
	return f.LastH
}

// naturalSize returns w's own size along the container's main axis, used
// for children with no weight.
func (f *Flex) naturalSize(w Widget) int {
	if f.Direction == FlexRow {
		return w.GetFixedW()
	}
	return w.GetFixedH()
}

// DrawRelative implements Widget. Invisible children are skipped entirely —
// neither drawn nor given a share of space — so hiding a child closes the
// gap it would otherwise leave, matching CSS's `display: none`.
func (f *Flex) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	f.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	var visible []flexChild
	for _, ch := range f.children {
		if ch.Widget.IsVisible() {
			visible = append(visible, ch)
		}
	}
	n := len(visible)
	if n == 0 {
		return
	}

	available := f.mainAxisLength() - f.Gap*(n-1)
	if available < 0 {
		available = 0
	}

	totalWeight, fixedTotal := 0, 0
	for _, ch := range visible {
		if ch.Weight > 0 {
			totalWeight += ch.Weight
		} else {
			fixedTotal += f.naturalSize(ch.Widget)
		}
	}
	remaining := available - fixedTotal
	if remaining < 0 {
		remaining = 0
	}

	cursor := 0
	for _, ch := range visible {
		size := f.naturalSize(ch.Widget)
		if ch.Weight > 0 && totalWeight > 0 {
			size = remaining * ch.Weight / totalWeight
		}

		if f.Direction == FlexRow {
			ch.Widget.DrawRelative(c, f.AbsX+cursor, f.AbsY, size, f.LastH)
		} else {
			ch.Widget.DrawRelative(c, f.AbsX, f.AbsY+cursor, f.LastW, size)
		}
		cursor += size + f.Gap
	}
}

// DrawOverlay implements Widget.
func (f *Flex) DrawOverlay(c *Canvas, offX, offY, pW, pH int) {
	for _, ch := range f.children {
		if ch.Widget.IsVisible() {
			ch.Widget.DrawOverlay(c, f.AbsX, f.AbsY, f.LastW, f.LastH)
		}
	}
}
