package chrome_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

// logo is a 1x1 PNG.
var logo = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\b\x00\x00\x00\x00:~\x9bU" +
	"\x00\x00\x00\x0fIDATx\x9c\x00\x02\x00\xfd\xff\x02\x00\x03\x00\x00\x06\x00\x03!\xfc\xac\x06\x00\x00\x00\x00IEND\xaeB`\x82")

func networkSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	serve := func(path, mime, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", mime)
			_, _ = io.WriteString(w, body)
		})
	}
	serve("/shop", "text/html", `<!doctype html><link rel="stylesheet" href="/style.css">`+
		`<img src="/missing.png" alt=""><script src="/app.js"></script>`)
	serve("/style.css", "text/css", "p { color: red }")
	serve("/app.js", "text/javascript", "let app")
	serve("/blank", "text/html", "<!doctype html><p>blank")
	serve("/logo.png", "image/png", string(logo))
	mux.HandleFunc("/missing.png", http.NotFound)
	mux.Handle("/old", http.RedirectHandler("/blank", http.StatusFound))
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		got, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"got":%s}`, got)
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// waitRequests reads the list until done accepts it. The log takes the
// browser's events after they arrive, so a request that has just finished
// can still read as loading. The browser asks for the favicon after the
// load, so it is left out.
func waitRequests(t *testing.T, b *chrome.Browser, done func([]browser.Request) bool) []browser.Request {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		reqs, err := b.Network(t.Context(), browser.NetworkRequest{})
		if err != nil {
			t.Fatalf("network: %v", err)
		}
		reqs = slices.DeleteFunc(reqs, func(r browser.Request) bool { return strings.HasSuffix(r.URL, "/favicon.ico") })
		if done(reqs) || time.Now().After(deadline) {
			return reqs
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func allDone(n int) func([]browser.Request) bool {
	return func(reqs []browser.Request) bool {
		for _, r := range reqs {
			if !r.Done {
				return false
			}
		}
		return len(reqs) >= n
	}
}

func summaries(srv *httptest.Server, reqs []browser.Request) []string {
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, fmt.Sprintf("%s %d %s %s %s", r.Method, r.Status, r.Type, r.MimeType, strings.TrimPrefix(r.URL, srv.URL)))
	}
	return out
}

func eval(t *testing.T, b *chrome.Browser, expr string) {
	t.Helper()
	if _, err := b.Eval(t.Context(), browser.EvalRequest{Expression: expr}); err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
}

func detail(t *testing.T, b *chrome.Browser, id string) browser.RequestDetail {
	t.Helper()
	d, err := b.RequestDetail(t.Context(), browser.RequestDetailRequest{ID: id})
	if err != nil {
		t.Fatalf("request %s: %v", id, err)
	}
	return d
}

func TestNetworkListsThePagesRequestsInOrder(t *testing.T) {
	t.Parallel()
	srv := networkSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/shop"})
	reqs := waitRequests(t, b, allDone(4))
	want := []string{
		"GET 200 document text/html /shop",
		"GET 200 stylesheet text/css /style.css",
		"GET 404 image text/plain /missing.png",
		"GET 200 script text/javascript /app.js",
	}
	if got := summaries(srv, reqs); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests:\n got %q\nwant %q", got, want)
	}
	for _, r := range reqs {
		if r.Size <= 0 || r.Duration <= 0 || r.Error != "" {
			t.Errorf("%s: size %d, duration %s, error %q; want a finished load", r.URL, r.Size, r.Duration, r.Error)
		}
	}
}

func TestNetworkShowsEachHopOfARedirect(t *testing.T) {
	t.Parallel()
	srv := networkSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/old"})
	reqs := waitRequests(t, b, allDone(2))
	want := []string{"GET 302 document text/html /old", "GET 200 document text/html /blank"}
	if got := summaries(srv, reqs); !reflect.DeepEqual(got, want) {
		t.Fatalf("requests:\n got %q\nwant %q", got, want)
	}
	hop := detail(t, b, reqs[0].ID)
	if hop.ResponseBody.Missing == "" || !hasHeader(hop.ResponseHeaders, "Location", "/blank") {
		t.Errorf("redirect hop: body %+v, headers %v; want no body and the Location", hop.ResponseBody, hop.ResponseHeaders)
	}
}

func TestNetworkForgetsThePreviousPage(t *testing.T) {
	t.Parallel()
	srv := networkSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/shop"})
	nav(t, b, browser.NavRequest{URL: srv.URL + "/blank"})
	reqs := waitRequests(t, b, allDone(1))
	if got := summaries(srv, reqs); !reflect.DeepEqual(got, []string{"GET 200 document text/html /blank"}) {
		t.Errorf("requests after leaving /shop = %q, want only /blank", got)
	}
}

func TestNetworkReadsTheBodiesOfAPost(t *testing.T) {
	t.Parallel()
	srv := networkSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/blank"})
	eval(t, b, `fetch("/api", {method: "POST", body: '{"sku":"tea"}'}).then(r => r.text())`)
	reqs := waitRequests(t, b, allDone(2))
	if got := summaries(srv, reqs); len(got) != 2 || got[1] != "POST 201 fetch application/json /api" {
		t.Fatalf("requests = %q, want the page then the POST", got)
	}
	d := detail(t, b, reqs[1].ID)
	if d.RequestBody.Text != `{"sku":"tea"}` || d.ResponseBody.Text != `{"got":{"sku":"tea"}}` {
		t.Errorf("bodies: request %+v, response %+v", d.RequestBody, d.ResponseBody)
	}
	if !hasHeader(d.RequestHeaders, "Content-Type", "text/plain;charset=UTF-8") || !hasHeader(d.ResponseHeaders, "Content-Type", "application/json") {
		t.Errorf("headers: request %v, response %v", d.RequestHeaders, d.ResponseHeaders)
	}
}

func TestNetworkGivesOnlyTheSizeOfABinaryBody(t *testing.T) {
	t.Parallel()
	srv := networkSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/blank"})
	eval(t, b, `fetch("/logo.png").then(r => r.arrayBuffer())`)
	reqs := waitRequests(t, b, allDone(2))
	d := detail(t, b, reqs[len(reqs)-1].ID)
	if want := (browser.Body{Size: len(logo), Binary: true}); d.ResponseBody != want {
		t.Errorf("body of %s = %+v, want %+v", d.URL, d.ResponseBody, want)
	}
}

func TestNetworkSaysWhyCORSBlockedARequest(t *testing.T) {
	t.Parallel()
	srv := networkSite(t)
	other := networkSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/blank"})
	eval(t, b, `fetch("`+other.URL+`/api").catch(e => e.message)`)
	reqs := waitRequests(t, b, allDone(2))
	last := reqs[len(reqs)-1]
	if last.Error != "net::ERR_FAILED (CORS MissingAllowOriginHeader)" {
		t.Errorf("error of %s = %q, want the CORS reason", last.URL, last.Error)
	}
}

func TestRequestDetailOfAnUnknownIDFails(t *testing.T) {
	t.Parallel()
	srv := networkSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/blank"})
	if _, err := b.RequestDetail(t.Context(), browser.RequestDetailRequest{ID: "n999"}); err == nil {
		t.Error("detail of n999 succeeded, want an error")
	}
}

func hasHeader(hs []browser.Header, name, value string) bool {
	for _, h := range hs {
		if strings.EqualFold(h.Name, name) && h.Value == value {
			return true
		}
	}
	return false
}
