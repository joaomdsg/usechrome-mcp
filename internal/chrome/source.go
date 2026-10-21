package chrome

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Source reads a resource of a tab's current page from the browser's cache
// of what the page loaded, as the DevTools Sources panel does.
func (b *Browser) Source(ctx context.Context, req browser.SourceRequest) (browser.Source, error) {
	var out browser.Source
	err := b.doTab(ctx, req.Tab, func(tctx context.Context, t *tab) error {
		res, err := chromedp.Call(tctx, page.GetResourceTree, cdp.Empty{})
		if err != nil {
			return err
		}
		src, frame, ok := findResource(res.FrameTree, req.URL)
		if !ok {
			return fmt.Errorf("the page loaded no resource %s; the network tool lists what it loaded", req.URL)
		}
		data, err := content(tctx, t.net, frame, src.URL)
		if err != nil {
			return err
		}
		src.Body = body(data)
		out = src
		return nil
	})
	return out, err
}

// content falls back to the response the network log has, because the
// browser keeps no content for the document of a child frame.
func content(ctx context.Context, traffic *netLog, frame cdp.FrameID, url string) ([]byte, error) {
	res, err := chromedp.Call(ctx, page.GetResourceContent, page.GetResourceContentParams{FrameID: frame, URL: url})
	if err == nil {
		return res.Content, nil
	}
	id, ok := traffic.loaded(url)
	if !ok {
		return nil, err
	}
	got, err := chromedp.Call(ctx, network.GetResponseBody, network.GetResponseBodyParams{RequestID: id})
	if err != nil {
		return nil, err
	}
	return got.Body, nil
}

// findResource looks for url among the documents and resources of the
// frames under tree. An empty url is the document of tree's frame.
func findResource(tree *page.FrameResourceTree, url string) (browser.Source, cdp.FrameID, bool) {
	if url == "" {
		url = tree.Frame.URL
	}
	frames := []*page.FrameResourceTree{tree}
	for len(frames) > 0 {
		f := frames[len(frames)-1]
		frames = append(frames[:len(frames)-1], f.ChildFrames...)
		if f.Frame.URL == url {
			return browser.Source{URL: url, MimeType: f.Frame.MimeType}, f.Frame.ID, true
		}
		for _, r := range f.Resources {
			if r.URL == url {
				return browser.Source{URL: url, MimeType: r.MimeType}, f.Frame.ID, true
			}
		}
	}
	return browser.Source{}, "", false
}
