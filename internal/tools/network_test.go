package tools_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeNetworker struct {
	listed []browser.NetworkRequest
	asked  []browser.RequestDetailRequest
	reqs   []browser.Request
	detail browser.RequestDetail
	err    error
}

func (f *fakeNetworker) Network(_ context.Context, req browser.NetworkRequest) ([]browser.Request, error) {
	f.listed = append(f.listed, req)
	return f.reqs, f.err
}

func (f *fakeNetworker) RequestDetail(_ context.Context, req browser.RequestDetailRequest) (browser.RequestDetail, error) {
	f.asked = append(f.asked, req)
	return f.detail, f.err
}

func networkSession(t *testing.T, n tools.Networker) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddNetwork(s, n)
	return connect(t, s)
}

var pageRequests = []browser.Request{
	{ID: "n1", Method: "GET", URL: "http://x/", Type: "document", Status: 200, MimeType: "text/html", Size: 1234, Duration: 12 * time.Millisecond, Done: true},
	{ID: "n2", Method: "GET", URL: "http://x/logo.png", Type: "image", Status: 404, MimeType: "text/plain", Size: 120, Duration: 3 * time.Millisecond, Done: true},
	{ID: "n3", Method: "GET", URL: "http://x/app.js", Type: "script", Status: 200, MimeType: "text/javascript", Cache: "disk cache", Done: true},
	{ID: "n4", Method: "POST", URL: "http://x/api/cart", Type: "fetch", Status: 201, MimeType: "application/json"},
	{ID: "n5", Method: "GET", URL: "http://y/feed", Type: "fetch", Size: 0, Duration: 40 * time.Millisecond, Done: true, Error: "net::ERR_FAILED (CORS MissingAllowOriginHeader)"},
	{ID: "n6", Method: "GET", URL: "http://x/video", Status: 0},
	{ID: "n7", Method: "GET", URL: "http://x/big.bin", Type: "other", Status: 200, Size: 2_500_000, Duration: 1500 * time.Millisecond, Done: true},
}

func TestNetworkWritesALinePerRequest(t *testing.T) {
	t.Parallel()
	n := &fakeNetworker{reqs: pageRequests}
	res := call(t, networkSession(t, n), "network", map[string]any{"tab": "t2"})
	want := `id method status type mime size time url
n1 GET 200 document text/html 1.2kB 12ms http://x/
n2 GET 404 image text/plain 120B 3ms http://x/logo.png
n3 GET 200 script text/javascript (disk cache) 0ms http://x/app.js
n4 POST 201 fetch application/json - - http://x/api/cart
n5 GET failed fetch - 0B 40ms http://y/feed net::ERR_FAILED (CORS MissingAllowOriginHeader)
n6 GET pending - - - - http://x/video
n7 GET 200 other - 2.5MB 1500ms http://x/big.bin
`
	if got := errorText(res); res.IsError || got != want {
		t.Errorf("network:\n%s\nwant:\n%s", got, want)
	}
	if want := (browser.NetworkRequest{Tab: "t2"}); len(n.listed) != 1 || n.listed[0] != want {
		t.Errorf("browser called with %+v, want %+v", n.listed, want)
	}
}

func TestNetworkFiltersLinesIgnoringCase(t *testing.T) {
	t.Parallel()
	for filter, want := range map[string][]string{
		"API":    {"n4"},
		"failed": {"n5"},
		"404":    {"n2"},
		"fetch":  {"n4", "n5"},
	} {
		got := errorText(call(t, networkSession(t, &fakeNetworker{reqs: pageRequests}), "network", map[string]any{"filter": filter}))
		var ids []string
		for line := range strings.Lines(got) {
			ids = append(ids, strings.Fields(line)[0])
		}
		if fmt.Sprint(ids) != fmt.Sprint(append([]string{"id"}, want...)) {
			t.Errorf("filter %q shows %q, want the columns and %q", filter, ids, want)
		}
	}
}

