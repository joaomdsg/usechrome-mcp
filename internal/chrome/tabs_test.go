package chrome_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

func tabSite(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	page := func(html string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(html)) }
	}
	mux.HandleFunc("/{$}", page("<!doctype html><title>Home</title>"))
	mux.HandleFunc("/second", page("<!doctype html><title>Second</title>"))
	mux.HandleFunc("/opener", page(`<!doctype html><title>Opener</title><a id="pop" href="/second" target="_blank">pop</a>`+
		`<button id="two" onclick="window.open('/'); window.open('/second')">two</button>`))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func tabs(t *testing.T, b *chrome.Browser, req browser.TabsRequest) []string {
	t.Helper()
	got, err := b.Tabs(t.Context(), req)
	if err != nil {
		t.Fatalf("tabs %+v: %v", req, err)
	}
	return tabLines(got)
}

// tabLines writes each tab as its id and title, the current one starred.
func tabLines(tabs []browser.Tab) []string {
	out := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		line := tab.ID + " " + tab.Title
		if tab.Current {
			line = "*" + line
		}
		out = append(out, line)
	}
	return out
}

func waitTabs(t *testing.T, b *chrome.Browser, want []string) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := tabs(t, b, browser.TabsRequest{})
		if slices.Equal(got, want) || time.Now().After(deadline) {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func title(t *testing.T, b *chrome.Browser, tab string) string {
	t.Helper()
	r, err := b.Eval(t.Context(), browser.EvalRequest{Tab: tab, Expression: "document.title"})
	if err != nil {
		t.Fatalf("title of tab %q: %v", tab, err)
	}
	return strings.Trim(string(r.Value), `"`)
}

func TestTheFirstTabIsCurrent(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	url := tabSite(t) + "/"
	nav(t, b, browser.NavRequest{URL: url})
	got, err := b.Tabs(t.Context(), browser.TabsRequest{Action: browser.ListTabs})
	if want := []browser.Tab{{ID: "t1", Title: "Home", URL: url, Current: true}}; err != nil || !slices.Equal(got, want) {
		t.Errorf("tabs = %+v, %v; want %+v", got, err, want)
	}
}

func TestAnOpenedTabBecomesCurrent(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	site := tabSite(t)
	nav(t, b, browser.NavRequest{URL: site + "/"})
	if got, want := tabs(t, b, browser.TabsRequest{Action: browser.OpenTab, URL: site + "/second"}), []string{"t1 Home", "*t2 Second"}; !slices.Equal(got, want) {
		t.Fatalf("tabs after open = %q, want %q", got, want)
	}
	if got := title(t, b, ""); got != "Second" {
		t.Errorf("current tab shows %q, want Second", got)
	}
	if got := title(t, b, "t1"); got != "Home" {
		t.Errorf("tab t1 shows %q, want Home", got)
	}
	if got, want := tabs(t, b, browser.TabsRequest{Action: browser.OpenTab}), []string{"t1 Home", "t2 Second", "*t3 about:blank"}; !slices.Equal(got, want) {
		t.Errorf("tabs after opening a blank one = %q, want %q", got, want)
	}
}

func TestASelectedTabBecomesCurrent(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	site := tabSite(t)
	nav(t, b, browser.NavRequest{URL: site + "/"})
	tabs(t, b, browser.TabsRequest{Action: browser.OpenTab, URL: site + "/second"})
	if got, want := tabs(t, b, browser.TabsRequest{Action: browser.SelectTab, Tab: "t1"}), []string{"*t1 Home", "t2 Second"}; !slices.Equal(got, want) {
		t.Fatalf("tabs after select = %q, want %q", got, want)
	}
	if got := title(t, b, ""); got != "Home" {
		t.Errorf("current tab shows %q, want Home", got)
	}
}

func TestClosingTheCurrentTabSelectsTheNewest(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	site := tabSite(t)
	nav(t, b, browser.NavRequest{URL: site + "/"})
	tabs(t, b, browser.TabsRequest{Action: browser.OpenTab, URL: site + "/second"})
	tabs(t, b, browser.TabsRequest{Action: browser.OpenTab, URL: site + "/opener"})
	tabs(t, b, browser.TabsRequest{Action: browser.SelectTab, Tab: "t2"})
	if got, want := tabs(t, b, browser.TabsRequest{Action: browser.CloseTab}), []string{"t1 Home", "*t3 Opener"}; !slices.Equal(got, want) {
		t.Errorf("tabs after closing the current t2 = %q, want %q", got, want)
	}
}

func TestClosingTheFirstTabKeepsTheBrowser(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	site := tabSite(t)
	nav(t, b, browser.NavRequest{URL: site + "/"})
	tabs(t, b, browser.TabsRequest{Action: browser.OpenTab, URL: site + "/second"})
	if got, want := tabs(t, b, browser.TabsRequest{Action: browser.CloseTab, Tab: "t1"}), []string{"*t2 Second"}; !slices.Equal(got, want) {
		t.Fatalf("tabs after closing t1 = %q, want %q", got, want)
	}
	if got, want := tabs(t, b, browser.TabsRequest{Action: browser.OpenTab, URL: site + "/"}), []string{"t2 Second", "*t3 Home"}; !slices.Equal(got, want) {
		t.Errorf("tabs after opening another = %q, want %q", got, want)
	}
}

func TestClosingTheLastTabFails(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	if _, err := b.Tabs(t.Context(), browser.TabsRequest{Action: browser.CloseTab}); err == nil || !strings.Contains(err.Error(), "last tab") {
		t.Errorf("closing the only tab: %v, want an error that it is the last", err)
	}
}

func TestTabsAdoptsATabThePageOpenedAndForgetsItOnceClosed(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: tabSite(t) + "/opener"})
	if _, err := b.Eval(t.Context(), browser.EvalRequest{Expression: "open('/second'), 0"}); err != nil {
		t.Fatal(err)
	}
	if got, want := waitTabs(t, b, []string{"*t1 Opener", "t2 Second"}), []string{"*t1 Opener", "t2 Second"}; !slices.Equal(got, want) {
		t.Fatalf("tabs after the page opened one = %q, want %q", got, want)
	}
	tabs(t, b, browser.TabsRequest{Action: browser.SelectTab, Tab: "t2"})
	if got := title(t, b, ""); got != "Second" {
		t.Errorf("adopted tab shows %q, want Second", got)
	}
	if _, err := b.Eval(t.Context(), browser.EvalRequest{Expression: "window.close()"}); err != nil {
		t.Fatal(err)
	}
	if got, want := waitTabs(t, b, []string{"*t1 Opener"}), []string{"*t1 Opener"}; !slices.Equal(got, want) {
		t.Errorf("tabs after the page closed itself = %q, want %q", got, want)
	}
}

