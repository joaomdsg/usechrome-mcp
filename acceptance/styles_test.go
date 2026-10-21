//go:build acceptance

package acceptance_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestAgentTracesAPhoneOnlyStyleToItsMediaRule(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/responsive.html"))
	agent.Emulate(dsl.Phone)
	css := agent.Styles(dsl.CSS("body"))
	if got := css.Computed["background-color"]; got != "rgb(0, 128, 0)" {
		t.Errorf("computed background-color = %q, want the phone green", got)
	}
	want := []dsl.Rule{
		{Selector: "body", Media: "(max-width: 600px)", Source: site.URL("/responsive.css") + ":6", Declarations: []string{"background: rgb(0, 128, 0)"}},
		{Selector: "body", Source: site.URL("/responsive.css") + ":1", Declarations: []string{"background: rgb(255, 255, 255)"}},
	}
	if !reflect.DeepEqual(css.Rules, want) {
		t.Errorf("rules:\n got %+v\nwant %+v", css.Rules, want)
	}
}

func TestStylesReadTheElementASnapshotRefNames(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	css := agent.Styles(dsl.Ref(agent.Snapshot("").Ref("button", "Send")), "cursor")
	if css.Element != "button#send" || !reflect.DeepEqual(css.Computed, map[string]string{"cursor": "default"}) {
		t.Errorf("styles of the Send ref: %s %v, want button#send with only cursor computed", css.Element, css.Computed)
	}
}

func TestStylesOfMissingElementFails(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/form.html"))
	if err := agent.TryStyles(dsl.CSS("#nope")); err == nil || !strings.Contains(err.Error(), "no element matches") {
		t.Errorf("styles of #nope: err = %v, want no element matches", err)
	}
}
