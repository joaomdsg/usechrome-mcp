package main

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-version"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != version+"\n" {
		t.Fatalf("stdout = %q, want %q", got, version+"\n")
	}
}

func TestUnknownFlagExitsWithUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-nope"}, nil, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestHelpFlagExitsCleanly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-h"}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestBrowserFlagsSetLaunchOptions(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := parseFlags([]string{
		"--headed", "--cdp-url", "ws://127.0.0.1:9222", "--chrome-path", "/opt/chrome",
		"--no-sandbox", "--timeout", "5s",
	}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	want := chrome.Options{
		Headed: true, CDPURL: "ws://127.0.0.1:9222", ExecPath: "/opt/chrome",
		NoSandbox: true, Timeout: 5 * time.Second,
	}
	if cfg.browser != want {
		t.Errorf("options = %+v, want %+v", cfg.browser, want)
	}
}

func TestServesUntilClientDisconnects(t *testing.T) {
	serverT, clientT := mcp.NewInMemoryTransports()
	var stderr bytes.Buffer
	exit := make(chan int, 1)
	go func() { exit <- run(t.Context(), nil, serverT, io.Discard, &stderr) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(t.Context(), clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	if info := cs.InitializeResult().ServerInfo; info.Version != version {
		t.Errorf("server version = %q, want %q", info.Version, version)
	}
	if err := cs.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-exit:
		if code != 0 {
			t.Errorf("exit code = %d, stderr = %q", code, stderr.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server still running after the client disconnected")
	}
}
