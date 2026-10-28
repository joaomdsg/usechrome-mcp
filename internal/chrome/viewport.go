package chrome

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// The keys match browser.Viewport's fields, so the result decodes as is.
const viewportJS = `({Width: innerWidth, Height: innerHeight, Scale: devicePixelRatio})`

// measureViewportJS waits a frame so a read follows any pending layout.
const measureViewportJS = `new Promise(done => requestAnimationFrame(() => done(` + viewportJS + `)))`

// settleViewportJS waits until the page sees a size other than was, or 10
// frames for a change that keeps the size, as a reset with no override
// does. The renderer applies an override or reset some frames after the CDP
// call returns: read at once, the old size showed about one time in four,
// and one frame later it still showed on a loaded CI runner.
const settleViewportJS = `was => new Promise(done => {
	let frames = 0;
	const check = () => {
		const now = ` + viewportJS + `;
		if (++frames >= 10 || now.Width !== was.Width || now.Height !== was.Height || now.Scale !== was.Scale) done(now);
		else requestAnimationFrame(check);
	};
	requestAnimationFrame(check);
})`

// Viewport applies the override or reset req asks for, then measures what
// the page sees.
func (b *Browser) Viewport(ctx context.Context, req browser.ViewportRequest) (browser.Viewport, error) {
	var vp browser.Viewport
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		var err error
		if !req.Reset && !req.Overrides() {
			vp, err = chromedp.Run(tctx, chromedp.Evaluate[browser.Viewport](measureViewportJS, chromedp.EvalAwaitPromise))
		} else {
			vp, err = changeViewport(tctx, req)
		}
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

// changeViewport measures before applying req, since the page only shows
// the change some frames later and settling needs the old size to spot it.
func changeViewport(ctx context.Context, req browser.ViewportRequest) (browser.Viewport, error) {
	was, err := chromedp.Run(ctx, chromedp.Evaluate[browser.Viewport](viewportJS))
	if err == nil {
		err = applyViewport(ctx, req)
	}
	if err == nil {
		was, err = settleViewport(ctx, was)
	}
	return was, err
}

func settleViewport(ctx context.Context, was browser.Viewport) (browser.Viewport, error) {
	expr := fmt.Sprintf("(%s)({Width: %d, Height: %d, Scale: %g})", settleViewportJS, was.Width, was.Height, was.Scale)
	return chromedp.Run(ctx, chromedp.Evaluate[browser.Viewport](expr, chromedp.EvalAwaitPromise))
}
