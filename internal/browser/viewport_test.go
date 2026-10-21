package browser_test

import (
	"testing"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

func TestViewportRequestOverridesWhenAnySettingIsGiven(t *testing.T) {
	t.Parallel()
	cases := []struct {
		req  browser.ViewportRequest
		want bool
	}{
		{browser.ViewportRequest{}, false},
		{browser.ViewportRequest{Tab: "t1", Reset: true}, false},
		{browser.ViewportRequest{Width: 1}, true},
		{browser.ViewportRequest{Height: 1}, true},
		{browser.ViewportRequest{Scale: 0.5}, true},
		{browser.ViewportRequest{Mobile: true}, true},
	}
	for _, c := range cases {
		if got := c.req.Overrides(); got != c.want {
			t.Errorf("%+v.Overrides() = %v, want %v", c.req, got, c.want)
		}
	}
}
