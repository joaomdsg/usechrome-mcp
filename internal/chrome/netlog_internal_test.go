package chrome

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

const netlogLoader cdp.LoaderID = "L1"

func netlogSent(l *netLog, id network.RequestID, url string, at cdp.MonotonicTime, redirect *network.Response) {
	l.sent(network.EventRequestWillBeSent{
		RequestID:        id,
		LoaderID:         netlogLoader,
		Request:          &network.Request{Method: "GET", URL: url},
		Type:             network.ResourceTypeDocument,
		Timestamp:        at,
		RedirectResponse: redirect,
	})
}

func netlogRequests(l *netLog) []browser.Request {
	return l.requests(map[cdp.LoaderID]bool{netlogLoader: true})
}

func TestNetLogRequestsFiltersByLoader(t *testing.T) {
	l := newNetLog()
	netlogSent(l, "a", "https://a.test/", 1, nil)
	if got := l.requests(map[cdp.LoaderID]bool{"other": true}); len(got) != 0 {
		t.Errorf("requests(other loader) = %v, want none", got)
	}
	if got := netlogRequests(l); len(got) != 1 || got[0].ID != "n1" {
		t.Errorf("requests(L1) = %v, want one row n1", got)
	}
}

func TestNetLogRedirectAnswersThePreviousHop(t *testing.T) {
	l := newNetLog()
	netlogSent(l, "a", "https://a.test/old", 1, nil)
	netlogSent(l, "a", "https://a.test/new", 1.25, &network.Response{Status: 301, StatusText: "Moved", EncodedDataLength: 40})
	got := netlogRequests(l)
	if len(got) != 2 {
		t.Fatalf("requests = %v, want 2 rows", got)
	}
	type hopFields struct {
		ID, StatusText string
		Status         int
		Done           bool
		Duration       time.Duration
		Size           int
	}
	hop := got[0]
	if h := (hopFields{hop.ID, hop.StatusText, hop.Status, hop.Done, hop.Duration, hop.Size}); h != (hopFields{"n1", "Moved", 301, true, 250 * time.Millisecond, 40}) {
		t.Errorf("redirect hop = %+v, want n1 301 Moved done in 250ms, 40 bytes", hop)
	}
	if next := got[1]; next.ID != "n2" || next.Done || next.Status != 0 {
		t.Errorf("next hop = %+v, want n2 pending", next)
	}
}

func TestNetLogRedirectResponseWithoutAHopStartsTheExchange(t *testing.T) {
	l := newNetLog()
	netlogSent(l, "a", "https://a.test/", 1, &network.Response{Status: 302})
	if got := netlogRequests(l); len(got) != 1 || got[0].ID != "n1" || got[0].Done {
		t.Errorf("requests = %+v, want one pending row n1", got)
	}
}

func TestNetLogRowsMarkOnlyTheFinalHopDone(t *testing.T) {
	l := newNetLog()
	netlogSent(l, "a", "https://a.test/1", 1, nil)
	netlogSent(l, "a", "https://a.test/2", 2, nil)
	got := netlogRequests(l)
	if got[0].Done || got[1].Done {
		t.Errorf("before finish: done = %v, %v; want false, false", got[0].Done, got[1].Done)
	}
	l.finished(network.EventLoadingFinished{RequestID: "a", Timestamp: 2.5, EncodedDataLength: 900})
	got = netlogRequests(l)
	if got[0].Done {
		t.Errorf("first hop done = true, want false")
	}
	if f := got[1]; !f.Done || f.Duration != 500*time.Millisecond || f.Size != 900 || f.Error != "" {
		t.Errorf("final hop = %+v, want done in 500ms, 900 bytes", f)
	}
}

func TestNetLogRowsCarryTheFailure(t *testing.T) {
	l := newNetLog()
	netlogSent(l, "a", "https://a.test/", 1, nil)
	l.failed(network.EventLoadingFailed{RequestID: "a", Timestamp: 3, ErrorText: "net::ERR_FAILED"})
	if got := netlogRequests(l); len(got) != 1 || !got[0].Done || got[0].Error != "net::ERR_FAILED" || got[0].Duration != 2*time.Second {
		t.Errorf("requests = %+v, want done after 2s with net::ERR_FAILED", got)
	}
}

