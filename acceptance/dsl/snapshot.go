package dsl

import (
	"fmt"
	"regexp"
	"strings"
)

// Outline is the snapshot tool's text outline of a page.
type Outline string

// Snapshot outlines the current tab, or only the element selector picks
// when it is not empty.
func (a *Agent) Snapshot(selector string) Outline {
	a.t.Helper()
	return Outline(a.call("snapshot", snapshotArgs(selector), nil).Text)
}

// TrySnapshot is Snapshot for a selector expected to fail; it returns the
// failure.
func (a *Agent) TrySnapshot(selector string) error {
	a.t.Helper()
	return a.try("snapshot", snapshotArgs(selector))
}

func snapshotArgs(selector string) map[string]any {
	if selector == "" {
		return map[string]any{}
	}
	return map[string]any{"selector": selector}
}

// Line returns the outline's first line for an element with role and name,
// or "" when it shows none.
func (o Outline) Line(role, name string) string {
	head := fmt.Sprintf("- %s %q", role, name)
	for line := range strings.SplitSeq(string(o), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == head || strings.HasPrefix(trimmed, head+" ") || strings.HasPrefix(trimmed, head+":") {
			return trimmed
		}
	}
	return ""
}

var refPattern = regexp.MustCompile(`\[ref=(e\d+)\]`)

// Ref returns the ref of the element with role and name, or "" when the
// outline shows none.
func (o Outline) Ref(role, name string) string {
	if m := refPattern.FindStringSubmatch(o.Line(role, name)); m != nil {
		return m[1]
	}
	return ""
}
