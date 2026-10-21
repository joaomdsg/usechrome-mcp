package chrome_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

const (
	sourcePage = `<!doctype html><link rel="stylesheet" href="/style.css"><img src="/logo.png" alt="">` +
		`<iframe src="/frame"></iframe><p id="n">0</p><script>document.getElementById("n").textContent = "1"</script>`
	frameDoc = "<!doctype html><p>in a frame"
	styleCSS = "p { color: red }"
)

func sourceSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	serve := func(path, mime, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", mime)
			_, _ = io.WriteString(w, body)
		})
	}
	serve("/{$}", "text/html", sourcePage)
	serve("/frame", "text/html", frameDoc)
	serve("/style.css", "text/css", styleCSS)
	serve("/logo.png", "image/png", string(logo))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func source(t *testing.T, b *chrome.Browser, url string) browser.Source {
	t.Helper()
	s, err := b.Source(t.Context(), browser.SourceRequest{URL: url})
	if err != nil {
		t.Fatalf("source %q: %v", url, err)
	}
	return s
}

func TestSourceIsTheDocumentAsServed(t *testing.T) {
	t.Parallel()
	srv := sourceSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	want := browser.Source{URL: srv.URL + "/", MimeType: "text/html", Body: browser.Body{Text: sourcePage, Size: len(sourcePage)}}
	if got := source(t, b, ""); got != want {
		t.Errorf("source of the page:\n got %+v\nwant %+v", got, want)
	}
}

func TestSourceReadsAResourceOrAFrameByURL(t *testing.T) {
	t.Parallel()
	srv := sourceSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	for _, want := range []browser.Source{
		{URL: srv.URL + "/style.css", MimeType: "text/css", Body: browser.Body{Text: styleCSS, Size: len(styleCSS)}},
		{URL: srv.URL + "/frame", MimeType: "text/html", Body: browser.Body{Text: frameDoc, Size: len(frameDoc)}},
		{URL: srv.URL + "/logo.png", MimeType: "image/png", Body: browser.Body{Size: len(logo), Binary: true}},
	} {
		if got := source(t, b, want.URL); got != want {
			t.Errorf("source of %s:\n got %+v\nwant %+v", want.URL, got, want)
		}
	}
}

func TestSourceOfAURLThePageDidNotLoadFails(t *testing.T) {
	t.Parallel()
	srv := sourceSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	_, err := b.Source(t.Context(), browser.SourceRequest{URL: srv.URL + "/nope.js"})
	if err == nil || !strings.Contains(err.Error(), "/nope.js") {
		t.Errorf("source of /nope.js: %v, want an error naming it", err)
	}
}
