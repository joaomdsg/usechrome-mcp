package chrome

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

func TestCappedCutsEachSideAtMaxImageSide(t *testing.T) {
	for name, tc := range map[string]struct {
		in   region
		want region
		cut  bool
	}{
		"both at limit":       {region{Width: 8000, Height: 8000, DPR: 1}, region{Width: 8000, Height: 8000, DPR: 1}, false},
		"width over":          {region{Width: 8001, Height: 10, DPR: 1}, region{Width: 8000, Height: 10, DPR: 1}, true},
		"height over":         {region{Width: 10, Height: 8001, DPR: 1}, region{Width: 10, Height: 8000, DPR: 1}, true},
		"limit scales by dpr": {region{X: 3, Y: 4, Width: 4001, Height: 4000, DPR: 2}, region{X: 3, Y: 4, Width: 4000, Height: 4000, DPR: 2}, true},
	} {
		got, cut := tc.in.capped()
		if got != tc.want || cut != tc.cut {
			t.Errorf("%s: capped() = %+v, %v; want %+v, %v", name, got, cut, tc.want, tc.cut)
		}
	}
}

func TestDecodeImageReadsSizeAndRejectsGarbage(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	img, err := decodeImage(buf.Bytes(), browser.PNG)
	if err != nil || img.Width != 3 || img.Height != 2 || img.Format != browser.PNG || !bytes.Equal(img.Data, buf.Bytes()) {
		t.Errorf("decodeImage(png 3x2) = %dx%d %v, %v; want 3x2 png, nil", img.Width, img.Height, img.Format, err)
	}
	img, err = decodeImage([]byte("not an image"), browser.PNG)
	if err == nil || img.Data != nil {
		t.Errorf("decodeImage(garbage) = %+v, %v; want zero image and an error", img, err)
	}
}
