package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeEvaluator struct {
	got    []browser.EvalRequest
	result browser.EvalResult
	err    error
}

func (f *fakeEvaluator) Eval(_ context.Context, req browser.EvalRequest) (browser.EvalResult, error) {
	f.got = append(f.got, req)
	return f.result, f.err
}

func evalSession(t *testing.T, ev tools.Evaluator) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddEval(s, ev)
	return connect(t, s)
}

func TestEvalPassesExpressionAndReturnsValue(t *testing.T) {
	t.Parallel()
	ev := &fakeEvaluator{result: browser.EvalResult{Type: "object", Value: json.RawMessage(`{"a":[1,"x"]}`)}}
	out := decode[tools.EvalOutput](t, call(t, evalSession(t, ev), "eval", map[string]any{"expression": "f()", "tab": "t2"}))
	if out.Type != "object" {
		t.Errorf("type = %q, want object", out.Type)
	}
	if b, _ := json.Marshal(out.Value); string(b) != `{"a":[1,"x"]}` {
		t.Errorf("value = %s", b)
	}
	if want := (browser.EvalRequest{Tab: "t2", Expression: "f()"}); len(ev.got) != 1 || ev.got[0] != want {
		t.Errorf("browser called with %+v, want %+v", ev.got, want)
	}
}

func TestEvalReturnsNullValueForUndefined(t *testing.T) {
	t.Parallel()
	ev := &fakeEvaluator{result: browser.EvalResult{Type: "undefined"}}
	out := decode[tools.EvalOutput](t, call(t, evalSession(t, ev), "eval", map[string]any{"expression": "void 0"}))
	if out.Type != "undefined" || out.Value != nil {
		t.Errorf("got %+v, want undefined with no value", out)
	}
}

func TestEvalWithoutExpressionFails(t *testing.T) {
	t.Parallel()
	ev := &fakeEvaluator{}
	res := call(t, evalSession(t, ev), "eval", map[string]any{})
	if !res.IsError || len(ev.got) != 0 {
		t.Errorf("result = %+v, browser calls = %d; want a validation error and no call", res, len(ev.got))
	}
}

func TestEvalReportsBrowserError(t *testing.T) {
	t.Parallel()
	ev := &fakeEvaluator{err: errors.New(`exception "Uncaught" (0:0): Error: boom`)}
	res := call(t, evalSession(t, ev), "eval", map[string]any{"expression": "boom()"})
	if !res.IsError || !strings.Contains(errorText(res), "boom") {
		t.Errorf("result = %+v, want the thrown message", res)
	}
}