func TestInteractReportsTheTabsItsActionOpened(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	site := tabSite(t)
	nav(t, b, browser.NavRequest{URL: site + "/opener"})
	for _, tc := range []struct {
		selector string
		want     []browser.Tab
	}{
		{"#pop", []browser.Tab{{ID: "t2", URL: site + "/second"}}},
		{"#two", []browser.Tab{{ID: "t3", URL: site + "/"}, {ID: "t4", URL: site + "/second"}}},
	} {
		r, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Click, Selector: tc.selector})
		if err != nil || !slices.Equal(r.Opened, tc.want) {
			t.Errorf("click on %s opened %+v, %v; want %+v", tc.selector, r.Opened, err, tc.want)
		}
	}
	if got, want := waitTabs(t, b, []string{"*t1 Opener", "t2 Second", "t3 Home", "t4 Second"}), []string{"*t1 Opener", "t2 Second", "t3 Home", "t4 Second"}; !slices.Equal(got, want) {
		t.Errorf("tabs after the clicks = %q, want %q", got, want)
	}
}

func TestACallToATabThatClosedFailsAtOnce(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: tabSite(t) + "/opener"})
	if _, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Click, Selector: "#pop"}); err != nil {
		t.Fatal(err)
	}
	waitTabs(t, b, []string{"*t1 Opener", "t2 Second"})
	start := time.Now()
	if _, err := b.Eval(t.Context(), browser.EvalRequest{Tab: "t2", Expression: "window.close()"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Eval(t.Context(), browser.EvalRequest{Tab: "t2", Expression: "1"}); err == nil || !strings.Contains(err.Error(), "tab t2 closed") {
		t.Errorf("eval in a tab that closed: %v, want an error that t2 closed", err)
	}
	if took := time.Since(start); took > opTimeout/2 {
		t.Errorf("closing the tab and calling it took %s, want well under the %s timeout", took, opTimeout)
	}
}

func TestUnknownTabOrTabActionFails(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	for _, tc := range []struct {
		req  browser.TabsRequest
		want string
	}{
		{browser.TabsRequest{Action: browser.SelectTab, Tab: "t9"}, `unknown tab "t9"`},
		{browser.TabsRequest{Action: browser.CloseTab, Tab: "t9"}, `unknown tab "t9"`},
		{browser.TabsRequest{Action: "shuffle"}, `unknown tab action "shuffle"`},
	} {
		if _, err := b.Tabs(t.Context(), tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("tabs %+v: %v, want an error with %q", tc.req, err, tc.want)
		}
	}
}
