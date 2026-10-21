package browser

import "encoding/json"

// EvalRequest asks a tab to run a JavaScript expression. A promise is
// awaited.
type EvalRequest struct {
	Tab        string
	Expression string
}

// EvalResult is the value an expression produced. Type is "undefined",
// "null", "boolean", "number", "string", "bigint", "symbol", "function",
// "array" or "object". Value is that value as JSON, nil for undefined. JSON
// cannot hold NaN, Infinity, -0, bigints, symbols or functions, so for
// those Value is a JSON string that describes them, such as "NaN" or "12n".
type EvalResult struct {
	Type  string
	Value json.RawMessage
}
