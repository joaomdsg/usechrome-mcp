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

type fakeTabser struct {
	got  []browser.TabsRequest
	tabs []browser.Tab
	err  error
}

func (f *fakeTabser) Tabs(_ context.Context, req browser.TabsRequest) ([]browser.Tab, error) {
	f.got = append(f.got, req)
	return f.tabs, f.err
}

func tabsSession(t *testing.T, ts tools.Tabser) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddTabs(s, ts)
	return connect(t, s)
}

func TestTabsListsEachTabAndMarksTheCurrent(t *testing.T) {
	t.Parallel()
	f := &fakeTabser{tabs: []browser.Tab{
		{ID: "t1", Title: "Home", URL: "http://x/"},
		{ID: "t3", Title: `Say "hi"`, URL: "http://x/hi", Current: true},
		{ID: "t4", URL: "about:blank"},
	}}
	res := call(t, tabsSession(t, f), "tabs", map[string]any{})
	want := "t1 \"Home\" http://x/\n" +
		"t3 \"Say \\\"hi\\\"\" http://x/hi (current)\n" +
		"t4 \"\" about:blank\n"
	if got := errorText(res); res.IsError || got != want {
		t.Errorf("tabs =\n%s\nwant\n%s", got, want)
	}
	if want := []browser.TabsRequest{{}}; !slices.Equal(f.got, want) {
		t.Errorf("requests = %+v, want %+v", f.got, want)
	}
}

func TestTabsPassesTheActionAndItsArguments(t *testing.T) {
	t.Parallel()
	f := &fakeTabser{}
	cs := tabsSession(t, f)
	for _, args := range []map[string]any{
		{"action": "list"},
		{"action": "open", "url": "http://x/"},
		{"action": "select", "tab": "t2"},
		{"action": "close"},
		{"action": "close", "tab": "t2"},
	} {
		if res := call(t, cs, "tabs", args); res.IsError {
			t.Errorf("tabs %v failed: %s", args, errorText(res))
		}
	}
	want := []browser.TabsRequest{
		{Action: browser.ListTabs},
		{Action: browser.OpenTab, URL: "http://x/"},
		{Action: browser.SelectTab, Tab: "t2"},
		{Action: browser.CloseTab},
		{Action: browser.CloseTab, Tab: "t2"},
	}
	if !slices.Equal(f.got, want) {
		t.Errorf("requests = %+v, want %+v", f.got, want)
	}
}

func TestTabsRejectsArgumentsTheActionDoesNotTake(t *testing.T) {
	t.Parallel()
	f := &fakeTabser{}
	cs := tabsSession(t, f)
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"url": "http://x/"}, "url goes with the open action only"},
		{map[string]any{"action": "select", "url": "http://x/"}, "url goes with the open action only"},
		{map[string]any{"tab": "t2"}, "tab goes with the select and close actions only"},
		{map[string]any{"action": "open", "tab": "t2"}, "tab goes with the select and close actions only"},
		{map[string]any{"action": "shuffle"}, "shuffle"},
	} {
		if res := call(t, cs, "tabs", tc.args); !res.IsError || !strings.Contains(errorText(res), tc.want) {
			t.Errorf("tabs %v = %+v, want an error with %q", tc.args, res, tc.want)
		}
	}
	if len(f.got) != 0 {
		t.Errorf("requests = %+v, want none", f.got)
	}
}

func TestTabsReportsBrowserError(t *testing.T) {
	t.Parallel()
	res := call(t, tabsSession(t, &fakeTabser{err: errors.New("t1 is the last tab")}), "tabs", map[string]any{"action": "close"})
	if !res.IsError || !strings.Contains(errorText(res), "t1 is the last tab") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
