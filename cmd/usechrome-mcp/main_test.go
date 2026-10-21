package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-version"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != version+"\n" {
		t.Fatalf("stdout = %q, want %q", got, version+"\n")
	}
}

func TestUnknownFlagExitsWithUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestHelpFlagExitsCleanly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"-h"}, &stdout, &stderr); code != 0 {
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
