package Graphite

import "time"

// Image is a widget that renders a GphImage.
type Image struct {
	BaseWidget
	Img           *GphImage
	AutoSize      bool
	currentFrame  int
	direction     int
	isPlaying     bool
	quitChan      chan struct{}
	OnFrameUpdate func()
}

// NewImage creates an Image widget at (x, y) with the exact dimensions
// of the provided GphImage. If the image is nil, it creates a 1x1 empty widget.
func NewImage(x, y int, img *GphImage) *Image {
	w, h := 1, 1
	if img != nil {
		w, h = img.Width, img.Height
	}
	base := NewBaseWidget(x, y, w, h)
	base.IsFocusable = false
	return &Image{BaseWidget: base, Img: img, AutoSize: true, currentFrame: 0, direction: 1}
}

// Play starts the animation loop if the image has multiple frames.
// It stops any currently running animation for this widget.
func (img *Image) Play(app *Application) {
	if img.isPlaying {
		close(img.quitChan)
	}
	if img.Img == nil || len(img.Img.Frames) <= 1 || img.Img.Mode == PlaybackStatic {
		return
	}

	img.isPlaying = true
	img.quitChan = make(chan struct{})

	delayMs := img.Img.DelayMs
	if delayMs < 10 {
		delayMs = 100 // fallback 10fps
	}

	go func() {
		ticker := time.NewTicker(time.Duration(delayMs) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !img.IsVisible() {
					continue // Pause animation and do not trigger redraw if tab is hidden
				}
				img.advanceFrame()
				app.Invoke(func() {
					// Just wake up the main loop; double-buffering handles partial redraw
				})
			case <-img.quitChan:
				img.isPlaying = false
				return
			}
		}
	}()
}

func (img *Image) advanceFrame() {
	if img.Img == nil || len(img.Img.Frames) == 0 {
		return
	}

	frameCount := len(img.Img.Frames)
	if img.Img.Mode == PlaybackOnce {
		if img.currentFrame < frameCount-1 {
			img.currentFrame++
			if img.OnFrameUpdate != nil {
				img.OnFrameUpdate()
			}
		}
		return
	}

	if img.Img.Mode == PlaybackLoop {
		img.currentFrame = (img.currentFrame + 1) % frameCount
		if img.OnFrameUpdate != nil {
			img.OnFrameUpdate()
		}
		return
	}

	if img.Img.Mode == PlaybackBoomerang {
		img.currentFrame += img.direction
		if img.currentFrame >= frameCount {
			img.currentFrame = frameCount - 2
			if img.currentFrame < 0 {
				img.currentFrame = 0
			}
			img.direction = -1
		} else if img.currentFrame < 0 {
			img.currentFrame = 1
			if img.currentFrame >= frameCount {
				img.currentFrame = 0
			}
			img.direction = 1
		}
		if img.OnFrameUpdate != nil {
			img.OnFrameUpdate()
		}
	}
}

// NextFrame advances the animation to the next frame manually.
func (img *Image) NextFrame() {
	if img.Img == nil || len(img.Img.Frames) == 0 {
		return
	}
	img.currentFrame = (img.currentFrame + 1) % len(img.Img.Frames)
	if img.OnFrameUpdate != nil {
		img.OnFrameUpdate()
	}
}

// PrevFrame advances the animation to the previous frame manually.
func (img *Image) PrevFrame() {
	if img.Img == nil || len(img.Img.Frames) == 0 {
		return
	}
	img.currentFrame--
	if img.currentFrame < 0 {
		img.currentFrame = len(img.Img.Frames) - 1
	}
	if img.OnFrameUpdate != nil {
		img.OnFrameUpdate()
	}
}

// Stop halts the animation playback.
func (img *Image) Stop() {
	if img.isPlaying {
		close(img.quitChan)
		img.isPlaying = false
	}
}

// DrawRelative implements Widget.
func (img *Image) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	if img.Img != nil && img.AutoSize {
		img.Width = img.Img.Width
		img.Height = img.Img.Height
	}

	img.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	if img.Img == nil || img.Img.Width == 0 || img.Img.Height == 0 || len(img.Img.Frames) == 0 {
		return
	}

	if img.currentFrame >= len(img.Img.Frames) {
		img.currentFrame = 0
	}

	frame := img.Img.Frames[img.currentFrame]

	drawPixel := func(screenX, screenY, srcX, srcY int) {
		pixel := frame[srcY*img.Img.Width+srcX]
		if pixel.Bg == ColorNone && pixel.Fg == ColorNone && pixel.Level == 0 {
			return
		}
		var char string
		switch pixel.Level {
		case 0:
			char = " "
		case 1:
			char = "░"
		case 2:
			char = "▒"
		case 3:
			char = "▓"
		default:
			char = "█"
		}
		bg := pixel.Bg
		if bg == ColorNone {
			bg = c.GetCellBg(screenX, screenY)
		}
		c.DrawCell(screenX, screenY, char, bg, pixel.Fg)
	}

	if img.AutoSize {
		for y := 0; y < img.Img.Height; y++ {
			screenY := img.AbsY + y
			if screenY < img.AbsY || screenY >= img.AbsY+img.LastH {
				continue
			}
			for x := 0; x < img.Img.Width; x++ {
				screenX := img.AbsX + x
				if screenX < img.AbsX || screenX >= img.AbsX+img.LastW {
					continue
				}
				drawPixel(screenX, screenY, x, y)
			}
		}
	} else {
		scaleX := float64(img.LastW) / float64(img.Img.Width)
		scaleY := float64(img.LastH) / float64(img.Img.Height)
		scale := scaleX
		if scaleY < scale {
			scale = scaleY
		}

		drawW := int(float64(img.Img.Width) * scale)
		drawH := int(float64(img.Img.Height) * scale)
		if drawW < 1 {
			drawW = 1
		}
		if drawH < 1 {
			drawH = 1
		}

		offsetX := (img.LastW - drawW) / 2
		offsetY := (img.LastH - drawH) / 2

		for y := 0; y < drawH; y++ {
			screenY := img.AbsY + offsetY + y
			if screenY < img.AbsY || screenY >= img.AbsY+img.LastH {
				continue
			}
			srcY := int(float64(y) / scale)
			if srcY >= img.Img.Height {
				srcY = img.Img.Height - 1
			}
			for x := 0; x < drawW; x++ {
				screenX := img.AbsX + offsetX + x
				if screenX < img.AbsX || screenX >= img.AbsX+img.LastW {
					continue
				}
				srcX := int(float64(x) / scale)
				if srcX >= img.Img.Width {
					srcX = img.Img.Width - 1
				}
				drawPixel(screenX, screenY, srcX, srcY)
			}
		}
	}
}
