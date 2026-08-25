package Graphite

import (
	"fmt"
	"testing"
	"time"
)

// newTestFader creates a Fader and resolves its layout (trackTop,
// trackHeight, buttonsRow, clipRow) via a real DrawRelative call, the same
// way Window would before routing any event to it.
func newTestFader() *Fader {
	f := NewFader(0, 0, 16, "CH1", RGB(255, 200, 0))
	f.DrawRelative(NewCanvas(), 0, 0, 16, 18)
	return f
}

func TestFader_StretchesToFillOfferedHeight(t *testing.T) {
	f := NewFader(0, 0, 16, "CH1", RGB(255, 200, 0))
	if f.Height > 0 {
		t.Fatalf("NewFader's Height = %d, want <= 0 (stretch by default)", f.Height)
	}

	f.DrawRelative(NewCanvas(), 0, 0, 16, 12)
	if f.LastH != 12 {
		t.Errorf("offered pH=12: LastH = %d, want 12", f.LastH)
	}

	f.DrawRelative(NewCanvas(), 0, 0, 16, 30)
	if f.LastH != 30 {
		t.Errorf("offered pH=30: LastH = %d, want 30 (should track the new offered height)", f.LastH)
	}
}

func TestFader_SetLevelClampsAndSetsClip(t *testing.T) {
	f := newTestFader()

	f.SetLevel(150)
	if f.Level != 100 {
		t.Errorf("Level = %v, want clamped to 100", f.Level)
	}
	if !f.Clipping {
		t.Error("Level at 100 (>= default ClipThreshold) should set Clipping")
	}

	f.SetLevel(0)
	if f.Level != 0 {
		t.Errorf("Level = %v, want clamped to 0", f.Level)
	}
	if f.Clipping {
		t.Error("Clipping should turn off when Level drops back down")
	}
}

func TestFader_ClickOnTrackJumpsValue(t *testing.T) {
	f := newTestFader()

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: f.trackTop})
	if f.Value != 100 {
		t.Errorf("click on the top track row: Value = %v, want 100", f.Value)
	}

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: f.trackTop + f.trackHeight - 1})
	if f.Value != 0 {
		t.Errorf("click on the bottom track row: Value = %v, want 0", f.Value)
	}
}

func TestFader_DragUpdatesValueLikeAClick(t *testing.T) {
	f := newTestFader()
	var changes []float64
	f.OnChange = func(v float64) { changes = append(changes, v) }

	top := f.trackTop
	bottom := f.trackTop + f.trackHeight - 1

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: top})
	f.HandleEvent(Event{Type: EventMouseDrag, MouseX: 10, MouseY: bottom})

	if f.Value != 0 {
		t.Errorf("after dragging to the bottom row, Value = %v, want 0", f.Value)
	}
	if len(changes) != 2 {
		t.Fatalf("OnChange fired %d times, want 2 (press + drag)", len(changes))
	}
}

func TestFader_DoubleClickFiresCallbackInsteadOfJumping(t *testing.T) {
	f := newTestFader()
	doubleClicks := 0
	f.OnDoubleClick = func() { doubleClicks++ }

	row := f.trackTop + 2
	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: row})
	valueAfterFirstClick := f.Value

	// Immediate second click at (almost) the same spot.
	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: row})

	if doubleClicks != 1 {
		t.Fatalf("OnDoubleClick fired %d times, want 1", doubleClicks)
	}
	if f.Value != valueAfterFirstClick {
		t.Errorf("Value changed on the double-click's second press: %v -> %v", valueAfterFirstClick, f.Value)
	}
}

func TestFader_SlowSecondClickIsNotADoubleClick(t *testing.T) {
	f := newTestFader()
	doubleClicks := 0
	f.OnDoubleClick = func() { doubleClicks++ }

	row := f.trackTop + 2
	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: row})
	f.lastClickAt = f.lastClickAt.Add(-time.Second) // simulate time passing

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: row})

	if doubleClicks != 0 {
		t.Errorf("OnDoubleClick fired for two clicks 1s apart, want 0")
	}
}

func TestFader_FarApartSecondClickIsNotADoubleClick(t *testing.T) {
	f := newTestFader()
	doubleClicks := 0
	f.OnDoubleClick = func() { doubleClicks++ }

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: f.trackTop})
	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 10, MouseY: f.trackTop + f.trackHeight - 1})

	if doubleClicks != 0 {
		t.Errorf("OnDoubleClick fired for two clicks far apart in the track, want 0")
	}
}

