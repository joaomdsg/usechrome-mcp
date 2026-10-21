package tools_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeScreenshotter struct {
	got []browser.ScreenshotRequest
	img browser.Image
	err error
}

func (f *fakeScreenshotter) Screenshot(_ context.Context, req browser.ScreenshotRequest) (browser.Image, error) {
	f.got = append(f.got, req)
	return f.img, f.err
}

func screenshotSession(t *testing.T, sc tools.Screenshotter) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddScreenshot(s, sc)
	return connect(t, s)
}

func TestScreenshotReturnsImageAndSize(t *testing.T) {
	t.Parallel()
	data := []byte("\x89PNG fake")
	sc := &fakeScreenshotter{img: browser.Image{Data: data, Format: browser.PNG, Width: 800, Height: 600, Truncated: true}}
	res := call(t, screenshotSession(t, sc), "screenshot", map[string]any{"fullPage": true, "tab": "t2"})
	out := decode[tools.ScreenshotOutput](t, res)
	if want := (tools.ScreenshotOutput{Width: 800, Height: 600, Format: "png", Truncated: true}); out != want {
		t.Errorf("output %+v, want %+v", out, want)
	}
	img, ok := res.Content[0].(*mcp.ImageContent)
	if !ok || img.MIMEType != "image/png" || !bytes.Equal(img.Data, data) {
		t.Errorf("first content = %#v, want the PNG bytes", res.Content[0])
	}
	if text, ok := res.Content[1].(*mcp.TextContent); !ok || !strings.Contains(text.Text, `"width":800`) {
		t.Errorf("second content = %#v, want the output as JSON text", res.Content[1])
	}
}

func TestScreenshotDefaultsToPNGAndPassesRequest(t *testing.T) {
	t.Parallel()
	sc := &fakeScreenshotter{img: browser.Image{Format: browser.PNG}}
	decode[tools.ScreenshotOutput](t, call(t, screenshotSession(t, sc), "screenshot", map[string]any{"selector": "#box"}))
	want := browser.ScreenshotRequest{Selector: "#box", Format: browser.PNG, Quality: 80}
	if len(sc.got) != 1 || sc.got[0] != want {
		t.Errorf("browser called with %+v, want %+v", sc.got, want)
	}
}

func TestScreenshotPassesJPEGQuality(t *testing.T) {
	t.Parallel()
	sc := &fakeScreenshotter{img: browser.Image{Format: browser.JPEG}}
	res := call(t, screenshotSession(t, sc), "screenshot", map[string]any{"format": "jpeg", "quality": 40})
	decode[tools.ScreenshotOutput](t, res)
	if len(sc.got) != 1 || sc.got[0].Format != browser.JPEG || sc.got[0].Quality != 40 {
		t.Errorf("browser called with %+v, want jpeg at 40", sc.got)
	}
	if img, ok := res.Content[0].(*mcp.ImageContent); !ok || img.MIMEType != "image/jpeg" {
		t.Errorf("first content = %#v, want image/jpeg", res.Content[0])
	}
}

func TestScreenshotRejectsBadArguments(t *testing.T) {
	t.Parallel()
	for _, args := range []map[string]any{
		{"fullPage": true, "selector": "#box"},
		{"format": "gif"},
		{"quality": 101},
		{"quality": -1},
	} {
		sc := &fakeScreenshotter{}
		res := call(t, screenshotSession(t, sc), "screenshot", args)
		if !res.IsError || len(sc.got) != 0 {
			t.Errorf("%v: result = %+v, browser calls = %d; want an error and no call", args, res, len(sc.got))
		}
	}
}

func TestScreenshotReportsBrowserError(t *testing.T) {
	t.Parallel()
	sc := &fakeScreenshotter{err: errors.New(`no element matches "#gone"`)}
	res := call(t, screenshotSession(t, sc), "screenshot", map[string]any{"selector": "#gone"})
	if !res.IsError || !strings.Contains(errorText(res), "#gone") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
