// Command showcase exercises every widget Graphite ships, its Flex layout
// container, and a custom theme, in a single tabbed window. Run it with
// `go run ./showcase` (or a build produced by the Makefile/build.ps1) to see
// them rendered in a real terminal.
package main

import (
	"fmt"
	"math"
	"time"

	"github.com/yeoblyv/graphite"
)

// swatch is a small custom widget — built the same way any external
// consumer would build one, by embedding Graphite.BaseWidget — that paints
// a solid color block with a centered, automatically contrasting label.
// It is used both to visualize Flex proportions and to display the active
// theme's palette.
type swatch struct {
	Graphite.BaseWidget
	Label string
	Fill  Graphite.Color
}

func newSwatch(label string, fill Graphite.Color) *swatch {
	return &swatch{BaseWidget: Graphite.NewBaseWidget(0, 0, 0, 0), Label: label, Fill: fill}
}

// contrastText picks black or white text, whichever reads better against
// bg, using perceived luminance (ITU-R BT.601).
func contrastText(bg Graphite.Color) Graphite.Color {
	r, g, b := bg.Components()
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if luma > 140 {
		return Graphite.RGB(0, 0, 0)
	}
	return Graphite.RGB(255, 255, 255)
}

// DrawRelative implements Graphite.Widget.
func (s *swatch) DrawRelative(c *Graphite.Canvas, offX, offY, pW, pH int) {
	s.BaseWidget.DrawRelative(c, offX, offY, pW, pH)
	fg := contrastText(s.Fill)
	for y := 0; y < s.LastH; y++ {
		for x := 0; x < s.LastW; x++ {
			c.DrawCell(s.AbsX+x, s.AbsY+y, " ", s.Fill, fg)
		}
	}
	labelY := s.AbsY + s.LastH/2
	if labelY < s.AbsY+s.LastH {
		c.DrawText(s.AbsX+1, labelY, s.Label, s.Fill, fg)
	}
}

// nordTheme is a custom RGB palette distinct from Graphite.DefaultTheme(),
// proving theme customization still works — not just that the default
// looks good.
func nordTheme() Graphite.Theme {
	return Graphite.Theme{
		BgScreen:   Graphite.RGB(46, 52, 64),
		BgWindow:   Graphite.RGB(59, 66, 82),
		FgWindow:   Graphite.RGB(216, 222, 233),
		BgWidget:   Graphite.RGB(67, 76, 94),
		BgFocused:  Graphite.RGB(136, 192, 208),
		FgFocused:  Graphite.RGB(46, 52, 64),
		Primary:    Graphite.RGB(136, 192, 208),
		Success:    Graphite.RGB(163, 190, 140),
		Danger:     Graphite.RGB(191, 97, 106),
		Warning:    Graphite.RGB(235, 203, 139),
		Disabled:   Graphite.RGB(76, 86, 106),
		FgDisabled: Graphite.RGB(143, 153, 168),
	}
}

// buildWidgetsTab exercises every widget type in two columns: static/input
// widgets on the left, data-display widgets on the right.
func buildWidgetsTab(pb *Graphite.ProgressBar) []Graphite.Widget {
	colLeft := Graphite.NewPanel(0, 2, 0, 0)
	colLeft.SetPercentLayout(0, 0, 48, 90)
	colLeft.AddWidget(Graphite.NewLabel(0, 0, "Label widget"))
	colLeft.AddWidget(Graphite.NewSpinner(0, 2, "Spinner widget"))
	colLeft.AddWidget(Graphite.NewCheckbox(0, 4, "Checkbox (unchecked)", false))
	colLeft.AddWidget(Graphite.NewCheckbox(0, 5, "Checkbox (checked)", true))
	colLeft.AddWidget(Graphite.NewLabel(0, 7, "Button styles:"))
	colLeft.AddWidget(Graphite.NewButton(0, 8, "Default", Graphite.BtnDefault, nil))
	colLeft.AddWidget(Graphite.NewButton(14, 8, "Success", Graphite.BtnSuccess, nil))
	colLeft.AddWidget(Graphite.NewButton(0, 10, "Danger", Graphite.BtnDanger, nil))
	colLeft.AddWidget(Graphite.NewButton(14, 10, "Warning", Graphite.BtnWarning, nil))
	colLeft.AddWidget(Graphite.NewButton(0, 12, "Info", Graphite.BtnInfo, nil))
	colLeft.AddWidget(Graphite.NewInputBox(0, 14, 30, "Input: "))

	colRight := Graphite.NewPanel(0, 2, 0, 0)
	colRight.SetPercentLayout(52, 0, 48, 90)
	colRight.AddWidget(pb)
	colRight.AddWidget(Graphite.NewLabel(0, 2, "ListBox:"))
	colRight.AddWidget(Graphite.NewListBox(0, 3, 0, 5, []string{
		"First item", "Second item", "Third item", "Fourth item", "Fifth item", "Sixth item",
	}, nil))
	colRight.AddWidget(Graphite.NewLabel(0, 9, "TodoList (Space/Enter to toggle):"))
	colRight.AddWidget(Graphite.NewTodoList(0, 10, 0, 4, []string{
		"Write the showcase", "Wire up Flex layout", "Ship truecolor theme",
	}, false))
	colRight.AddWidget(Graphite.NewLabel(0, 15, "TextArea:"))
	ta := Graphite.NewTextArea(0, 16, 0, 4)
	ta.SetText("This is a multi-line TextArea.\nIt wraps long lines automatically and\nsupports full cursor navigation.")
	colRight.AddWidget(ta)

	return []Graphite.Widget{colLeft, colRight}
}

