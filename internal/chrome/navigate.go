package chrome

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Navigate moves a tab to a URL or through its history and reports the
// document it lands on.
func (b *Browser) Navigate(ctx context.Context, req browser.NavRequest) (browser.Page, error) {
	var p browser.Page
	err := b.do(ctx, req.Tab, func(tctx context.Context) error {
		if err := navigate(tctx, req); err != nil {
			return err
		}
		d, err := readDocument(tctx)
		p = browser.Page{URL: d.URL, Status: d.Status, Title: d.Title}
		return err
	})
	return p, err
}

func navigate(ctx context.Context, req browser.NavRequest) error {
	switch req.Action {
	case browser.Goto, "":
		return chromedp.Do(ctx, chromedp.Navigate(req.URL))
	case browser.Back:
		return historyStep(ctx, -1)
	case browser.Forward:
		return historyStep(ctx, 1)
	case browser.Reload:
		_, err := chromedp.RunResponse(ctx, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			_, err := cdp.Call(ctx, t, page.Reload, page.ReloadParams{})
			return err
		}))
		return err
	default:
		return fmt.Errorf("unknown navigation action %q", req.Action)
	}
}

// documentJS reads the shown document in one round trip. The status comes
// from Navigation Timing because a page restored from the back-forward
// cache fires no network events to read it from.
const documentJS = `({
	url: location.href,
	title: document.title,
	status: performance.getEntriesByType("navigation")[0]?.responseStatus ?? 0,
	complete: document.readyState === "complete",
})`

type document struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Complete bool   `json:"complete"`
}

func readDocument(ctx context.Context) (document, error) {
	return chromedp.Run(ctx, chromedp.Evaluate[document](documentJS))
}

// historyStep moves through session history and polls until the tab shows
// the target entry, because a page restored from the back-forward cache
// fires none of the load events chromedp waits for.
func historyStep(ctx context.Context, delta int64) error {
	var url string
	err := chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		e, err := historyEntry(ctx, t, delta)
		if err != nil {
			return err
		}
		url = e.URL
		_, err = cdp.Call(ctx, t, page.NavigateToHistoryEntry, page.NavigateToHistoryEntryParams{EntryID: e.ID})
		return err
	}))
	if err != nil {
		return err
	}
	return waitShowing(ctx, url)
}

func historyEntry(ctx context.Context, t *chromedp.Target, delta int64) (*page.NavigationEntry, error) {
	h, err := cdp.Call(ctx, t, page.GetNavigationHistory, cdp.Empty{})
	if err != nil {
		return nil, err
	}
	i := h.CurrentIndex + delta
	if i < 0 {
		return nil, errors.New("no page to go back to")
	}
	if i >= int64(len(h.Entries)) {
		return nil, errors.New("no page to go forward to")
	}
	return h.Entries[i], nil
}

// waitShowing matches the URL, not the history index, because the index
// moves before the new document commits.
func waitShowing(ctx context.Context, url string) error {
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		// Evaluation fails while the old document is torn down; the next tick retries.
		if d, err := readDocument(ctx); err == nil && d.URL == url && d.Complete {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
