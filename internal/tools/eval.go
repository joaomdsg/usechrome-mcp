package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Evaluator runs JavaScript in a tab.
type Evaluator interface {
	Eval(ctx context.Context, req browser.EvalRequest) (browser.EvalResult, error)
}

// EvalInput is the arguments of the eval tool.
type EvalInput struct {
	Expression string `json:"expression" jsonschema:"JavaScript to run, as typed into the DevTools console"`
	Target
}

// EvalOutput is the value an expression produced.
type EvalOutput struct {
	Type  string `json:"type" jsonschema:"undefined, null, boolean, number, string, bigint, symbol, function, object, array, or a DevTools subtype such as node, date or error"`
	Value any    `json:"value" jsonschema:"the value as JSON; a descriptive string for what JSON cannot hold, such as NaN, 12n, a function's source or a node's selector"`
}

// AddEval registers the eval tool, which drives ev.
func AddEval(s *mcp.Server, ev Evaluator) {
	tool := &mcp.Tool{
		Name:        "eval",
		Description: "Run JavaScript in a tab as in the DevTools console and return the result's type and value. Promises are awaited, and $, $$ and $x work. A thrown error fails the call with its message.",
		InputSchema: inputSchema[EvalInput](),
	}
	mcp.AddTool(s, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in EvalInput) (*mcp.CallToolResult, EvalOutput, error) {
		r, err := ev.Eval(ctx, browser.EvalRequest{Tab: in.Tab, Expression: in.Expression})
		if err != nil {
			return nil, EvalOutput{}, err
		}
		return nil, EvalOutput{Type: r.Type, Value: r.Value}, nil
	})
}
