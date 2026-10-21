//go:build acceptance

package acceptance_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func title(agent *dsl.Agent) any {
	return agent.Eval("document.title").Value
}

func TestAgentWorksInTwoTabs(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/"))
	home, second := `t1 "Fixture Home" `+site.URL("/"), `t2 "Second Page" `+site.URL("/second.html")
	if got, want := agent.OpenTab(site.URL("/second.html")), []string{home, second + " (current)"}; !slices.Equal(got, want) {
		t.Fatalf("tabs after open = %q, want %q", got, want)
	}
	if got := title(agent); got != "Second Page" {
		t.Errorf("the opened tab shows %v, want Second Page", got)
	}
	if got, want := agent.SelectTab("t1"), []string{home + " (current)", second}; !slices.Equal(got, want) {
		t.Fatalf("tabs after select = %q, want %q", got, want)
	}
	if got := title(agent); got != "Fixture Home" {
		t.Errorf("the selected tab shows %v, want Fixture Home", got)
	}
	if got, want := agent.CloseTab("t2"), []string{home + " (current)"}; !slices.Equal(got, want) {
		t.Errorf("tabs after close = %q, want %q", got, want)
	}
}

func TestAgentFollowsALinkThatOpensANewTab(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/terms.html"))
	if after := agent.Click(dsl.CSS("#terms")); !slices.Equal(after.Opened, []dsl.Opened{{Tab: "t2", URL: site.URL("/second.html")}}) {
		t.Errorf("click on a link to a new tab opened %+v, want t2 at second.html", after.Opened)
	}
	want := []string{`t1 "Sign Up" ` + site.URL("/terms.html") + " (current)", `t2 "Second Page" ` + site.URL("/second.html")}
	if got := agent.Tabs(2); !slices.Equal(got, want) {
		t.Fatalf("tabs after a link to a new tab = %q, want %q", got, want)
	}
	agent.SelectTab("t2")
	if got := title(agent); got != "Second Page" {
		t.Errorf("the tab the link opened shows %v, want Second Page", got)
	}
}

func TestAgentCannotCloseTheLastTab(t *testing.T) {
	t.Parallel()
	agent := dsl.NewAgent(t)
	if err := agent.TryCloseTab("t1"); err == nil || !strings.Contains(err.Error(), "t1 is the last tab") {
		t.Errorf("closing the only tab: %v, want an error that it is the last", err)
	}
}
