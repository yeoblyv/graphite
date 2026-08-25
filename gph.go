package Graphite

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// GphMagic is the expected file signature for version 1 of the GPH format.
var GphMagic = []byte{'G', 'P', 'H', '\x01'}

// PlaybackMode defines how animation frames are played.
type PlaybackMode uint8

const (
	PlaybackStatic    PlaybackMode = 0
	PlaybackLoop      PlaybackMode = 1
	PlaybackBoomerang PlaybackMode = 2
	PlaybackOnce      PlaybackMode = 3
)

// GphPixel represents a single "pixel" in a console pseudographics image.
type GphPixel struct {
	Bg    Color
	Fg    Color
	Level uint8 // 0=0%, 1=25%, 2=50%, 3=75%, 4=100% density
}

// GphImage contains the dimensions, playback metadata, and frames of a GPH file.
type GphImage struct {
	Width   int
	Height  int
	Mode    PlaybackMode
	DelayMs uint16
	Frames  [][]GphPixel
}

// WriteGph serializes the image into the GPH binary format.
// It uses bitmask-based delta encoding for frames after the first one.
func WriteGph(w io.Writer, img *GphImage) error {
	if img.Width < 0 || img.Height < 0 {
		return errors.New("Graphite.WriteGph: invalid image dimensions")
	}

	if _, err := w.Write(GphMagic); err != nil {
		return err
	}

	// Header: 15 bytes total
	// Width(4) + Height(4) + Mode(1) + DelayMs(2) + FrameCount(4)
	header := make([]byte, 15)
	binary.LittleEndian.PutUint32(header[0:4], uint32(img.Width))
	binary.LittleEndian.PutUint32(header[4:8], uint32(img.Height))
	header[8] = uint8(img.Mode)
	binary.LittleEndian.PutUint16(header[9:11], img.DelayMs)
	binary.LittleEndian.PutUint32(header[11:15], uint32(len(img.Frames)))
	if _, err := w.Write(header); err != nil {
		return err
	}

	pixelsPerFrame := img.Width * img.Height
	if pixelsPerFrame == 0 {
		return nil
	}

	bitmaskLen := (pixelsPerFrame + 7) / 8
	var prevFrame []GphPixel // starts as nil

	for _, frame := range img.Frames {
		if len(frame) != pixelsPerFrame {
			return errors.New("Graphite.WriteGph: frame size mismatch")
		}

		bitmask := make([]byte, bitmaskLen)
		var changed []GphPixel

		for i := 0; i < pixelsPerFrame; i++ {
			changedPixel := true
			if prevFrame != nil {
				if frame[i] == prevFrame[i] {
					changedPixel = false
				}
			}
			if changedPixel {
				bitmask[i/8] |= 1 << (i % 8)
				changed = append(changed, frame[i])
			}
		}

		// Write bitmask
		if _, err := w.Write(bitmask); err != nil {
			return err
		}

		// Write changed pixels
		pixelData := make([]byte, len(changed)*9)
		for i, p := range changed {
			offset := i * 9
			binary.LittleEndian.PutUint32(pixelData[offset:offset+4], uint32(p.Bg))
			binary.LittleEndian.PutUint32(pixelData[offset+4:offset+8], uint32(p.Fg))
			pixelData[offset+8] = p.Level
		}
		if _, err := w.Write(pixelData); err != nil {
			return err
		}

		prevFrame = frame
	}

	return nil
}

