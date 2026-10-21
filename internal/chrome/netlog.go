package chrome

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// maxLoggedRequests bounds a tab's log over a long session; the oldest
// requests go first.
const maxLoggedRequests = 1000

// netLog records a tab's network traffic, which the browser, unlike the
// console, does not replay. Each event type arrives on its own
// subscription, so an exchange takes the events in any order.
type netLog struct {
	mu    sync.Mutex
	seq   int
	byID  map[network.RequestID]*exchange
	order []network.RequestID
}

// exchange is what the browser reports under one request ID: a request
// and its redirects, each a hop, then the response to the last hop.
type exchange struct {
	id       network.RequestID
	loader   cdp.LoaderID
	typ      network.ResourceType
	hops     []*hop
	response *network.Response
	end      cdp.MonotonicTime
	size     float64
	failure  string
	done     bool
}

type hop struct {
	seq      int
	request  *network.Request
	typ      network.ResourceType
	start    cdp.MonotonicTime
	redirect *network.Response
	end      cdp.MonotonicTime
}

// logged is a row of the log with the exchange it belongs to.
type logged struct {
	seq   int
	row   browser.Request
	id    network.RequestID
	final bool
}

func newNetLog() *netLog {
	return &netLog{byID: map[network.RequestID]*exchange{}}
}

func (l *netLog) sent(e network.EventRequestWillBeSent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.exchange(e.RequestID)
	x.loader = e.LoaderID
	// A redirect reuses the request ID, and its response comes with the
	// request it leads to.
	if e.RedirectResponse != nil && len(x.hops) > 0 {
		last := x.hops[len(x.hops)-1]
		last.redirect, last.end = e.RedirectResponse, e.Timestamp
	}
	l.seq++
	x.hops = append(x.hops, &hop{seq: l.seq, request: e.Request, typ: e.Type, start: e.Timestamp})
}

func (l *netLog) received(e network.EventResponseReceived) {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.exchange(e.RequestID)
	x.loader, x.typ, x.response = e.LoaderID, e.Type, e.Response
}

func (l *netLog) finished(e network.EventLoadingFinished) {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.exchange(e.RequestID)
	x.done, x.end, x.size = true, e.Timestamp, e.EncodedDataLength
}

func (l *netLog) failed(e network.EventLoadingFailed) {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.exchange(e.RequestID)
	x.done, x.end, x.typ, x.failure = true, e.Timestamp, e.Type, failure(e)
}

// failure says why a load failed as DevTools does: the network error, and
// what blocked it, which a CORS failure needs to be told apart.
func failure(e network.EventLoadingFailed) string {
	switch {
	case e.CorsErrorStatus != nil:
		return fmt.Sprintf("%s (CORS %s)", e.ErrorText, e.CorsErrorStatus.CorsError)
	case e.BlockedReason != "":
		return fmt.Sprintf("%s (blocked: %s)", e.ErrorText, e.BlockedReason)
	}
	return e.ErrorText
}

func (l *netLog) exchange(id network.RequestID) *exchange {
	if x, ok := l.byID[id]; ok {
		return x
	}
	x := &exchange{id: id}
	l.byID[id] = x
	l.order = append(l.order, id)
	if len(l.order) > maxLoggedRequests {
		delete(l.byID, l.order[0])
		l.order = l.order[1:]
	}
	return x
}

// requests returns the rows of the exchanges that loaders made, in the
// order they started.
func (l *netLog) requests(loaders map[cdp.LoaderID]bool) []browser.Request {
	l.mu.Lock()
	defer l.mu.Unlock()
	var rows []logged
	for _, id := range l.order {
		if x := l.byID[id]; loaders[x.loader] {
			rows = append(rows, x.rows()...)
		}
	}
	slices.SortFunc(rows, func(a, b logged) int { return cmp.Compare(a.seq, b.seq) })
	out := make([]browser.Request, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.row)
	}
	return out
}

// loaded returns the latest exchange that loaded url without failing.
func (l *netLog) loaded(url string) (network.RequestID, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, id := range slices.Backward(l.order) {
		x := l.byID[id]
		if x.done && x.failure == "" && len(x.hops) > 0 && x.hops[len(x.hops)-1].request.URL == url {
			return id, true
		}
	}
	return "", false
}

func (l *netLog) find(id string) (logged, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, x := range l.byID {
		for _, r := range x.rows() {
			if r.row.ID == id {
				return r, true
			}
		}
	}
	return logged{}, false
}

func (x *exchange) rows() []logged {
	out := make([]logged, 0, len(x.hops))
	for i, h := range x.hops {
		r := browser.Request{
			ID:             fmt.Sprintf("n%d", h.seq),
			Method:         h.request.Method,
			URL:            h.request.URL,
			Type:           strings.ToLower(string(cmp.Or(h.typ, x.typ))),
			RequestHeaders: headers(h.request.Headers),
			HasPostData:    h.request.HasPostData,
		}
		final := i == len(x.hops)-1
		switch {
		case h.redirect != nil:
			respond(&r, h.redirect)
			r.Done, r.Duration, r.Size = true, duration(h.start, h.end), int(h.redirect.EncodedDataLength)
		case final && x.done:
			r.Done, r.Duration, r.Size, r.Error = true, duration(h.start, x.end), int(x.size), x.failure
		}
		if final && x.response != nil {
			respond(&r, x.response)
		}
		out = append(out, logged{seq: h.seq, row: r, id: x.id, final: final})
	}
	return out
}

func respond(r *browser.Request, resp *network.Response) {
	r.Status, r.StatusText, r.MimeType = int(resp.Status), resp.StatusText, resp.MimeType
	r.ResponseHeaders = headers(resp.Headers)
	switch {
	case resp.FromDiskCache:
		r.Cache = "disk cache"
	case resp.FromServiceWorker:
		r.Cache = "service worker"
	case resp.FromPrefetchCache:
		r.Cache = "prefetch cache"
	}
}

func duration(start, end cdp.MonotonicTime) time.Duration {
	return time.Duration(float64(end-start) * float64(time.Second))
}

// headers sorts by name for a stable listing. The browser joins the values
// of a repeated header, such as Set-Cookie, with newlines.
func headers(h network.Headers) []browser.Header {
	out := make([]browser.Header, 0, len(h))
	for name, v := range h {
		for value := range strings.SplitSeq(fmt.Sprint(v), "\n") {
			out = append(out, browser.Header{Name: name, Value: value})
		}
	}
	slices.SortStableFunc(out, func(a, b browser.Header) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}
