package server_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/server"
)

func TestServerReportsNameAndVersion(t *testing.T) {
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := server.New("1.2.3").Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	info := cs.InitializeResult().ServerInfo
	if info.Name != server.Name || info.Version != "1.2.3" {
		t.Fatalf("server info = %s %s, want %s 1.2.3", info.Name, info.Version, server.Name)
	}
}
