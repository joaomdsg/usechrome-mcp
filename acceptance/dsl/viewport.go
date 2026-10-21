package dsl

// Viewport is the size the page sees and its device pixel ratio.
type Viewport struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Scale  float64 `json:"scale"`
}

// Device is a viewport override, as picked in DevTools device mode.
type Device struct {
	Width  int     `json:"width,omitempty"`
	Height int     `json:"height,omitempty"`
	Scale  float64 `json:"scale,omitempty"`
	Mobile bool    `json:"mobile,omitempty"`
}

// Phone is a common phone screen.
var Phone = Device{Width: 390, Height: 844, Scale: 3, Mobile: true}

// Emulate overrides the current tab's viewport with d.
func (a *Agent) Emulate(d Device) Viewport {
	a.t.Helper()
	var v Viewport
	a.call("viewport", d, &v)
	return v
}

// Viewport reports the current tab's viewport without changing it.
func (a *Agent) Viewport() Viewport {
	a.t.Helper()
	var v Viewport
	a.call("viewport", map[string]any{}, &v)
	return v
}

// ResetViewport drops any override from the current tab.
func (a *Agent) ResetViewport() Viewport {
	a.t.Helper()
	var v Viewport
	a.call("viewport", map[string]any{"reset": true}, &v)
	return v
}
