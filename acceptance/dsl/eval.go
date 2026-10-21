package dsl

// Value is what a JavaScript expression produced.
type Value struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// Eval runs JavaScript in the current tab, as in the DevTools console.
func (a *Agent) Eval(expression string) Value {
	a.t.Helper()
	var v Value
	a.call("eval", map[string]any{"expression": expression}, &v)
	return v
}

// TryEval is Eval for code expected to fail; it returns the failure.
func (a *Agent) TryEval(expression string) error {
	a.t.Helper()
	return a.try("eval", map[string]any{"expression": expression})
}
