package main

import (
	"flag"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ojrac/opensimplex-go"
	Graphite "github.com/yeoblyv/graphite"
)

const logoText = `   █████████                                █████       ███   █████                ███████████ █████  █████ █████
  ███▒▒▒▒▒███                              ▒▒███       ▒▒▒   ▒▒███                ▒█▒▒▒███▒▒▒█▒▒███  ▒▒███ ▒▒███ 
 ███     ▒▒▒  ████████   ██████   ████████  ▒███████   ████  ███████    ██████    ▒   ▒███  ▒  ▒███   ▒███  ▒███ 
▒███         ▒▒███▒▒███ ▒▒▒▒▒███ ▒▒███▒▒███ ▒███▒▒███ ▒▒███ ▒▒▒███▒    ███▒▒███       ▒███     ▒███   ▒███  ▒███ 
▒███    █████ ▒███ ▒▒▒   ███████  ▒███ ▒███ ▒███ ▒███  ▒███   ▒███    ▒███████        ▒███     ▒███   ▒███  ▒███ 
▒▒███  ▒▒███  ▒███      ███▒▒███  ▒███ ▒███ ▒███ ▒███  ▒███   ▒███ ███▒███▒▒▒         ▒███     ▒███   ▒███  ▒███ 
 ▒▒█████████  █████    ▒▒████████ ▒███████  ████ █████ █████  ▒▒█████ ▒▒██████        █████    ▒▒████████   █████
  ▒▒▒▒▒▒▒▒▒  ▒▒▒▒▒      ▒▒▒▒▒▒▒▒  ▒███▒▒▒  ▒▒▒▒ ▒▒▒▒▒ ▒▒▒▒▒    ▒▒▒▒▒   ▒▒▒▒▒▒        ▒▒▒▒▒      ▒▒▒▒▒▒▒▒   ▒▒▒▒▒ 
                                  ▒███                                                                           
                                  █████                                                                          
                                 ▒▒▒▒▒                                                                           `

type NoiseBackground struct {
	Graphite.BaseWidget
	noise     opensimplex.Noise
	startTime time.Time
}

func NewNoiseBackground() *NoiseBackground {
	base := Graphite.NewBaseWidget(0, 0, 0, 0)
	base.SetPercentLayout(0, 0, 100, 100)
	return &NoiseBackground{
		BaseWidget: base,
		noise:      opensimplex.New(time.Now().UnixNano()),
		startTime:  time.Now(),
	}
}

var globalStartTime = time.Now()
var boomerangMode bool
var flowerMode bool
var showFps bool

type FPSWidget struct {
	*Graphite.BaseWidget
	lastDraw time.Time
	fps      float64
}

func NewFPSWidget() *FPSWidget {
	base := Graphite.NewBaseWidget(0, 0, 0, 0)
	f := &FPSWidget{BaseWidget: &base}
	f.lastDraw = time.Now()
	return f
}

func (f *FPSWidget) DrawRelative(c *Graphite.Canvas, offX, offY, pW, pH int) {
	now := time.Now()
	dt := now.Sub(f.lastDraw).Seconds()
	f.lastDraw = now

	if dt > 0 {
		currFps := 1.0 / dt
		f.fps = f.fps*0.9 + currFps*0.1
	}

	text := fmt.Sprintf(" FPS: %3.0f ", f.fps)
	c.DrawText(offX, offY, text, Graphite.RGB(255, 255, 255), Graphite.RGB(0, 0, 0))
}

func getTime() float64 {
	elapsed := time.Since(globalStartTime).Seconds()
	if boomerangMode {
		cycle := math.Mod(elapsed, 20.0)
		if cycle <= 10.0 {
			return cycle
		}
		return 20.0 - cycle
	}
	return elapsed
}

