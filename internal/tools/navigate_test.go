package tools_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeNavigator struct {
	got  []browser.NavRequest
	page browser.Page
	err  error
}

func (f *fakeNavigator) Navigate(_ context.Context, req browser.NavRequest) (browser.Page, error) {
	f.got = append(f.got, req)
	return f.page, f.err
}

func navigateSession(t *testing.T, nav tools.Navigator) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddNavigate(s, nav)
	return connect(t, s)
}

func TestNavigateDefaultsToGotoAndReturnsPage(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{page: browser.Page{URL: "http://x/final", Status: 200, Title: "Final"}}
	res := call(t, navigateSession(t, nav), "navigate", map[string]any{"url": "http://x/start", "tab": "t2"})

	got := decode[tools.NavigateOutput](t, res)
	want := tools.NavigateOutput{URL: "http://x/final", Status: 200, Title: "Final"}
	if got != want {
		t.Errorf("output = %+v, want %+v", got, want)
	}
	wantReq := browser.NavRequest{Tab: "t2", Action: browser.Goto, URL: "http://x/start"}
	if len(nav.got) != 1 || nav.got[0] != wantReq {
		t.Errorf("requests = %+v, want [%+v]", nav.got, wantReq)
	}
}

func TestNavigatePassesHistoryActions(t *testing.T) {
	t.Parallel()
	for _, action := range []browser.NavAction{browser.Back, browser.Forward, browser.Reload} {
		nav := &fakeNavigator{}
		decode[tools.NavigateOutput](t, call(t, navigateSession(t, nav), "navigate", map[string]any{"action": action}))
		if len(nav.got) != 1 || nav.got[0].Action != action {
			t.Errorf("%s: requests = %+v", action, nav.got)
		}
	}
}

func TestNavigateGotoWithoutURLFails(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{}
	res := call(t, navigateSession(t, nav), "navigate", map[string]any{})
	if !res.IsError || errorText(res) != "url is required for goto" {
		t.Errorf("result = %+v, want error about the missing url", res)
	}
	if len(nav.got) != 0 {
		t.Errorf("browser called with %+v", nav.got)
	}
}

func TestNavigateRejectsUnknownAction(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{}
	res := call(t, navigateSession(t, nav), "navigate", map[string]any{"action": "sideways"})
	if !res.IsError {
		t.Errorf("result = %+v, want error", res)
	}
	if len(nav.got) != 0 {
		t.Errorf("browser called with %+v", nav.got)
	}
}

func TestNavigateReportsBrowserError(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{err: errors.New("page load error net::ERR_NAME_NOT_RESOLVED")}
	res := call(t, navigateSession(t, nav), "navigate", map[string]any{"url": "http://nowhere.invalid/"})
	if !res.IsError || !strings.Contains(errorText(res), "ERR_NAME_NOT_RESOLVED") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}

func TestNavigateOmitsZeroStatus(t *testing.T) {
	t.Parallel()
	nav := &fakeNavigator{page: browser.Page{URL: "file:///x.html", Title: "X"}}
	res := call(t, navigateSession(t, nav), "navigate", map[string]any{"url": "file:///x.html"})
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T", res.StructuredContent)
	}
	if _, has := m["status"]; has {
		t.Errorf("structured content = %v, want no status", m)
	}
}
