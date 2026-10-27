package chrome

import (
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

func sourceTree() *page.FrameResourceTree {
	return &page.FrameResourceTree{
		Frame: &cdp.Frame{ID: "top", URL: "https://a.test/", MimeType: "text/html"},
		Resources: []*page.FrameResource{
			{URL: "https://a.test/app.js", MimeType: "text/javascript"},
		},
		ChildFrames: []*page.FrameResourceTree{{
			Frame: &cdp.Frame{ID: "child", URL: "https://b.test/frame", MimeType: "text/html"},
			Resources: []*page.FrameResource{
				{URL: "https://b.test/frame.css", MimeType: "text/css"},
			},
		}},
	}
}

func TestFindResourceReturnsDocumentsAndResourcesOfEveryFrame(t *testing.T) {
	for url, want := range map[string]struct {
		src   browser.Source
		frame cdp.FrameID
	}{
		"":                         {browser.Source{URL: "https://a.test/", MimeType: "text/html"}, "top"},
		"https://a.test/app.js":    {browser.Source{URL: "https://a.test/app.js", MimeType: "text/javascript"}, "top"},
		"https://b.test/frame":     {browser.Source{URL: "https://b.test/frame", MimeType: "text/html"}, "child"},
		"https://b.test/frame.css": {browser.Source{URL: "https://b.test/frame.css", MimeType: "text/css"}, "child"},
	} {
		src, frame, ok := sourceFind(t, url)
		if !ok || src != want.src || frame != want.frame {
			t.Errorf("findResource(%q) = %+v, %q, %v; want %+v, %q, true", url, src, frame, ok, want.src, want.frame)
		}
	}
}

func TestFindResourceStopsWhenNoFrameHasTheURL(t *testing.T) {
	src, frame, ok := sourceFind(t, "https://c.test/missing.js")
	if ok || src != (browser.Source{}) || frame != "" {
		t.Errorf("findResource(missing) = %+v, %q, %v; want zero, \"\", false", src, frame, ok)
	}
}

// sourceFind fails the test instead of hanging when the frame walk never ends.
func sourceFind(t *testing.T, url string) (browser.Source, cdp.FrameID, bool) {
	t.Helper()
	type result struct {
		src   browser.Source
		frame cdp.FrameID
		ok    bool
	}
	done := make(chan result, 1)
	go func() {
		src, frame, ok := findResource(sourceTree(), url)
		done <- result{src, frame, ok}
	}()
	select {
	case r := <-done:
		return r.src, r.frame, r.ok
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("findResource(%q) did not return", url)
		return browser.Source{}, "", false
	}
}