// buildLayoutTab demonstrates Flex: a row split 1:2:1 and a column split
// 2:1, both driven entirely by weights instead of hand-computed percentages.
func buildLayoutTab(theme Graphite.Theme) []Graphite.Widget {
	panel := Graphite.NewPanel(0, 2, 0, 0)
	panel.SetPercentLayout(0, 0, 100, 90)

	panel.AddWidget(Graphite.NewLabel(0, 0, "Flex row, weights 1 : 2 : 1 (resize the terminal to see it adapt):"))
	row := Graphite.NewFlex(0, 2, 0, 5, Graphite.FlexRow)
	row.Gap = 1
	row.AddChild(newSwatch("weight 1", theme.Primary), 1)
	row.AddChild(newSwatch("weight 2", theme.Success), 2)
	row.AddChild(newSwatch("weight 1", theme.Danger), 1)
	panel.AddWidget(row)

	panel.AddWidget(Graphite.NewLabel(0, 8, "Flex column, weights 2 : 1, fixed 40x10 box:"))
	col := Graphite.NewFlex(0, 10, 40, 10, Graphite.FlexColumn)
	col.Gap = 1
	col.AddChild(newSwatch("weight 2", theme.Warning), 2)
	col.AddChild(newSwatch("weight 1", theme.BgFocused), 1)
	panel.AddWidget(col)

	return []Graphite.Widget{panel}
}

// buildThemeTab lays every Theme field out as a labeled swatch, arranged in
// a 3-column grid built from a Flex column of Flex rows — a real use of
// nested Flex, not just a synthetic demo of it.
func buildThemeTab(theme Graphite.Theme) []Graphite.Widget {
	panel := Graphite.NewPanel(0, 2, 0, 0)
	panel.SetPercentLayout(0, 0, 100, 90)
	panel.AddWidget(Graphite.NewLabel(0, 0, "Every Theme field, rendered with its actual color:"))

	type namedColor struct {
		Name  string
		Color Graphite.Color
	}
	fields := []namedColor{
		{"BgScreen", theme.BgScreen}, {"BgWindow", theme.BgWindow}, {"FgWindow", theme.FgWindow},
		{"BgWidget", theme.BgWidget}, {"BgFocused", theme.BgFocused}, {"FgFocused", theme.FgFocused},
		{"Primary", theme.Primary}, {"Success", theme.Success}, {"Danger", theme.Danger},
		{"Warning", theme.Warning}, {"Disabled", theme.Disabled}, {"FgDisabled", theme.FgDisabled},
	}

	grid := Graphite.NewFlex(0, 2, 0, 20, Graphite.FlexColumn)
	grid.Gap = 1
	for i := 0; i < len(fields); i += 3 {
		row := Graphite.NewFlex(0, 0, 0, 0, Graphite.FlexRow)
		row.Gap = 1
		for _, f := range fields[i:min(i+3, len(fields))] {
			row.AddChild(newSwatch(f.Name, f.Color), 1)
		}
		grid.AddChild(row, 1)
	}
	panel.AddWidget(grid)

	return []Graphite.Widget{panel}
}

// openNestedModal opens a modal that can itself open another, up to three
// levels deep, exercising the modal stack: closing one reveals the
// previous, not the base window.
func openNestedModal(app *Graphite.Application, depth int) {
	mod := Graphite.NewWindow(54, 10, fmt.Sprintf(" Modal level %d ", depth))
	mod.AddWidget(Graphite.NewLabel(2, 2, fmt.Sprintf(
		"This is modal #%d. Closing it reveals the one beneath it.", depth)))
	if depth < 3 {
		mod.AddWidget(Graphite.NewButton(2, 5, "Open Another On Top", Graphite.BtnDefault, func() {
			openNestedModal(app, depth+1)
		}))
	}
	mod.AddWidget(Graphite.NewButton(2, 7, "Close This One", Graphite.BtnDanger, func() {
		app.CloseModal()
	}))
	app.SetModal(mod)
}

