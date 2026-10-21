package dsl

// Console reads the current page's console at level and above; empty
// means the default.
func (a *Agent) Console(level string) string {
	a.t.Helper()
	args := map[string]any{}
	if level != "" {
		args["level"] = level
	}
	return a.call("console", args, nil).Text
}