func TestFader_MuteAndSoloToggleAndFireCallbacks(t *testing.T) {
	f := newTestFader()
	var mutedStates, soloedStates []bool
	f.OnMuteChange = func(m bool) { mutedStates = append(mutedStates, m) }
	f.OnSoloChange = func(s bool) { soloedStates = append(soloedStates, s) }

	// Mute and Solo now share one row, split left (mute) / right (solo).
	muteX := f.AbsX
	soloX := f.AbsX + f.LastW - 1

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: muteX, MouseY: f.buttonsRow})
	if !f.Muted {
		t.Error("clicking the left (mute) half should set Muted")
	}
	f.HandleEvent(Event{Type: EventMouseDown, MouseX: muteX, MouseY: f.buttonsRow})
	if f.Muted {
		t.Error("clicking the left (mute) half again should clear Muted")
	}
	if len(mutedStates) != 2 || mutedStates[0] != true || mutedStates[1] != false {
		t.Errorf("OnMuteChange sequence = %v, want [true false]", mutedStates)
	}

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: soloX, MouseY: f.buttonsRow})
	if !f.Soloed {
		t.Error("clicking the right (solo) half should set Soloed")
	}
	if len(soloedStates) != 1 || soloedStates[0] != true {
		t.Errorf("OnSoloChange sequence = %v, want [true]", soloedStates)
	}
	if f.Muted {
		t.Error("clicking the solo half must not affect Muted")
	}
}

func TestFader_ClipIndicatorClickClearsClipping(t *testing.T) {
	f := newTestFader()
	f.SetLevel(100)
	if !f.Clipping {
		t.Fatal("setup: SetLevel(100) should set Clipping")
	}

	f.HandleEvent(Event{Type: EventMouseDown, MouseX: 5, MouseY: f.clipRow})
	if f.Clipping {
		t.Error("clicking the clip indicator should clear Clipping")
	}
}

func TestFader_ArrowKeysNudgeValue(t *testing.T) {
	f := newTestFader()
	f.Value = 50

	f.HandleEvent(Event{Type: EventKey, Key: KeyUp})
	if f.Value != 52 {
		t.Errorf("KeyUp: Value = %v, want 52", f.Value)
	}
	f.HandleEvent(Event{Type: EventKey, Key: KeyDown})
	f.HandleEvent(Event{Type: EventKey, Key: KeyDown})
	if f.Value != 48 {
		t.Errorf("Value after two KeyDown = %v, want 48", f.Value)
	}
}

// TestFader_EndToEndDragFromRawSGRBytes drives the whole input pipeline —
// raw SGR mouse bytes (what the terminal actually sends) through parseANSI
// through Window.HandleEvent's mouse-capture logic — into a Fader inside a
// real Window, rather than calling Fader.HandleEvent directly like the
// tests above. The tab-strip layout bug earlier only showed up once real
// rendering/routing was exercised end to end, not from unit-level checks
// alone, so this closes that gap for the new drag machinery specifically.
func TestFader_EndToEndDragFromRawSGRBytes(t *testing.T) {
	win := NewWindow(60, 40, "mixer")
	f := NewFader(2, 2, 16, "CH1", RGB(255, 200, 0))
	win.AddWidget(f)

	c := NewCanvas()
	c.Resize(80, 50)
	win.Draw(c)

	if f.trackHeight < 2 {
		t.Fatalf("setup: trackHeight = %d, too small for this test", f.trackHeight)
	}
	topRow := f.trackTop
	bottomRow := f.trackTop + f.trackHeight - 1

	press := []byte(fmt.Sprintf("\033[<0;%d;%dM", f.AbsX+8+1, topRow+1))
	drag := []byte(fmt.Sprintf("\033[<32;%d;%dM", f.AbsX+8+1, bottomRow+1))
	release := []byte(fmt.Sprintf("\033[<0;%d;%dm", f.AbsX+8+1, bottomRow+1))

	pressEvs := parseANSI(press)
	pressEv := pressEvs[0]
	if pressEv.Type != EventMouseDown || pressEv.MouseY != topRow {
		t.Fatalf("parseANSI(press) = %+v, want Type=EventMouseDown MouseY=%d", pressEv, topRow)
	}
	win.HandleEvent(pressEv)
	if f.Value != 100 {
		t.Fatalf("after press on the top row, Value = %v, want 100", f.Value)
	}

	dragEvs := parseANSI(drag)
	dragEv := dragEvs[0]
	if dragEv.Type != EventMouseDrag || dragEv.MouseY != bottomRow {
		t.Fatalf("parseANSI(drag) = %+v, want Type=EventMouseDrag MouseY=%d", dragEv, bottomRow)
	}
	win.HandleEvent(dragEv)
	if f.Value != 0 {
		t.Fatalf("after dragging to the bottom row, Value = %v, want 0", f.Value)
	}

	releaseEvs := parseANSI(release)
	releaseEv := releaseEvs[0]
	if releaseEv.Type != EventMouseUp {
		t.Fatalf("parseANSI(release) = %+v, want Type=EventMouseUp", releaseEv)
	}
	win.HandleEvent(releaseEv)

	// Capture should now be clear: a drag arriving with no prior press (as
	// would happen if the terminal ever sent one without a matching press,
	// or simply as a defensive check that release really cleared it) must
	// not move the fader.
	strays := parseANSI([]byte(fmt.Sprintf("\033[<32;%d;%dM", f.AbsX+8+1, topRow+1)))
	stray := strays[0]
	win.HandleEvent(stray)
	if f.Value != 0 {
		t.Errorf("a drag event after release moved Value to %v, want it to stay 0 (capture not cleared)", f.Value)
	}
}
