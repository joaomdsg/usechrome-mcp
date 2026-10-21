package chrome

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Tabs carries out req, then lists the browser's tabs, oldest first. Pages
// and people open and close tabs too, so it first adopts the tabs it has
// not seen and forgets the ones that are gone.
func (b *Browser) Tabs(ctx context.Context, req browser.TabsRequest) ([]browser.Tab, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.start(); err != nil {
		return nil, err
	}
	bctx, cancel := b.bound(ctx, b.first)
	defer cancel()
	tabs, err := b.tabAction(bctx, req)
	return tabs, b.timedOut(ctx, err)
}

func (b *Browser) tabAction(ctx context.Context, req browser.TabsRequest) ([]browser.Tab, error) {
	if err := b.sync(ctx); err != nil {
		return nil, err
	}
	var err error
	switch req.Action {
	case browser.ListTabs, "":
	case browser.OpenTab:
		err = b.openTab(ctx, req.URL)
	case browser.SelectTab:
		err = b.selectTab(ctx, req.Tab)
	case browser.CloseTab:
		err = b.closeTab(ctx, req.Tab)
	default:
		err = fmt.Errorf("unknown tab action %q", req.Action)
	}
	if err != nil {
		return nil, err
	}
	return b.listTabs(ctx)
}

func pages(ctx context.Context) ([]*target.Info, error) {
	infos, err := chromedp.Targets(ctx)
	return slices.DeleteFunc(infos, func(i *target.Info) bool { return i.Type != "page" }), err
}

func (b *Browser) sync(ctx context.Context) error {
	open, err := pages(ctx)
	if err != nil {
		return err
	}
	known := map[target.ID]bool{}
	for _, t := range b.tabs {
		if !slices.ContainsFunc(open, func(i *target.Info) bool { return i.TargetID == t.target }) {
			b.forget(t)
		}
		known[t.target] = true
	}
	for _, i := range open {
		if !known[i.TargetID] && !b.closed[i.TargetID] {
			// A tab can close before the attach; the next listing settles it.
			_, _ = b.adopt(ctx, i.TargetID)
		}
	}
	return nil
}

// forget drops a tab whose page is gone and releases its context, except
// the first tab's, which holds the browser.
func (b *Browser) forget(t *tab) {
	if t.ctx != b.first {
		t.cancel()
	}
	delete(b.tabs, t.id)
	if b.current == t.id {
		b.current = b.newest()
	}
}

func (b *Browser) newest() string {
	var last *tab
	for _, t := range b.tabs {
		if last == nil || t.seq > last.seq {
			last = t
		}
	}
	if last == nil {
		return ""
	}
	return last.id
}

func (b *Browser) adopt(ctx context.Context, id target.ID) (*tab, error) {
	tctx, cancel := chromedp.NewContext(b.first, chromedp.WithTargetID(id))
	if err := attach(ctx, tctx, cancel); err != nil {
		return nil, err
	}
	return b.addTab(tctx, cancel), nil
}

// adoptOpened adopts the tabs an action opened, so that the agent can
// select them by id. A tab that closed at once is left out.
func (b *Browser) adoptOpened(ctx context.Context, w *pageWatch) []browser.Tab {
	ids, urls := w.popups()
	var out []browser.Tab
	for i, id := range ids {
		t, err := b.adopt(ctx, id)
		if err != nil {
			continue
		}
		opened := browser.Tab{ID: t.id}
		if i < len(urls) {
			opened.URL = urls[i]
		}
		out = append(out, opened)
	}
	return out
}

// attach runs tctx's first action, which attaches it to its tab, within
// ctx. Running it under ctx would tie the tab's life to ctx, so ctx only
// cancels it.
func attach(ctx, tctx context.Context, cancel context.CancelFunc) error {
	stop := context.AfterFunc(ctx, cancel)
	err := chromedp.Do(tctx)
	if !stop() && err == nil {
		err = ctx.Err()
	}
	if err != nil {
		cancel()
	}
	return err
}

func (b *Browser) openTab(ctx context.Context, url string) error {
	tctx, cancel := chromedp.NewContext(b.first)
	if err := attach(ctx, tctx, cancel); err != nil {
		return fmt.Errorf("open a tab: %w", err)
	}
	t := b.addTab(tctx, cancel)
	b.current = t.id
	if url == "" {
		return nil
	}
	nctx, stop := b.bound(ctx, t.alive)
	defer stop()
	if err := chromedp.Do(nctx, chromedp.Navigate(url)); err != nil {
		return fmt.Errorf("opened %s, but it could not load %s: %w", t.id, url, err)
	}
	return nil
}

// selectTab brings the tab to the front too, since a browser slows the
// timers and stops the animation frames of a tab in the background.
func (b *Browser) selectTab(ctx context.Context, id string) error {
	t, err := b.tab(id)
	if err != nil {
		return err
	}
	sctx, stop := b.bound(ctx, t.alive)
	defer stop()
	if _, err := chromedp.Call(sctx, page.BringToFront, cdp.Empty{}); err != nil {
		return err
	}
	b.current = t.id
	return nil
}

// closeTab refuses the last tab, because a browser closes with it.
func (b *Browser) closeTab(ctx context.Context, id string) error {
	t, err := b.tab(id)
	if err != nil {
		return err
	}
	if len(b.tabs) == 1 {
		return fmt.Errorf("%s is the last tab; navigate it instead of closing it", t.id)
	}
	b.closed[t.target] = true
	if t.ctx == b.first {
		_, err = cdp.Call(ctx, chromedp.FromContext(ctx).Browser, target.CloseTarget, target.CloseTargetParams{TargetID: t.target})
	} else {
		err = chromedp.Cancel(t.ctx)
	}
	if err != nil {
		return fmt.Errorf("close %s: %w", t.id, err)
	}
	b.forget(t)
	return nil
}

func (b *Browser) listTabs(ctx context.Context) ([]browser.Tab, error) {
	open, err := pages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]browser.Tab, 0, len(b.tabs))
	for _, t := range b.tabs {
		tab := browser.Tab{ID: t.id, Current: t.id == b.current}
		if i := slices.IndexFunc(open, func(i *target.Info) bool { return i.TargetID == t.target }); i >= 0 {
			tab.Title, tab.URL = open[i].Title, open[i].URL
		}
		out = append(out, tab)
	}
	slices.SortFunc(out, func(x, y browser.Tab) int { return cmp.Compare(b.tabs[x.ID].seq, b.tabs[y.ID].seq) })
	return out, nil
}