// ReadGph parses a GPH binary stream with delta decoding.
func ReadGph(r io.Reader) (*GphImage, error) {
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("Graphite.ReadGph: failed to read magic: %w", err)
	}
	if string(magic) != string(GphMagic) {
		return nil, fmt.Errorf("Graphite.ReadGph: invalid or unsupported file format magic: %q", magic)
	}

	header := make([]byte, 15)
	// For backwards compatibility, we might not have 15 bytes if it was an old V1 file.
	// The old header was just Width(4) + Height(4).
	// To safely handle this, let's read the first 8 bytes, then try to read the next 7 bytes.
	if _, err := io.ReadFull(r, header[0:8]); err != nil {
		return nil, fmt.Errorf("Graphite.ReadGph: failed to read dimensions: %w", err)
	}

	width := int(binary.LittleEndian.Uint32(header[0:4]))
	height := int(binary.LittleEndian.Uint32(header[4:8]))

	if width < 0 || height < 0 || width > 10000 || height > 10000 {
		return nil, fmt.Errorf("Graphite.ReadGph: insane image dimensions %dx%d", width, height)
	}
	pixelsPerFrame := width * height

	img := &GphImage{
		Width:  width,
		Height: height,
	}

	n, err := io.ReadFull(r, header[8:15])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("Graphite.ReadGph: error reading extended header: %w", err)
	}

	isLegacyV1 := n < 7
	if isLegacyV1 {
		img.Mode = PlaybackStatic
		img.DelayMs = 0
		img.Frames = make([][]GphPixel, 1)
		img.Frames[0] = make([]GphPixel, pixelsPerFrame)

		// The rest of the stream is just the pixel data for 1 frame
		pixelData := make([]byte, pixelsPerFrame*9)
		// We already consumed `n` bytes of pixel data by accident!
		// Let's copy the `n` bytes we read into `pixelData` and read the rest.
		copy(pixelData, header[8:8+n])
		if _, err := io.ReadFull(r, pixelData[n:]); err != nil {
			return nil, fmt.Errorf("Graphite.ReadGph: failed to read legacy pixel data: %w", err)
		}

		for i := 0; i < pixelsPerFrame; i++ {
			offset := i * 9
			bg := int32(binary.LittleEndian.Uint32(pixelData[offset : offset+4]))
			fg := int32(binary.LittleEndian.Uint32(pixelData[offset+4 : offset+8]))
			level := pixelData[offset+8]
			img.Frames[0][i] = GphPixel{Bg: Color(bg), Fg: Color(fg), Level: level}
		}
		return img, nil
	}

	// Normal reading for animated/delta V1
	img.Mode = PlaybackMode(header[8])
	img.DelayMs = binary.LittleEndian.Uint16(header[9:11])
	frameCount := binary.LittleEndian.Uint32(header[11:15])

	img.Frames = make([][]GphPixel, frameCount)
	if pixelsPerFrame == 0 || frameCount == 0 {
		return img, nil
	}

	bitmaskLen := (pixelsPerFrame + 7) / 8
	currentFrame := make([]GphPixel, pixelsPerFrame)
	bitmask := make([]byte, bitmaskLen)

	for f := uint32(0); f < frameCount; f++ {
		if _, err := io.ReadFull(r, bitmask); err != nil {
			return nil, fmt.Errorf("Graphite.ReadGph: failed to read bitmask for frame %d: %w", f, err)
		}

		changedCount := 0
		for i := 0; i < pixelsPerFrame; i++ {
			if (bitmask[i/8] & (1 << (i % 8))) != 0 {
				changedCount++
			}
		}

		pixelData := make([]byte, changedCount*9)
		if _, err := io.ReadFull(r, pixelData); err != nil {
			return nil, fmt.Errorf("Graphite.ReadGph: failed to read pixels for frame %d: %w", f, err)
		}

		pixelIdx := 0
		for i := 0; i < pixelsPerFrame; i++ {
			if (bitmask[i/8] & (1 << (i % 8))) != 0 {
				offset := pixelIdx * 9
				bg := int32(binary.LittleEndian.Uint32(pixelData[offset : offset+4]))
				fg := int32(binary.LittleEndian.Uint32(pixelData[offset+4 : offset+8]))
				level := pixelData[offset+8]
				currentFrame[i] = GphPixel{Bg: Color(bg), Fg: Color(fg), Level: level}
				pixelIdx++
			}
		}

		frameCopy := make([]GphPixel, pixelsPerFrame)
		copy(frameCopy, currentFrame)
		img.Frames[f] = frameCopy
	}

	return img, nil
}

// LoadGphFile is a convenience function to read a GPH image from the filesystem.
func LoadGphFile(path string) (*GphImage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadGph(f)
}
