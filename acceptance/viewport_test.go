//go:build acceptance

package acceptance_test

import (
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func TestPhoneViewportAppliesMobileStyles(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/responsive.html"))
	if got := agent.Emulate(dsl.Phone); got != (dsl.Viewport{Width: 390, Height: 844, Scale: 3}) {
		t.Errorf("emulate phone = %+v", got)
	}
	if got := agent.Eval("getComputedStyle(document.body).backgroundColor"); got.Value != "rgb(0, 128, 0)" {
		t.Errorf("body background = %v, want the max-width: 600px rule", got.Value)
	}
}

func TestPhoneViewportWithoutMetaTagLaysOutWide(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Emulate(dsl.Phone)
	agent.Open(site.URL("/index.html"))
	if got := agent.Viewport(); got.Width != 980 {
		t.Errorf("viewport = %+v, want the 980px layout a phone gives pages without a meta viewport tag", got)
	}
}

func TestResetViewportRestoresWindowSize(t *testing.T) {
	t.Parallel()
	agent := dsl.NewAgent(t)
	before := agent.Viewport()
	agent.Emulate(dsl.Device{Width: 300, Height: 200})
	if got := agent.ResetViewport(); got != before {
		t.Errorf("after reset = %+v, want %+v", got, before)
	}
}
