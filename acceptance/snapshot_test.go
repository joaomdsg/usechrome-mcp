//go:build acceptance

package acceptance_test

import (
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestSnapshotShowsWhatTheUserSees(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	outline := agent.Snapshot("")
	if want := "url: " + site.URL("/form.html") + "\ntitle: Sign up\n"; !strings.HasPrefix(string(outline), want) {
		t.Errorf("outline starts %.80q, want %q", outline, want)
	}
	refs := map[string]bool{}
	for _, el := range [][2]string{{"heading", "Sign up"}, {"textbox", "Email"}, {"checkbox", "Agree"}, {"button", "Send"}} {
		ref := outline.Ref(el[0], el[1])
		if ref == "" || refs[ref] {
			t.Errorf("%s %q has ref %q, want its own ref in:\n%s", el[0], el[1], ref, outline)
		}
		refs[ref] = true
	}
	if strings.Contains(string(outline), "Secret") {
		t.Errorf("outline shows the hidden Secret button:\n%s", outline)
	}
}

func TestSnapshotShowsTheCurrentState(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	agent.Eval(`email.value = "ada@example.com"; agree.checked = true; send.disabled = true`)
	outline := agent.Snapshot("")
	for _, want := range []struct{ role, name, has string }{
		{"textbox", "Email", `: "ada@example.com"`},
		{"checkbox", "Agree", "[checked]"},
		{"button", "Send", "[disabled]"},
	} {
		if line := outline.Line(want.role, want.name); !strings.Contains(line, want.has) {
			t.Errorf("%s %q line = %q, want %s", want.role, want.name, line, want.has)
		}
	}
}

func TestSnapshotOfPartOfThePage(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	outline := agent.Snapshot("form")
	if outline.Ref("button", "Send") == "" || outline.Ref("heading", "Sign up") != "" {
		t.Errorf("outline of the form:\n%s\nwant the Send button without the page heading", outline)
	}
}

func TestSnapshotOfMissingElementFails(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	if err := agent.TrySnapshot("#nope"); err == nil || !strings.Contains(err.Error(), "no element matches") {
		t.Errorf("snapshot of #nope: err = %v, want no element matches", err)
	}
}