func getProgress(t float64) float64 {
	cycle := math.Mod(t, 10.0)
	if cycle < 0 {
		cycle += 10.0
	}

	if cycle < 4.0 {
		return 0.0
	} else if cycle < 5.0 {
		p := cycle - 4.0
		return p * p * (3.0 - 2.0*p) // smoothstep
	} else if cycle < 9.0 {
		return 1.0
	} else {
		p := cycle - 9.0
		p = p * p * (3.0 - 2.0*p)
		return 1.0 - p
	}
}

func getPaletteColor(v float64, inv float64) Graphite.Color {
	type stop struct {
		pos     float64
		r, g, b float64
	}
	stops := []stop{
		{0.0, 0, 0, 0},
		{0.35, 0, 0, 0},
		{0.55, 0, 5, 40},
		{0.65, 0, 10, 80},
		{0.75, 0, 120, 200},
		{0.85, 0, 200, 100},
		{0.92, 220, 180, 0},
		{0.96, 220, 40, 0},
		{1.0, 180, 0, 80},
	}

	var r, g, b float64
	if v <= 0 {
		r, g, b = stops[0].r, stops[0].g, stops[0].b
	} else if v >= 1 {
		last := stops[len(stops)-1]
		r, g, b = last.r, last.g, last.b
	} else {
		for i := 0; i < len(stops)-1; i++ {
			if v >= stops[i].pos && v <= stops[i+1].pos {
				t := (v - stops[i].pos) / (stops[i+1].pos - stops[i].pos)
				t = t * t * (3.0 - 2.0*t) // Smoothstep for buttery smooth color gradients
				r = stops[i].r + t*(stops[i+1].r-stops[i].r)
				g = stops[i].g + t*(stops[i+1].g-stops[i].g)
				b = stops[i].b + t*(stops[i+1].b-stops[i].b)
				break
			}
		}
	}

	if inv > 0 {
		r = r*(1.0-inv) + (255.0-r)*inv
		g = g*(1.0-inv) + (255.0-g)*inv
		b = b*(1.0-inv) + (255.0-b)*inv
	}

	return Graphite.RGB(uint8(r), uint8(g), uint8(b))
}

