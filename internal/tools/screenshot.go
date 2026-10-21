package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Screenshotter captures what a tab shows.
type Screenshotter interface {
	Screenshot(ctx context.Context, req browser.ScreenshotRequest) (browser.Image, error)
}

// ScreenshotInput is the arguments of the screenshot tool.
type ScreenshotInput struct {
	FullPage bool   `json:"fullPage,omitempty" jsonschema:"capture the whole scrollable page instead of the viewport"`
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector of one element to capture instead of the viewport"`
	Format   string `json:"format,omitempty" jsonschema:"png (default) or jpeg"`
	Quality  int    `json:"quality,omitempty" jsonschema:"jpeg quality from 0 to 100; ignored for png"`
	Target
}

// ScreenshotOutput describes the image the screenshot tool returns.
type ScreenshotOutput struct {
	Width     int    `json:"width" jsonschema:"image width in pixels: CSS pixels times the device pixel ratio"`
	Height    int    `json:"height" jsonschema:"image height in pixels"`
	Format    string `json:"format" jsonschema:"png or jpeg"`
	Truncated bool   `json:"truncated,omitempty" jsonschema:"true when the capture was cut to keep each side within 8000 pixels"`
}

var errFullPageWithSelector = errors.New("fullPage and selector cannot be combined")

// AddScreenshot registers the screenshot tool, which drives sc.
func AddScreenshot(s *mcp.Server, sc Screenshotter) {
	mcp.AddTool(s, screenshotTool(), func(ctx context.Context, _ *mcp.CallToolRequest, in ScreenshotInput) (*mcp.CallToolResult, ScreenshotOutput, error) {
		if in.FullPage && in.Selector != "" {
			return nil, ScreenshotOutput{}, errFullPageWithSelector
		}
		img, err := sc.Screenshot(ctx, browser.ScreenshotRequest{
			Tab: in.Tab, Selector: in.Selector, FullPage: in.FullPage,
			Format: browser.ImageFormat(in.Format), Quality: in.Quality,
		})
		if err != nil {
			return nil, ScreenshotOutput{}, err
		}
		out := ScreenshotOutput{Width: img.Width, Height: img.Height, Format: string(img.Format), Truncated: img.Truncated}
		// The SDK writes the text copy of the output only when Content is
		// empty, and the MCP spec asks for one alongside structured content.
		text, err := json.Marshal(out)
		if err != nil {
			return nil, ScreenshotOutput{}, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.ImageContent{Data: img.Data, MIMEType: "image/" + string(img.Format)},
			&mcp.TextContent{Text: string(text)},
		}}, out, nil
	})
}

func screenshotTool() *mcp.Tool {
	in := inputSchema[ScreenshotInput]()
	format := in.Properties["format"]
	format.Enum = []any{string(browser.PNG), string(browser.JPEG)}
	format.Default = json.RawMessage(`"png"`)
	quality := in.Properties["quality"]
	quality.Minimum, quality.Maximum = new(0.0), new(100.0)
	quality.Default = json.RawMessage(`80`)
	return &mcp.Tool{
		Name: "screenshot",
		Description: "Capture what a tab shows as an image: the viewport, the whole page with fullPage, or one element with selector. " +
			"The image is in device pixels, so set scale with the viewport tool to trade sharpness for size.",
		InputSchema: in,
	}
}
