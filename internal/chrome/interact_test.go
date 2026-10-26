package chrome_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

const actPage = `<!doctype html><title>Act</title>
<style>
body { margin: 0; font: 16px sans-serif; }
#covered { position: absolute; top: 1000px; left: 0; }
#lid { position: absolute; top: 990px; left: 0; width: 300px; height: 60px; background: white; }
#far { position: absolute; top: 3000px; left: 0; }
#fancy { position: relative; display: inline-block; }
#box { position: absolute; left: 0; top: 0; margin: 0; }
.skin { position: relative; display: inline-block; width: 20px; height: 20px; background: red; }
</style>
<button id="count" onclick="this.textContent = +this.textContent + 1">0</button>
<button id="off" disabled>Off</button>
<button id="hidden" hidden>Hidden</button>
<label id="fancy"><input type="checkbox" id="box"><span class="skin"></span> Fancy</label>
<div id="hover" onmouseenter="this.textContent = 'hovered'">Hover me</div>
<input id="who" value="old" onkeydown="this.dataset.keys = +(this.dataset.keys || 0) + 1">
<input id="ro" readonly value="fixed">
<div id="keylog" tabindex="0" onkeydown="this.textContent += (event.ctrlKey ? 'Control+' : '') + event.key + ' '"></div>
<select id="plan" onchange="this.dataset.changes = +(this.dataset.changes || 0) + 1">
<option value="free">Free</option><option value="pro">Pro plan</option><option value="ent" disabled>Enterprise</option>
</select>
<a id="next" href="/next">Next</a> <a id="hash" href="#section">Section</a>
<a id="empty" href="/empty">Empty</a> <a id="slow" href="/slow">Slow</a>
<button id="warn" onclick="alert('Saved'); this.textContent = confirm('Sure?')">Alert</button>
<form action="/next"><input name="q" id="q"></form>
<button id="covered" onclick="this.textContent = 'clicked'">Covered</button><div id="lid"></div>
<button id="far" onclick="this.textContent = 'reached'">Far</button>
<p id="section">Section</p>`

func actSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(actPage)) })
	mux.HandleFunc("/next", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><title>Next</title><p>Next</p>"))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("<!doctype html><title>Slow</title><p>Slow</p>"))
	})
	mux.HandleFunc("/empty", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func openActPage(t *testing.T) (*chrome.Browser, string) {
	t.Helper()
	srv := actSite(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	return b, srv.URL
}

func interact(t *testing.T, b *chrome.Browser, req browser.InteractRequest) browser.Interaction {
	t.Helper()
	r, err := b.Interact(t.Context(), req)
	if err != nil {
		t.Fatalf("interact %+v: %v", req, err)
	}
	return r
}

func click(sel string) browser.InteractRequest {
	return browser.InteractRequest{Action: browser.Click, Selector: sel}
}

func refOf(t *testing.T, b *chrome.Browser, role, name string) string {
	t.Helper()
	snap, err := b.Snapshot(t.Context(), browser.SnapshotRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var find func([]*browser.Node) string
	find = func(nodes []*browser.Node) string {
		for _, n := range nodes {
			if n.Role == role && n.Name == name {
				return n.Ref
			}
			if r := find(n.Children); r != "" {
				return r
			}
		}
		return ""
	}
	if r := find(snap.Nodes); r != "" {
		return r
	}
	t.Fatalf("no %s %q in the snapshot", role, name)
	return ""
}

func TestClickPressesTheElement(t *testing.T) {
	t.Parallel()
	b, base := openActPage(t)
	interact(t, b, click("#count"))
	r := interact(t, b, click("#count"))
	if got := evalValue(t, b, "count.textContent"); got != `"2"` {
		t.Errorf("count after two clicks = %s, want \"2\"", got)
	}
	if want := (browser.Interaction{URL: base + "/", Title: "Act"}); r.URL != want.URL || r.Title != want.Title || r.Navigated || r.Dialogs != nil {
		t.Errorf("click reported %+v, want %+v", r, want)
	}
}

func TestClickByRefScrollsElementIntoView(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	interact(t, b, browser.InteractRequest{Action: browser.Click, Ref: refOf(t, b, "button", "Far")})
	if got := evalValue(t, b, "far.textContent"); got != `"reached"` {
		t.Errorf("far button text = %s, want \"reached\"", got)
	}
}

func TestClickRefusesWhatAPersonCouldNotClick(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	for _, tc := range []struct {
		req  browser.InteractRequest
		want string
	}{
		{click("#off"), `"#off" is disabled`},
		{click("#hidden"), `"#hidden" is not visible`},
		{click("#covered"), `"#covered" is covered by div#lid`},
		{click("#nope"), `no element matches "#nope"`},
		{browser.InteractRequest{Action: browser.Click, Ref: "e99999999"}, "ref e99999999 is not in the page; take a new snapshot"},
		{browser.InteractRequest{Action: browser.Click, Ref: "button"}, `"button" is not a ref`},
		{browser.InteractRequest{Action: browser.Click}, "click needs a ref or a selector"},
		{browser.InteractRequest{Action: "drag", Selector: "#count"}, `unknown action "drag"`},
	} {
		if _, err := b.Interact(t.Context(), tc.req); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("interact %+v: err = %v, want %q", tc.req, err, tc.want)
		}
	}
	if got := evalValue(t, b, "covered.textContent"); got != `"Covered"` {
		t.Errorf("covered button text = %s, want it unclicked", got)
	}
}

func TestClickAtAPointHitsWhatIsThere(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	var at browser.Point
	if err := json.Unmarshal([]byte(evalValue(t, b, "(r => ({X: r.x + r.width / 2, Y: r.y + r.height / 2}))(count.getBoundingClientRect())")), &at); err != nil {
		t.Fatal(err)
	}
	interact(t, b, browser.InteractRequest{Action: browser.Click, At: &at})
	if got := evalValue(t, b, "count.textContent"); got != `"1"` {
		t.Errorf("count after a click at its centre %+v = %s, want \"1\"", at, got)
	}
}

func TestClickAtAPointOutsideTheViewportFails(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	size := evalValue(t, b, "innerWidth + 'x' + innerHeight")
	w, h := 0.0, 0.0
	if _, err := fmt.Sscanf(strings.Trim(size, `"`), "%gx%g", &w, &h); err != nil {
		t.Fatal(err)
	}
	for _, at := range []browser.Point{{X: -1, Y: 10}, {X: 10, Y: -0.5}, {X: w, Y: 10}, {X: 10, Y: h}} {
		_, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Click, At: &at})
		if want := "is outside the viewport, which is " + strings.Trim(size, `"`); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("click at %+v: err = %v, want %q", at, err, want)
		}
	}
}

