package dsl

import (
	"strings"
	"time"
)

// Perf reads how the current page has performed, asking again for up to
// 5 s until the report contains until, since the browser reports an
// interaction only after the frame that follows it.
func (a *Agent) Perf(until string) string {
	a.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		text := a.call("perf", map[string]any{}, nil).Text
		if strings.Contains(text, until) || time.Now().After(deadline) {
			return text
		}
		time.Sleep(20 * time.Millisecond)
	}
}
