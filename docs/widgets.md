# Widgets reference

Every widget below embeds `BaseWidget` (see
[architecture.md](architecture.md#widget-and-basewidget)) and follows the
layout rules in [layout.md](layout.md). Two widgets get their own
document instead of a section here because there's enough to say about
each: [`Fader`](fader.md) (the channel-strip mixer control) and modal
dialogs / `ShowFilePicker` (see [modals.md](modals.md)). `Panel`, `Flex`,
and `GroupBox` are layout containers, covered in [layout.md](layout.md)
rather than repeated here.

## Table of contents

- [Label](#label)
- [Spinner](#spinner)
- [Checkbox](#checkbox)
- [Button](#button)
- [InputBox](#inputbox)
- [TextArea](#textarea)
- [ProgressBar](#progressbar)
- [ListBox](#listbox)
- [TodoList](#todolist)
- [ComboBox](#combobox)
- [TabView](#tabview)
- [Slider](#slider)
- [MenuStrip](#menustrip)

---

## Label

Static, non-focusable, word-wrapped text.

```go
lbl := Graphite.NewLabel(0, 0, "Some text, as long as you like.")
```

```go
type Label struct {
	BaseWidget
	Text string
}

func NewLabel(x, y int, text string) *Label
func (l *Label) SetText(text string)
```

- Sizes itself to fit `text` on construction (`Width = len([]rune(text))`);
  `SetText` recomputes that width.
- Draws via `Canvas.DrawTextWrapped`, so long text word-wraps within
  whatever width it's given (from a fixed `Width`, a percentage layout, or
  a `Flex` slot) rather than overflowing into whatever draws after it.
  `LastH` is updated to the number of lines actually drawn, so a `Label`
  correctly reports its own height to layout even when its text spans
  multiple lines.
- Never focusable; `Tab` skips it.

---

## Spinner

An animated braille-style busy indicator next to a label.

```go
spinner := Graphite.NewSpinner(0, 2, "Loading…")
```

```go
type Spinner struct {
	BaseWidget
	Label string
}

func NewSpinner(x, y int, label string) *Spinner
```

- The animation frame is picked from wall-clock time (`time.Now().UnixMilli()/150 % 4`),
  not a widget-local counter — every `Spinner` on screen stays in sync
  with every other one automatically, with no shared ticker to wire up.
- Never focusable — it's purely a display widget. Redraw it every frame
  (e.g. from the idle callback, or just let the normal render loop redraw
  it since its glyph is a pure function of time) to see it animate.

---

## Checkbox

A focusable boolean toggle.

```go
cb := Graphite.NewCheckbox(0, 4, "Enable feature", false)
cb.OnChange = func(checked bool) { /* ... */ }
```

```go
type Checkbox struct {
	BaseWidget
	Label    string
	Checked  bool
	OnChange func(checked bool)
}

func NewCheckbox(x, y int, label string, checked bool) *Checkbox
```

- Renders `[ ] Label` or `[■] Label`; focused state uses `BgFocused`/`FgFocused`.
- `Space`, `Enter`, or a mouse click all toggle `Checked` and fire
  `OnChange`.

---

## Button

A focusable, clickable action.

```go
btn := Graphite.NewButton(0, 8, "Save", Graphite.BtnSuccess, func() {
	// ...
})
```

```go
type ButtonStyle int

const (
	BtnDefault ButtonStyle = iota
	BtnSuccess
	BtnDanger
	BtnWarning
	BtnInfo
)

type Button struct {
	BaseWidget
	Text    string
	Style   ButtonStyle
	OnClick func()
	BgColor Color // ColorNone (default): theme-driven, see below
	FgColor Color // ColorNone (default): auto-contrast once BgColor is set, else theme.FgWindow
}

func NewButton(x, y int, text string, style ButtonStyle, onClick func()) *Button
```

- Renders `[ Text ]`. Sizes itself to `len(Text) + 4` on construction.
- `Style` only visibly affects a **focused** button's background
  (`BtnSuccess`→`theme.Success`, `BtnDanger`→`theme.Danger`, others→
  `theme.BgFocused`) — an *unfocused* `BtnDanger` button is dimmed
  (`theme.Danger.Darken(0.3)`) so a destructive action still reads as
  "dangerous" even before it's focused; every other unfocused style uses
  the plain `theme.BgWidget`, unless `BgColor` is set (see below).
- A disabled button (`SetEnabled(false)`) renders with `theme.BgWidget`/
  `theme.FgDisabled` and — enforced by `Window`, not by `Button` itself —
  never receives input regardless of what `HandleEvent` would do.
- `Enter` or a mouse click both fire `OnClick`.
- `BgColor`/`FgColor` override only the plain idle state (enabled,
  unfocused, non-`BtnDanger`) — focused, disabled, and unfocused-danger
  rendering are unchanged, so a button that must stay readable as
  "dangerous" or "focused" always does. Set `BgColor` to give a button
  its own accent even before it's focused — e.g. a toolbar button that
  would otherwise blend into a plain list background —
  `btn.BgColor = Graphite.Hex("#5DE4FF")`; leave `FgColor` at `ColorNone`
  to auto-pick a contrasting text color (`Color.ContrastText`), or set it
  explicitly for a specific one.

---

## InputBox

A focusable, single-line text field with horizontal scrolling and a
visible cursor.

```go
input := Graphite.NewInputBox(0, 14, 30, "Name: ")
input.OnSubmit = func(value string) { /* ... */ }
```

```go
type InputBox struct {
	BaseWidget
	Label     string
	Value     string
	CursorPos int
	OnSubmit  func(string)
}

func NewInputBox(x, y, w int, label string) *InputBox
```

- `Label` is drawn as static prefix text, not editable; the field itself
  (`[...]`) starts right after it.
- Supports: `Left`/`Right` to move the cursor, `Backspace`/`Delete` to
  remove a rune, any printable `CharCode` to insert at the cursor,
  `Ctrl+C`/`Ctrl+X`/`Ctrl+V` for clipboard (via `github.com/atotto/clipboard`
  — `Ctrl+V` strips `\n`/`\r` since this is single-line), `Enter` fires
  `OnSubmit(Value)`, and a mouse click moves the cursor to the clicked
  column (accounting for horizontal scroll).
- Scrolls horizontally just enough to keep the cursor visible when
  `Value` is wider than the field — you don't need to manage this
  yourself.
- The cursor is drawn as a white-on-black block over the character it's
  on, independent of the active theme, so it stays visible against any
  palette.

`ShowValueEditor` and `ShowFilePicker` (see [modals.md](modals.md)) both
use `InputBox.OnSubmit` to let `Enter` confirm a modal dialog, not just
its OK button.

---

## TextArea

A focusable, scrollable, multi-line text editor with word wrap.

```go
ta := Graphite.NewTextArea(0, 16, 0, 4)
ta.SetText("This is a multi-line TextArea.\nIt wraps long lines automatically.")
```

```go
type TextArea struct {
	BaseWidget
	Text      string
	Scroll    int
	CursorPos int // absolute rune index into Text
}

func NewTextArea(x, y, w, h int) *TextArea
func (ta *TextArea) SetText(text string)
```

- Wraps at the widget's resolved width and breaks on `\n`, computing
  *visual* lines (`buildLines`) so cursor navigation and scrolling both
  work in terms of what's actually on screen, not raw text offsets.
- `Up`/`Down` move by visual line, preserving column where possible (not
  jumping to line start) — the same behavior as a real text editor.
- Same clipboard bindings as `InputBox` (`Ctrl+C`/`Ctrl+X`/`Ctrl+V`), but
  `Enter` inserts a newline instead of submitting (there's no `OnSubmit`
  — `TextArea` has no notion of "done").
- A vertical scrollbar is drawn automatically along the right edge when
  content exceeds the visible height, draggable and clickable to jump —
  same scrollbar behavior as `ListBox` and `TodoList` below.
- Mouse wheel (`EventMouseScrollUp`/`Down`) scrolls without moving the
  cursor; a click moves the cursor to the clicked line/column.

---

## ProgressBar

A labeled, filled completion bar.

```go
pb := Graphite.NewProgressBar(0, 0, 30, "Progress:")
pb.SetProgress(42.5) // 0-100
```

```go
type ProgressBar struct {
	BaseWidget
	Label    string
	Progress float32
}

func NewProgressBar(x, y, w int, label string) *ProgressBar
func (pb *ProgressBar) SetProgress(p float32) // clamped to [0, 100]
```

- Never focusable — purely a display widget. Drive `Progress` from your
  own logic (a real download/task, or the idle callback for a simulated
  one, as `showcase` does).
- Renders `Label [████░░░░] 42.5%`, filled portion using `theme.Primary`,
  empty using `theme.Disabled`.

---

## ListBox

A focusable, scrollable, single-selection list.

```go
lb := Graphite.NewListBox(0, 3, 0, 5, []string{
	"First item", "Second item", "Third item",
}, func(idx int, item string) {
	// fires on Enter or a click on a row
})
```

```go
type ListBox struct {
	BaseWidget
	Items         []string
	Selected      int
	Scroll        int
	OnSelect      func(int, string)
	OnDoubleClick func(int, string)
}

func NewListBox(x, y, w, h int, items []string, onSelect func(int, string)) *ListBox
```

- `Up`/`Down` move `Selected`, auto-scrolling to keep it visible.
- `Enter` fires `OnDoubleClick` if set, else `OnSelect`. A single mouse
  click on a row sets `Selected` **and** fires `OnSelect` immediately
  (there is no separate "highlight without selecting" state); a second
  click on the same row within 500ms additionally fires `OnDoubleClick`.
- `Items` is a plain exported slice — safe to reassign to a shorter list
  at runtime (e.g. re-filtering) without resetting `Selected` yourself,
  since every bound-check re-validates `Selected` against the current
  `len(Items)` rather than assuming it from a stale clamp.
- A vertical scrollbar appears automatically once `len(Items) > LastH`,
  draggable by dragging the thumb or clicking the track to jump.

---

## TodoList

A scrollable checklist, either interactive or driven programmatically.

```go
todo := Graphite.NewTodoList(0, 10, 0, 4, []string{
	"Write the docs", "Ship the release",
}, false) // false = interactive; true = ReadOnly

todo.SetItemState(0, Graphite.TodoDone)
```

```go
type TodoState int

const (
	TodoPending TodoState = iota
	TodoRunning
	TodoDone
)

type TodoItem struct {
	Text  string
	State TodoState
}

type TodoList struct {
	BaseWidget
	Items    []TodoItem
	Selected int
	Scroll   int
	ReadOnly bool
}

func NewTodoList(x, y, w, h int, items []string, readOnly bool) *TodoList
func (tl *TodoList) SetItemState(idx int, state TodoState)
```

- **Interactive mode** (`readOnly: false`): `Up`/`Down` move selection;
  `Space`, `Enter`, or a click on a row toggles that row between
  `TodoPending` and `TodoDone`.
- **Read-only mode** (`readOnly: true`): not focusable, ignores all
  input entirely. Meant to be driven entirely via `SetItemState` from
  your own code — e.g. reflecting the live progress of a background task,
  one item per step, moving each to `TodoRunning` then `TodoDone` as it
  completes. `SetItemState` silently ignores out-of-range `idx`, so a
  caller doesn't need to bounds-check on every call from a progress loop.
- `TodoRunning` renders the same animated braille glyph as `Spinner`, in
  `theme.Warning`; `TodoDone` renders `[■]` in `theme.Success`.
- Same scrollbar behavior as `ListBox`.

---

## ComboBox

A focusable dropdown selector.

```go
cb := Graphite.NewComboBox(0, 17, 20, []string{
	"Option 1", "Option 2", "Option 3",
}, func(idx int, item string) {
	// ...
})
```

```go
type ComboBox struct {
	BaseWidget
	Items    []string
	Selected int
	IsOpen   bool
	OnSelect func(idx int, item string)
}

func NewComboBox(x, y, w int, items []string, onSelect func(int, string)) *ComboBox
```

- Closed state draws the selected item's text and a `▼` indicator. A
  click opens it (`IsOpen = true`); a further click on an item selects it,
  fires `OnSelect`, and closes it again.
- The open dropdown is drawn via `DrawOverlay`, not `DrawRelative` — this
  is what makes it appear on top of whatever widgets are positioned below
  it in the same container, rather than being drawn over by them (see
  [architecture.md](architecture.md#widget-and-basewidget) on why
  `DrawOverlay` exists).
- `HitTest` is overridden to expand the clickable area to cover the open
  dropdown, not just the closed control's own bounds — otherwise a click
  on an open item wouldn't register as hitting this widget at all.
- Losing focus automatically closes the dropdown (checked at the top of
  `DrawRelative`), so tabbing away doesn't leave a stale dropdown open.
- The dropdown shows at most 5 items with no scrolling for longer lists
  currently — for a long, filterable list, `ListBox` is the better fit.

---

## TabView

Switches between named pages of widgets, showing exactly one at a time.

```go
tabs := Graphite.NewTabView(0, 0, 0, Graphite.TabDefault)
tabs.SetPercentLayout(0, 0, 100, 0)
win.AddWidget(tabs)

widgetsTab := []Graphite.Widget{ /* ... */ }
tabs.AddTab("Widgets", widgetsTab)
for _, w := range widgetsTab {
	win.AddWidget(w) // still added to the Window directly
}
```

```go
type TabStyle int

const (
	TabDefault  TabStyle = iota // focusable, navigable strip
	TabTimeline                 // read-only progress indicator, e.g. a wizard
)

type Tab struct {
	Name    string
	Widgets []Widget
}

type TabView struct {
	BaseWidget
	Tabs   []Tab
	Active int
	Style  TabStyle
}

func NewTabView(x, y, w int, style TabStyle) *TabView
func (tv *TabView) AddTab(name string, widgets []Widget)
func (tv *TabView) UpdateVisibility()
```

**Important: `TabView` does not parent its tabs' widgets.** It only
tracks which page is active and toggles `SetVisible` on every widget in
every tab accordingly (`UpdateVisibility`, called automatically by
`AddTab`, and which you must call yourself after changing `Active`
directly). Every widget belonging to a tab still needs to be added to the
`Window` (or containing `Panel`) as normal — `TabView` is purely a
selector plus a visibility switch, not a container. This is why the
pattern above adds `tabs` to `win` *and* adds every one of `widgetsTab`'s
widgets to `win` as well.

- `TabDefault`: focusable; `Left`/`Right` (while focused) or a mouse
  click on a tab label switches `Active`.
- `TabTimeline`: never takes focus or input — purely a visual progress
  indicator. Completed steps (`i < Active`) render in `theme.Success`;
  the active step is highlighted; steps are joined with `➔ ` separators
  instead of the tab-strip look.

---

## Slider

A horizontal draggable control for a value within an arbitrary
`[Min, Max]` range (not fixed to 0-100 like `Fader`'s `Value`).

```go
slider := Graphite.NewSlider(0, 20, 30, "Level: ", 0, 100)
slider.OnChange = func(v float64) { /* ... */ }
```

```go
type Slider struct {
	BaseWidget
	Label    string
	Min      float64
	Max      float64
	Value    float64
	OnChange func(value float64)

	OnDoubleClick func()
}

func NewSlider(x, y, w int, label string, min, max float64) *Slider
func (s *Slider) SetValue(v float64) // clamped to [Min, Max]
```

- Renders `Label [----█-----]  42`. Clicking or dragging the track jumps
  `Value` to that position; `Left`/`Down` and `Right`/`Up` nudge it by
  `(Max-Min)/20`.
- `OnChange` only fires when `Value` actually changes (same no-op-on-equal
  rule as `Fader.setValue` — see [fader.md](fader.md)).
- A second click within 500ms of the first fires `OnDoubleClick`, if set
  — there's no built-in numeric-entry modal wired to it the way `Fader`
  wires `ShowValueEditor` to a double-click; add that yourself if wanted.
- For a 0-100 gain control with a VU meter, clip indicator, and
  Mute/Solo — a mixer channel strip, not a generic range input — use
  [`Fader`](fader.md) instead.

---

## MenuStrip

A top-level horizontal menu bar with clickable categories that open
dropdown menus.

```go
menu := Graphite.NewMenuStrip([]Graphite.MenuCategory{
	{Label: "File", Items: []Graphite.MenuItem{
		{Label: "New", Action: func() { /* ... */ }},
		{Label: "Open…", Action: func() { /* ... */ }},
	}},
	{Label: "Help", Items: []Graphite.MenuItem{
		{Label: "About", Action: func() { /* ... */ }},
	}},
})
win.AddWidget(menu)
```

```go
type MenuItem struct {
	Label  string
	Action func()
}

type MenuCategory struct {
	Label string
	Items []MenuItem
}

type MenuStrip struct {
	BaseWidget
	Categories []MenuCategory
	OpenIdx    int // -1 when no category's dropdown is open
	BgColor    Color // ColorNone (default): theme-driven, see below
	FgColor    Color // ColorNone (default): auto-contrast once BgColor is set, else theme.FgWindow
}

func NewMenuStrip(categories []MenuCategory) *MenuStrip
```

- Always spans the full width of its parent (`SetPercentLayout(0, 0, 100, 0)`
  is set in the constructor) and sits one row tall — the conventional
  top-of-window application menu bar.
- Clicking a category's label toggles its dropdown open/closed; clicking
  an item in an open dropdown runs its `Action` and closes the dropdown.
- `HitTest` is overridden to capture the *entire* hit area while any
  dropdown is open (not just the strip's own row), so a click anywhere —
  including on the dropdown itself, which is drawn via `DrawOverlay` — is
  correctly routed here rather than falling through to whatever's
  underneath.
- There is currently no keyboard navigation for `MenuStrip` (no
  `Left`/`Right`/`Enter` support) — it's mouse-only. Add your own
  keyboard shortcuts as separate top-level key handling if you need
  keyboard-driven menus.
- By default (`BgColor`/`FgColor` both `ColorNone`) the strip reads
  `theme.BgWidget`/`FgWindow`, its open category highlights with
  `theme.Primary`/`BgWindow`, and its dropdown reads `theme.BgWindow`/
  `FgWindow` — unchanged from before these fields existed. Set `BgColor`
  to give the whole strip (bar and dropdown alike) one flat accent color
  instead — e.g. `menu.BgColor = Graphite.Hex("#FFD23D")` for a bright
  amber strip — and leave `FgColor` at `ColorNone` to have the text color
  picked automatically for contrast (via `Color.ContrastText`) rather than
  guessing black or white yourself. Set `FgColor` explicitly if you want a
  specific text color instead of the auto-computed one.
