//go:build acceptance

package acceptance_test

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(os.DirFS("testdata"), name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAgentComparesTheServedPageWithTheLiveOne(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/cart.html"))
	agent.Click(dsl.CSS("#add"))
	counted := agent.Eval(`new Promise(done => {
		const check = () => document.getElementById("count").textContent === "1" ? done(true) : setTimeout(check, 10);
		check();
	})`)
	if counted.Value != true {
		t.Fatalf("the live count = %+v, want it to reach 1", counted)
	}
	html := fixture(t, "cart.html")
	want := fmt.Sprintf("%s (text/html, %dB):\n%s", site.URL("/cart.html"), len(html), html)
	if got := agent.Source("", 0); got != want || !strings.Contains(got, `<output id="count">0</output>`) {
		t.Errorf("source of the page:\n%s\nwant the page as served:\n%s", got, want)
	}
}

func TestAgentReadsAScriptThePageLoaded(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/shop.html"))
	js := fixture(t, "shop.js")
	want := fmt.Sprintf("%s (text/javascript, %dB):\n%s", site.URL("/shop.js"), len(js), js)
	if got := agent.Source(site.URL("/shop.js"), 0); got != want {
		t.Errorf("source of shop.js:\n%s\nwant:\n%s", got, want)
	}
}

func TestSourceOfAFileThePageDidNotLoadFails(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/index.html"))
	if err := agent.TrySource(site.URL("/shop.js")); err == nil || !strings.Contains(err.Error(), "shop.js") {
		t.Errorf("source of a script index.html does not load: %v, want an error naming it", err)
	}
}