func TestNetworkSaysWhenNoRequestShows(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args map[string]any
		reqs []browser.Request
		want string
	}{
		{map[string]any{}, nil, "no requests on this page\n"},
		{map[string]any{"filter": "woff"}, pageRequests, "no requests on this page match \"woff\"\n"},
	} {
		if got := errorText(call(t, networkSession(t, &fakeNetworker{reqs: tc.reqs}), "network", tc.args)); got != tc.want {
			t.Errorf("network %v = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestNetworkCutsTheOldestRequestsPastTheLimit(t *testing.T) {
	t.Parallel()
	var reqs []browser.Request
	for i := range 1000 {
		reqs = append(reqs, browser.Request{ID: fmt.Sprintf("n%d", i), Method: "GET", URL: "http://x/" + strings.Repeat("p", 150), Status: 200, Done: true})
	}
	text := errorText(call(t, networkSession(t, &fakeNetworker{reqs: reqs}), "network", map[string]any{}))
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(text) > 40_000 || !strings.HasPrefix(lines[0], "... ") || !strings.Contains(lines[0], "earlier requests cut") {
		t.Errorf("network is %d chars starting %q; want at most 40000 starting with a cut notice", len(text), lines[0])
	}
	if lines[1] != "id method status type mime size time url" || !strings.HasPrefix(lines[len(lines)-1], "n999 ") {
		t.Errorf("network keeps %q to %q, want the columns and whole lines up to the newest", lines[1], lines[len(lines)-1][:10])
	}
}

func TestNetworkShortensALongURLInTheList(t *testing.T) {
	t.Parallel()
	url := "data:image/png;base64," + strings.Repeat("A", 1000)
	n := &fakeNetworker{reqs: []browser.Request{{ID: "n1", Method: "GET", URL: url, Type: "image", Status: 200, Done: true}}}
	got := errorText(call(t, networkSession(t, n), "network", map[string]any{}))
	if want := url[:200] + " ... 822 more characters\n"; !strings.HasSuffix(got, want) {
		t.Errorf("network = %q, want the URL cut to 200 characters", got)
	}
}

func TestNetworkShowsARequestWithItsHeadersAndBodies(t *testing.T) {
	t.Parallel()
	n := &fakeNetworker{detail: browser.RequestDetail{
		Request: browser.Request{
			ID: "n4", Method: "POST", URL: "http://x/api/cart", Type: "fetch", Status: 201, StatusText: "Created",
			MimeType: "application/json", Size: 215, Duration: 4 * time.Millisecond, Done: true, HasPostData: true,
			RequestHeaders:  []browser.Header{{Name: "Content-Type", Value: "application/json"}},
			ResponseHeaders: []browser.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "Set-Cookie", Value: "a=1"}, {Name: "Set-Cookie", Value: "b=2"}},
		},
		RequestBody:  browser.Body{Text: `{"sku":"tea"}`, Size: 13},
		ResponseBody: browser.Body{Text: `{"items":1}`, Size: 11},
	}}
	res := call(t, networkSession(t, n), "network", map[string]any{"request": "n4", "tab": "t2"})
	want := `POST http://x/api/cart
status: 201 Created
type: fetch
mime: application/json
size: 215B
time: 4ms

request headers:
  Content-Type: application/json

request body (13B):
{"sku":"tea"}

response headers:
  Content-Type: application/json
  Set-Cookie: a=1
  Set-Cookie: b=2

response body (11B):
{"items":1}
`
	if got := errorText(res); res.IsError || got != want {
		t.Errorf("request n4:\n%s\nwant:\n%s", got, want)
	}
	if want := (browser.RequestDetailRequest{Tab: "t2", ID: "n4"}); len(n.asked) != 1 || n.asked[0] != want || len(n.listed) != 0 {
		t.Errorf("browser asked %+v and listed %+v, want only %+v", n.asked, n.listed, want)
	}
}

