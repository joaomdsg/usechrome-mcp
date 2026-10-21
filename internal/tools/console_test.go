package tools_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeConsoler struct {
	got  []browser.ConsoleRequest
	msgs []browser.ConsoleMessage
	err  error
}

func (f *fakeConsoler) Console(_ context.Context, req browser.ConsoleRequest) ([]browser.ConsoleMessage, error) {
	f.got = append(f.got, req)
	return f.msgs, f.err
}

func consoleSession(t *testing.T, c tools.Consoler) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddConsole(s, c)
	return connect(t, s)
}

var everyLevel = []browser.ConsoleMessage{
	{Type: "debug", Text: "noise"},
	{Type: "log", Text: "hello"},
	{Type: "info", Text: "fyi"},
	{Type: "trace", Text: "here"},
	{Type: "warning", Text: "careful"},
	{Type: "error", Text: "bad"},
	{Type: "exception", Text: "Uncaught Error: boom"},
	{Type: "assert", Text: "Assertion failed"},
}

func TestConsoleWritesEachMessageWithWhereItCameFrom(t *testing.T) {
	t.Parallel()
	c := &fakeConsoler{msgs: []browser.ConsoleMessage{
		{Type: "log", Text: `state {"x":1}`, Source: "http://x/app.js:3:9"},
		{Type: "exception", Text: "Uncaught Error: deep", Source: "http://x/app.js:1:26", Stack: []string{"inner (http://x/app.js:1:26)", "outer (http://x/app.js:2:20)"}},
		{Type: "error", Text: "Failed to load resource: 404", Source: "http://x/missing.png"},
		{Type: "info", Text: "from nowhere"},
	}}
	res := call(t, consoleSession(t, c), "console", map[string]any{"tab": "t2"})
	want := `log: state {"x":1}
    at http://x/app.js:3:9
exception: Uncaught Error: deep
    at inner (http://x/app.js:1:26)
    at outer (http://x/app.js:2:20)
error: Failed to load resource: 404
    at http://x/missing.png
info: from nowhere
`
	if got := errorText(res); res.IsError || got != want {
		t.Errorf("console:\n%s\nwant:\n%s", got, want)
	}
	if want := (browser.ConsoleRequest{Tab: "t2"}); len(c.got) != 1 || c.got[0] != want {
		t.Errorf("browser called with %+v, want %+v", c.got, want)
	}
}

func TestConsoleShowsTheLevelAndMoreSevere(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		level string
		want  []string
	}{
		{"", []string{"hello", "fyi", "here", "careful", "bad", "Uncaught Error: boom", "Assertion failed"}},
		{"debug", []string{"noise", "hello", "fyi", "here", "careful", "bad", "Uncaught Error: boom", "Assertion failed"}},
		{"warning", []string{"careful", "bad", "Uncaught Error: boom", "Assertion failed"}},
		{"error", []string{"bad", "Uncaught Error: boom", "Assertion failed"}},
	} {
		args := map[string]any{}
		if tc.level != "" {
			args["level"] = tc.level
		}
		got := errorText(call(t, consoleSession(t, &fakeConsoler{msgs: everyLevel}), "console", args))
		var texts []string
		for line := range strings.Lines(got) {
			texts = append(texts, strings.TrimSpace(line[strings.Index(line, ": ")+2:]))
		}
		if fmt.Sprint(texts) != fmt.Sprint(tc.want) {
			t.Errorf("level %q shows %q, want %q", tc.level, texts, tc.want)
		}
	}
}

func TestConsoleSaysWhenNothingIsAtTheLevel(t *testing.T) {
	t.Parallel()
	c := &fakeConsoler{msgs: []browser.ConsoleMessage{{Type: "log", Text: "hello"}}}
	if got := errorText(call(t, consoleSession(t, c), "console", map[string]any{"level": "warning"})); got != "no console messages at level warning or above\n" {
		t.Errorf("console = %q", got)
	}
}

func TestConsoleRejectsAnUnknownLevel(t *testing.T) {
	t.Parallel()
	c := &fakeConsoler{}
	res := call(t, consoleSession(t, c), "console", map[string]any{"level": "verbose"})
	if !res.IsError || !strings.Contains(errorText(res), "level") || len(c.got) != 0 {
		t.Errorf("console level verbose: %q, browser got %+v; want an error about level before the browser", errorText(res), c.got)
	}
}

func TestConsoleCutsTheOldestMessagesPastTheLimit(t *testing.T) {
	t.Parallel()
	var msgs []browser.ConsoleMessage
	for i := range 200 {
		msgs = append(msgs, browser.ConsoleMessage{Type: "log", Text: fmt.Sprintf("%03d %s", i, strings.Repeat("x", 400))})
	}
	text := errorText(call(t, consoleSession(t, &fakeConsoler{msgs: msgs}), "console", map[string]any{}))
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(text) > 40_000 || !strings.HasPrefix(lines[0], "... ") || !strings.Contains(lines[0], "earlier messages cut") {
		t.Errorf("console is %d chars starting %q; want at most 40000 starting with a cut notice", len(text), lines[0])
	}
	if !strings.HasPrefix(lines[1], "log: ") || !strings.HasPrefix(lines[len(lines)-1], "log: 199 ") {
		t.Errorf("console keeps %q to %q, want whole messages up to the newest", lines[1][:10], lines[len(lines)-1][:10])
	}
}

func TestConsoleShortensAHugeMessage(t *testing.T) {
	t.Parallel()
	// 4000 bytes end inside the 2000th é.
	c := &fakeConsoler{msgs: []browser.ConsoleMessage{{Type: "log", Text: "a" + strings.Repeat("é", 3000)}, {Type: "log", Text: "after"}}}
	text := errorText(call(t, consoleSession(t, c), "console", map[string]any{}))
	if !strings.HasPrefix(text, "log: a"+strings.Repeat("é", 1999)+" ... 1001 more characters\n") || !strings.HasSuffix(text, "log: after\n") {
		t.Errorf("console = %q, want the first message cut before the split é", text[3990:])
	}
}

func TestConsoleReportsBrowserError(t *testing.T) {
	t.Parallel()
	res := call(t, consoleSession(t, &fakeConsoler{err: errors.New(`unknown tab "t9"`)}), "console", map[string]any{"tab": "t9"})
	if !res.IsError || !strings.Contains(errorText(res), "t9") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
