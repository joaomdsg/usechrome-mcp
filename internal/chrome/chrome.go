// Package chrome drives a Chrome or Chromium browser through chromedp and
// exposes it in terms of the browser package.
package chrome

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

const defaultTimeout = 30 * time.Second

// Options controls how the browser is launched or attached.
type Options struct {
	// ExecPath is the browser binary to launch. Empty means search PATH.
	ExecPath string
	// CDPURL attaches to a running browser instead of launching one.
	CDPURL string
	// Headed shows a browser window instead of running headless.
	Headed bool
	// NoSandbox disables the Chromium sandbox, which containers and CI
	// runners without user namespaces need.
	NoSandbox bool
	// Timeout bounds each browser operation. Zero means 30 seconds.
	Timeout time.Duration
}

// Browser is a lazily started browser whose tabs the MCP tools drive. It is
// safe for concurrent use; operations run one at a time.
type Browser struct {
	root context.Context
	opts Options

	mu      sync.Mutex
	stop    context.CancelFunc
	first   context.Context
	tabs    map[string]*tab
	current string
	seq     int
}

type tab struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
}

// New returns a Browser that starts on its first operation and lives until
// Close or until root is cancelled.
func New(root context.Context, opts Options) *Browser {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	return &Browser{root: root, opts: opts}
}

// Close shuts down the browser, or detaches from it when attached by URL.
func (b *Browser) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tabs == nil {
		return nil
	}
	err := chromedp.Cancel(b.first)
	b.stop()
	b.tabs, b.first, b.stop = nil, nil, nil
	return err
}

func (b *Browser) start() error {
	if b.tabs != nil {
		return nil
	}
	alloc, stopAlloc := b.allocator()
	tctx, cancel := chromedp.NewContext(alloc)
	if err := chromedp.Do(tctx); err != nil {
		cancel()
		stopAlloc()
		return fmt.Errorf("start browser: %w", err)
	}
	b.stop, b.first = stopAlloc, tctx
	b.tabs = map[string]*tab{}
	b.addTab(tctx, cancel)
	return nil
}

func (b *Browser) allocator() (context.Context, context.CancelFunc) {
	if b.opts.CDPURL != "" {
		return remote.NewAllocator(b.root, b.opts.CDPURL)
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if b.opts.Headed {
		opts = append(opts, chromedp.VisibleWindow)
	}
	if b.opts.NoSandbox {
		opts = append(opts, chromedp.NoSandbox)
	}
	if b.opts.ExecPath != "" {
		opts = append(opts, chromedp.ExecPath(b.opts.ExecPath))
	}
	return chromedp.NewExecAllocator(b.root, opts...)
}

func (b *Browser) addTab(ctx context.Context, cancel context.CancelFunc) *tab {
	b.seq++
	t := &tab{id: fmt.Sprintf("t%d", b.seq), ctx: ctx, cancel: cancel}
	b.tabs[t.id] = t
	b.current = t.id
	return t
}

// do runs fn against a tab, bounded by both the request context and the
// operation timeout. chromedp needs the tab's own context, so the request
// context can only cancel it, not replace it.
func (b *Browser) do(ctx context.Context, tabID string, fn func(context.Context) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.start(); err != nil {
		return err
	}
	if tabID == "" {
		tabID = b.current
	}
	t, ok := b.tabs[tabID]
	if !ok {
		return fmt.Errorf("unknown tab %q", tabID)
	}
	octx, cancel := context.WithTimeout(t.ctx, b.opts.Timeout)
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()
	err := fn(octx)
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return fmt.Errorf("timed out after %s: %w", b.opts.Timeout, err)
	}
	return err
}

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
