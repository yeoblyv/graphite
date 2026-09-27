package Graphite

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

// GphMagic is the file signature for version 1 of the GPH format (no alpha
// channel — every pixel renders fully opaque). Still recognized for reading;
// WriteGph no longer produces it.
var GphMagic = []byte{'G', 'P', 'H', '\x01'}

// GphMagicV2 is the file signature for version 2 of the GPH format, which
// adds GphPixel.Alpha. WriteGph always emits this now; ReadGph accepts both.
var GphMagicV2 = []byte{'G', 'P', 'H', '\x02'}

// PlaybackMode defines how animation frames are played.
type PlaybackMode uint8

const (
	PlaybackStatic    PlaybackMode = 0
	PlaybackLoop      PlaybackMode = 1
	PlaybackBoomerang PlaybackMode = 2
	PlaybackOnce      PlaybackMode = 3
)

// GphPixel represents a single "pixel" in a console pseudographics image.
//
// Alpha follows PNG convention (0 = fully transparent, 255 = fully opaque)
// and blends this pixel's Bg/Fg into whatever is already on the canvas
// underneath (see Image.DrawRelative) — the same role a PNG's alpha channel
// plays, rather than the all-or-nothing transparency Bg==ColorNone alone
// provides. Its zero value is 0 (fully transparent), NOT 255: a GphPixel{}
// literal that doesn't set Alpha is invisible. This is a deliberate
// (documented in CHANGELOG.md), source-breaking choice for any code
// constructing pixels directly rather than through ReadGph, which always
// sets Alpha explicitly (255 for every pre-alpha V1 file, so those keep
// rendering exactly as before).
type GphPixel struct {
	Bg    Color
	Fg    Color
	Level uint8 // 0=0%, 1=25%, 2=50%, 3=75%, 4=100% density
	Alpha uint8 // 0=transparent, 255=opaque; see the field's doc comment above
}

// GphImage contains the dimensions, playback metadata, and frames of a GPH file.
type GphImage struct {
	Width   int
	Height  int
	Mode    PlaybackMode
	DelayMs uint16
	Frames  [][]GphPixel
}

// gphPixelSize returns the on-disk byte size of one pixel: 9 bytes
// (Bg+Fg+Level) for V1, 10 bytes (+Alpha) for V2.
func gphPixelSize(hasAlpha bool) int {
	if hasAlpha {
		return 10
	}
	return 9
}

// encodeGphPixel writes p into dst (which must be at least
// gphPixelSize(hasAlpha) bytes long), including Alpha only when hasAlpha.
func encodeGphPixel(dst []byte, p GphPixel, hasAlpha bool) {
	binary.LittleEndian.PutUint32(dst[0:4], uint32(p.Bg))
	binary.LittleEndian.PutUint32(dst[4:8], uint32(p.Fg))
	dst[8] = p.Level
	if hasAlpha {
		dst[9] = p.Alpha
	}
}

// decodeGphPixel reads a pixel from src (which must be at least
// gphPixelSize(hasAlpha) bytes long). A V1 pixel (hasAlpha=false) always
// decodes as fully opaque (Alpha=255): V1 files predate the alpha channel
// and were always rendered fully opaque, so this keeps them looking
// identical after the format gained one.
func decodeGphPixel(src []byte, hasAlpha bool) GphPixel {
	p := GphPixel{
		Bg:    Color(int32(binary.LittleEndian.Uint32(src[0:4]))),
		Fg:    Color(int32(binary.LittleEndian.Uint32(src[4:8]))),
		Level: src[8],
		Alpha: 255,
	}
	if hasAlpha {
		p.Alpha = src[9]
	}
	return p
}

// WriteGph serializes the image into the GPH binary format (always V2, with
// per-pixel Alpha). It uses bitmask-based delta encoding for frames after
// the first one.
func WriteGph(w io.Writer, img *GphImage) error {
	if img.Width < 0 || img.Height < 0 {
		return errors.New("Graphite.WriteGph: invalid image dimensions")
	}

	if _, err := w.Write(GphMagicV2); err != nil {
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

	const hasAlpha = true
	psize := gphPixelSize(hasAlpha)
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
		pixelData := make([]byte, len(changed)*psize)
		for i, p := range changed {
			encodeGphPixel(pixelData[i*psize:(i+1)*psize], p, hasAlpha)
		}
		if _, err := w.Write(pixelData); err != nil {
			return err
		}

		prevFrame = frame
	}

	return nil
}

// ReadGph parses a GPH binary stream (V1 or V2) with delta decoding.
func ReadGph(r io.Reader) (*GphImage, error) {
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("Graphite.ReadGph: failed to read magic: %w", err)
	}
	if magic[0] != 'G' || magic[1] != 'P' || magic[2] != 'H' {
		return nil, fmt.Errorf("Graphite.ReadGph: invalid or unsupported file format magic: %q", magic)
	}
	var hasAlpha bool
	switch magic[3] {
	case GphMagic[3]:
		hasAlpha = false
	case GphMagicV2[3]:
		hasAlpha = true
	default:
		return nil, fmt.Errorf("Graphite.ReadGph: invalid or unsupported file format magic: %q", magic)
	}
	psize := gphPixelSize(hasAlpha)

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
		pixelData := make([]byte, pixelsPerFrame*psize)
		// We already consumed `n` bytes of pixel data by accident!
		// Let's copy the `n` bytes we read into `pixelData` and read the rest.
		copy(pixelData, header[8:8+n])
		if _, err := io.ReadFull(r, pixelData[n:]); err != nil {
			return nil, fmt.Errorf("Graphite.ReadGph: failed to read legacy pixel data: %w", err)
		}

		for i := 0; i < pixelsPerFrame; i++ {
			img.Frames[0][i] = decodeGphPixel(pixelData[i*psize:(i+1)*psize], hasAlpha)
		}
		return img, nil
	}

	// Normal reading for animated/delta
	img.Mode = PlaybackMode(header[8])
	img.DelayMs = binary.LittleEndian.Uint16(header[9:11])
	frameCount := binary.LittleEndian.Uint32(header[11:15])

	img.Frames = make([][]GphPixel, frameCount)
	if pixelsPerFrame == 0 || frameCount == 0 {
		return img, nil
	}

	bitmaskLen := (pixelsPerFrame + 7) / 8
	// currentFrame starts zero-valued; WriteGph's delta encoder always marks
	// every pixel of frame 0 as "changed" (prevFrame is nil then), so every
	// entry gets explicitly decoded before frameCopy ever reads it.
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

		pixelData := make([]byte, changedCount*psize)
		if _, err := io.ReadFull(r, pixelData); err != nil {
			return nil, fmt.Errorf("Graphite.ReadGph: failed to read pixels for frame %d: %w", f, err)
		}

		pixelIdx := 0
		for i := 0; i < pixelsPerFrame; i++ {
			if (bitmask[i/8] & (1 << (i % 8))) != 0 {
				currentFrame[i] = decodeGphPixel(pixelData[pixelIdx*psize:(pixelIdx+1)*psize], hasAlpha)
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
