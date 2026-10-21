//go:build acceptance

package acceptance_test

import (
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestAgentFillsInAndSendsTheForm(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	outline := agent.Snapshot("")
	agent.Type(dsl.Ref(outline.Ref("textbox", "Email")), "ada@example.com")
	agent.Select(dsl.Ref(outline.Ref("combobox", "Plan")), "Pro")
	agent.Click(dsl.Ref(outline.Ref("checkbox", "Agree")))
	after := agent.Click(dsl.Ref(outline.Ref("button", "Send")))
	if after.Navigated || after.Title != "Sign up" {
		t.Errorf("after sending: %+v, want to stay on the form", after)
	}
	if got := agent.Snapshot("[role=status]"); !strings.Contains(string(got), "Thanks, ada@example.com, on the Pro plan") {
		t.Errorf("status after sending:\n%s", got)
	}
}

func TestPressingEnterInAFieldSendsTheForm(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	agent.Type(dsl.CSS("#email"), "ada@example.com")
	agent.Press("Enter")
	if got := agent.Snapshot("[role=status]"); !strings.Contains(string(got), "Please agree first") {
		t.Errorf("status after Enter without agreeing:\n%s", got)
	}
}

func TestClickingALinkOpensItsPage(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/index.html"))
	after := agent.Click(dsl.Ref(agent.Snapshot("").Ref("link", "Second page")))
	if !after.Navigated || after.URL != site.URL("/second.html") {
		t.Errorf("after clicking the link: %+v, want the second page", after)
	}
}

func TestClickingAStaleRefAsksForANewSnapshot(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/index.html"))
	link := agent.Snapshot("").Ref("link", "Second page")
	agent.Click(dsl.Ref(link))
	if err := agent.TryClick(dsl.Ref(link)); err == nil || !strings.Contains(err.Error(), "take a new snapshot") {
		t.Errorf("click on a ref from the previous page: err = %v, want to take a new snapshot", err)
	}
}
