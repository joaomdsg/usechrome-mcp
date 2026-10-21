package tools_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
	"github.com/joaomdsg/usechrome-mcp/internal/tools"
)

type fakeSnapshotter struct {
	got  []browser.SnapshotRequest
	snap browser.Snapshot
	err  error
}

func (f *fakeSnapshotter) Snapshot(_ context.Context, req browser.SnapshotRequest) (browser.Snapshot, error) {
	f.got = append(f.got, req)
	return f.snap, f.err
}

func snapshotSession(t *testing.T, sn tools.Snapshotter) *mcp.ClientSession {
	t.Helper()
	s := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	tools.AddSnapshot(s, sn)
	return connect(t, s)
}

var formSnapshot = browser.Snapshot{URL: "http://x/form", Title: "Form", Nodes: []*browser.Node{
	{Ref: "e1", Role: "main", Children: []*browser.Node{
		{Ref: "e2", Role: "heading", Name: "Sign up", Props: []browser.Prop{{Key: "level", Value: "2"}}},
		{Role: "text", Name: "Read the terms"},
		{Ref: "e3", Role: "textbox", Name: "Email", Value: "a@b.c", Props: []browser.Prop{{Key: "required", Value: "true"}}},
		{Ref: "e4", Role: "checkbox", Name: `Say "yes"`, Props: []browser.Prop{{Key: "checked", Value: "false"}}},
	}},
}}

func TestSnapshotRendersOneLinePerNode(t *testing.T) {
	t.Parallel()
	sn := &fakeSnapshotter{snap: formSnapshot}
	res := call(t, snapshotSession(t, sn), "snapshot", map[string]any{"selector": "main", "tab": "t2"})
	want := `url: http://x/form
title: Form
- main [ref=e1]
  - heading "Sign up" [level=2] [ref=e2]
  - text: Read the terms
  - textbox "Email" [required] [ref=e3]: "a@b.c"
  - checkbox "Say \"yes\"" [checked=false] [ref=e4]
`
	if got := errorText(res); res.IsError || got != want {
		t.Errorf("snapshot:\n%s\nwant:\n%s", got, want)
	}
	if want := (browser.SnapshotRequest{Tab: "t2", Selector: "main"}); len(sn.got) != 1 || sn.got[0] != want {
		t.Errorf("browser called with %+v, want %+v", sn.got, want)
	}
	if res.StructuredContent != nil {
		t.Errorf("structured content = %v, want none so the outline is not sent twice", res.StructuredContent)
	}
}

func TestSnapshotCutsWholeLinesPastTheLimit(t *testing.T) {
	t.Parallel()
	var nodes []*browser.Node
	for range 5000 {
		nodes = append(nodes, &browser.Node{Ref: "e99999", Role: "listitem", Name: "an item with some words in it"})
	}
	res := call(t, snapshotSession(t, &fakeSnapshotter{snap: browser.Snapshot{Nodes: nodes}}), "snapshot", map[string]any{})
	text := errorText(res)
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	last := lines[len(lines)-1]
	if len(text) > 40_000 || !strings.HasPrefix(last, "... ") || !strings.Contains(last, "more lines cut") {
		t.Errorf("snapshot is %d chars ending %q; want at most 40000 ending in a cut notice", len(text), last)
	}
	if prev := lines[len(lines)-2]; !strings.HasSuffix(prev, `"an item with some words in it" [ref=e99999]`) {
		t.Errorf("line before the notice = %q, want a whole item", prev)
	}
}

func TestSnapshotReportsBrowserError(t *testing.T) {
	t.Parallel()
	sn := &fakeSnapshotter{err: errors.New(`no element matches "#x"`)}
	res := call(t, snapshotSession(t, sn), "snapshot", map[string]any{"selector": "#x"})
	if !res.IsError || !strings.Contains(errorText(res), "#x") {
		t.Errorf("result = %+v, want the browser error", res)
	}
}