func TestClickOnStyledCheckboxReachesItsControl(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	interact(t, b, click("#box"))
	if got := evalValue(t, b, "box.checked"); got != "true" {
		t.Errorf("checkbox checked = %s, want true", got)
	}
}

func TestRefFromAnEarlierPageFails(t *testing.T) {
	t.Parallel()
	b, base := openActPage(t)
	ref := refOf(t, b, "button", "Far")
	nav(t, b, browser.NavRequest{URL: base + "/next"})
	_, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Click, Ref: ref})
	if err == nil || !strings.Contains(err.Error(), "take a new snapshot") {
		t.Errorf("click on %s after leaving its page: err = %v, want a stale ref error", ref, err)
	}
}

func TestHoverMovesTheMouseOver(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	interact(t, b, browser.InteractRequest{Action: browser.Hover, Selector: "#hover"})
	if got := evalValue(t, b, "hover.textContent"); got != `"hovered"` {
		t.Errorf("hover target text = %s, want \"hovered\"", got)
	}
}

func TestTypeReplacesTheFieldContentKeyByKey(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	interact(t, b, browser.InteractRequest{Action: browser.Type, Selector: "#who", Text: "Ada L"})
	if got := evalValue(t, b, "[who.value, who.dataset.keys]"); got != `["Ada L","5"]` {
		t.Errorf("[value, key presses] = %s, want [\"Ada L\",\"5\"]", got)
	}
	interact(t, b, browser.InteractRequest{Action: browser.Type, Selector: "#who"})
	if got := evalValue(t, b, "who.value"); got != `""` {
		t.Errorf("value after typing nothing = %s, want it cleared", got)
	}
}

func TestTypeWithoutElementGoesToTheFocus(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	evalValue(t, b, "q.focus()")
	interact(t, b, browser.InteractRequest{Action: browser.Type, Text: "abc"})
	if got := evalValue(t, b, "q.value"); got != `"abc"` {
		t.Errorf("focused field value = %s, want \"abc\"", got)
	}
}

