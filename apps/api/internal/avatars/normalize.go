package avatars

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
)

// MaxUploadBytes is the largest upload Normalize reads. A browser-cropped
// 256-pixel square is a few tens of kilobytes; the ceiling is for a phone
// photo sent as is, and stays under porte's 5 MiB fetch limit once encoded.
const MaxUploadBytes = 4 << 20

// MinSide and MaxSide bound each dimension of an upload. The maximum is
// checked from the header before any pixel is decoded, so a small file that
// declares a huge canvas is refused before it can allocate one.
const (
	MinSide = 16
	MaxSide = 2048
)

// ErrInvalid wraps every refusal of an upload. Its message is safe to show the
// person who sent the file: it describes their own picture and nothing else.
var ErrInvalid = errors.New("avatars")

// accepted names the formats an upload may arrive in. The list is explicit
// rather than "whatever image has registered", because a dependency that
// registers a decoder must not widen what a public endpoint parses.
var accepted = map[string]bool{"png": true, "jpeg": true, "gif": true}

// subImager is the method every standard decoder's result carries.
type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

// Normalize turns an upload into the PNG registre stores: decoded, cropped to
// the centred square and re-encoded.
//
// Re-encoding is the security property, not a format preference. It drops
// EXIF, and with it the GPS position a phone photo carries; and it drops
// anything appended after the image data, so a file that is both a picture and
// something else stops being the something else. A GIF keeps its first frame.
func Normalize(upload []byte) ([]byte, error) {
	if len(upload) == 0 || len(upload) > MaxUploadBytes {
		return nil, fmt.Errorf("%w: the picture must be at most %d MiB", ErrInvalid, MaxUploadBytes>>20)
	}
	header, format, err := image.DecodeConfig(bytes.NewReader(upload))
	if err != nil || !accepted[format] {
		return nil, fmt.Errorf("%w: the picture must be a PNG, JPEG or GIF", ErrInvalid)
	}
	if !withinBounds(header.Width) || !withinBounds(header.Height) {
		return nil, fmt.Errorf("%w: each side must be between %d and %d pixels", ErrInvalid, MinSide, MaxSide)
	}
	decoded, _, err := image.Decode(bytes.NewReader(upload))
	if err != nil {
		return nil, fmt.Errorf("%w: the picture could not be read", ErrInvalid)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, square(decoded)); err != nil {
		return nil, fmt.Errorf("avatars: encode the picture: %w", err)
	}
	return out.Bytes(), nil
}

// withinBounds reports whether one side of an upload is an acceptable size.
func withinBounds(side int) bool { return side >= MinSide && side <= MaxSide }

// square crops an image to its centred square, so every app receives the same
// shape whatever the uploader's client did. An image whose type cannot be
// cropped is returned whole.
func square(source image.Image) image.Image {
	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	origin := bounds.Min.Add(image.Pt((bounds.Dx()-side)/2, (bounds.Dy()-side)/2))
	cropper, ok := source.(subImager)
	if !ok {
		return source
	}
	return cropper.SubImage(image.Rectangle{Min: origin, Max: origin.Add(image.Pt(side, side))})
}
