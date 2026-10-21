package dsl

import (
	"bytes"
	"image"
	_ "image/jpeg" // registers the decoder for JPEG captures
	_ "image/png"  // registers the decoder for PNG captures
)

// Shot is what the agent sees in a screenshot: the decoded image, as the
// tool's image content carried it, and the size and format the tool reported.
type Shot struct {
	Image     image.Image `json:"-"`
	MIMEType  string      `json:"-"`
	Width     int         `json:"width"`
	Height    int         `json:"height"`
	Format    string      `json:"format"`
	Truncated bool        `json:"truncated"`
}

// Capture is how much of the page a screenshot takes.
type Capture struct {
	FullPage bool   `json:"fullPage,omitempty"`
	Selector string `json:"selector,omitempty"`
	Format   string `json:"format,omitempty"`
}

// Screenshot captures the current tab.
func (a *Agent) Screenshot(c Capture) Shot {
	a.t.Helper()
	var s Shot
	images := a.call("screenshot", c, &s)
	if len(images) != 1 {
		a.t.Fatalf("screenshot returned %d images, want 1", len(images))
	}
	img, _, err := image.Decode(bytes.NewReader(images[0].Data))
	if err != nil {
		a.t.Fatalf("decode screenshot: %v", err)
	}
	s.Image, s.MIMEType = img, images[0].MIMEType
	return s
}

// TryScreenshot is Screenshot for a capture expected to fail; it returns the
// failure.
func (a *Agent) TryScreenshot(c Capture) error {
	a.t.Helper()
	return a.try("screenshot", c)
}
