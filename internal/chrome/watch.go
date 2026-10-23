package chrome

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// navigationWait is how long an action has to start a navigation. A link
// click requests one before the click returns, but the event reaches us
// through a queue, and handlers often navigate from a zero-delay timer.
const navigationWait = 50 * time.Millisecond

// pageWatch records what an action sets off in a tab's main frame.
type pageWatch struct {
	stop      context.CancelFunc
	requested <-chan struct{}
	committed <-chan struct{}
	sameDoc   <-chan struct{}
	stopped   <-chan struct{}

	mu      sync.Mutex
	dialogs []string
	windows []string
	created []target.ID
}

// watchPage starts recording before the action, so that no event is lost.
func watchPage(ctx context.Context) (*pageWatch, error) {
	tree, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		return nil, err
	}
	main := tree.FrameTree.Frame.ID
	wctx, stop := context.WithCancel(ctx)
	w := &pageWatch{
		stop:      stop,
		requested: signal(wctx, page.FrameRequestedNavigation, func(e page.EventFrameRequestedNavigation) bool { return e.FrameID == main }),
		committed: signal(wctx, page.FrameNavigated, func(e page.EventFrameNavigated) bool { return e.Frame.ID == main }),
		sameDoc:   signal(wctx, page.NavigatedWithinDocument, func(e page.EventNavigatedWithinDocument) bool { return e.FrameID == main }),
		stopped:   signal(wctx, page.FrameStoppedLoading, func(e page.EventFrameStoppedLoading) bool { return e.FrameID == main }),
	}
	follow(wctx, page.JavascriptDialogOpening, func(e page.EventJavascriptDialogOpening) {
		w.mu.Lock()
		w.dialogs = append(w.dialogs, fmt.Sprintf("%s: %s", e.Type, e.Message)) // trial
		w.mu.Unlock()                                                           // trial
	})
	follow(wctx, page.WindowOpen, func(e page.EventWindowOpen) {
		w.mu.Lock()
		w.windows = append(w.windows, e.URL) // trial
		w.mu.Unlock()                        // trial
	})
	opener := chromedp.FromContext(ctx).Target.TargetID
	followBrowser(wctx, target.TargetCreated, func(e target.EventTargetCreated) {
		if e.TargetInfo.OpenerID == opener && e.TargetInfo.Type == "page" { // trial
			w.mu.Lock()
			w.created = append(w.created, e.TargetInfo.TargetID) // trial
			w.mu.Unlock()                                        // trial
		}
	})
	return w, nil
}

// settle waits until a navigation the action started, if any, has loaded.
func (w *pageWatch) settle(ctx context.Context) error {
	select {
	case <-w.requested:
	case <-w.sameDoc:
		return nil
	case <-time.After(navigationWait):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-w.stopped:
	case <-w.sameDoc:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (w *pageWatch) navigated() bool {
	return fired(w.committed) || fired(w.sameDoc)
}

func (w *pageWatch) opened() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dialogs
}

// popups returns the tabs the page opened and the URL it asked each to
// load. The browser creates a tab before it knows the URL, so the two come
// from separate events, which it sends in the same order.
func (w *pageWatch) popups() ([]target.ID, []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.created, w.windows
}

// signal subscribes to ev at once and returns a channel that closes on the
// first event match accepts.
func signal[E any](ctx context.Context, ev cdp.Event[E], match func(E) bool) <-chan struct{} {
	events := chromedp.Events(ctx, ev)
	ch := make(chan struct{})
	go func() {
		for e, err := range events {
			if err != nil {
				return
			}
			if match(e) {
				close(ch)
				return
			}
		}
	}()
	return ch
}

// follow subscribes to ev at once and calls fn with each event until ctx
// ends.
func follow[E any](ctx context.Context, ev cdp.Event[E], fn func(E)) {
	each(chromedp.Events(ctx, ev), fn)
}

// followBrowser is follow for the events of the browser.
func followBrowser[E any](ctx context.Context, ev cdp.Event[E], fn func(E)) {
	each(chromedp.BrowserEvents(ctx, ev), fn)
}

func each[E any](events iter.Seq2[E, error], fn func(E)) {
	go func() {
		for e, err := range events {
			if err != nil {
				return
			}
			fn(e)
		}
	}()
}

func fired(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// acceptDialogs answers every JavaScript dialog in a tab as a person
// clicking OK would, because an open dialog blocks the page and every call
// that waits on it.
func acceptDialogs(ctx context.Context) {
	follow(ctx, page.JavascriptDialogOpening, func(e page.EventJavascriptDialogOpening) {
		_, _ = chromedp.Call(ctx, page.HandleJavaScriptDialog, page.HandleJavaScriptDialogParams{Accept: true, PromptText: e.DefaultPrompt})
	})
}