// buildModalsTab demonstrates ShowMessage and the nested modal stack.
func buildModalsTab(app *Graphite.Application) []Graphite.Widget {
	panel := Graphite.NewPanel(0, 2, 0, 0)
	panel.SetPercentLayout(0, 0, 100, 90)
	panel.AddWidget(Graphite.NewLabel(0, 0, "Modal windows stack — open several and close them one at a time."))
	panel.AddWidget(Graphite.NewButton(0, 2, "Show A Message", Graphite.BtnDefault, func() {
		app.ShowMessage(" Notice ", "This is a simple single modal dialog.", Graphite.BtnDefault)
	}))
	panel.AddWidget(Graphite.NewButton(0, 4, "Open Nested Modals", Graphite.BtnSuccess, func() {
		openNestedModal(app, 1)
	}))
	return []Graphite.Widget{panel}
}

// buildMixerTab demonstrates Fader: three channel strips with different
// optional features enabled, arranged with Flex so they share the row
// evenly. It returns the tab's widgets plus the two Faders whose VU meter
// the caller should animate (there's no real audio here, so the level is
// simulated) to make the point that Level is independent of Value — the
// meter moves on its own while the fader stays wherever it was left.
func buildMixerTab(app *Graphite.Application) ([]Graphite.Widget, *Graphite.Fader, *Graphite.Fader) {
	panel := Graphite.NewPanel(0, 2, 0, 0)
	panel.SetPercentLayout(0, 0, 100, 90)
	panel.AddWidget(Graphite.NewLabel(0, 0,
		"Drag the handle, click the track to jump, double-click to type an exact value:"))

	row := Graphite.NewFlex(0, 2, 0, 20, Graphite.FlexRow)
	row.Gap = 2

	mic := Graphite.NewFader(0, 0, 16, 20, "MIC 1", Graphite.RGB(235, 203, 139))
	mic.OnDoubleClick = func() {
		Graphite.ShowFaderValueEditor(app, "MIC 1 Value", mic.Value, func(v float64) {
			mic.Value = v
		})
	}

	desktop := Graphite.NewFader(0, 0, 16, 20, "DESKTOP", Graphite.RGB(136, 192, 208))
	desktop.ShowSolo = false // an optional feature turned off, for contrast.
	desktop.OnDoubleClick = func() {
		Graphite.ShowFaderValueEditor(app, "DESKTOP Value", desktop.Value, func(v float64) {
			desktop.Value = v
		})
	}

	aux := Graphite.NewFader(0, 0, 12, 20, "AUX", Graphite.RGB(191, 97, 106))
	aux.ShowMeter, aux.ShowClip, aux.ShowMute, aux.ShowSolo = false, false, false, false
	aux.OnDoubleClick = func() {
		Graphite.ShowFaderValueEditor(app, "AUX Value", aux.Value, func(v float64) {
			aux.Value = v
		})
	}

	row.AddChild(mic, 1)
	row.AddChild(desktop, 1)
	row.AddChild(aux, 1)
	panel.AddWidget(row)

	return []Graphite.Widget{panel}, mic, desktop
}

func main() {
	theme := nordTheme()
	app := Graphite.NewApplication()
	app.SetTheme(theme)
	app.SetStatus(" Tab: Switch Tabs | Arrows/Mouse: Navigate | Enter/Space: Activate | Esc: Exit ")

	win := Graphite.NewWindow(110, 34, " Graphite Showcase ")
	win.SetPercentSize(92, 92)

	tabs := Graphite.NewTabView(0, 0, 0, Graphite.TabDefault)
	tabs.SetPercentLayout(0, 0, 100, 0)
	win.AddWidget(tabs)

	pb := Graphite.NewProgressBar(0, 0, 30, "Progress:")
	tWidgets := buildWidgetsTab(pb)
	tLayout := buildLayoutTab(theme)
	tTheme := buildThemeTab(theme)
	tModals := buildModalsTab(app)
	tMixer, micFader, desktopFader := buildMixerTab(app)

	startTime := time.Now()
	app.SetIdleCallback(func() {
		elapsed := time.Since(startTime).Seconds()
		pb.SetProgress(float32(math.Mod(elapsed*10, 100)))

		// No real audio input exists here, so fake a plausible-looking
		// signal: this is what makes SetLevel visibly independent of
		// Value — the meter moves on its own while the fader stays put.
		micFader.SetLevel(55 + 40*math.Sin(elapsed*2))
		desktopFader.SetLevel(35 + 25*math.Sin(elapsed*3.3+1))
	})

	tabs.AddTab("Widgets", tWidgets)
	for _, w := range tWidgets {
		win.AddWidget(w)
	}
	tabs.AddTab("Layout", tLayout)
	for _, w := range tLayout {
		win.AddWidget(w)
	}
	tabs.AddTab("Theme", tTheme)
	for _, w := range tTheme {
		win.AddWidget(w)
	}
	tabs.AddTab("Modals", tModals)
	for _, w := range tModals {
		win.AddWidget(w)
	}
	tabs.AddTab("Mixer", tMixer)
	for _, w := range tMixer {
		win.AddWidget(w)
	}

	app.SetWindow(win)
	app.Run()
}
