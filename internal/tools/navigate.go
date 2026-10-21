package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Navigator moves a tab and reports the document it lands on.
type Navigator interface {
	Navigate(ctx context.Context, req browser.NavRequest) (browser.Page, error)
}

// NavigateInput is the arguments of the navigate tool.
type NavigateInput struct {
	Action string `json:"action,omitempty" jsonschema:"goto (default), back, forward or reload"`
	URL    string `json:"url,omitempty" jsonschema:"address to open; required for goto"`
	Target
}

// NavigateOutput is the document a tab shows after the navigate tool runs.
type NavigateOutput struct {
	URL    string `json:"url" jsonschema:"final address after redirects"`
	Status int    `json:"status,omitempty" jsonschema:"HTTP status of the document; absent when the browser reports none"`
	Title  string `json:"title" jsonschema:"document title"`
}

var errURLRequired = errors.New("url is required for goto")

// AddNavigate registers the navigate tool, which drives nav.
func AddNavigate(s *mcp.Server, nav Navigator) {
	mcp.AddTool(s, navigateTool(), func(ctx context.Context, _ *mcp.CallToolRequest, in NavigateInput) (*mcp.CallToolResult, NavigateOutput, error) {
		req := browser.NavRequest{Tab: in.Tab, Action: browser.NavAction(in.Action), URL: in.URL}
		if req.Action == browser.Goto && req.URL == "" {
			return nil, NavigateOutput{}, errURLRequired
		}
		p, err := nav.Navigate(ctx, req)
		if err != nil {
			return nil, NavigateOutput{}, err
		}
		return nil, NavigateOutput{URL: p.URL, Status: p.Status, Title: p.Title}, nil
	})
}

func navigateTool() *mcp.Tool {
	in := inputSchema[NavigateInput]()
	action := in.Properties["action"]
	action.Enum = []any{string(browser.Goto), string(browser.Back), string(browser.Forward), string(browser.Reload)}
	action.Default = json.RawMessage(`"goto"`)
	return &mcp.Tool{
		Name:        "navigate",
		Description: "Open a URL in a tab, or go back, forward or reload. Returns the final URL after redirects, the HTTP status and the title.",
		InputSchema: in,
	}
}
