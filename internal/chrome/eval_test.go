package chrome_test

import (
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

func TestEvalReportsTypeAndJSONValue(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	cases := []struct{ expr, typ, value string }{
		{"undefined", "undefined", ""},
		{"null", "null", "null"},
		{"1 + 1", "number", "2"},
		{"'a' + 'b'", "string", `"ab"`},
		{"true", "boolean", "true"},
		{"[1, 'x']", "array", `[1,"x"]`},
		{"({a: {b: 1}})", "object", `{"a":{"b":1}}`},
		{"NaN", "number", `"NaN"`},
		{"-0", "number", `"-0"`},
		{"12n", "bigint", `"12n"`},
		{"Symbol('s')", "symbol", `"Symbol(s)"`},
		{"Promise.resolve(7)", "number", "7"},
		{"(() => { const a = {}; a.self = a; return a })()", "object", `"Object"`},
		{"document.body", "node", `"body"`},
		{"$$('body').length", "number", "1"},
	}
	for _, c := range cases {
		got, err := b.Eval(t.Context(), browser.EvalRequest{Expression: c.expr})
		if err != nil {
			t.Errorf("eval %s: %v", c.expr, err)
			continue
		}
		if got.Type != c.typ || string(got.Value) != c.value {
			t.Errorf("eval %s = %s %s, want %s %s", c.expr, got.Type, got.Value, c.typ, c.value)
		}
	}
}

func TestEvalDescribesFunctions(t *testing.T) {
	t.Parallel()
	got, err := newBrowser(t).Eval(t.Context(), browser.EvalRequest{Expression: "(function f(x) { return x })"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "function" || !strings.Contains(string(got.Value), "return x") {
		t.Errorf("got %s %s, want a function with its source", got.Type, got.Value)
	}
}

func TestEvalReportsThrownError(t *testing.T) {
	t.Parallel()
	_, err := newBrowser(t).Eval(t.Context(), browser.EvalRequest{Expression: "throw new Error('boom')"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("err = %v, want the thrown message", err)
	}
}

func TestEvalReportsRejectedPromise(t *testing.T) {
	t.Parallel()
	_, err := newBrowser(t).Eval(t.Context(), browser.EvalRequest{Expression: "Promise.reject(new Error('nope'))"})
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("err = %v, want the rejection message", err)
	}
}

func TestEvalRunsInTheNavigatedPage(t *testing.T) {
	t.Parallel()
	srv := site(t)
	b := newBrowser(t)
	nav(t, b, browser.NavRequest{URL: srv.URL + "/second"})
	got, err := b.Eval(t.Context(), browser.EvalRequest{Expression: "document.title"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Value) != `"Second"` {
		t.Errorf("document.title = %s, want \"Second\"", got.Value)
	}
}
