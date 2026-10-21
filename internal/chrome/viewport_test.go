package chrome_test

import (
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

func viewport(t *testing.T, b *chrome.Browser, req browser.ViewportRequest) browser.Viewport {
	t.Helper()
	vp, err := b.Viewport(t.Context(), req)
	if err != nil {
		t.Fatalf("viewport %+v: %v", req, err)
	}
	return vp
}

func evalValue(t *testing.T, b *chrome.Browser, expr string) string {
	t.Helper()
	r, err := b.Eval(t.Context(), browser.EvalRequest{Expression: expr})
	if err != nil {
		t.Fatalf("eval %s: %v", expr, err)
	}
	return string(r.Value)
}

func TestViewportOverrideSetsSizeAndScale(t *testing.T) {
	t.Parallel()
	got := viewport(t, newBrowser(t), browser.ViewportRequest{Width: 375, Height: 667, Scale: 2})
	if want := (browser.Viewport{Width: 375, Height: 667, Scale: 2}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestViewportWithoutChangesOnlyReads(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	viewport(t, b, browser.ViewportRequest{Width: 500, Height: 400})
	if got := viewport(t, b, browser.ViewportRequest{}); got.Width != 500 || got.Height != 400 {
		t.Errorf("got %+v, want the 500x400 override kept", got)
	}
}

func TestViewportResetRestoresWindowSize(t *testing.T) {
	t.Parallel()
	b := newBrowser(t)
	before := viewport(t, b, browser.ViewportRequest{})
	viewport(t, b, browser.ViewportRequest{Width: 320, Height: 480, Scale: 3, Mobile: true})
	if got := viewport(t, b, browser.ViewportRequest{Reset: true}); got != before {
		t.Errorf("after reset got %+v, want %+v", got, before)
	}
	if touch := evalValue(t, b, "navigator.maxTouchPoints"); touch != "0" {
		t.Errorf("maxTouchPoints after reset = %s, want 0", touch)
	}
}

func TestViewportMobileEmulatesTouch(t *testing.T) {
	t.Parallel()
	srv := site(t)
	b := newBrowser(t)
	viewport(t, b, browser.ViewportRequest{Width: 390, Height: 844, Mobile: true})
	media := "[navigator.maxTouchPoints > 0, matchMedia('(pointer: coarse)').matches, matchMedia('(hover: none)').matches]"
	if got := evalValue(t, b, media); got != "[true,true,true]" {
		t.Errorf("touch points, coarse pointer, no hover = %s, want all true", got)
	}
	// Chrome exposes touch events only to documents loaded after the switch.
	nav(t, b, browser.NavRequest{URL: srv.URL + "/second"})
	if got := evalValue(t, b, "'ontouchstart' in window"); got != "true" {
		t.Errorf("ontouchstart after navigation = %s, want true", got)
	}
}

func TestViewportOverrideSurvivesNavigation(t *testing.T) {
	t.Parallel()
	srv := site(t)
	b := newBrowser(t)
	viewport(t, b, browser.ViewportRequest{Width: 600, Height: 300})
	nav(t, b, browser.NavRequest{URL: srv.URL + "/second"})
	if got := evalValue(t, b, "innerWidth + 'x' + innerHeight"); got != `"600x300"` {
		t.Errorf("size after navigation = %s, want 600x300", got)
	}
}
