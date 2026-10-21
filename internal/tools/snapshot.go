package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// maxSnapshotChars keeps a snapshot near 10,000 tokens, where Claude Code
// starts warning: "Claude Code displays a warning when MCP tool output
// exceeds 10,000 tokens" (code.claude.com/docs/en/mcp).
const maxSnapshotChars = 40_000

// Snapshotter reads a tab's accessibility tree.
type Snapshotter interface {
	Snapshot(ctx context.Context, req browser.SnapshotRequest) (browser.Snapshot, error)
}

// SnapshotInput is the arguments of the snapshot tool.
type SnapshotInput struct {
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector of the element to outline instead of the whole page"`
	Target
}

// AddSnapshot registers the snapshot tool, which drives sn.
func AddSnapshot(s *mcp.Server, sn Snapshotter) {
	tool := &mcp.Tool{
		Name: "snapshot",
		Description: "Outline what a tab shows as assistive technology sees it: roles, names, values and states, one element per line, " +
			"with a ref such as e12 that the interact tool takes. Hidden elements are left out. Take a new snapshot after the page changes; " +
			"refs of elements that are gone stop working.",
		InputSchema: inputSchema[SnapshotInput](),
	}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in SnapshotInput) (*mcp.CallToolResult, any, error) {
		snap, err := sn.Snapshot(ctx, browser.SnapshotRequest{Tab: in.Tab, Selector: in.Selector})
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: renderSnapshot(snap, maxSnapshotChars)}}}, nil, nil
	})
}

// renderSnapshot writes the page header and one line per node, indented by
// depth, and cuts whole lines past limit characters.
func renderSnapshot(snap browser.Snapshot, limit int) string {
	lines := []string{"url: " + snap.URL, "title: " + snap.Title}
	lines = appendNodes(lines, snap.Nodes, 0)
	const noticeRoom = 100
	var b strings.Builder
	for i, line := range lines {
		if b.Len()+len(line)+1+noticeRoom > limit {
			fmt.Fprintf(&b, "... %d more lines cut; pass selector to outline part of the page\n", len(lines)-i)
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func appendNodes(lines []string, nodes []*browser.Node, depth int) []string {
	for _, n := range nodes {
		lines = append(lines, strings.Repeat("  ", depth)+nodeLine(n))
		lines = appendNodes(lines, n.Children, depth+1)
	}
	return lines
}

// nodeLine renders a node as `- role "name" [state] [ref=e1]: value`, close
// to Playwright's ARIA snapshots, which agents already read.
func nodeLine(n *browser.Node) string {
	if n.Role == "text" {
		return "- text: " + n.Name
	}
	var b strings.Builder
	b.WriteString("- " + n.Role)
	if n.Name != "" {
		fmt.Fprintf(&b, " %q", n.Name)
	}
	for _, p := range n.Props {
		if p.Value == "true" {
			fmt.Fprintf(&b, " [%s]", p.Key)
		} else {
			fmt.Fprintf(&b, " [%s=%s]", p.Key, p.Value)
		}
	}
	if n.Ref != "" {
		fmt.Fprintf(&b, " [ref=%s]", n.Ref)
	}
	if n.Value != "" {
		fmt.Fprintf(&b, ": %q", n.Value)
	}
	return b.String()
}
