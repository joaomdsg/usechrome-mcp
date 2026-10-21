// Command usechrome-mcp is an MCP server that gives an AI agent a real
// Chrome/Chromium browser over stdio.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/chrome"
	"github.com/joaomdsg/usechrome-mcp/internal/server"
)

// version is overwritten at release time via -ldflags.
var version = "dev"

type config struct {
	showVersion bool
	browser     chrome.Options
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], &mcp.StdioTransport{}, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, transport mcp.Transport, stdout, stderr io.Writer) int {
	cfg, err := parseFlags(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if cfg.showVersion {
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	}
	b := chrome.New(ctx, cfg.browser)
	err = server.New(version, b).Run(ctx, transport)
	// After a signal the browser is already going down with ctx, so neither
	// error says anything the user needs.
	if cerr := b.Close(); cerr != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, "close browser:", cerr)
	}
	if err != nil && ctx.Err() == nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func parseFlags(args []string, stderr io.Writer) (config, error) {
	var cfg config
	flags := newFlags(&cfg)
	flags.SetOutput(stderr)
	return cfg, flags.Parse(args)
}

// newFlags defines the command line, which sets cfg.
func newFlags(cfg *config) *flag.FlagSet {
	flags := flag.NewFlagSet(server.Name, flag.ContinueOnError)
	flags.BoolVar(&cfg.showVersion, "version", false, "print the version and exit")
	flags.BoolVar(&cfg.browser.Headed, "headed", false, "show a browser window instead of running headless")
	flags.StringVar(&cfg.browser.CDPURL, "cdp-url", "", "attach to a running browser's DevTools URL instead of launching one")
	flags.StringVar(&cfg.browser.ExecPath, "chrome-path", "", "browser binary to launch (default: search PATH)")
	flags.BoolVar(&cfg.browser.NoSandbox, "no-sandbox", false, "disable the Chromium sandbox, for containers without user namespaces")
	flags.DurationVar(&cfg.browser.Timeout, "timeout", 0, "limit for each browser operation (default 30s)")
	return flags
}
