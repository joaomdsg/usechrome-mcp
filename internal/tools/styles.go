package tools

import (
	"context"
	"errors"
	"maps"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

var errNoStylesTarget = errors.New("styles needs a ref or a selector")

// Styler reads the CSS of a tab's elements.
type Styler interface {
	Styles(ctx context.Context, req browser.StylesRequest) (browser.Styles, error)
}

// StylesInput is the arguments of the styles tool.
type StylesInput struct {
	Ref        string   `json:"ref,omitempty" jsonschema:"ref of the element from the snapshot tool, such as e12"`
	Selector   string   `json:"selector,omitempty" jsonschema:"CSS selector of the element, instead of a ref"`
	Properties []string `json:"properties,omitempty" jsonschema:"CSS properties to compute, such as gap or line-height; default is those the rules set plus layout basics"`
	Target
}

// StylesOutput is an element's CSS.
type StylesOutput struct {
	Element  string            `json:"element" jsonschema:"tag, id and classes of the element"`
	Box      BoxOutput         `json:"box" jsonschema:"border box in CSS pixels, relative to the viewport"`
	Computed map[string]string `json:"computed" jsonschema:"computed value of each property; empty for a property the browser does not know"`
	Rules    []RuleOutput      `json:"rules,omitempty" jsonschema:"rules that match the element, highest priority first; the style attribute comes first"`
}

// BoxOutput is a rectangle in CSS pixels.
type BoxOutput struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// RuleOutput is one CSS rule that matches the element.
type RuleOutput struct {
	Selector     string   `json:"selector"`
	Media        string   `json:"media,omitempty" jsonschema:"media query the rule sits under"`
	Source       string   `json:"source,omitempty" jsonschema:"stylesheet URL and line of the rule"`
	Declarations []string `json:"declarations"`
}

// AddStyles registers the styles tool, which drives st.
func AddStyles(s *mcp.Server, st Styler) {
	tool := &mcp.Tool{
		Name: "styles",
		Description: "Read an element's CSS as the DevTools Styles and Computed panes show it: the rules that match it, " +
			"highest priority first, with their stylesheet URL and line, its box, and computed values. " +
			"Name the element by its ref from the snapshot tool, such as e12, or by a CSS selector. " +
			"Without properties, it computes those the rules set plus basics such as display, position, width, margin, color and font.",
		InputSchema: inputSchema[StylesInput](),
	}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in StylesInput) (*mcp.CallToolResult, StylesOutput, error) {
		if err := in.checkElement(); err != nil {
			return nil, StylesOutput{}, err
		}
		css, err := st.Styles(ctx, browser.StylesRequest{Tab: in.Tab, Ref: in.Ref, Selector: in.Selector, Properties: in.Properties})
		if err != nil {
			return nil, StylesOutput{}, err
		}
		return nil, stylesOutput(css), nil
	})
}

func (in StylesInput) checkElement() error {
	switch {
	case in.Ref != "" && in.Selector != "":
		return errBothTargets
	case in.Ref == "" && in.Selector == "":
		return errNoStylesTarget
	}
	return nil
}

// stylesOutput turns nil into empty, as the output schema wants an object
// and arrays, never null; an empty rule such as .card {} has nil declarations.
func stylesOutput(css browser.Styles) StylesOutput {
	out := StylesOutput{
		Element:  css.Element,
		Box:      BoxOutput{X: css.Box.X, Y: css.Box.Y, Width: css.Box.Width, Height: css.Box.Height},
		Computed: map[string]string{},
	}
	maps.Copy(out.Computed, css.Computed)
	for _, r := range css.Rules {
		decl := append([]string{}, r.Declarations...)
		out.Rules = append(out.Rules, RuleOutput{Selector: r.Selector, Media: r.Media, Source: r.Source, Declarations: decl})
	}
	return out
}
