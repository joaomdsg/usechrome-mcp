package chrome_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

const formPage = `<!doctype html><title>Form</title>
<main>
<h2>Sign up</h2>
<p>Read the <a href="/terms">terms</a> first.</p>
<form id="f">
<label>Email <input type="email" value="a@b.c" required></label>
<label><input type="checkbox"> Agree</label>
<button disabled>Send</button>
</form>
<div hidden><button>Secret</button></div>
</main>`

func snapshotOf(t *testing.T, html string, req browser.SnapshotRequest) (browser.Snapshot, *chrome.Browser) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	t.Cleanup(srv.Close)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/"})
	snap, err := b.Snapshot(t.Context(), req)
	if err != nil {
		t.Fatalf("snapshot %+v: %v", req, err)
	}
	return snap, b
}

func flatten(nodes []*browser.Node, depth int, out *[]string) {
	for _, n := range nodes {
		line := strings.Repeat("  ", depth) + n.Role
		if n.Name != "" {
			line += " " + n.Name
		}
		if n.Value != "" {
			line += " = " + n.Value
		}
		for _, p := range n.Props {
			line += " " + p.Key + "=" + p.Value
		}
		*out = append(*out, line)
		flatten(n.Children, depth+1, out)
	}
}

func outlineText(snap browser.Snapshot) string {
	var lines []string
	flatten(snap.Nodes, 0, &lines)
	return strings.Join(lines, "\n")
}

func TestSnapshotOutlinesThePage(t *testing.T) {
	t.Parallel()
	snap, _ := snapshotOf(t, formPage, browser.SnapshotRequest{})
	want := strings.Join([]string{
		"main",
		"  heading Sign up level=2",
		"  paragraph",
		"    text Read the",
		"    link terms url=" + strings.TrimSuffix(snap.URL, "/") + "/terms",
		"    text first.",
		"  form",
		"    textbox Email = a@b.c required=true",
		"    checkbox Agree checked=false",
		"    button Send disabled=true",
	}, "\n")
	if got := outlineText(snap); got != want {
		t.Errorf("outline:\n%s\nwant:\n%s", got, want)
	}
	if snap.Title != "Form" || !strings.HasPrefix(snap.URL, "http://127.0.0.1:") {
		t.Errorf("url %q title %q, want the served page titled Form", snap.URL, snap.Title)
	}
}

func TestSnapshotGivesEachElementItsOwnRef(t *testing.T) {
	t.Parallel()
	snap, _ := snapshotOf(t, formPage, browser.SnapshotRequest{})
	seen := map[string]bool{}
	var walk func([]*browser.Node)
	walk = func(nodes []*browser.Node) {
		for _, n := range nodes {
			if n.Role != "text" && (n.Ref == "" || seen[n.Ref]) {
				t.Errorf("%s %q has ref %q, want a fresh one", n.Role, n.Name, n.Ref)
			}
			seen[n.Ref] = true
			walk(n.Children)
		}
	}
	walk(snap.Nodes)
}

func TestSnapshotScopesToSelector(t *testing.T) {
	t.Parallel()
	snap, _ := snapshotOf(t, formPage, browser.SnapshotRequest{Selector: "#f"})
	want := "form\n  textbox Email = a@b.c required=true\n  checkbox Agree checked=false\n  button Send disabled=true"
	if got := outlineText(snap); got != want {
		t.Errorf("outline:\n%s\nwant:\n%s", got, want)
	}
}

func TestSnapshotSelectorErrors(t *testing.T) {
	t.Parallel()
	_, b := snapshotOf(t, formPage, browser.SnapshotRequest{})
	for sel, want := range map[string]string{
		"#missing":    "no element matches",
		"div[hidden]": "not in the accessibility tree",
		"[[":          "not a valid selector",
	} {
		_, err := b.Snapshot(t.Context(), browser.SnapshotRequest{Selector: sel})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("selector %q: err = %v, want %q", sel, err, want)
		}
	}
}