func TestNetLogRequestsOrderHopsAcrossExchangesByStart(t *testing.T) {
	l := newNetLog()
	netlogSent(l, "a", "https://a.test/1", 1, nil)
	netlogSent(l, "b", "https://b.test/", 2, nil)
	netlogSent(l, "a", "https://a.test/2", 3, &network.Response{Status: 302})
	got := netlogRequests(l)
	var ids []string
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	if want := []string{"n1", "n2", "n3"}; !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestFailureNamesTheCause(t *testing.T) {
	for _, tc := range []struct {
		e    network.EventLoadingFailed
		want string
	}{
		{network.EventLoadingFailed{ErrorText: "net::ERR_ABORTED"}, "net::ERR_ABORTED"},
		{network.EventLoadingFailed{ErrorText: "net::ERR_BLOCKED_BY_CLIENT", BlockedReason: network.BlockedReasonInspector}, "net::ERR_BLOCKED_BY_CLIENT (blocked: inspector)"},
		{network.EventLoadingFailed{ErrorText: "net::ERR_FAILED", CorsErrorStatus: &network.CorsErrorStatus{CorsError: network.CorsErrorMissingAllowOriginHeader}}, "net::ERR_FAILED (CORS MissingAllowOriginHeader)"},
	} {
		if got := failure(tc.e); got != tc.want {
			t.Errorf("failure(%+v) = %q, want %q", tc.e, got, tc.want)
		}
	}
}

func TestNetLogKeepsTheLastThousandRequests(t *testing.T) {
	l := newNetLog()
	url := func(i int) string { return fmt.Sprintf("https://a.test/%d", i) }
	for i := range 1000 {
		netlogSent(l, network.RequestID(fmt.Sprint(i)), url(i), 1, nil)
	}
	if got := netlogRequests(l); len(got) != 1000 || got[0].URL != url(0) {
		t.Fatalf("at 1000: %d rows; want 1000, the first %q", len(got), url(0))
	}
	netlogSent(l, "1000", url(1000), 1, nil)
	if got := netlogRequests(l); len(got) != 1000 || got[0].URL != url(1) || got[999].URL != url(1000) {
		t.Errorf("past 1000: %d rows, first %q, last %q; want 1000, %q, %q", len(got), got[0].URL, got[len(got)-1].URL, url(1), url(1000))
	}
}

func TestNetLogLoadedSkipsPendingFailedAndOtherURLs(t *testing.T) {
	const url = "https://a.test/"
	l := newNetLog()
	netlogSent(l, "ok", url, 1, nil)
	l.finished(network.EventLoadingFinished{RequestID: "ok", Timestamp: 2})
	netlogSent(l, "pending", url, 3, nil)
	netlogSent(l, "failed", url, 3, nil)
	l.failed(network.EventLoadingFailed{RequestID: "failed", Timestamp: 4, ErrorText: "net::ERR_FAILED"})
	netlogSent(l, "other", "https://other.test/", 5, nil)
	l.finished(network.EventLoadingFinished{RequestID: "other", Timestamp: 6})
	if id, ok := l.loaded(url); !ok || id != "ok" {
		t.Errorf("loaded(%q) = %q, %v; want \"ok\", true", url, id, ok)
	}
}

func TestDurationConvertsSecondsToDuration(t *testing.T) {
	if got, want := duration(1, 1.5), 500*time.Millisecond; got != want {
		t.Errorf("duration(1, 1.5) = %v, want %v", got, want)
	}
}

func TestHeadersSortByNameIgnoringCase(t *testing.T) {
	got := headers(network.Headers{"b": "2", "D": "4", "a": "1", "C": "3", "f": "6", "E": "5"})
	var want []browser.Header
	for _, n := range []string{"a", "b", "C", "D", "E", "f"} {
		want = append(want, browser.Header{Name: n, Value: fmt.Sprint(len(want) + 1)})
	}
	if !slices.Equal(got, want) {
		t.Errorf("headers = %v, want %v", got, want)
	}
}
