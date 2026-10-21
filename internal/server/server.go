// Package server assembles the usechrome-mcp MCP server from its tools.
package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

// Name is the implementation name the server reports to MCP clients.
const Name = "usechrome-mcp"

// Browser is everything the tools need from the browser.
type Browser interface {
	tools.Navigator
	tools.Evaluator
	tools.Viewporter
	tools.Screenshotter
	tools.Snapshotter
	tools.Interacter
}

// New returns an MCP server that reports the given version and whose tools
// drive b.
func New(version string, b Browser) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: Name, Version: version}, nil)
	tools.AddNavigate(s, b)
	tools.AddEval(s, b)
	tools.AddViewport(s, b)
	tools.AddScreenshot(s, b)
	tools.AddSnapshot(s, b)
	tools.AddInteract(s, b)
	return s
}
