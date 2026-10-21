package tools

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Viewporter reads and overrides a tab's viewport.
type Viewporter interface {
	Viewport(ctx context.Context, req browser.ViewportRequest) (browser.Viewport, error)
}

// ViewportInput is the arguments of the viewport tool.
type ViewportInput struct {
	Width  int     `json:"width,omitempty" jsonschema:"CSS pixels; absent keeps the window's width"`
	Height int     `json:"height,omitempty" jsonschema:"CSS pixels; absent keeps the window's height"`
	Scale  float64 `json:"scale,omitempty" jsonschema:"device pixel ratio, 2 or 3 for a high-density phone; absent keeps the screen's"`
	Mobile bool    `json:"mobile,omitempty" jsonschema:"emulate a phone: meta viewport tag, overlay scrollbars and touch"`
	Reset  bool    `json:"reset,omitempty" jsonschema:"drop the override and return to the window's own size"`
	Target
}

// ViewportOutput is the viewport the page sees after the tool runs.
type ViewportOutput struct {
	Width  int     `json:"width" jsonschema:"layout viewport width in CSS pixels (innerWidth)"`
	Height int     `json:"height" jsonschema:"layout viewport height in CSS pixels (innerHeight)"`
	Scale  float64 `json:"scale" jsonschema:"device pixel ratio"`
}

var errResetWithOverride = errors.New("reset cannot be combined with width, height, scale or mobile")

// AddViewport registers the viewport tool, which drives vp.
func AddViewport(s *mcp.Server, vp Viewporter) {
	mcp.AddTool(s, viewportTool(), func(ctx context.Context, _ *mcp.CallToolRequest, in ViewportInput) (*mcp.CallToolResult, ViewportOutput, error) {
		req := browser.ViewportRequest{Tab: in.Tab, Width: in.Width, Height: in.Height, Scale: in.Scale, Mobile: in.Mobile, Reset: in.Reset}
		if req.Reset && req.Overrides() {
			return nil, ViewportOutput{}, errResetWithOverride
		}
		v, err := vp.Viewport(ctx, req)
		if err != nil {
			return nil, ViewportOutput{}, err
		}
		return nil, ViewportOutput{Width: v.Width, Height: v.Height, Scale: v.Scale}, nil
	})
}

func viewportTool() *mcp.Tool {
	in := inputSchema[ViewportInput]()
	for _, p := range []string{"width", "height", "scale"} {
		in.Properties[p].Minimum = new(0.0)
	}
	return &mcp.Tool{
		Name: "viewport",
		Description: "Read or change a tab's viewport, as DevTools device mode does. With no arguments it reports the size the page sees. " +
			"width, height, scale and mobile replace any earlier override, which lasts across navigations until reset. " +
			"With mobile, a page without a meta viewport tag lays out 980px wide, as on a phone, and touch events reach pages loaded afterwards.",
		InputSchema: in,
	}
}
