// Package tools implements the MCP tools that usechrome-mcp exposes.
package tools

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

// cutText keeps the first limit bytes of text, ending on a whole character,
// and says how many characters it cut.
func cutText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	kept := strings.ToValidUTF8(text[:limit], "")
	return kept + fmt.Sprintf(" ... %d more characters", utf8.RuneCountInString(text[len(kept):]))
}

// newest keeps the last blocks that fit in limit with room for a notice,
// and returns how many it cut.
func newest(blocks []string, limit int) ([]string, int) {
	first, size := len(blocks), 0
	for first > 0 && size+len(blocks[first-1])+noticeRoom <= limit {
		first--
		size += len(blocks[first])
	}
	return blocks[first:], first
}
