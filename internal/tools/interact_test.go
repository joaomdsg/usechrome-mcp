package tools_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeInteracter struct {
	got []browser.InteractRequest
	r   browser.Interaction
	err error
}

func (f *fakeInteracter) Interact(_ context.Context, req browser.InteractRequest) (browser.Interaction, error) {
	f.got = append(f.got, req)
	return f.r, f.err
}

func interactSession(t *testing.T, in tools.Interacter) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddInteract(s, in)
	return connect(t, s)
}

func TestInteractPassesEachActionsArgument(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args map[string]any
		want browser.InteractRequest
	}{
		{map[string]any{"action": "click", "ref": "e3", "tab": "t2"}, browser.InteractRequest{Tab: "t2", Action: browser.Click, Ref: "e3"}},
		{map[string]any{"action": "hover", "selector": "#m"}, browser.InteractRequest{Action: browser.Hover, Selector: "#m"}},
		{map[string]any{"action": "type", "ref": "e4", "text": "Ada"}, browser.InteractRequest{Action: browser.Type, Ref: "e4", Text: "Ada"}},
		{map[string]any{"action": "type", "ref": "e4"}, browser.InteractRequest{Action: browser.Type, Ref: "e4"}},
		{map[string]any{"action": "type", "text": "Ada"}, browser.InteractRequest{Action: browser.Type, Text: "Ada"}},
		{map[string]any{"action": "press", "key": "Enter"}, browser.InteractRequest{Action: browser.Press, Text: "Enter"}},
		{map[string]any{"action": "select", "selector": "#plan", "option": "Pro"}, browser.InteractRequest{Action: browser.Select, Selector: "#plan", Text: "Pro"}},
	} {
		in := &fakeInteracter{}
		res := call(t, interactSession(t, in), "interact", tc.args)
		if res.IsError || !slices.Equal(in.got, []browser.InteractRequest{tc.want}) {
			t.Errorf("interact %v: browser got %+v (error %q), want %+v", tc.args, in.got, errorText(res), tc.want)
		}
	}
}

func TestInteractReturnsThePageAfter(t *testing.T) {
	t.Parallel()
	in := &fakeInteracter{r: browser.Interaction{URL: "http://x/next", Title: "Next", Navigated: true, Dialogs: []string{"alert: Saved"}}}
	got := decode[tools.InteractOutput](t, call(t, interactSession(t, in), "interact", map[string]any{"action": "click", "ref": "e1"}))
	want := tools.InteractOutput{URL: "http://x/next", Title: "Next", Navigated: true, Dialogs: []string{"alert: Saved"}}
	if got.URL != want.URL || got.Title != want.Title || !got.Navigated || !slices.Equal(got.Dialogs, want.Dialogs) {
		t.Errorf("output = %+v, want %+v", got, want)
	}
}

func TestInteractReturnsTheTabsTheActionOpened(t *testing.T) {
	t.Parallel()
	in := &fakeInteracter{r: browser.Interaction{URL: "http://x/", Opened: []browser.Tab{{ID: "t2", URL: "http://x/terms"}, {ID: "t3", URL: "http://x/help"}}}}
	got := decode[tools.InteractOutput](t, call(t, interactSession(t, in), "interact", map[string]any{"action": "click", "ref": "e1"}))
	if want := []tools.OpenedTab{{Tab: "t2", URL: "http://x/terms"}, {Tab: "t3", URL: "http://x/help"}}; !slices.Equal(got.Opened, want) {
		t.Errorf("opened = %+v, want %+v", got.Opened, want)
	}
}

func TestInteractRejectsArgumentsTheActionDoesNotTake(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"action": "click"}, "click needs a ref or a selector"},
		{map[string]any{"action": "select", "option": "Pro"}, "select needs a ref or a selector"},
		{map[string]any{"action": "click", "ref": "e1", "selector": "#a"}, "not both"},
		{map[string]any{"action": "click", "ref": "e1", "text": "x"}, "click takes no text"},
		{map[string]any{"action": "type", "ref": "e1", "key": "Enter"}, "type takes no key"},
		{map[string]any{"action": "press", "key": "Enter", "option": "a"}, "press takes no option"},
		{map[string]any{"action": "press"}, "press needs key"},
		{map[string]any{"action": "type"}, "type needs text"},
		{map[string]any{"action": "select", "ref": "e1"}, "select needs option"},
		{map[string]any{"action": "drag", "ref": "e1"}, "action"},
		{map[string]any{"ref": "e1"}, "action"},
	} {
		in := &fakeInteracter{}
		res := call(t, interactSession(t, in), "interact", tc.args)
		if !res.IsError || !strings.Contains(errorText(res), tc.want) || len(in.got) != 0 {
			t.Errorf("interact %v: %q, browser got %+v; want error %q before the browser", tc.args, errorText(res), in.got, tc.want)
		}
	}
}

func TestInteractReportsBrowserError(t *testing.T) {
	t.Parallel()
	in := &fakeInteracter{err: errors.New(`"#send" is covered by div#lid`)}
	res := call(t, interactSession(t, in), "interact", map[string]any{"action": "click", "selector": "#send"})
	if !res.IsError || !strings.Contains(errorText(res), "covered by div#lid") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
