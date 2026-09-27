package Graphite

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestGphSerialization(t *testing.T) {
	original := &GphImage{
		Width:  2,
		Height: 2,
		Mode:   PlaybackLoop,
		Frames: [][]GphPixel{
			{
				{Bg: ColorNone, Fg: RGB(255, 0, 0), Level: 0, Alpha: 255},
				{Bg: RGB(0, 255, 0), Fg: RGB(0, 0, 255), Level: 1, Alpha: 255},
				{Bg: RGB(255, 255, 255), Fg: RGB(0, 0, 0), Level: 2, Alpha: 128},
				{Bg: RGB(10, 20, 30), Fg: RGB(40, 50, 60), Level: 4, Alpha: 0},
			},
		},
	}

	var buf bytes.Buffer
	err := WriteGph(&buf, original)
	if err != nil {
		t.Fatalf("WriteGph failed: %v", err)
	}

	decoded, err := ReadGph(&buf)
	if err != nil {
		t.Fatalf("ReadGph failed: %v", err)
	}

	if decoded.Width != original.Width || decoded.Height != original.Height {
		t.Errorf("Dimensions mismatch: got %dx%d, want %dx%d", decoded.Width, decoded.Height, original.Width, original.Height)
	}

	if !reflect.DeepEqual(decoded.Frames, original.Frames) {
		t.Errorf("Frames mismatch.\nGot:  %+v\nWant: %+v", decoded.Frames, original.Frames)
	}
}

func TestGphInvalidMagic(t *testing.T) {
	invalidData := []byte("BAD\x01\x00\x00\x00\x00\x00\x00\x00\x00")
	_, err := ReadGph(bytes.NewReader(invalidData))
	if err == nil {
		t.Error("ReadGph should fail on invalid magic bytes")
	}
}

// TestWriteGphAlwaysEmitsV2 guards WriteGph's documented behavior: every
// file it produces is readable by decoding logic that assumes a real Alpha
// channel is present, not the legacy 9-byte-per-pixel V1 shape.
func TestWriteGphAlwaysEmitsV2(t *testing.T) {
	var buf bytes.Buffer
	img := &GphImage{Width: 1, Height: 1, Frames: [][]GphPixel{{{Alpha: 255}}}}
	if err := WriteGph(&buf, img); err != nil {
		t.Fatalf("WriteGph failed: %v", err)
	}
	got := buf.Bytes()[:4]
	if !bytes.Equal(got, GphMagicV2) {
		t.Errorf("WriteGph wrote magic %q, want V2 magic %q", got, GphMagicV2)
	}
}

// TestReadGphV1FileDefaultsToFullyOpaque ensures a pre-alpha V1 file (9
// bytes/pixel, no Alpha byte at all) keeps rendering exactly as before:
// every decoded pixel must come back Alpha=255, never the Go zero value 0
// (which would make every existing .gph file in the wild vanish).
func TestReadGphV1FileDefaultsToFullyOpaque(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(GphMagic) // V1 magic
	header := make([]byte, 15)
	header[8] = uint8(PlaybackStatic)
	binary.LittleEndian.PutUint32(header[11:15], 1) // 1 frame
	buf.Write(header)

	// Two 9-byte V1 pixels, both marked "changed" in the bitmask:
	// Bg=RGB(1,2,3), Fg=RGB(4,5,6), Level=2 — no Alpha byte exists in this
	// format at all.
	buf.Write([]byte{0b11})
	pixel := make([]byte, 9)
	binary.LittleEndian.PutUint32(pixel[0:4], uint32(RGB(1, 2, 3)))
	binary.LittleEndian.PutUint32(pixel[4:8], uint32(RGB(4, 5, 6)))
	pixel[8] = 2
	buf.Write(pixel)
	buf.Write(pixel)

	img, err := ReadGph(&buf)
	if err != nil {
		t.Fatalf("ReadGph failed on a legacy V1 stream: %v", err)
	}
	for i, p := range img.Frames[0] {
		if p.Alpha != 255 {
			t.Errorf("pixel %d: Alpha = %d, want 255 (V1 files predate alpha and were always opaque)", i, p.Alpha)
		}
	}
}

// TestImageDrawRelative_AlphaBlendsIntoCanvasUnderneath is the actual
// user-facing behavior the alpha channel exists for: a partially
// transparent pixel should render as a mix of its own color and whatever
// was already on the canvas at that cell, not overwrite it outright.
func TestImageDrawRelative_AlphaBlendsIntoCanvasUnderneath(t *testing.T) {
	c := NewCanvas()
	c.Resize(1, 1)
	underBg := RGB(0, 0, 0)
	c.DrawCell(0, 0, " ", underBg, ColorNone)

	img := &GphImage{
		Width: 1, Height: 1,
		Frames: [][]GphPixel{{{Bg: RGB(255, 255, 255), Fg: ColorNone, Level: 4, Alpha: 128}}},
	}
	w := NewImage(0, 0, img)
	w.DrawRelative(c, 0, 0, 1, 1)

	got := c.buffer[0].BgColor
	want := blendColor(underBg, RGB(255, 255, 255), 128)
	if got != want {
		t.Errorf("blended bg = %v, want %v (halfway between %v and white)", got, want, underBg)
	}
	if got == RGB(255, 255, 255) {
		t.Errorf("alpha=128 pixel rendered fully opaque white — blending didn't happen")
	}
}

// TestImageDrawRelative_ZeroAlphaSkipsDrawEntirely mirrors the existing
// "fully blank pixel" skip: Alpha=0 must leave whatever was already drawn
// completely untouched, the same as a Bg=Fg=ColorNone/Level=0 pixel does.
func TestImageDrawRelative_ZeroAlphaSkipsDrawEntirely(t *testing.T) {
	c := NewCanvas()
	c.Resize(1, 1)
	c.DrawCell(0, 0, "x", RGB(9, 9, 9), RGB(8, 8, 8))

	img := &GphImage{
		Width: 1, Height: 1,
		Frames: [][]GphPixel{{{Bg: RGB(255, 0, 0), Fg: RGB(0, 255, 0), Level: 4, Alpha: 0}}},
	}
	w := NewImage(0, 0, img)
	w.DrawRelative(c, 0, 0, 1, 1)

	if c.buffer[0].Symbol != "x" || c.buffer[0].BgColor != RGB(9, 9, 9) {
		t.Errorf("Alpha=0 pixel drew over the existing cell: got %+v", c.buffer[0])
	}
}
