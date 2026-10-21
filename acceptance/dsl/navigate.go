package dsl

// Page is what the agent learns about the document its tab shows.
type Page struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
	Title  string `json:"title"`
}

// Open sends the browser to url.
func (a *Agent) Open(url string) Page {
	a.t.Helper()
	return a.navigate(map[string]any{"url": url})
}

// TryOpen is Open for a navigation expected to fail; it returns the failure.
func (a *Agent) TryOpen(url string) error {
	a.t.Helper()
	return a.try("navigate", map[string]any{"url": url})
}

// GoBack returns to the previous page in the tab's history.
func (a *Agent) GoBack() Page {
	a.t.Helper()
	return a.navigate(map[string]any{"action": "back"})
}

// TryGoBack is GoBack for a step expected to fail; it returns the failure.
func (a *Agent) TryGoBack() error {
	a.t.Helper()
	return a.try("navigate", map[string]any{"action": "back"})
}

// GoForward undoes a GoBack.
func (a *Agent) GoForward() Page {
	a.t.Helper()
	return a.navigate(map[string]any{"action": "forward"})
}

// Reload loads the current page again.
func (a *Agent) Reload() Page {
	a.t.Helper()
	return a.navigate(map[string]any{"action": "reload"})
}

func (a *Agent) navigate(args map[string]any) Page {
	a.t.Helper()
	var p Page
	a.call("navigate", args, &p)
	return p
}
