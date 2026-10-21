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

var consolePages = map[string]string{
	"/logs": `<!doctype html><script>
console.log("plain", 1, true, null, undefined);
console.warn("careful");
console.error("bad");
console.debug("noise");
console.log("state", {x: 1, list: [1, 2]});
console.log("n=%d", 5);
const c = {}; c.self = c; console.log("cyclic", c);
</script>`,
	"/boom": `<!doctype html><script>
function inner() { throw new Error("deep") }
function outer() { inner() }
setTimeout(outer)
</script>`,
	"/broken": `<!doctype html><img src="/missing.png" alt="">`,
	"/quiet":  `<!doctype html><script>console.log("quiet")</script>`,
}

func consoleSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, html := range consolePages {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(html)) })
	}
	// Without it every page logs a failed favicon load.
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func console(t *testing.T, b *chrome.Browser) []browser.ConsoleMessage {
	t.Helper()
	msgs, err := b.Console(t.Context(), browser.ConsoleRequest{})
	if err != nil {
		t.Fatalf("console: %v", err)
	}
	return msgs
}

func typesAndTexts(msgs []browser.ConsoleMessage) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Type+": "+m.Text)
	}
	return out
}

func TestConsoleListsThePagesMessagesInOrder(t *testing.T) {
	t.Parallel()
	srv := consoleSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/logs"})
	msgs := console(t, b)
	want := []string{
		"log: plain 1 true null undefined",
		"warning: careful",
		"error: bad",
		"debug: noise",
		`log: state {"x":1,"list":[1,2]}`,
		"log: n=5",
		"log: cyclic Object",
	}
	if got := typesAndTexts(msgs); !reflect.DeepEqual(got, want) {
		t.Fatalf("messages:\n got %q\nwant %q", got, want)
	}
	if want := srv.URL + "/logs:3:9"; msgs[1].Source != want {
		t.Errorf("source of the warning = %q, want %q", msgs[1].Source, want)
	}
}

func TestConsoleReportsAnUncaughtExceptionWithItsStack(t *testing.T) {
	t.Parallel()
	srv := consoleSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/boom"})
	msgs := console(t, b)
	if len(msgs) != 1 || msgs[0].Type != "exception" || msgs[0].Text != "Uncaught Error: deep" {
		t.Fatalf("messages = %q, want the uncaught error", typesAndTexts(msgs))
	}
	page := srv.URL + "/boom"
	want := []string{"inner (" + page + ":2:26)", "outer (" + page + ":3:20)"}
	if got := msgs[0].Stack; !reflect.DeepEqual(got, want) {
		t.Errorf("stack = %q, want %q", got, want)
	}
}

func TestConsoleReportsAFailedLoadWithItsURL(t *testing.T) {
	t.Parallel()
	srv := consoleSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/broken"})
	msgs := console(t, b)
	if len(msgs) != 1 || msgs[0].Type != "error" || !strings.Contains(msgs[0].Text, "404") || msgs[0].Source != srv.URL+"/missing.png" {
		t.Errorf("messages = %+v, want the 404 of /missing.png", msgs)
	}
}

func TestConsoleForgetsThePreviousPage(t *testing.T) {
	t.Parallel()
	srv := consoleSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/logs"})
	nav(t, b, browser.NavRequest{URL: srv.URL + "/quiet"})
	if got := typesAndTexts(console(t, b)); !reflect.DeepEqual(got, []string{"log: quiet"}) {
		t.Errorf("messages after leaving /logs = %q, want only those of /quiet", got)
	}
}

func TestConsoleIncludesAMessageLoggedJustBefore(t *testing.T) {
	t.Parallel()
	srv := consoleSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/quiet"})
	for i := range 3 {
		if _, err := b.Eval(t.Context(), browser.EvalRequest{Expression: `console.log("now")`}); err != nil {
			t.Fatal(err)
		}
		got := typesAndTexts(console(t, b))
		if len(got) != i+2 || got[len(got)-1] != "log: now" {
			t.Fatalf("read %d: messages = %q, want each earlier now and one more", i, got)
		}
	}
}
