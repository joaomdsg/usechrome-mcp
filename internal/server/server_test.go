package server_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/server"
)

type noBrowser struct{}

func (noBrowser) Navigate(context.Context, browser.NavRequest) (browser.Page, error) {
	return browser.Page{}, errors.New("no browser")
}

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := server.New("1.2.3", noBrowser{}).Connect(t.Context(), serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(t.Context(), clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestServerReportsNameAndVersion(t *testing.T) {
	info := connect(t).InitializeResult().ServerInfo
	if info.Name != server.Name || info.Version != "1.2.3" {
		t.Fatalf("server info = %s %s, want %s 1.2.3", info.Name, info.Version, server.Name)
	}
}

func TestServerListsNavigateTool(t *testing.T) {
	res, err := connect(t).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "navigate") {
		t.Errorf("tools = %v, want navigate", names)
	}
}
