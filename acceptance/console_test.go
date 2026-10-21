//go:build acceptance

package acceptance_test

import (
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestAgentTracesAClickToTheErrorItThrows(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/shop.html"))
	agent.Click(dsl.CSS("#checkout"))
	want := `error: Failed to load resource: the server responded with a status of 404 (Not Found)
    at ` + site.URL("/missing-logo.png") + `
exception: Uncaught TypeError: Cannot read properties of undefined (reading 'items')
    at checkout (` + site.URL("/shop.js") + `:4:15)
`
	if got := agent.Console("error"); got != want {
		t.Errorf("errors after the click:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleShowsWhatThePageLogged(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/shop.html"))
	want := `error: Failed to load resource: the server responded with a status of 404 (Not Found)
    at ` + site.URL("/missing-logo.png") + `
log: shop ready {"items":0}
    at ` + site.URL("/shop.js") + `:8:11
`
	if got := agent.Console(""); got != want {
		t.Errorf("console:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleStartsEmptyOnTheNextPage(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/shop.html"))
	agent.Open(site.URL("/index.html"))
	if got := agent.Console("debug"); got != "no console messages at level debug or above\n" {
		t.Errorf("console of the next page:\n%s", got)
	}
}
