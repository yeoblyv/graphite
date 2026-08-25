package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeoblyv/graphite"
)

type AppState struct {
	OriginalFrames []image.Image
	GphImage       *Graphite.GphImage
	Width          int
	Height         int
	DitherLevel    float64 // Threshold for █▓▒░ space (0 to 1)
	Saturation     float64 // 0 to 200 (100 = normal)
	Contrast       float64 // 0 to 200 (100 = normal)
	Grayscale      bool
	Filename       string
}

func main() {
	app := Graphite.NewApplication()
	state := &AppState{
		Width:       40,
		Height:      20,
		DitherLevel: 0.5,
		Saturation:  100,
		Contrast:    100,
	}

	win := Graphite.NewWindow(120, 36, " GPH Editor ")
	win.SetPercentSize(100, 100)

	var renderImage func()

	// MenuStrip
	menu := Graphite.NewMenuStrip([]Graphite.MenuCategory{
		{
			Label: "File",
			Items: []Graphite.MenuItem{
				{Label: "Open Media...", Action: func() {
					Graphite.ShowFilePicker(app, ".", func(path string) {
						ext := strings.ToLower(filepath.Ext(path))
						isVideo := ext == ".mp4" || ext == ".avi" || ext == ".mkv" || ext == ".webm"

						state.Filename = path
						var frames []image.Image

						if isVideo {
							tmpDir, err := os.MkdirTemp("", "gph_video")
							if err != nil {
								app.ShowMessage("Error", "Could not create temp dir", Graphite.BtnDanger)
								return
							}

							app.ShowMessage("Loading", "Extracting video frames...", Graphite.BtnDefault)

							go func() {
								cmd := exec.Command("ffmpeg", "-y", "-i", path, "-vf", "fps=10,scale=320:-1", filepath.Join(tmpDir, "frame_%04d.jpg"))
								err := cmd.Run()

								app.Invoke(func() {
									if err != nil {
										os.RemoveAll(tmpDir)
										app.CloseModal()
										app.ShowMessage("Error", "Failed to extract video frames. Is ffmpeg installed?", Graphite.BtnDanger)
										return
									}

									entries, _ := os.ReadDir(tmpDir)
									for _, e := range entries {
										if strings.HasSuffix(e.Name(), ".jpg") {
											f, _ := os.Open(filepath.Join(tmpDir, e.Name()))
											img, _, _ := image.Decode(f)
											f.Close()
											if img != nil {
												frames = append(frames, img)
											}
										}
									}
									os.RemoveAll(tmpDir)
									app.CloseModal() // Close loading message

									if len(frames) == 0 {
										app.ShowMessage("Error", "No frames extracted", Graphite.BtnDanger)
										return
									}

									state.OriginalFrames = frames
									b := frames[0].Bounds()
									ratio := float64(b.Dx()) / float64(b.Dy())
									state.Width = 60
									state.Height = int(60.0 / ratio / 2.0)
									if state.Height < 1 {
										state.Height = 1
									}

									renderImage()
								})
							}()
						} else {
							f, err := os.Open(path)
							if err != nil {
								app.ShowMessage("Error", "Could not open file", Graphite.BtnDanger)
								return
							}
							defer f.Close()
							img, _, err := image.Decode(f)
							if err != nil {
								app.ShowMessage("Error", "Invalid image format", Graphite.BtnDanger)
								return
							}
							frames = append(frames, img)
							state.OriginalFrames = frames
							b := frames[0].Bounds()
							ratio := float64(b.Dx()) / float64(b.Dy())
							state.Width = 60
							state.Height = int(60.0 / ratio / 2.0)
							if state.Height < 1 {
								state.Height = 1
							}
							renderImage()
						}
					})
				}},
				{Label: "Save .gph...", Action: func() {
					if state.GphImage == nil {
						app.ShowMessage("Error", "No image to save", Graphite.BtnDanger)
						return
					}
					outPath := state.Filename + ".gph"
					f, err := os.Create(outPath)
					if err != nil {
						app.ShowMessage("Error", "Could not create file", Graphite.BtnDanger)
						return
					}
					defer f.Close()
					if err := Graphite.WriteGph(f, state.GphImage); err != nil {
						app.ShowMessage("Error", "Could not write .gph", Graphite.BtnDanger)
						return
					}
					app.ShowMessage("Success", "Saved to "+outPath, Graphite.BtnSuccess)
				}},
				{Label: "Exit", Action: func() { app.Quit() }},
			},
		},
		{
			Label: "Help",
			Items: []Graphite.MenuItem{
				{Label: "GitHub Repo", Action: func() {
					app.ShowMessage("GitHub Repo", "Find the source code at:\nhttps://github.com/yeoblyv/graphite", Graphite.BtnDefault)
				}},
				{Label: "About", Action: func() {
					app.ShowMessage("About GPH Editor", "GPH Editor v1.0\nSupported Protocol: GPH v1\nWritten in Go using Graphite UI.\n(c) 2026 yeoblyv", Graphite.BtnDefault)
				}},
			},
		},
	})

	// Layout
	mainFlex := Graphite.NewFlex(0, 1, 0, 0, Graphite.FlexRow)
	mainFlex.SetPercentLayout(0, 0, 100, 100)
	win.AddWidget(mainFlex)

	leftCol := Graphite.NewFlex(0, 0, 0, 0, Graphite.FlexColumn)

	// Right Panel (Image preview)
	imgGroup := Graphite.NewGroupBox(0, 0, 0, 0, "Preview")
	imgWidget := Graphite.NewImage(0, 0, nil)
	imgWidget.SetPercentLayout(0, 0, 100, 100)
	imgGroup.AddWidget(imgWidget)

	mainFlex.AddChild(leftCol, 35)
	mainFlex.AddChild(imgGroup, 65)

	// Settings GroupBox
	settingsGroup := Graphite.NewGroupBox(0, 0, 0, 0, "Image Settings")
	leftCol.AddChild(settingsGroup, 2)

	settingsFlex := Graphite.NewFlex(0, 0, 0, 0, Graphite.FlexColumn)
	settingsFlex.SetPercentLayout(0, 0, 100, 100)
	settingsFlex.Gap = 1
	settingsGroup.AddWidget(settingsFlex)

	// Auto Proportion controls
	propCheckbox := Graphite.NewCheckbox(0, 0, "Proportional Scaling", true)
	settingsFlex.AddChild(propCheckbox, 0)

	autoSizeCheckbox := Graphite.NewCheckbox(0, 0, "Auto Size (Stretch)", true)
	autoSizeCheckbox.OnChange = func(checked bool) {
		imgWidget.AutoSize = checked
	}
	settingsFlex.AddChild(autoSizeCheckbox, 0)

	resetBtn := Graphite.NewButton(0, 0, "Reset Proportions", Graphite.BtnDefault, func() {
		if len(state.OriginalFrames) > 0 {
			b := state.OriginalFrames[0].Bounds()
			ratio := float64(b.Dx()) / float64(b.Dy())
			state.Width = 60
			state.Height = int(60.0 / ratio / 2.0)
			if state.Height < 1 {
				state.Height = 1
			}
			renderImage()
		}
	})
	settingsFlex.AddChild(resetBtn, 0)

	// Sliders
	widthSlider := Graphite.NewSlider(0, 0, 0, "Width: ", 10, 240)
	var heightSlider *Graphite.Slider

	widthSlider.Value = float64(state.Width)
	widthSlider.OnChange = func(v float64) {
		oldW := float64(state.Width)
		state.Width = int(v)
		if propCheckbox.Checked && oldW > 0 {
			ratio := float64(state.Width) / oldW
			newH := float64(state.Height) * ratio
			if newH < 10 {
				newH = 10
			}
			if newH > 160 {
				newH = 160
			}
			state.Height = int(newH)
			if heightSlider != nil {
				heightSlider.Value = float64(state.Height)
			}
		}
		renderImage()
	}
	widthSlider.OnDoubleClick = func() {
		Graphite.ShowValueEditor(app, "Width", float64(state.Width), 10, 240, func(v float64) {
			widthSlider.SetValue(v)
		})
	}
	settingsFlex.AddChild(widthSlider, 0)

	heightSlider = Graphite.NewSlider(0, 0, 0, "Height:", 10, 160)
	heightSlider.Value = float64(state.Height)
	heightSlider.OnChange = func(v float64) {
		oldH := float64(state.Height)
		state.Height = int(v)
		if propCheckbox.Checked && oldH > 0 {
			ratio := float64(state.Height) / oldH
			newW := float64(state.Width) * ratio
			if newW < 10 {
				newW = 10
			}
			if newW > 240 {
				newW = 240
			}
			state.Width = int(newW)
			widthSlider.Value = float64(state.Width)
		}
		renderImage()
	}
	heightSlider.OnDoubleClick = func() {
		Graphite.ShowValueEditor(app, "Height", float64(state.Height), 10, 160, func(v float64) {
			heightSlider.SetValue(v)
		})
	}
	settingsFlex.AddChild(heightSlider, 0)

	contrastSlider := Graphite.NewSlider(0, 0, 0, "Contr: ", 0, 400)
	contrastSlider.Value = state.Contrast
	contrastSlider.OnChange = func(v float64) { state.Contrast = v; renderImage() }
	contrastSlider.OnDoubleClick = func() {
		Graphite.ShowValueEditor(app, "Contrast", state.Contrast, 0, 400, func(v float64) {
			contrastSlider.SetValue(v)
		})
	}
	settingsFlex.AddChild(contrastSlider, 0)

	satSlider := Graphite.NewSlider(0, 0, 0, "Satur: ", 0, 400)
	satSlider.Value = state.Saturation
	satSlider.OnChange = func(v float64) { state.Saturation = v; renderImage() }
	satSlider.OnDoubleClick = func() {
		Graphite.ShowValueEditor(app, "Saturation", state.Saturation, 0, 400, func(v float64) {
			satSlider.SetValue(v)
		})
	}
	settingsFlex.AddChild(satSlider, 0)

	ditherSlider := Graphite.NewSlider(0, 0, 0, "Dither:", 0, 200)
	ditherSlider.Value = state.DitherLevel * 100
	ditherSlider.OnChange = func(v float64) { state.DitherLevel = v / 100.0; renderImage() }
	ditherSlider.OnDoubleClick = func() {
		Graphite.ShowValueEditor(app, "Dither", state.DitherLevel*100, 0, 200, func(v float64) {
			ditherSlider.SetValue(v)
		})
	}
	settingsFlex.AddChild(ditherSlider, 0)

	isGray := false
	var grayBtn *Graphite.Button
	grayBtn = Graphite.NewButton(0, 0, "Toggle Grayscale: OFF", Graphite.BtnDefault, func() {
		isGray = !isGray
		if isGray {
			grayBtn.Text = "Toggle Grayscale: ON "
			grayBtn.Style = Graphite.BtnSuccess
		} else {
			grayBtn.Text = "Toggle Grayscale: OFF"
			grayBtn.Style = Graphite.BtnDefault
		}
		state.Grayscale = isGray
		renderImage()
	})
	settingsFlex.AddChild(grayBtn, 0)

	// Animation Settings
	animGroup := Graphite.NewGroupBox(0, 0, 0, 0, "Animation")
	leftCol.AddChild(animGroup, 1)

	animFlex := Graphite.NewFlex(0, 0, 0, 0, Graphite.FlexColumn)
	animFlex.SetPercentLayout(0, 0, 100, 100)
	animFlex.Gap = 1
	animGroup.AddWidget(animFlex)

	fpsLabel := Graphite.NewLabel(0, 0, "Real FPS: 0.0")
	animFlex.AddChild(fpsLabel, 0)

	var lastFrameTime time.Time
	var frameCount int
	var currentFPS float64

	imgWidget.OnFrameUpdate = func() {
		now := time.Now()
		frameCount++
		if elapsed := now.Sub(lastFrameTime); elapsed >= time.Second {
			currentFPS = float64(frameCount) / elapsed.Seconds()
			fpsLabel.SetText(fmt.Sprintf("Real FPS: %.1f", currentFPS))
			frameCount = 0
			lastFrameTime = now
		}
	}

	prevBtn := Graphite.NewButton(0, 0, "< Prev Frame", Graphite.BtnDefault, func() {
		imgWidget.PrevFrame()
	})
	animFlex.AddChild(prevBtn, 0)

	nextBtn := Graphite.NewButton(0, 0, "Next Frame >", Graphite.BtnDefault, func() {
		imgWidget.NextFrame()
	})
	animFlex.AddChild(nextBtn, 0)

	modeLabels := []string{"Mode: Static", "Mode: Once", "Mode: Loop", "Mode: Boomerang"}
	modeVals := []Graphite.PlaybackMode{Graphite.PlaybackStatic, Graphite.PlaybackOnce, Graphite.PlaybackLoop, Graphite.PlaybackBoomerang}
	modeIdx := 2

	var modeBtn *Graphite.Button
	modeBtn = Graphite.NewButton(0, 0, modeLabels[modeIdx], Graphite.BtnInfo, func() {
		modeIdx = (modeIdx + 1) % len(modeVals)
		modeBtn.Text = modeLabels[modeIdx]
		if state.GphImage != nil {
			state.GphImage.Mode = modeVals[modeIdx]
			imgWidget.Play(app)
		}
	})
	animFlex.AddChild(modeBtn, 0)

	fpsSlider := Graphite.NewSlider(0, 0, 0, "Target FPS:", 1, 60)
	fpsSlider.Value = 10
	fpsSlider.OnChange = func(v float64) {
		if state.GphImage != nil {
			state.GphImage.DelayMs = uint16(1000.0 / v)
			imgWidget.Play(app)
		}
	}
	fpsSlider.OnDoubleClick = func() {
		Graphite.ShowValueEditor(app, "Target FPS", fpsSlider.Value, 1, 60, func(v float64) {
			fpsSlider.SetValue(v)
		})
	}
	animFlex.AddChild(fpsSlider, 0)

	renderImage = func() {
		if len(state.OriginalFrames) == 0 {
			return
		}

		gph := &Graphite.GphImage{
			Width:   state.Width,
			Height:  state.Height,
			Mode:    Graphite.PlaybackLoop,
			DelayMs: 100,
			Frames:  make([][]Graphite.GphPixel, len(state.OriginalFrames)),
		}

		// Contrast factor
		cFactor := (259.0 * (state.Contrast - 100.0 + 255.0)) / (255.0 * (259.0 - (state.Contrast - 100.0)))

		for i, src := range state.OriginalFrames {
			bounds := src.Bounds()
			srcW, srcH := bounds.Dx(), bounds.Dy()
			framePixels := make([]Graphite.GphPixel, state.Width*state.Height)

			for y := 0; y < state.Height; y++ {
				for x := 0; x < state.Width; x++ {
					// Nearest neighbor coordinates
					srcX := bounds.Min.X + int(float64(x)/float64(state.Width)*float64(srcW))
					srcY := bounds.Min.Y + int(float64(y)/float64(state.Height)*float64(srcH))
					if srcX >= bounds.Max.X {
						srcX = bounds.Max.X - 1
					}
					if srcY >= bounds.Max.Y {
						srcY = bounds.Max.Y - 1
					}

					r, g, b, _ := src.At(srcX, srcY).RGBA()
					r8, g8, b8 := float64(r>>8), float64(g>>8), float64(b>>8)

					// Contrast
					r8 = cFactor*(r8-128) + 128
					g8 = cFactor*(g8-128) + 128
					b8 = cFactor*(b8-128) + 128

					// Grayscale / Saturation
					luma := 0.299*r8 + 0.587*g8 + 0.114*b8
					if state.Grayscale {
						r8, g8, b8 = luma, luma, luma
					} else if state.Saturation != 100 {
						sat := state.Saturation / 100.0
						r8 = luma + (r8-luma)*sat
						g8 = luma + (g8-luma)*sat
						b8 = luma + (b8-luma)*sat
					}

					// Clamp
					r8 = math.Min(math.Max(r8, 0), 255)
					g8 = math.Min(math.Max(g8, 0), 255)
					b8 = math.Min(math.Max(b8, 0), 255)

					// Dithering level
					intensity := 1.0 - (luma / 255.0)
					levelFloat := intensity * 5.0 * (state.DitherLevel * 2.0)
					level := uint8(math.Min(math.Max(levelFloat, 0), 4))

					framePixels[y*state.Width+x] = Graphite.GphPixel{
						Bg:    Graphite.RGB(uint8(r8), uint8(g8), uint8(b8)),
						Fg:    Graphite.RGB(uint8(r8/2), uint8(g8/2), uint8(b8/2)),
						Level: level,
					}
				}
			}
			gph.Frames[i] = framePixels
		}

		state.GphImage = gph
		imgWidget.Img = gph
		imgWidget.Play(app)
	}

	win.AddWidget(menu)
	app.SetWindow(win)
	app.Run()
}
