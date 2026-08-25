//go:build !windows

package Graphite

// enableVirtualTerminal is a no-op outside Windows: every other terminal
// this package targets already interprets ANSI/VT100 escapes natively.
func enableVirtualTerminal() {}
