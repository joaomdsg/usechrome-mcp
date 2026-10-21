package tools_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeStyler struct {
	got []browser.StylesRequest
	css browser.Styles
	err error
}

func (f *fakeStyler) Styles(_ context.Context, req browser.StylesRequest) (browser.Styles, error) {
	f.got = append(f.got, req)
	return f.css, f.err
}

func stylesSession(t *testing.T, st tools.Styler) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddStyles(s, st)
	return connect(t, s)
}

func TestStylesPassesTheElementAndProperties(t *testing.T) {
	t.Parallel()
	st := &fakeStyler{}
	res := call(t, stylesSession(t, st), "styles", map[string]any{"ref": "e3", "properties": []string{"gap", "color"}, "tab": "t2"})
	want := browser.StylesRequest{Tab: "t2", Ref: "e3", Properties: []string{"gap", "color"}}
	if res.IsError || len(st.got) != 1 || !reflect.DeepEqual(st.got[0], want) {
		t.Errorf("browser got %+v (error %q), want %+v", st.got, errorText(res), want)
	}
}

func TestStylesReturnsRulesBoxAndComputedValues(t *testing.T) {
	t.Parallel()
	st := &fakeStyler{css: browser.Styles{
		Element:  "div#card.card",
		Box:      browser.Box{X: 4, Y: 8, Width: 316, Height: 20},
		Computed: map[string]string{"color": "rgb(255, 0, 0)"},
		Rules: []browser.Rule{
			{Selector: "style attribute", Declarations: []string{"color: red"}},
			{Selector: ".card", Media: "(min-width: 100px)", Source: "http://x/app.css:2", Declarations: []string{"width: 300px"}},
			{Selector: ".empty", Source: "http://x/app.css:9"},
		},
	}}
	got := decode[tools.StylesOutput](t, call(t, stylesSession(t, st), "styles", map[string]any{"selector": "#card"}))
	want := tools.StylesOutput{
		Element:  "div#card.card",
		Box:      tools.BoxOutput{X: 4, Y: 8, Width: 316, Height: 20},
		Computed: map[string]string{"color": "rgb(255, 0, 0)"},
		Rules: []tools.RuleOutput{
			{Selector: "style attribute", Declarations: []string{"color: red"}},
			{Selector: ".card", Media: "(min-width: 100px)", Source: "http://x/app.css:2", Declarations: []string{"width: 300px"}},
			{Selector: ".empty", Source: "http://x/app.css:9", Declarations: []string{}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("output:\n got %+v\nwant %+v", got, want)
	}
}

func TestStylesNeedsExactlyOneElement(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{}, "styles needs a ref or a selector"},
		{map[string]any{"ref": "e1", "selector": "#a"}, "not both"},
	} {
		st := &fakeStyler{}
		res := call(t, stylesSession(t, st), "styles", tc.args)
		if !res.IsError || !strings.Contains(errorText(res), tc.want) || len(st.got) != 0 {
			t.Errorf("styles %v: %q, browser got %+v; want error %q before the browser", tc.args, errorText(res), st.got, tc.want)
		}
	}
}

func TestStylesReportsBrowserError(t *testing.T) {
	t.Parallel()
	st := &fakeStyler{err: errors.New(`no element matches "#nope"`)}
	res := call(t, stylesSession(t, st), "styles", map[string]any{"selector": "#nope"})
	if !res.IsError || !strings.Contains(errorText(res), "no element matches") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
