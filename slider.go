package Graphite

import (
	"fmt"
	"math"
	"time"
)

// Slider is a horizontal draggable control for setting a value within a range.
type Slider struct {
	BaseWidget

	Label    string
	Min      float64
	Max      float64
	Value    float64
	OnChange func(value float64)

	OnDoubleClick func()
	lastClickTime time.Time
}

// NewSlider creates a Slider at (x, y) with the given width.
// Min and Max define the range of the slider. Value starts at Min.
func NewSlider(x, y, w int, label string, min, max float64) *Slider {
	base := NewBaseWidget(x, y, w, 1)
	base.IsFocusable = true
	return &Slider{
		BaseWidget: base,
		Label:      label,
		Min:        min,
		Max:        max,
		Value:      min,
	}
}

// SetValue sets the slider's value, clamped to [Min, Max], and calls OnChange.
func (s *Slider) SetValue(v float64) {
	v = math.Min(math.Max(v, s.Min), s.Max)
	if v == s.Value {
		return
	}
	s.Value = v
	if s.OnChange != nil {
		s.OnChange(s.Value)
	}
}

// setValueFromCol maps a screen X coordinate to a value and updates the slider.
func (s *Slider) setValueFromCol(col int) {
	lblLen := len([]rune(s.Label)) + 1
	trackX := s.AbsX + lblLen
	trackW := s.LastW - lblLen - 7 // 7 for value display, e.g. " [100%]" or " [ 5.0]"
	if trackW < 1 {
		return
	}

	relX := col - trackX
	pct := float64(relX) / float64(trackW-1)
	pct = math.Min(math.Max(pct, 0), 1)

	val := s.Min + pct*(s.Max-s.Min)
	s.SetValue(val)
}

// DrawRelative implements Widget.
func (s *Slider) DrawRelative(c *Canvas, offX, offY, pW, pH int) {
	s.BaseWidget.DrawRelative(c, offX, offY, pW, pH)

	// Format: "Label [----|----] 100%"
	lblLen := len([]rune(s.Label))
	c.DrawText(s.AbsX, s.AbsY, s.Label, c.theme.BgWindow, c.theme.FgWindow)

	trackX := s.AbsX + lblLen + 1
	trackW := s.LastW - lblLen - 7
	if trackW < 1 {
		return // Too narrow to draw track
	}

	c.DrawCell(trackX-1, s.AbsY, "[", c.theme.BgWindow, c.theme.FgWindow)
	c.DrawCell(trackX+trackW, s.AbsY, "]", c.theme.BgWindow, c.theme.FgWindow)

	pct := 0.0
	if s.Max > s.Min {
		pct = (s.Value - s.Min) / (s.Max - s.Min)
	}

	handleCol := int(pct * float64(trackW-1))
	handleBg := c.theme.BgWidget
	if s.IsFocused {
		handleBg = c.theme.BgFocused
	}

	for i := 0; i < trackW; i++ {
		char := "-"
		if i == handleCol {
			char = "█"
			c.DrawCell(trackX+i, s.AbsY, char, handleBg, c.theme.Primary)
		} else {
			c.DrawCell(trackX+i, s.AbsY, char, c.theme.BgWindow, c.theme.FgDisabled)
		}
	}

	valStr := fmt.Sprintf(" %4.0f", s.Value)
	c.DrawText(trackX+trackW+1, s.AbsY, valStr, c.theme.BgWindow, c.theme.FgWindow)
}

// HandleEvent implements Widget.
func (s *Slider) HandleEvent(ev Event) {
	switch ev.Type {
	case EventKey:
		step := (s.Max - s.Min) / 20.0
		if step == 0 {
			step = 1
		}
		if ev.Key == KeyLeft || ev.Key == KeyDown {
			s.SetValue(s.Value - step)
		} else if ev.Key == KeyRight || ev.Key == KeyUp {
			s.SetValue(s.Value + step)
		}
	case EventMouseDown, EventMouseDrag:
		s.setValueFromCol(ev.MouseX)
		if ev.Type == EventMouseDown {
			now := time.Now()
			if s.OnDoubleClick != nil && now.Sub(s.lastClickTime) < 500*time.Millisecond {
				s.OnDoubleClick()
			}
			s.lastClickTime = now
		}
	}
}
