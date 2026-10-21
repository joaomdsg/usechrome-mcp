package dsl

import (
	"strings"
	"time"
)

// Tabs lists the browser's tabs, a line each, asking again for up to 5 s
// until there are want, since a page opens a tab after the click that asked
// for it returns.
func (a *Agent) Tabs(want int) []string {
	a.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		tabs := a.tabs(map[string]any{})
		if len(tabs) == want || time.Now().After(deadline) {
			return tabs
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// OpenTab opens url in a new tab and lists the tabs.
func (a *Agent) OpenTab(url string) []string {
	a.t.Helper()
	return a.tabs(map[string]any{"action": "open", "url": url})
}

// SelectTab makes the tab current and lists the tabs.
func (a *Agent) SelectTab(id string) []string {
	a.t.Helper()
	return a.tabs(map[string]any{"action": "select", "tab": id})
}

// CloseTab closes the tab and lists the tabs.
func (a *Agent) CloseTab(id string) []string {
	a.t.Helper()
	return a.tabs(map[string]any{"action": "close", "tab": id})
}

// TryCloseTab is CloseTab for a call expected to fail; it returns the
// failure.
func (a *Agent) TryCloseTab(id string) error {
	a.t.Helper()
	return a.try("tabs", map[string]any{"action": "close", "tab": id})
}

func (a *Agent) tabs(args map[string]any) []string {
	a.t.Helper()
	return strings.Split(strings.TrimSuffix(a.call("tabs", args, nil).Text, "\n"), "\n")
}
