package chrome

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // registers the decoder that reads a capture's size
	_ "image/png"  // registers the decoder that reads a capture's size

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// maxImageSide caps each side of a capture, in image pixels, at the largest
// image the Claude API accepts: "The maximum dimensions per image are
// 8000x8000 px" (platform.claude.com/docs/en/build-with-claude/vision).
const maxImageSide = 8000

// region is the part of the page to capture, in CSS pixels from the document
// origin, and the device pixel ratio that turns them into image pixels.
type region struct {
	X, Y, Width, Height, DPR float64
}

// The keys match region's fields, so the result decodes as is.
const regionJS = `((sel, full) => {
	const dpr = devicePixelRatio;
	if (sel) {
		const el = document.querySelector(sel);
		if (!el) return null;
		const r = el.getBoundingClientRect();
		return {X: r.left + scrollX, Y: r.top + scrollY, Width: r.width, Height: r.height, DPR: dpr};
	}
	if (full) {
		const s = document.scrollingElement || document.documentElement;
		return {X: 0, Y: 0, Width: s.scrollWidth, Height: s.scrollHeight, DPR: dpr};
	}
	const v = visualViewport;
	return {X: v.pageLeft, Y: v.pageTop, Width: v.width, Height: v.height, DPR: dpr};
})(%s, %t)`

// Screenshot captures the viewport, the full page or one element of a tab.
func (b *Browser) Screenshot(ctx context.Context, req browser.ScreenshotRequest) (browser.Image, error) {
	var img browser.Image
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		r, err := captureRegion(tctx, req)
		if err != nil {
			return err
		}
		r, truncated := r.capped()
		params := page.CaptureScreenshotParams{
			Format: page.CaptureScreenshotFormat(req.Format),
			Clip:   &page.Viewport{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height, Scale: 1},
			// Only what lies outside the viewport needs it, and it resizes
			// the page for the duration of the capture.
			CaptureBeyondViewport: new(req.FullPage || req.Selector != ""),
		}
		if req.Format == browser.JPEG {
			params.Quality = new(int64(req.Quality))
		}
		res, err := chromedp.Call(tctx, page.CaptureScreenshot, params)
		if err != nil {
			return err
		}
		img, err = decodeImage(res.Data, req.Format)
		img.Truncated = truncated
		return err
	})
	return img, err
}

func captureRegion(ctx context.Context, req browser.ScreenshotRequest) (region, error) {
	js := fmt.Sprintf(regionJS, jsonString(req.Selector), req.FullPage)
	r, err := chromedp.Run(ctx, chromedp.Evaluate[*region](js))
	switch {
	case err != nil:
		return region{}, err
	case r == nil:
		return region{}, fmt.Errorf("no element matches %q", req.Selector)
	case r.Width <= 0 || r.Height <= 0:
		return region{}, fmt.Errorf("nothing to capture: %q is %gx%g", req.Selector, r.Width, r.Height)
	}
	return *r, nil
}

// capped cuts the region so neither side of the image exceeds maxImageSide,
// and reports whether it had to.
func (r region) capped() (region, bool) {
	limit := maxImageSide / r.DPR
	cut := r.Width > limit || r.Height > limit
	r.Width = min(r.Width, limit)
	r.Height = min(r.Height, limit)
	return r, cut
}

func decodeImage(data []byte, format browser.ImageFormat) (browser.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return browser.Image{}, fmt.Errorf("read captured image: %w", err)
	}
	return browser.Image{Data: data, Format: format, Width: cfg.Width, Height: cfg.Height}, nil
}
