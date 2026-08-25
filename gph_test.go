package Graphite

import (
	"bytes"
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
				{Bg: ColorNone, Fg: RGB(255, 0, 0), Level: 0},
				{Bg: RGB(0, 255, 0), Fg: RGB(0, 0, 255), Level: 1},
				{Bg: RGB(255, 255, 255), Fg: RGB(0, 0, 0), Level: 2},
				{Bg: RGB(10, 20, 30), Fg: RGB(40, 50, 60), Level: 4},
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
