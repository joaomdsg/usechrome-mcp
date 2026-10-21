// Package server assembles the usechrome-mcp MCP server from its tools.
package server

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Name is the implementation name the server reports to MCP clients.
const Name = "usechrome-mcp"

// New returns an MCP server that reports the given version.
func New(version string) *mcp.Server {
	return mcp.NewServer(&mcp.Implementation{Name: Name, Version: version}, nil)
}
