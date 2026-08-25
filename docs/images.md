# Images: the GPH format and the `Image` widget

Graphite has no way to display a real bitmap image at full fidelity — a
terminal cell is a character cell, not a pixel. What it has instead is
**GPH**, a small binary format for *console pseudographics images*: each
"pixel" is a foreground/background color pair plus a 5-level density
(picking among `" "`, `░`, `▒`, `▓`, `█`), optionally animated. `gphedit`
(an example program in this repository) converts real images/video into
this format; `Image` displays a `*GphImage` as a widget.

## `GphPixel` and `GphImage`

```go
type GphPixel struct {
	Bg    Color
	Fg    Color
	Level uint8 // 0=blank, 1=25%, 2=50%, 3=75%, 4=100% density
}

type GphImage struct {
	Width   int
	Height  int
	Mode    PlaybackMode
	DelayMs uint16
	Frames  [][]GphPixel // each frame is Width*Height pixels, row-major
}
```

`Bg == ColorNone` means "transparent at this pixel" — `Image`'s draw code
blends it with whatever is already on the canvas underneath
(`Canvas.GetCellBg`) instead of painting a solid background, which is how
a non-rectangular sprite over an arbitrary background is achieved.

```go
type PlaybackMode uint8

const (
	PlaybackStatic    PlaybackMode = 0 // a single frame, never animates
	PlaybackLoop      PlaybackMode = 1 // repeats from the start
	PlaybackBoomerang PlaybackMode = 2 // plays forward then backward, repeating
	PlaybackOnce      PlaybackMode = 3 // plays through once and stops on the last frame
)
```

`DelayMs` is the per-frame delay; if it's below 10, the library falls
back to a 100ms (10fps) default rather than treating a near-zero value as
a request to animate as fast as possible.

## Reading and writing `.gph` files

```go
func WriteGph(w io.Writer, img *GphImage) error
func ReadGph(r io.Reader) (*GphImage, error)
func LoadGphFile(path string) (*GphImage, error) // convenience: os.Open + ReadGph
```

The on-disk format is a 4-byte magic (`GphMagic`, `"GPH\x01"`) followed by
a 15-byte header (width, height, mode, delay, frame count) and then each
frame's pixel data. Frames after the first are **delta-encoded**: a
per-frame bitmask marks which pixels changed since the previous frame,
and only those changed pixels are written, which is why a mostly-static
animation with a small moving region compresses far better than storing
every frame in full. `WriteGph` computes these deltas automatically —
callers just provide the full pixel data for every frame; you never
construct a delta yourself.

`ReadGph` also transparently accepts an older, pre-delta legacy V1
format that stored only a single static frame with an 8-byte header
(width+height only) — if the extended 7-byte header can't be fully read,
it falls back to treating the rest of the stream as one uncompressed
frame's raw pixel data (`PlaybackStatic`, `DelayMs: 0`). You don't need
to handle this distinction yourself; `ReadGph` picks the right path based
on how much of the header is actually present in the stream.

```go
img, err := Graphite.LoadGphFile("logo.gph")
if err != nil {
	// ...
}
```

## Building a `GphImage` programmatically

You don't need a `.gph` file on disk — constructing one directly in code
is how you'd generate a procedural animation, a rendered chart, or (as
`showcase`'s Image tab does) a synthetic demo image:

```go
width, height := 50, 16
img := &Graphite.GphImage{
	Width:   width,
	Height:  height,
	Mode:    Graphite.PlaybackBoomerang,
	DelayMs: 80,
	Frames:  make([][]Graphite.GphPixel, 15),
}

for f := 0; f < 15; f++ {
	frame := make([]Graphite.GphPixel, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			frame[y*width+x] = Graphite.GphPixel{
				Bg:    theme.BgWidget,
				Fg:    theme.Primary,
				Level: uint8((x + y + f) % 5),
			}
		}
	}
	img.Frames[f] = frame
}
```

Each frame must have exactly `Width * Height` pixels, row-major
(`frame[y*Width+x]`) — this is asserted (returns an error) by `WriteGph`
if you ever serialize it, though nothing stops you from handing a
malformed slice straight to the `Image` widget, so get this right when
building frames by hand.

## The `Image` widget

```go
imgWidget := Graphite.NewImage(0, 5, img)
imgWidget.Play(app)
```

```go
type Image struct {
	BaseWidget
	Img           *GphImage
	AutoSize      bool
	OnFrameUpdate func()
}

func NewImage(x, y int, img *GphImage) *Image
func (img *Image) Play(app *Application)
func (img *Image) Stop()
func (img *Image) NextFrame()
func (img *Image) PrevFrame()
```

- **`AutoSize`** (default `true`): the widget's `Width`/`Height` are kept
  in sync with `Img.Width`/`Img.Height` every frame, so it always renders
  at the image's native resolution regardless of the space its parent
  offers. Set it to `false` to instead scale the image to fill whatever
  size the widget resolves to (aspect-ratio-preserved, letterboxed if the
  offered box's proportions don't match the image's) — useful inside a
  `Flex`/`Panel` slot you want the image to fit rather than overflow.
- **`Play(app)`** starts a background goroutine driving the animation on
  a `time.Ticker` at `Img.DelayMs`, and is a no-op if `Img` is `nil`, has
  one or zero frames, or `Img.Mode == PlaybackStatic` — there's nothing to
  animate in those cases. Calling `Play` again while already playing
  stops the previous goroutine first (`close(img.quitChan)`), so it's
  safe to call `Play` again after swapping in a new `Img` without leaking
  the old animation goroutine.
- Frame advancement respects `Mode`: `PlaybackLoop` wraps around,
  `PlaybackOnce` stops advancing once it reaches the last frame,
  `PlaybackBoomerang` reverses direction at each end instead of wrapping.
- **This is the one widget in the library whose animation runs off the
  main render loop, in its own goroutine** — every other "animated"
  widget (`Spinner`, `TodoList`'s running-state glyph) derives its frame
  purely from wall-clock time and gets redrawn for free by the normal
  render loop. `Image.Play`'s goroutine calls `app.Invoke(func(){})` on
  each tick purely to wake the main loop for a redraw (see
  [architecture.md](architecture.md#concurrency-and-invoke) on why
  `Invoke` is the only safe way to do this) — it does not touch widget
  state directly from the goroutine; `advanceFrame()` (which does mutate
  `currentFrame`) still runs after being invoked back onto the main
  goroutine. A hidden `Image` (`SetVisible(false)`, e.g. an inactive
  `TabView` tab) pauses advancing entirely rather than animating unseen
  and catching up all at once when it becomes visible again.
- **`Stop()`** halts playback; **`NextFrame()`/`PrevFrame()`** step
  manually (wrapping at either end), useful for a frame-by-frame preview
  scrubber like `gphedit`'s.
- **`OnFrameUpdate`**, if set, fires every time the current frame changes
  (from `Play`'s ticker or a manual `NextFrame`/`PrevFrame` call) — use it
  to drive something outside the widget itself that needs to track the
  current frame index (a frame counter label, a scrubber position).

## `gphedit`: converting real images to GPH

The `gphedit` example program (`go run ./gphedit`) loads real JPEG/PNG
images (and, via `ffmpeg`, video) and converts them to `.gph`, quantizing
each source pixel down to a background/foreground color pair and a
5-level density approximation. It's the reference implementation for
"how do I get a real photo or video into this format" — read its source
if you need to build a similar conversion pipeline rather than
hand-authoring `GphPixel` frames.
