// Package chrome drives a Chrome or Chromium browser through chromedp and
// exposes it in terms of the browser package.
package chrome

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
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
	// closed holds the tabs this server closed: the browser answers a close
	// before the tab is gone, and a listing in between must not adopt it.
	closed map[target.ID]bool
}

type tab struct {
	id     string
	seq    int
	target target.ID
	ctx    context.Context
	cancel context.CancelFunc
	// alive is ctx until the tab closes. chromedp drops a closed tab's
	// session without answering its pending calls, so operations run in
	// alive to fail at once instead of waiting out the timeout.
	alive context.Context
	net   *netLog
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
	b.tabs, b.closed = map[string]*tab{}, map[target.ID]bool{}
	b.current = b.addTab(tctx, cancel).id
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

// addTab starts recording a tab that ctx is attached to.
func (b *Browser) addTab(ctx context.Context, cancel context.CancelFunc) *tab {
	b.seq++
	t := &tab{
		id: fmt.Sprintf("t%d", b.seq), seq: b.seq, target: chromedp.FromContext(ctx).Target.TargetID,
		ctx: ctx, cancel: cancel, net: newNetLog(),
	}
	alive, gone := context.WithCancelCause(ctx)
	t.alive = alive
	followBrowser(alive, target.TargetDestroyed, func(e target.EventTargetDestroyed) {
		if e.TargetID == t.target {
			gone(fmt.Errorf("tab %s closed; the tabs tool lists the open ones", t.id))
		}
	})
	acceptDialogs(ctx)
	follow(ctx, network.RequestWillBeSent, t.net.sent)
	follow(ctx, network.ResponseReceived, t.net.received)
	follow(ctx, network.LoadingFinished, t.net.finished)
	follow(ctx, network.LoadingFailed, t.net.failed)
	b.tabs[t.id] = t
	return t
}

// do runs fn against a tab, bounded by both the request context and the
// operation timeout.
func (b *Browser) do(ctx context.Context, tabID string, fn func(context.Context) error) error {
	return b.doTab(ctx, tabID, func(tctx context.Context, _ *tab) error { return fn(tctx) })
}

// doTab is do for an fn that needs the tab itself.
func (b *Browser) doTab(ctx context.Context, tabID string, fn func(context.Context, *tab) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.start(); err != nil {
		return err
	}
	t, err := b.tab(tabID)
	if err != nil {
		return err
	}
	octx, cancel := b.bound(ctx, t.alive)
	defer cancel()
	err = fn(octx, t)
	if cause := context.Cause(t.alive); err != nil && t.alive.Err() != nil && t.ctx.Err() == nil {
		return cause
	}
	return b.timedOut(ctx, err)
}

// tab finds a tab by id, the current one for an empty id.
func (b *Browser) tab(id string) (*tab, error) {
	id = cmp.Or(id, b.current)
	t, ok := b.tabs[id]
	if !ok {
		return nil, fmt.Errorf("unknown tab %q; the tabs tool lists the open ones", id)
	}
	return t, nil
}

// bound derives from cctx, a chromedp context, one that also ends with ctx
// or after the operation timeout. chromedp needs its own context, so the
// request context can only cancel it, not replace it.
func (b *Browser) bound(ctx, cctx context.Context) (context.Context, context.CancelFunc) {
	octx, cancel := context.WithTimeout(cctx, b.opts.Timeout)
	stop := context.AfterFunc(ctx, cancel)
	return octx, func() {
		stop()
		cancel()
	}
}

// timedOut says that err came from the operation timeout, unless the request
// itself ended.
func (b *Browser) timedOut(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return fmt.Errorf("timed out after %s: %w", b.opts.Timeout, err)
	}
	return err
}
