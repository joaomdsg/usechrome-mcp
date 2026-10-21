package dsl

// Source reads the source of url, or of the page when url is empty, from
// offset.
func (a *Agent) Source(url string, offset int) string {
	a.t.Helper()
	return a.call("source", sourceArgs(url, offset), nil).Text
}

// TrySource is Source for a call expected to fail; it returns the failure.
func (a *Agent) TrySource(url string) error {
	a.t.Helper()
	return a.try("source", sourceArgs(url, 0))
}

func sourceArgs(url string, offset int) map[string]any {
	args := map[string]any{}
	if url != "" {
		args["url"] = url
	}
	if offset > 0 {
		args["offset"] = offset
	}
	return args
}
