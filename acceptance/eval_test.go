//go:build acceptance

package acceptance_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestEvalReadsTheOpenPage(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/index.html"))
	got := agent.Eval("({heading: $('h1').textContent, links: $$('a').map(a => a.getAttribute('href'))})")
	want := dsl.Value{Type: "object", Value: map[string]any{"heading": "Fixture Home", "links": []any{"second.html"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestEvalChangesPersistInTheTab(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/index.html"))
	agent.Eval("document.title = 'Renamed'")
	if got := agent.Eval("document.title"); got.Value != "Renamed" {
		t.Errorf("title = %+v, want Renamed", got)
	}
}

func TestEvalAwaitsPromise(t *testing.T) {
	t.Parallel()
	got := dsl.NewAgent(t).Eval("new Promise(r => setTimeout(() => r(42), 50))")
	if got.Type != "number" || got.Value != 42.0 {
		t.Errorf("got %+v, want number 42", got)
	}
}

func TestEvalDescribesWhatJSONCannotHold(t *testing.T) {
	t.Parallel()
	got := dsl.NewAgent(t).Eval("2n ** 64n")
	if want := (dsl.Value{Type: "bigint", Value: "18446744073709551616n"}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestEvalReportsThrownError(t *testing.T) {
	t.Parallel()
	err := dsl.NewAgent(t).TryEval("throw new TypeError('bad input')")
	if err == nil || !strings.Contains(err.Error(), "TypeError: bad input") {
		t.Errorf("err = %v, want the thrown error", err)
	}
}
