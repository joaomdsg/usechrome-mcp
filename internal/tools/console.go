package tools

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// maxMessageChars keeps one message to a tenth of a result, so a large dump
// cannot push every other message out.
const maxMessageChars = maxTextChars / 10

// levels are DevTools' console levels, least severe first.
var levels = []string{"debug", "info", "warning", "error"}

// Consoler reads a tab's console.
type Consoler interface {
	Console(ctx context.Context, req browser.ConsoleRequest) ([]browser.ConsoleMessage, error)
}

// ConsoleInput is the arguments of the console tool.
type ConsoleInput struct {
	Level string `json:"level,omitempty" jsonschema:"least severe level to show: debug, info, warning or error; default info, as in DevTools"`
	Target
}

// AddConsole registers the console tool, which drives c.
func AddConsole(s *mcp.Server, c Consoler) {
	in := inputSchema[ConsoleInput]()
	in.Properties["level"].Enum = []any{levels[0], levels[1], levels[2], levels[3]}
	tool := &mcp.Tool{
		Name: "console",
		Description: "Read the DevTools console of a tab's current page, oldest first: console calls, uncaught exceptions with their stack, " +
			"and browser messages such as failed resource loads. It holds what the page logged since it loaded, also before this server attached. " +
			"Objects are written as JSON. level picks the least severe level to show.",
		InputSchema: in,
	}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args ConsoleInput) (*mcp.CallToolResult, any, error) {
		msgs, err := c.Console(ctx, browser.ConsoleRequest{Tab: args.Tab})
		if err != nil {
			return nil, nil, err
		}
		level := cmp.Or(args.Level, "info")
		return textResult(renderConsole(atLeast(msgs, level), level, maxTextChars)), nil, nil
	})
}

func atLeast(msgs []browser.ConsoleMessage, level string) []browser.ConsoleMessage {
	var out []browser.ConsoleMessage
	for _, m := range msgs {
		if severity(m.Type) >= severity(level) {
			out = append(out, m)
		}
	}
	return out
}

// severity ranks a message type by DevTools' level for it: console.debug
// and the browser's verbose messages are debug, and a failed assertion is
// an error.
func severity(typ string) int {
	switch typ {
	case "debug":
		return 0
	case "warning":
		return 2
	case "error", "exception", "assert":
		return 3
	}
	return 1
}

// renderConsole writes each message with the place it came from, and cuts
// the oldest whole messages past limit characters.
func renderConsole(msgs []browser.ConsoleMessage, level string, limit int) string {
	if len(msgs) == 0 {
		return fmt.Sprintf("no console messages at level %s or above\n", level)
	}
	blocks := make([]string, len(msgs))
	for i, m := range msgs {
		blocks[i] = messageBlock(m)
	}
	first, size := len(blocks), 0
	for first > 0 && size+len(blocks[first-1])+noticeRoom <= limit {
		first--
		size += len(blocks[first])
	}
	var b strings.Builder
	if first > 0 {
		fmt.Fprintf(&b, "... %d earlier messages cut; pass level to see fewer\n", first)
	}
	for _, block := range blocks[first:] {
		b.WriteString(block)
	}
	return b.String()
}

func messageBlock(m browser.ConsoleMessage) string {
	text := m.Text
	if len(text) > maxMessageChars {
		kept := strings.ToValidUTF8(text[:maxMessageChars], "")
		text = kept + fmt.Sprintf(" ... %d more characters", utf8.RuneCountInString(text[len(kept):]))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", m.Type, text)
	at := m.Stack
	if len(at) == 0 && m.Source != "" {
		at = []string{m.Source}
	}
	for _, f := range at {
		fmt.Fprintf(&b, "    at %s\n", f)
	}
	return b.String()
}
