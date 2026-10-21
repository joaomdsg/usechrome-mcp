package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Tabser lists the browser's tabs and opens, selects and closes them.
type Tabser interface {
	Tabs(ctx context.Context, req browser.TabsRequest) ([]browser.Tab, error)
}

// TabsInput is the arguments of the tabs tool.
type TabsInput struct {
	Action string `json:"action,omitempty" jsonschema:"list, the default, open, select or close"`
	URL    string `json:"url,omitempty" jsonschema:"for open: the page the new tab loads; absent opens a blank tab"`
	Target
}

var (
	errURLWithoutOpen    = errors.New("url goes with the open action only")
	errTabWithListOrOpen = errors.New("tab goes with the select and close actions only")
)

// AddTabs registers the tabs tool, which drives t.
func AddTabs(s *mcp.Server, t Tabser) {
	mcp.AddTool(s, tabsTool(), func(ctx context.Context, _ *mcp.CallToolRequest, in TabsInput) (*mcp.CallToolResult, any, error) {
		req := browser.TabsRequest{Action: browser.TabAction(in.Action), Tab: in.Tab, URL: in.URL}
		if req.URL != "" && req.Action != browser.OpenTab {
			return nil, nil, errURLWithoutOpen
		}
		if req.Tab != "" && req.Action != browser.SelectTab && req.Action != browser.CloseTab {
			return nil, nil, errTabWithListOrOpen
		}
		got, err := t.Tabs(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		return textResult(cutText(renderTabs(got), maxTextChars)), nil, nil
	})
}

func tabsTool() *mcp.Tool {
	in := inputSchema[TabsInput]()
	in.Properties["action"].Enum = []any{
		string(browser.ListTabs), string(browser.OpenTab), string(browser.SelectTab), string(browser.CloseTab),
	}
	in.Properties["tab"].Description = "for select and close: the tab id; close defaults to the current tab"
	return &mcp.Tool{
		Name: "tabs",
		Description: "List the browser's tabs, oldest first, with the tabs that pages or a person opened, after opening, selecting or closing one. " +
			"The other tools act on the current tab unless given a tab id. open loads url in a new tab and makes it current. " +
			"select makes a tab current and brings it to the front, since a tab in the background slows its timers. " +
			"close shuts a tab; when it was current, the newest remaining tab becomes current. The last tab cannot be closed.",
		InputSchema: in,
	}
}

// renderTabs writes a line per tab: its id, title and URL.
func renderTabs(tabs []browser.Tab) string {
	var out strings.Builder
	for _, t := range tabs {
		fmt.Fprintf(&out, "%s %q %s", t.ID, t.Title, t.URL)
		if t.Current {
			out.WriteString(" (current)")
		}
		out.WriteByte('\n')
	}
	return out.String()
}
