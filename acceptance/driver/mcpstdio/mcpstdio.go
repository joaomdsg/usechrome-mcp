// Package mcpstdio drives a usechrome-mcp binary over the MCP stdio
// transport, the way an agent's MCP client does.
package mcpstdio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client is one running server process and its MCP session.
type Client struct {
	session *mcp.ClientSession
}

// ToolError is a tool call the server answered with an error result.
type ToolError struct {
	Tool string
	Text string
}

func (e *ToolError) Error() string { return e.Tool + ": " + e.Text }

// Start launches bin with args, sends the process's stderr to stderr and
// completes the MCP handshake.
func Start(ctx context.Context, stderr io.Writer, bin string, args ...string) (*Client, error) {
	// Not CommandContext: Close must stop the process gracefully, after the
	// test's context is already cancelled.
	cmd := exec.Command(bin, args...) //nolint:gosec // G204: launching the binary under test is the point
	cmd.Stderr = stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "usechrome-mcp-acceptance", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	return &Client{session: session}, nil
}

// Image is an image a tool returned.
type Image struct {
	MIMEType string
	Data     []byte
}

// Call invokes a tool, decodes its structured result into out and returns
// the images it carries. An error result comes back as a *ToolError.
func (c *Client) Call(ctx context.Context, tool string, args, out any) ([]Image, error) {
	res, err := c.session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tool, err)
	}
	if res.IsError {
		return nil, &ToolError{Tool: tool, Text: text(res.Content)}
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return nil, err
	}
	var images []Image
	for _, c := range res.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			images = append(images, Image{MIMEType: ic.MIMEType, Data: ic.Data})
		}
	}
	return images, json.Unmarshal(b, out)
}

// Close closes the server's stdin and waits for the process to exit.
func (c *Client) Close() error {
	return c.session.Close()
}

func text(content []mcp.Content) string {
	for _, c := range content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
