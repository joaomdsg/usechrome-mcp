package tools

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Sourcer reads the resources a tab's page loaded.
type Sourcer interface {
	Source(ctx context.Context, req browser.SourceRequest) (browser.Source, error)
}

// SourceInput is the arguments of the source tool.
type SourceInput struct {
	URL    string `json:"url,omitempty" jsonschema:"URL of a script, stylesheet, frame or other file the page loaded; default is the page's own document"`
	Offset int    `json:"offset,omitempty" jsonschema:"where to start reading, as the note at the end of a cut source gives it"`
	Target
}

// AddSource registers the source tool, which drives s.
func AddSource(s *mcp.Server, src Sourcer) {
	in := inputSchema[SourceInput]()
	in.Properties["offset"].Minimum = new(0.0)
	tool := &mcp.Tool{
		Name: "source",
		Description: "Read the source of a tab's current page as the server sent it, before any script changed it, as view-source and the DevTools Sources panel show it. " +
			"Pass url to read a script, stylesheet, frame or other file the page loaded instead; the network tool lists them. " +
			"A long source comes in parts: pass the offset that the note at its end gives to read on.",
		InputSchema: in,
	}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args SourceInput) (*mcp.CallToolResult, any, error) {
		got, err := src.Source(ctx, browser.SourceRequest{Tab: args.Tab, URL: args.URL})
		if err != nil {
			return nil, nil, err
		}
		text, err := renderSource(got, args.Offset, maxTextChars)
		if err != nil {
			return nil, nil, err
		}
		return textResult(text), nil, nil
	})
}

// renderSource writes a line about the source, then its text from offset
// up to limit characters, ending on a whole character.
func renderSource(s browser.Source, offset, limit int) (string, error) {
	about := fmt.Sprintf("%s (%s, %s)", s.URL, s.MimeType, byteSize(s.Size))
	if s.Binary {
		return about + " of binary data\n", nil
	}
	if offset > len(s.Text) {
		return "", fmt.Errorf("offset %d is past the end of %s, at %d", offset, s.URL, len(s.Text))
	}
	for offset < len(s.Text) && !utf8.RuneStart(s.Text[offset]) {
		offset++
	}
	if offset > 0 {
		about += fmt.Sprintf(" from offset %d", offset)
	}
	about += ":\n"
	end := offset + limit - len(about) - noticeRoom
	if end >= len(s.Text) {
		return about + s.Text[offset:], nil
	}
	for !utf8.RuneStart(s.Text[end]) {
		end--
	}
	rest := utf8.RuneCountInString(s.Text[end:])
	return about + s.Text[offset:end] + fmt.Sprintf("\n... %d more characters; pass offset %d to read on\n", rest, end), nil
}
