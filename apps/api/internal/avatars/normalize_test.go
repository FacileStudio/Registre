package avatars

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"testing"
)

// encodePNG draws a blank width by height PNG.
func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encoding the fixture: %v", err)
	}
	return out.Bytes()
}

// TestNormalizeCropsAndStripsTrailingBytes proves the two properties the
// stored picture relies on: every app gets a square, and nothing appended to
// the upload survives into what registre serves.
func TestNormalizeCropsAndStripsTrailingBytes(t *testing.T) {
	payload := []byte("<script>alert(1)</script>")
	out, err := Normalize(append(encodePNG(t, 40, 20), payload...))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if bytes.Contains(out, payload) {
		t.Error("the trailing payload survived re-encoding")
	}
	header, err := png.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("the output is not a PNG: %v", err)
	}
	if header.Width != 20 || header.Height != 20 {
		t.Errorf("output is %dx%d, want the centred 20x20 square", header.Width, header.Height)
	}
}

// TestNormalizeRefusesWhatIsNotAPicture covers the refusals a public upload
// route must make before it decodes a single pixel.
func TestNormalizeRefusesWhatIsNotAPicture(t *testing.T) {
	cases := map[string][]byte{
		"empty":     nil,
		"text":      []byte("definitely not a picture"),
		"svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		"too wide":  encodePNG(t, MaxSide+1, MinSide),
		"too small": encodePNG(t, MinSide-1, MinSide-1),
		"oversized": make([]byte, MaxUploadBytes+1),
	}
	for name, upload := range cases {
		if _, err := Normalize(upload); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
