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

func (noBrowser) Eval(context.Context, browser.EvalRequest) (browser.EvalResult, error) {
	return browser.EvalResult{}, errors.New("no browser")
}

func (noBrowser) Viewport(context.Context, browser.ViewportRequest) (browser.Viewport, error) {
	return browser.Viewport{}, errors.New("no browser")
}

func (noBrowser) Screenshot(context.Context, browser.ScreenshotRequest) (browser.Image, error) {
	return browser.Image{}, errors.New("no browser")
}

func (noBrowser) Snapshot(context.Context, browser.SnapshotRequest) (browser.Snapshot, error) {
	return browser.Snapshot{}, errors.New("no browser")
}

func (noBrowser) Interact(context.Context, browser.InteractRequest) (browser.Interaction, error) {
	return browser.Interaction{}, errors.New("no browser")
}

func (noBrowser) Styles(context.Context, browser.StylesRequest) (browser.Styles, error) {
	return browser.Styles{}, errors.New("no browser")
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

func TestServerListsEveryTool(t *testing.T) {
	res, err := connect(t).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	if want := []string{"eval", "interact", "navigate", "screenshot", "snapshot", "styles", "viewport"}; !slices.Equal(names, want) {
		t.Errorf("tools = %v, want %v", names, want)
	}
}