func TestTypeRefusesWhatTakesNoText(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	for sel, want := range map[string]string{
		"#ro":    "is read-only",
		"#count": "is not a text field",
		"#off":   "is disabled",
	} {
		_, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Type, Selector: sel, Text: "x"})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("type into %s: err = %v, want %q", sel, err, want)
		}
	}
	if _, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Type}); err == nil {
		t.Error("type with no text and no element succeeded")
	}
}

func TestPressSendsKeysWithModifiers(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	interact(t, b, browser.InteractRequest{Action: browser.Press, Selector: "#keylog", Text: "Enter"})
	interact(t, b, browser.InteractRequest{Action: browser.Press, Text: "Control+a"})
	if got := evalValue(t, b, "keylog.textContent"); got != `"Enter Control+a "` {
		t.Errorf("keys seen = %s, want \"Enter Control+a \"", got)
	}
	if _, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Press, Text: "Enterr"}); err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Errorf("press Enterr: err = %v, want unknown key", err)
	}
}

func TestPressEnterSubmitsTheFormAndWaitsForTheResult(t *testing.T) {
	t.Parallel()
	b, base := openActPage(t)
	interact(t, b, browser.InteractRequest{Action: browser.Type, Selector: "#q", Text: "cats"})
	r := interact(t, b, browser.InteractRequest{Action: browser.Press, Text: "Enter"})
	if r.URL != base+"/next?q=cats" || r.Title != "Next" || !r.Navigated {
		t.Errorf("after Enter: %+v, want the Next page for q=cats", r)
	}
}

func TestSelectPicksByValueOrLabel(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	interact(t, b, browser.InteractRequest{Action: browser.Select, Selector: "#plan", Text: "pro"})
	if got := evalValue(t, b, "plan.value"); got != `"pro"` {
		t.Errorf("value after selecting pro = %s", got)
	}
	interact(t, b, browser.InteractRequest{Action: browser.Select, Selector: "#plan", Text: "Free"})
	if got := evalValue(t, b, "[plan.value, plan.dataset.changes]"); got != `["free","2"]` {
		t.Errorf("[value, change events] after selecting Free = %s, want [\"free\",\"2\"]", got)
	}
	for _, tc := range []struct{ sel, option, want string }{
		{"#plan", "Gold", `has no option "Gold"; its options are "Free", "Pro plan", "Enterprise"`},
		{"#plan", "ent", `has option "ent" disabled`},
		{"#who", "pro", "is not a <select> element"},
	} {
		_, err := b.Interact(t.Context(), browser.InteractRequest{Action: browser.Select, Selector: tc.sel, Text: tc.option})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("select %q in %s: err = %v, want %q", tc.option, tc.sel, err, tc.want)
		}
	}
}

func TestClickOnLinkWaitsForTheNextPage(t *testing.T) {
	t.Parallel()
	for link, title := range map[string]string{"#next": "Next", "#slow": "Slow"} {
		b, base := openActPage(t)
		r := interact(t, b, click(link))
		if want := base + "/" + strings.ToLower(title); r.URL != want || r.Title != title || !r.Navigated {
			t.Errorf("click %s: %+v, want the %s page at %s", link, r, title, want)
		}
	}
}

func TestClickOnHashLinkNavigatesWithinThePage(t *testing.T) {
	t.Parallel()
	b, base := openActPage(t)
	r := interact(t, b, click("#hash"))
	if r.URL != base+"/#section" || !r.Navigated {
		t.Errorf("click on hash link: %+v, want navigated to #section", r)
	}
}

func TestClickOnLinkWithNoContentStays(t *testing.T) {
	t.Parallel()
	b, base := openActPage(t)
	start := time.Now()
	r := interact(t, b, click("#empty"))
	if r.URL != base+"/" || r.Navigated || time.Since(start) > 5*time.Second {
		t.Errorf("click on 204 link: %+v after %s, want to stay on the page at once", r, time.Since(start))
	}
}

func TestDialogsAreAcceptedAndReported(t *testing.T) {
	t.Parallel()
	b, _ := openActPage(t)
	r := interact(t, b, click("#warn"))
	if want := []string{"alert: Saved", "confirm: Sure?"}; !slices.Equal(r.Dialogs, want) {
		t.Errorf("dialogs = %q, want %q", r.Dialogs, want)
	}
	if got := evalValue(t, b, "warn.textContent"); got != `"true"` {
		t.Errorf("confirm returned %s, want true", got)
	}
	if got := evalValue(t, b, fmt.Sprintf("prompt(%q, %q)", "Name?", "Ada")); got != `"Ada"` {
		t.Errorf("prompt from eval returned %s, want its default", got)
	}
}