func (n *NoiseBackground) DrawRelative(c *Graphite.Canvas, offX, offY, pW, pH int) {
	n.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	t := getTime()
	progress := getProgress(t)
	noiseT := t * 0.4

	// Anti-aliased window for Luma Matte transition
	window := 0.05
	edge := 1.0 - (progress * (1.0 + window))

	for y := 0; y < n.LastH; y++ {
		for x := 0; x < n.LastW; x++ {
			var v1Top, v1Bot, v2Top, v2Bot float64

			if flowerMode {
				cx := float64(n.LastW) / 2.0
				cy := float64(n.LastH) / 2.0

				// Top pixel
				dxTop := float64(x) - cx
				dyTop := (float64(y*2) - cy*2.0)
				distTop := math.Sqrt(dxTop*dxTop + dyTop*dyTop)
				twistTop := distTop * 0.015
				angleTop := math.Atan2(dyTop, dxTop) - t*0.5 + twistTop

				// Use cylindrical mapping for seamless radial noise
				// Large radius (8.0) gives many petals. Slow Z variation gives long continuous strokes.
				nxTop := math.Cos(angleTop) * 8.0
				nyTop := math.Sin(angleTop) * 8.0
				nzTop := distTop*0.005 - noiseT*1.5
				v1Top = 1.0 - math.Abs(n.noise.Eval3(nxTop, nyTop, nzTop))
				v2Top = 1.0 - math.Abs(n.noise.Eval3(nxTop*2.0, nyTop*2.0, nzTop*1.8+100.0))

				maskTop := 1.0
				if distTop < 20.0 {
					maskTop = distTop / 20.0
				}
				v1Top *= maskTop
				v2Top *= maskTop

				// Bottom pixel
				dxBot := float64(x) - cx
				dyBot := (float64(y*2+1) - cy*2.0)
				distBot := math.Sqrt(dxBot*dxBot + dyBot*dyBot)
				twistBot := distBot * 0.015
				angleBot := math.Atan2(dyBot, dxBot) - t*0.5 + twistBot

				nxBot := math.Cos(angleBot) * 8.0
				nyBot := math.Sin(angleBot) * 8.0
				nzBot := distBot*0.005 - noiseT*1.5
				v1Bot = 1.0 - math.Abs(n.noise.Eval3(nxBot, nyBot, nzBot))
				v2Bot = 1.0 - math.Abs(n.noise.Eval3(nxBot*2.0, nyBot*2.0, nzBot*1.8+100.0))

				maskBot := 1.0
				if distBot < 20.0 {
					maskBot = distBot / 20.0
				}
				v1Bot *= maskBot
				v2Bot *= maskBot

			} else {
				// Zoom in noise to make lines longer and gradients smoother over space
				nxTop := float64(x) * 0.02
				nyTop := float64(y*2) * 0.04

				nxBot := float64(x) * 0.02
				nyBot := float64(y*2+1) * 0.04

				// Octave 1: Main organic streaks
				v1Top = 1.0 - math.Abs(n.noise.Eval3(nxTop, nyTop, noiseT))
				v1Bot = 1.0 - math.Abs(n.noise.Eval3(nxBot, nyBot, noiseT))

				// Octave 2: High frequency detail (reduced amplitude for smoother lines)
				v2Top = 1.0 - math.Abs(n.noise.Eval3(nxTop*2.0, nyTop*2.0, noiseT*1.8+100.0))
				v2Bot = 1.0 - math.Abs(n.noise.Eval3(nxBot*2.0, nyBot*2.0, noiseT*1.8+100.0))
			}

			// Fractal Brownian Motion (fBm) combination
			valTop := v1Top*0.9 + v2Top*0.1
			valBot := v1Bot*0.9 + v2Bot*0.1

			// Sharp, thin neon streaks with high contrast
			valTop = math.Pow(valTop, 5.0) * 1.9
			if valTop > 1.0 {
				valTop = 1.0
			}
			if valTop < 0.0 {
				valTop = 0.0
			}

			valBot = math.Pow(valBot, 5.0) * 1.9
			if valBot > 1.0 {
				valBot = 1.0
			}
			if valBot < 0.0 {
				valBot = 0.0
			}

			// Luma Matte: transition spreads across the bright streaks first
			invTop := (valTop - edge) / window
			if invTop < 0.0 {
				invTop = 0.0
			}
			if invTop > 1.0 {
				invTop = 1.0
			}

			invBot := (valBot - edge) / window
			if invBot < 0.0 {
				invBot = 0.0
			}
			if invBot > 1.0 {
				invBot = 1.0
			}

			fg := getPaletteColor(valTop, invTop)
			bg := getPaletteColor(valBot, invBot)

			c.DrawCell(n.AbsX+x, n.AbsY+y, "▀", bg, fg)
		}
	}
}

type LogoWidget struct {
	Graphite.BaseWidget
	lines     []string
	startTime time.Time
}

func NewLogoWidget(logo string) *LogoWidget {
	lines := strings.Split(logo, "\n")
	w := 0
	for _, l := range lines {
		if len([]rune(l)) > w {
			w = len([]rune(l))
		}
	}
	base := Graphite.NewBaseWidget(0, 0, w, len(lines))
	return &LogoWidget{
		BaseWidget: base,
		lines:      lines,
		startTime:  time.Now(),
	}
}

