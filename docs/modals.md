# Modals

A modal in Graphite is just a `Window` pushed onto `Application`'s modal
stack instead of set as the active window. This document covers the
modal stack itself and the three modal dialogs the library ships:
`ShowMessage`, `ShowValueEditor`, and `ShowFilePicker`.

## The modal stack

```go
func (app *Application) SetModal(mod *Window)
func (app *Application) CloseModal()
```

`SetModal` pushes `mod` on top of the stack; `CloseModal` pops the
topmost one, revealing whatever was beneath it (another modal, or the
plain active window if the stack is now empty). Modals stack — opening a
second modal doesn't replace the first, it layers on top, and closing it
reveals the first again. `showcase`'s Modals tab demonstrates this
explicitly with "Open Nested Modals," which can open up to three levels
deep, each with its own "Close This One" button.

Every open modal is drawn on top of the active window and any modal below
it, in stack order, every frame (`Application.Run` draws `activeWindow`,
then walks `modalStack` in order). Input routing always goes to the
topmost modal if the stack is non-empty; only `Esc` is special-cased —
it closes the topmost modal without reaching it as an event at all,
rather than the modal needing to handle `Esc` itself.

`SetModal`/`CloseModal` both call `ClearMouseCapture()` on the window(s)
involved, specifically so a mouse-down that triggered opening a modal
doesn't leave a stale capture that then misroutes the modal's first click
to whatever was underneath it (see
[architecture.md](architecture.md#mouse-routing-and-implicit-capture)).

### Building your own modal

Any `*Window` can be a modal — there's nothing special about the three
built-in ones beyond being convenience constructors. The pattern:

```go
mod := Graphite.NewWindow(50, 10, " Confirm ")
mod.AddWidget(Graphite.NewLabel(2, 2, "Are you sure?"))
mod.AddWidget(Graphite.NewButton(2, 5, "Yes", Graphite.BtnDanger, func() {
	// do the thing
	app.CloseModal()
}))
mod.AddWidget(Graphite.NewButton(10, 5, "Cancel", Graphite.BtnDefault, func() {
	app.CloseModal()
}))
app.SetModal(mod)
```

Remember to close the modal (`app.CloseModal()`) from whatever button/flow
represents "done" — nothing does this for you automatically except `Esc`.

## `ShowMessage`: a single-button dialog

```go
func (app *Application) ShowMessage(title, message string, style ButtonStyle)
```

```go
app.ShowMessage(" Notice ", "This is a simple single modal dialog.", Graphite.BtnDefault)
app.ShowMessage(" Error ", fmt.Sprintf("Could not open %s:\n%v", path, err), Graphite.BtnDanger)
```

The height is computed automatically from how many lines `message` wraps
to at a fixed 42-column width (with an 8-row floor), so a short message
doesn't leave a needlessly tall dialog and a long one doesn't get
truncated. `message` supports embedded `\n` for hard line breaks in
addition to the automatic wrapping. The dialog has exactly one button,
labeled "OK", styled per `style`, which closes the modal — pass
`Graphite.BtnDanger` for error messages, `Graphite.BtnDefault` for
neutral notices, etc.

## `ShowValueEditor`: type an exact number

```go
func ShowValueEditor(app *Application, title string, current, min, max float64, onConfirm func(float64))
```

```go
Graphite.ShowValueEditor(app, "MASTER Volume", masterFader.Value, 0, 100, func(v float64) {
	masterFader.Value = v
	setSystemVolume(v)
})
```

Opens a modal with an `InputBox` pre-filled with `current`. Both the "OK"
button and pressing `Enter` inside the field (via `InputBox.OnSubmit`)
validate and confirm; an invalid entry (not a number, or outside
`[min, max]`) shows an inline error label and leaves the modal open
instead of closing it, so the user can correct their input without
restarting. "Cancel" closes without calling `onConfirm`. This is
general-purpose, not `Fader`-specific — see
[fader.md](fader.md#showvalueeditor-typing-an-exact-value) for the
canonical way to wire it to a double-click.

## `ShowFilePicker`: browse and select a file

```go
func ShowFilePicker(app *Application, initialDir string, onSelect func(path string))
```

```go
Graphite.ShowFilePicker(app, ".", func(path string) {
	f, err := os.Open(path)
	// ...
})
```

A full filesystem browser modal: back/forward/up navigation with
history, a directly-editable path field (type a path and press `Enter` to
jump to it — works for both directories and, if you type a file path
directly, jumping to its containing directory with that file
pre-selected), a sortable directory listing (folders first, then files,
both alphabetical) with name/date/type/size columns, a file-type filter
dropdown (`All Files`, `GPH Files`, `Image Files`, `Video Files`), and a
file-name field.

- Double-clicking a folder navigates into it; double-clicking a file (or
  clicking "Open" with a name in the file-name field) closes the modal
  and calls `onSelect(path)` with the absolute path.
- "Cancel" closes the modal without calling `onSelect`.
- The filter dropdown re-lists the current directory when changed — it
  filters by extension against fixed lists (`.gph`; `.jpg`/`.jpeg`/`.png`;
  `.mp4`/`.avi`/`.mkv`/`.webm`), not a caller-supplied pattern, so if you
  need a different filter set you'll need to build a custom picker
  (`ShowFilePicker`'s own source in `filepicker.go` is a reasonable
  starting point to copy and adapt, since there's no configuration hook
  for the filter list).

`gphedit` uses this to open both source images/video and `.gph` files;
`showcase`'s Image tab uses it to load a `.gph` file to view.
