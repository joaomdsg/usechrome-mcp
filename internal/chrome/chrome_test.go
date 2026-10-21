package chrome_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

func site(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	page := func(title string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, "<!doctype html><title>%s</title><p>%s</p>", title, title)
		}
	}
	mux.HandleFunc("/{$}", page("Home"))
	mux.HandleFunc("/second", page("Second"))
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/second", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newBrowser(t *testing.T) *chrome.Browser {
	t.Helper()
	// CI runners forbid the user namespaces the sandbox needs, and tests
	// only load local fixtures.
	b := chrome.New(context.Background(), chrome.Options{NoSandbox: true, Timeout: 10 * time.Second})
	t.Cleanup(func() {
		if err := b.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return b
}

func nav(t *testing.T, b *chrome.Browser, req browser.NavRequest) browser.Page {
	t.Helper()
	p, err := b.Navigate(t.Context(), req)
	if err != nil {
		t.Fatalf("navigate %+v: %v", req, err)
	}
	return p
}

func TestNavigateFollowsRedirectToFinalPage(t *testing.T) {
	t.Parallel()
	srv := site(t)
	got := nav(t, newBrowser(t), browser.NavRequest{URL: srv.URL + "/redirect"})
	want := browser.Page{URL: srv.URL + "/second", Status: 200, Title: "Second"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestNavigateReportsErrorStatus(t *testing.T) {
	t.Parallel()
	srv := site(t)
	got := nav(t, newBrowser(t), browser.NavRequest{Action: browser.Goto, URL: srv.URL + "/missing"})
	if got.Status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", got.Status)
	}
}

func TestBackAndForwardWalkHistory(t *testing.T) {
	t.Parallel()
	srv := site(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	nav(t, b, browser.NavRequest{URL: srv.URL + "/second"})

	if got := nav(t, b, browser.NavRequest{Action: browser.Back}); got.Title != "Home" {
		t.Errorf("after back: %+v, want title Home", got)
	}
	if got := nav(t, b, browser.NavRequest{Action: browser.Forward}); got.Title != "Second" {
		t.Errorf("after forward: %+v, want title Second", got)
	}
}

func TestReloadStaysOnPage(t *testing.T) {
	t.Parallel()
	srv := site(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/second"})
	got := nav(t, b, browser.NavRequest{Action: browser.Reload})
	want := browser.Page{URL: srv.URL + "/second", Status: 200, Title: "Second"}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestHistoryStepPastEndsFails(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	for action, want := range map[browser.NavAction]string{
		browser.Back:    "no page to go back to",
		browser.Forward: "no page to go forward to",
	} {
		_, err := b.Navigate(t.Context(), browser.NavRequest{Action: action})
		if err == nil || err.Error() != want {
			t.Errorf("%s: err = %v, want %q", action, err, want)
		}
	}
}

func TestUnreachableHostFailsWithLoadError(t *testing.T) {
	t.Parallel()
	// A port that was free a moment ago refuses connections, and unlike
	// low ports it is not on Chrome's unsafe-port list.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = newBrowser(t).Navigate(t.Context(), browser.NavRequest{URL: "http://" + addr + "/"})
	if err == nil || !strings.Contains(err.Error(), "ERR_CONNECTION_REFUSED") {
		t.Errorf("err = %v, want connection refused", err)
	}
}

func TestUnknownActionAndTabFail(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	for _, req := range []browser.NavRequest{
		{Action: "sideways"},
		{Tab: "t99", Action: browser.Reload},
	} {
		if _, err := b.Navigate(t.Context(), req); err == nil {
			t.Errorf("%+v: want error", req)
		}
	}
}

func TestCloseThenNavigateRestartsBrowser(t *testing.T) {
	t.Parallel()
	srv := site(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if got := nav(t, b, browser.NavRequest{URL: srv.URL + "/second"}); got.Title != "Second" {
		t.Errorf("after restart: %+v", got)
	}
}
