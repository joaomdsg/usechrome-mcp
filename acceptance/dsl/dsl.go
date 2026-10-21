// Package dsl is the language the acceptance tests speak: an agent that uses
// a browser through usechrome-mcp, and a site for it to visit. It hides how
// the agent reaches the server, so a test reads as what the agent does.
package dsl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/joaomdsg/usechrome-mcp/acceptance/driver/mcpstdio"
)

const mainPkg = "github.com/joaomdsg/usechrome-mcp/cmd/usechrome-mcp"

var binary string

// agentSlots caps live servers, each with its own Chromium, whatever the core
// count: an idle headless Chromium is 12 processes and 130 tasks, and one per
// core on a 28-core box passes a 2048-task container limit.
var agentSlots = make(chan struct{}, 4)

// Run runs the tests against the binary that USECHROME_MCP_BIN names, or
// against a fresh build when it is unset. Call it from TestMain.
func Run(m *testing.M) int {
	bin, cleanup, err := serverBinary()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer cleanup()
	binary = bin
	return m.Run()
}

func serverBinary() (string, func(), error) {
	if bin := os.Getenv("USECHROME_MCP_BIN"); bin != "" {
		return bin, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "usechrome-mcp-acceptance-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	bin := filepath.Join(dir, "usechrome-mcp")
	cmd := exec.Command("go", "build", "-o", bin, mainPkg) //nolint:gosec // G204: bin is a path in our own temp dir
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("build %s: %w", mainPkg, err)
	}
	return bin, cleanup, nil
}

// Agent is an AI agent connected to its own usechrome-mcp server, and so to
// its own browser.
type Agent struct {
	t      testing.TB
	client *mcpstdio.Client
}

// NewAgent starts a server for the test and stops it when the test ends.
func NewAgent(t testing.TB) *Agent {
	t.Helper()
	agentSlots <- struct{}{}
	// Registered before the server stops, so it runs after: cleanups run
	// last-in first-out.
	t.Cleanup(func() { <-agentSlots })
	// CI runners forbid the user namespaces the sandbox needs, and the
	// browser only loads local fixtures.
	c, err := mcpstdio.Start(t.Context(), t.Output(), binary, "-no-sandbox")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Errorf("stop server: %v", err)
		}
	})
	return &Agent{t: t, client: c}
}

func (a *Agent) call(tool string, args, out any) {
	a.t.Helper()
	if err := a.client.Call(a.t.Context(), tool, args, out); err != nil {
		a.t.Fatal(err)
	}
}

func (a *Agent) try(tool string, args any) error {
	a.t.Helper()
	var out any
	return a.client.Call(a.t.Context(), tool, args, &out)
}
