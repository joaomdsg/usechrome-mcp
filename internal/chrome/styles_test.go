package chrome_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

const stylesPage = `<!doctype html><title>Styles</title>
<link rel="stylesheet" href="/app.css">
<style>
.card { padding: 8px; }
</style>
<div id="card" class="card wide" style="color: red">Hi</div>
<p id="plain">Plain</p>`

const appCSS = `body { margin: 0; }
.card {
  background: blue;
  margin: 4px !important;
}
@media (min-width: 100px) {
  .wide { width: 300px; }
}
`

func openStylesPage(t *testing.T) (*chrome.Browser, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(stylesPage)) })
	mux.HandleFunc("/app.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write([]byte(appCSS))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	return b, srv.URL
}

func styles(t *testing.T, b *chrome.Browser, req browser.StylesRequest) browser.Styles {
	t.Helper()
	s, err := b.Styles(t.Context(), req)
	if err != nil {
		t.Fatalf("styles %+v: %v", req, err)
	}
	return s
}

func TestStylesListsMatchedRulesHighestPriorityFirst(t *testing.T) {
	t.Parallel()
	b, base := openStylesPage(t)
	got := styles(t, b, browser.StylesRequest{Selector: "#card"}).Rules
	want := []browser.Rule{
		{Selector: "style attribute", Declarations: []string{"color: red"}},
		{Selector: ".card", Source: base + "/:4", Declarations: []string{"padding: 8px"}},
		{Selector: ".wide", Media: "(min-width: 100px)", Source: base + "/app.css:7", Declarations: []string{"width: 300px"}},
		{Selector: ".card", Source: base + "/app.css:2", Declarations: []string{"background: blue", "margin: 4px !important"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rules:\n got %+v\nwant %+v", got, want)
	}
}

func TestStylesComputesWhatTheRulesSetAndTheBasics(t *testing.T) {
	t.Parallel()
	b, _ := openStylesPage(t)
	s := styles(t, b, browser.StylesRequest{Selector: "#card"})
	for prop, want := range map[string]string{
		"color":    "rgb(255, 0, 0)",
		"width":    "300px",
		"margin":   "4px",
		"padding":  "8px",
		"display":  "block",
		"position": "static",
	} {
		if s.Computed[prop] != want {
			t.Errorf("computed %s = %q, want %q", prop, s.Computed[prop], want)
		}
	}
	if !strings.HasPrefix(s.Computed["background"], "rgb(0, 0, 255)") {
		t.Errorf("computed background = %q, want it blue", s.Computed["background"])
	}
	if want := (browser.Box{X: 4, Y: s.Box.Y, Width: 316, Height: s.Box.Height}); s.Box != want || s.Element != "div#card.card.wide" {
		t.Errorf("element %q box %+v, want div#card.card.wide at x 4, 316 wide", s.Element, s.Box)
	}
}

func TestStylesReadsOnlyTheAskedProperties(t *testing.T) {
	t.Parallel()
	b, _ := openStylesPage(t)
	s := styles(t, b, browser.StylesRequest{Selector: "#plain", Properties: []string{"font-style", "nope"}})
	if want := map[string]string{"font-style": "normal", "nope": ""}; !reflect.DeepEqual(s.Computed, want) {
		t.Errorf("computed = %v, want %v", s.Computed, want)
	}
	if s.Rules != nil {
		t.Errorf("rules of an unstyled element = %+v, want none from the browser's own sheet", s.Rules)
	}
}

func TestStylesOfMissingElementFails(t *testing.T) {
	t.Parallel()
	b, _ := openStylesPage(t)
	if _, err := b.Styles(t.Context(), browser.StylesRequest{Selector: "#nope"}); err == nil || !strings.Contains(err.Error(), "no element matches") {
		t.Errorf("styles of #nope: err = %v, want no element matches", err)
	}
}
