package chrome_test

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

// tallPage is 3000px of white with a 100x50 red box at (20, 2500), so the
// box shows only in full-page and element captures.
func tallPage(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><style>body{margin:0;background:#fff}</style>
<div style="height:3000px;position:relative">
<div id="box" style="position:absolute;left:20px;top:2500px;width:100px;height:50px;background:#f00"></div>
</div>`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func shoot(t *testing.T, b *chrome.Browser, req browser.ScreenshotRequest) browser.Image {
	t.Helper()
	img, err := b.Screenshot(t.Context(), req)
	if err != nil {
		t.Fatalf("screenshot %+v: %v", req, err)
	}
	return img
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func isRed(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r > 0xf000 && g < 0x1000 && b < 0x1000
}

func TestScreenshotCapturesViewport(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	viewport(t, b, browser.ViewportRequest{Width: 400, Height: 300, Scale: 1})
	nav(t, b, browser.NavRequest{URL: tallPage(t)})
	got := shoot(t, b, browser.ScreenshotRequest{Format: browser.PNG})
	if got.Width != 400 || got.Height != 300 || got.Truncated {
		t.Errorf("image %dx%d truncated=%v, want 400x300 whole", got.Width, got.Height, got.Truncated)
	}
	if b := decodePNG(t, got.Data).Bounds(); b.Dx() != 400 || b.Dy() != 300 {
		t.Errorf("decoded size %v, want 400x300", b)
	}
}

func TestScreenshotFullPageReachesBelowTheFold(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	viewport(t, b, browser.ViewportRequest{Width: 400, Height: 300, Scale: 1})
	nav(t, b, browser.NavRequest{URL: tallPage(t)})
	got := shoot(t, b, browser.ScreenshotRequest{FullPage: true, Format: browser.PNG})
	if got.Width != 400 || got.Height != 3000 {
		t.Fatalf("image %dx%d, want 400x3000", got.Width, got.Height)
	}
	if !isRed(decodePNG(t, got.Data), 70, 2525) {
		t.Error("full page misses the red box at y=2500")
	}
}

func TestScreenshotElementCapturesOnlyThatElement(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	viewport(t, b, browser.ViewportRequest{Width: 400, Height: 300, Scale: 2})
	nav(t, b, browser.NavRequest{URL: tallPage(t)})
	got := shoot(t, b, browser.ScreenshotRequest{Selector: "#box", Format: browser.PNG})
	if got.Width != 200 || got.Height != 100 {
		t.Fatalf("image %dx%d, want the 100x50 box at scale 2", got.Width, got.Height)
	}
	img := decodePNG(t, got.Data)
	if !isRed(img, 0, 0) || !isRed(img, 199, 99) {
		t.Error("element capture is not all red at its corners")
	}
}

func TestScreenshotTruncatesPastImageLimit(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	// 3000 CSS px at scale 3 would be a 9000px image.
	viewport(t, b, browser.ViewportRequest{Width: 200, Height: 300, Scale: 3})
	nav(t, b, browser.NavRequest{URL: tallPage(t)})
	got := shoot(t, b, browser.ScreenshotRequest{FullPage: true, Format: browser.PNG})
	// The cap is 2666.67 CSS px, which Chrome renders whole: up to 3px short.
	if got.Height > 8000 || got.Height < 7997 || got.Width != 600 || !got.Truncated {
		t.Errorf("image %dx%d truncated=%v, want 600 wide, 7997-8000 tall, truncated", got.Width, got.Height, got.Truncated)
	}
}

func TestScreenshotEncodesJPEG(t *testing.T) {
	t.Parallel()
	got := shoot(t, newBrowser(t), browser.ScreenshotRequest{Format: browser.JPEG, Quality: 50})
	if got.Format != browser.JPEG || !bytes.HasPrefix(got.Data, []byte{0xff, 0xd8}) {
		t.Errorf("format %s, first bytes % x; want JPEG", got.Format, got.Data[:2])
	}
}

func TestScreenshotElementErrors(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: tallPage(t)})
	for sel, want := range map[string]string{
		"#missing":     "no element matches",
		"[[bad":        "not a valid selector",
		"#box > span:": "not a valid selector",
	} {
		_, err := b.Screenshot(t.Context(), browser.ScreenshotRequest{Selector: sel, Format: browser.PNG})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("selector %q: err = %v, want %q", sel, err, want)
		}
	}
}

func TestScreenshotEmptyElementFails(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: tallPage(t)})
	if _, err := b.Eval(t.Context(), browser.EvalRequest{Expression: "document.body.append(Object.assign(document.createElement('i'), {id: 'empty'}))"}); err != nil {
		t.Fatal(err)
	}
	_, err := b.Screenshot(t.Context(), browser.ScreenshotRequest{Selector: "#empty", Format: browser.PNG})
	if err == nil || !strings.Contains(err.Error(), "nothing to capture") {
		t.Errorf("err = %v, want nothing to capture", err)
	}
}
