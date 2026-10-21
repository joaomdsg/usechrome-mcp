//go:build acceptance

package acceptance_test

import (
	"image"
	"strings"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/dsl"
)

func isBlue(img image.Image, x, y int) bool {
	r, g, b, _ := img.At(x, y).RGBA()
	return r < 0x1000 && g < 0x1000 && b > 0xf000
}

func TestScreenshotShowsTheViewportAtDeviceScale(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/long.html"))
	agent.Emulate(dsl.Device{Width: 400, Height: 300, Scale: 2})
	shot := agent.Screenshot(dsl.Capture{})
	if b := shot.Image.Bounds(); b.Dx() != 800 || b.Dy() != 600 || shot.Width != 800 || shot.Height != 600 {
		t.Errorf("image %v, reported %dx%d; want 800x600", b, shot.Width, shot.Height)
	}
	if shot.MIMEType != "image/png" || isBlue(shot.Image, 10, 590) {
		t.Errorf("type %s; want a PNG of the top of the page without the footer", shot.MIMEType)
	}
}

func TestFullPageScreenshotShowsTheFooter(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/long.html"))
	agent.Emulate(dsl.Device{Width: 400, Height: 300, Scale: 1})
	shot := agent.Screenshot(dsl.Capture{FullPage: true})
	if shot.Height != 2400 || !isBlue(shot.Image, 200, 2350) {
		t.Errorf("full page %dx%d; want 2400 tall with the blue footer at the bottom", shot.Width, shot.Height)
	}
}

func TestElementScreenshotIsJustTheElement(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/long.html"))
	agent.Emulate(dsl.Device{Width: 400, Height: 300, Scale: 1})
	shot := agent.Screenshot(dsl.Capture{Selector: "#footer", Format: "jpeg"})
	if shot.Width != 400 || shot.Height != 100 || shot.MIMEType != "image/jpeg" || !isBlue(shot.Image, 200, 50) {
		t.Errorf("element shot %dx%d %s; want a 400x100 blue JPEG", shot.Width, shot.Height, shot.MIMEType)
	}
}

func TestScreenshotOfMissingElementFails(t *testing.T) {
	t.Parallel()
	site := dsl.NewSite(t)
	agent := dsl.NewAgent(t)
	agent.Open(site.URL("/long.html"))
	err := agent.TryScreenshot(dsl.Capture{Selector: "#nope"})
	if err == nil || !strings.Contains(err.Error(), `no element matches "#nope"`) {
		t.Errorf("err = %v, want no element matches", err)
	}
}
