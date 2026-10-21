package chrome

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Network lists the requests of a tab's current page, as the DevTools
// Network panel shows them. A request belongs to the page when one of the
// page's documents, its frames' included, loaded it.
func (b *Browser) Network(ctx context.Context, req browser.NetworkRequest) ([]browser.Request, error) {
	var out []browser.Request
	err := b.doTab(ctx, req.Tab, func(tctx context.Context, t *tab) error {
		tree, err := chromedp.Call(tctx, page.GetFrameTree, cdp.Empty{})
		if err != nil {
			return err
		}
		loaders := map[cdp.LoaderID]bool{}
		frames := []*page.FrameTree{tree.FrameTree}
		for len(frames) > 0 {
			f := frames[len(frames)-1]
			frames = append(frames[:len(frames)-1], f.ChildFrames...)
			loaders[f.Frame.LoaderID] = true
		}
		out = t.net.requests(loaders)
		return nil
	})
	return out, err
}

// RequestDetail returns a request that a tab made, with its bodies.
func (b *Browser) RequestDetail(ctx context.Context, req browser.RequestDetailRequest) (browser.RequestDetail, error) {
	var out browser.RequestDetail
	err := b.doTab(ctx, req.Tab, func(tctx context.Context, t *tab) error {
		r, ok := t.net.find(req.ID)
		if !ok {
			return fmt.Errorf("no request %q in this tab; the request list gives the ids", req.ID)
		}
		out.Request = r.row
		if r.row.HasPostData {
			out.RequestBody = postData(tctx, r.id)
		}
		out.ResponseBody = responseBody(tctx, r)
		return nil
	})
	return out, err
}

func postData(ctx context.Context, id network.RequestID) browser.Body {
	res, err := chromedp.Call(ctx, network.GetRequestPostData, network.GetRequestPostDataParams{RequestID: id})
	if err != nil {
		return browser.Body{Missing: err.Error()}
	}
	return body(res.PostData)
}

func responseBody(ctx context.Context, r logged) browser.Body {
	switch {
	case !r.final:
		return browser.Body{Missing: "a redirect has none"}
	case !r.row.Done:
		return browser.Body{Missing: "still loading"}
	case r.row.Error != "":
		return browser.Body{Missing: "the request failed"}
	}
	res, err := chromedp.Call(ctx, network.GetResponseBody, network.GetResponseBodyParams{RequestID: r.id})
	if err != nil {
		return browser.Body{Missing: err.Error()}
	}
	return body(res.Body)
}

func body(data []byte) browser.Body {
	if !utf8.Valid(data) {
		return browser.Body{Size: len(data), Binary: true}
	}
	return browser.Body{Text: string(data), Size: len(data)}
}
