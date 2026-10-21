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

type fakeViewporter struct {
	got []browser.ViewportRequest
	vp  browser.Viewport
	err error
}

func (f *fakeViewporter) Viewport(_ context.Context, req browser.ViewportRequest) (browser.Viewport, error) {
	f.got = append(f.got, req)
	return f.vp, f.err
}

func viewportSession(t *testing.T, vp tools.Viewporter) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddViewport(s, vp)
	return connect(t, s)
}

func TestViewportPassesOverrideAndReturnsMeasuredSize(t *testing.T) {
	t.Parallel()
	vp := &fakeViewporter{vp: browser.Viewport{Width: 980, Height: 1734, Scale: 3}}
	args := map[string]any{"width": 390, "height": 844, "scale": 3, "mobile": true, "tab": "t2"}
	out := decode[tools.ViewportOutput](t, call(t, viewportSession(t, vp), "viewport", args))
	if want := (tools.ViewportOutput{Width: 980, Height: 1734, Scale: 3}); out != want {
		t.Errorf("got %+v, want %+v", out, want)
	}
	want := browser.ViewportRequest{Tab: "t2", Width: 390, Height: 844, Scale: 3, Mobile: true}
	if len(vp.got) != 1 || vp.got[0] != want {
		t.Errorf("browser called with %+v, want %+v", vp.got, want)
	}
}

func TestViewportPassesReset(t *testing.T) {
	t.Parallel()
	vp := &fakeViewporter{}
	decode[tools.ViewportOutput](t, call(t, viewportSession(t, vp), "viewport", map[string]any{"reset": true}))
	if len(vp.got) != 1 || !vp.got[0].Reset {
		t.Errorf("browser called with %+v, want a reset", vp.got)
	}
}

func TestViewportResetWithOverrideFails(t *testing.T) {
	t.Parallel()
	vp := &fakeViewporter{}
	res := call(t, viewportSession(t, vp), "viewport", map[string]any{"reset": true, "mobile": true})
	if !res.IsError || len(vp.got) != 0 {
		t.Errorf("result = %+v, browser calls = %d; want an error and no call", res, len(vp.got))
	}
}

func TestViewportRejectsNegativeSize(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"width", "height", "scale"} {
		vp := &fakeViewporter{}
		res := call(t, viewportSession(t, vp), "viewport", map[string]any{p: -1})
		if !res.IsError || len(vp.got) != 0 {
			t.Errorf("%s=-1: result = %+v, browser calls = %d; want a validation error", p, res, len(vp.got))
		}
	}
}

func TestViewportReportsBrowserError(t *testing.T) {
	t.Parallel()
	vp := &fakeViewporter{err: errors.New("unknown tab \"t9\"")}
	res := call(t, viewportSession(t, vp), "viewport", map[string]any{"tab": "t9"})
	if !res.IsError || !strings.Contains(errorText(res), "t9") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
