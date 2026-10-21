// Package tools implements the MCP tools that usechrome-mcp exposes.
package tools

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxTextChars keeps a text result near 10,000 tokens, where Claude Code
// starts warning: "Claude Code displays a warning when MCP tool output
// exceeds 10,000 tokens" (code.claude.com/docs/en/mcp).
const maxTextChars = 40_000

// noticeRoom is kept free under a limit for the line that says what was cut.
const noticeRoom = 100

// Target picks the tab a tool acts on. Tools embed it so the parameter reads
// the same everywhere.
type Target struct {
	Tab string `json:"tab,omitempty" jsonschema:"tab id from the tabs tool; default is the current tab"`
}

// inputSchema panics instead of returning an error because the input types
// are fixed at compile time, so any test run that registers the tool finds it.
func inputSchema[T any]() *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	return s
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
