package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
	"github.com/joaomdsg/usechrome-mcp/internal/server"
)

var update = flag.Bool("update", false, "rewrite the reference in README.md from the code")

const (
	readmePath = "../../README.md"
	refStart   = "<!-- reference: `make readme` writes this from the flags and the tool schemas; edit those instead -->\n"
	refEnd     = "<!-- end of reference -->\n"
)

func TestReadmeDescribesEveryFlagAndToolAsTheCodeDoes(t *testing.T) {
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	start, end := bytes.Index(readme, []byte(refStart)), bytes.Index(readme, []byte(refEnd))
	if start < 0 || end < start {
		t.Fatalf("README.md lacks the markers %q and %q around the reference", refStart, refEnd)
	}
	want := flagReference() + toolReference(t)
	if got := string(readme[start+len(refStart) : end]); got == want {
		return
	}
	if !*update {
		t.Fatal("the reference in README.md differs from the code; run `make readme` and commit the result")
	}
	out := slices.Concat(readme[:start+len(refStart)], []byte(want), readme[end:])
	if err := os.WriteFile(readmePath, out, 0o600); err != nil { //nolint:gosec // G703: readmePath is a constant
		t.Fatal(err)
	}
}

func flagReference() string {
	var out strings.Builder
	out.WriteString("### Flags\n\n| Flag | Description |\n| --- | --- |\n")
	newFlags(&config{}).VisitAll(func(f *flag.Flag) {
		kind, usage := flag.UnquoteUsage(f)
		fmt.Fprintf(&out, "| `-%s` | %s |\n", strings.TrimSpace(f.Name+" "+kind), cell(usage))
	})
	return out.String()
}

func toolReference(t *testing.T) string {
	var out strings.Builder
	out.WriteString("\n### Tools\n")
	for _, tool := range listTools(t) {
		fmt.Fprintf(&out, "\n#### `%s`\n\n%s\n", tool.Name, tool.Description)
		in, ok := tool.InputSchema.(*jsonschema.Schema)
		if !ok {
			t.Fatalf("tool %s has an input schema of type %T, want *jsonschema.Schema", tool.Name, tool.InputSchema)
		}
		if len(in.PropertyOrder) == 0 {
			continue
		}
		out.WriteString("\n| Argument | Type | Description |\n| --- | --- | --- |\n")
		for _, name := range in.PropertyOrder {
			arg := "`" + name + "`"
			if slices.Contains(in.Required, name) {
				arg += " (required)"
			}
			fmt.Fprintf(&out, "| %s | %s | %s |\n", arg, typeOf(in.Properties[name]), cell(in.Properties[name].Description))
		}
	}
	return out.String()
}

// listTools reads the tools as the server registered them, since a client
// receives the schemas as JSON objects, whose properties have no order.
func listTools(t *testing.T) []*mcp.Tool {
	var tools []*mcp.Tool
	s := server.New(version, chrome.New(t.Context(), chrome.Options{}))
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			res, err := next(ctx, method, req)
			if r, ok := res.(*mcp.ListToolsResult); ok {
				tools = r.Tools
			}
			return res, err
		}
	})
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := s.Connect(t.Context(), serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "readme", Version: "0"}, nil).Connect(t.Context(), clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	if _, err := cs.ListTools(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	return tools
}

func typeOf(s *jsonschema.Schema) string {
	kind := s.Type
	if kind == "" {
		// Go slices infer as ["null", "array"]; leaving out an argument is
		// how a caller passes null.
		kind = strings.Join(slices.DeleteFunc(slices.Clone(s.Types), func(t string) bool { return t == "null" }), " or ")
	}
	if kind == "array" && s.Items != nil {
		kind = "array of " + s.Items.Type
	}
	if len(s.Enum) == 0 {
		return kind
	}
	values := make([]string, 0, len(s.Enum))
	for _, v := range s.Enum {
		values = append(values, fmt.Sprintf("`%v`", v))
	}
	return kind + ": " + strings.Join(values, ", ")
}

// cell keeps text inside one cell of a Markdown table.
func cell(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}
