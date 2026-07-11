package Graphite

import "golang.org/x/text/width"

// runeWidth returns the number of terminal columns r occupies: 2 for
// East Asian Wide/Fullwidth runes (most CJK ideographs and fullwidth forms),
// 1 for everything else. Box-drawing and block-element glyphs (used
// throughout this package for borders, shadows, scrollbars, and progress
// bars) classify as Ambiguous or Narrow, not Wide/Fullwidth, so they
// correctly stay width 1 — matching how real terminals render them.
func runeWidth(r rune) int {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}
