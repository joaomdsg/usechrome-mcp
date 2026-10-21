// Package tools implements the MCP tools that usechrome-mcp exposes.
package tools

import "github.com/google/jsonschema-go/jsonschema"

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