func TestNetworkSaysWhyARequestHasNoBody(t *testing.T) {
	t.Parallel()
	n := &fakeNetworker{detail: browser.RequestDetail{
		Request:      browser.Request{ID: "n5", Method: "GET", URL: "http://y/feed", Type: "fetch", Done: true, Error: "net::ERR_FAILED (CORS MissingAllowOriginHeader)"},
		ResponseBody: browser.Body{Missing: "the request failed"},
	}}
	want := `GET http://y/feed
status: failed
type: fetch
mime: -
size: 0B
time: 0ms
error: net::ERR_FAILED (CORS MissingAllowOriginHeader)

response body: none, the request failed
`
	if got := errorText(call(t, networkSession(t, n), "network", map[string]any{"request": "n5"})); got != want {
		t.Errorf("request n5:\n%s\nwant:\n%s", got, want)
	}
}

func TestNetworkGivesTheSizeOfABinaryBody(t *testing.T) {
	t.Parallel()
	n := &fakeNetworker{detail: browser.RequestDetail{
		Request:      browser.Request{ID: "n2", Method: "GET", URL: "http://x/logo.png", Type: "image", Status: 200, Done: true},
		ResponseBody: browser.Body{Size: 4096, Binary: true},
	}}
	got := errorText(call(t, networkSession(t, n), "network", map[string]any{"request": "n2"}))
	if !strings.HasSuffix(got, "\nresponse body: 4.1kB of binary data\n") {
		t.Errorf("request n2:\n%s\nwant the size of the binary body", got)
	}
}

func TestNetworkCutsHugeBodiesToFitTheLimit(t *testing.T) {
	t.Parallel()
	sent, got := strings.Repeat("s", 30_000), strings.Repeat("g", 50_000)
	n := &fakeNetworker{detail: browser.RequestDetail{
		Request:      browser.Request{ID: "n1", Method: "POST", URL: "http://x/upload", Status: 200, Done: true, HasPostData: true},
		RequestBody:  browser.Body{Text: sent, Size: len(sent)},
		ResponseBody: browser.Body{Text: got, Size: len(got)},
	}}
	text := errorText(call(t, networkSession(t, n), "network", map[string]any{"request": "n1"}))
	kept := func(c string) int { return strings.Count(text, c) }
	if len(text) > 40_000 || kept("s") > 20_000 || kept("g") < 15_000 || strings.Count(text, "more characters") != 2 {
		t.Errorf("detail is %d chars with %d of the request body and %d of the response body; want both cut to fit 40000, the request body to half",
			len(text), kept("s"), kept("g"))
	}
}

func TestNetworkCutsHugeHeadersToFitTheLimit(t *testing.T) {
	t.Parallel()
	n := &fakeNetworker{detail: browser.RequestDetail{
		Request: browser.Request{
			ID: "n1", Method: "GET", URL: "http://x/", Status: 200, Done: true,
			ResponseHeaders: []browser.Header{{Name: "Content-Security-Policy", Value: strings.Repeat("c", 50_000)}},
		},
		ResponseBody: browser.Body{Text: "hello", Size: 5},
	}}
	text := errorText(call(t, networkSession(t, n), "network", map[string]any{"request": "n1"}))
	if len(text) > 40_000 || !strings.HasSuffix(text, "more characters") {
		t.Errorf("detail is %d chars ending %q, want at most 40000 with a cut notice", len(text), text[len(text)-30:])
	}
}

func TestNetworkRejectsAFilterWithARequest(t *testing.T) {
	t.Parallel()
	n := &fakeNetworker{}
	res := call(t, networkSession(t, n), "network", map[string]any{"request": "n1", "filter": "api"})
	if !res.IsError || !strings.Contains(errorText(res), "filter or request") || len(n.asked)+len(n.listed) != 0 {
		t.Errorf("network with both: %q; want an error before the browser", errorText(res))
	}
}

func TestNetworkReportsBrowserError(t *testing.T) {
	t.Parallel()
	for _, args := range []map[string]any{{"tab": "t9"}, {"tab": "t9", "request": "n1"}} {
		res := call(t, networkSession(t, &fakeNetworker{err: errors.New(`unknown tab "t9"`)}), "network", args)
		if !res.IsError || !strings.Contains(errorText(res), "t9") {
			t.Errorf("network %v = %+v, want the browser error", args, res)
		}
	}
}