func (l *LogoWidget) DrawRelative(c *Graphite.Canvas, offX, offY, pW, pH int) {
	l.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	// Center the logo in the available space manually
	absX := offX + (pW-l.LastW)/2
	absY := offY + (pH-l.LastH)/2

	t := getTime()

	// Synchronize logo transition with background. Background flips around t=4.5.
	// Since logo wipes top-to-bottom over 0.6s, setting base to t + 0.3 means the middle (0.3)
	// will be exactly at t when the background flips.
	tLogoBase := t + 0.3
	if tLogoBase > 10.0 {
		tLogoBase -= 10.0
	}

	// Glare sweeps across logo every 2 seconds.
	cycle := math.Mod(t, 2.0)
	glarePos := (cycle/2.0)*float64(l.LastW+40) - 20

	for i, line := range l.lines {
		// Y-offset for top-to-bottom wipe (takes 0.6 seconds to scan across the logo height)
		yOffset := float64(i) / float64(len(l.lines)) * 0.6

		tRow := tLogoBase - yOffset
		if tRow < 0 {
			tRow += 10.0
		}

		// Chromatic Aberration/Shift transition per row (top-to-bottom wipe)
		invR := getProgress(tRow)
		invG := getProgress(tRow - 0.05)
		invB := getProgress(tRow - 0.10)

		// Accentuate the logo during the transition wipe by adding a massive brightness glow.
		// The difference between the staggered channels peaks during the transition.
		transitionGlow := math.Abs(invR-invB) * 2000.0

		runes := []rune(line)
		for j, r := range runes {
			if r != ' ' {
				dist := math.Abs(float64(j) - glarePos)
				var baseR, baseG, baseB float64
				if dist < 6.0 {
					intensity := 1.0 - (dist / 6.0)
					// Cyan glare
					baseR = 255.0 - intensity*(255.0-0.0)
					baseG = 255.0
					baseB = 255.0
				} else {
					// Pure white base + transition glow
					baseR = math.Min(255.0, 255.0+transitionGlow)
					baseG = math.Min(255.0, 255.0+transitionGlow)
					baseB = math.Min(255.0, 255.0+transitionGlow)
				}

				rCol := baseR*(1.0-invR) + (255.0-baseR)*invR
				gCol := baseG*(1.0-invG) + (255.0-baseG)*invG
				bCol := baseB*(1.0-invB) + (255.0-baseB)*invB

				// Clamp colors just in case the glow pushes them out of bounds after inversion
				if rCol > 255.0 {
					rCol = 255.0
				}
				if gCol > 255.0 {
					gCol = 255.0
				}
				if bCol > 255.0 {
					bCol = 255.0
				}
				if rCol < 0.0 {
					rCol = 0.0
				}
				if gCol < 0.0 {
					gCol = 0.0
				}
				if bCol < 0.0 {
					bCol = 0.0
				}

				fg := Graphite.RGB(uint8(rCol), uint8(gCol), uint8(bCol))
				c.DrawCell(absX+j, absY+i, string(r), Graphite.ColorNone, fg)
			}
		}
	}

	copyright := "Copyright (c) 2026 Yehor Oblyvantsov"
	crX := offX + (pW-len(copyright))/2
	crY := absY + len(l.lines) + 1

	// Copyright color crossfades but without chromatic shift
	invCr := getProgress(tLogoBase)
	crL := 150.0
	if invCr > 0.5 {
		crL = 255.0 - 150.0
	}
	c.DrawText(crX, crY, copyright, Graphite.ColorNone, Graphite.RGB(uint8(crL), uint8(crL), uint8(crL)))
}

func main() {
	flag.BoolVar(&boomerangMode, "boomerang", false, "Play animation forward then backward for seamless looping")
	flag.BoolVar(&flowerMode, "flower", false, "Draw a procedural flower instead of horizontal streaks")
	flag.BoolVar(&showFps, "fps", false, "Show frames per second")
	flag.Parse()

	app := Graphite.NewApplication()

	win := Graphite.NewWindow(0, 0, "")
	win.SetPercentSize(100, 100)

	// Add noise background
	noiseBg := NewNoiseBackground()
	win.AddWidget(noiseBg)

	// Add logo on top
	logo := NewLogoWidget(logoText)
	win.AddWidget(logo)

	if showFps {
		win.AddWidget(NewFPSWidget())
	}

	app.SetWindow(win)

	// Render loop
	go func() {
		for {
			app.Invoke(func() {})             // Triggers redraw
			time.Sleep(30 * time.Millisecond) // ~30 fps
		}
	}()

	app.Run()
}
