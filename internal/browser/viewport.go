package browser

// ViewportRequest reads a tab's viewport, and first changes it when it asks
// to. Zero Width, Height or Scale keeps the window's own value.
type ViewportRequest struct {
	Tab    string
	Width  int
	Height int
	Scale  float64
	Mobile bool
	Reset  bool
}

// Overrides reports whether the request changes the viewport rather than
// only reading it or resetting it.
func (r ViewportRequest) Overrides() bool {
	return r.Width > 0 || r.Height > 0 || r.Scale > 0 || r.Mobile
}

// Viewport is the size the page sees, in CSS pixels, and its device pixel
// ratio.
type Viewport struct {
	Width  int
	Height int
	Scale  float64
}
