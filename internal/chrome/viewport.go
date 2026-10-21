package chrome

import (
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// The keys match browser.Viewport's fields, so the result decodes as is. It
// waits a frame because the renderer applies an override or reset after the
// CDP call returns; read at once, the old size shows about one time in four.
const measureViewportJS = `new Promise(done => requestAnimationFrame(() =>
	done({Width: innerWidth, Height: innerHeight, Scale: devicePixelRatio})))`

// Viewport applies the override or reset req asks for, then measures what
// the page sees.
func (b *Browser) Viewport(ctx context.Context, req browser.ViewportRequest) (browser.Viewport, error) {
	var vp browser.Viewport
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		if err := applyViewport(tctx, req); err != nil {
			return err
		}
		var err error
		vp, err = chromedp.Run(tctx, chromedp.Evaluate[browser.Viewport](measureViewportJS, chromedp.EvalAwaitPromise))
		return err
	})
	return vp, err
}

// applyViewport pairs mobile with touch, as DevTools device mode does, so
// pages see touch points and match (pointer: coarse) and (hover: none).
func applyViewport(ctx context.Context, req browser.ViewportRequest) error {
	switch {
	case req.Reset:
		if _, err := chromedp.Call(ctx, emulation.ClearDeviceMetricsOverride, cdp.Empty{}); err != nil {
			return err
		}
		return setTouch(ctx, false)
	case req.Overrides():
		if _, err := chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
			Width:             int64(req.Width),
			Height:            int64(req.Height),
			DeviceScaleFactor: req.Scale,
			Mobile:            req.Mobile,
		}); err != nil {
			return err
		}
		return setTouch(ctx, req.Mobile)
	}
	return nil
}

func setTouch(ctx context.Context, on bool) error {
	_, err := chromedp.Call(ctx, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: on})
	return err
}
