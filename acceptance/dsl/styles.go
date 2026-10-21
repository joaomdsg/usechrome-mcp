package dsl

// Styles is an element's CSS as DevTools shows it.
type Styles struct {
	Element  string            `json:"element"`
	Computed map[string]string `json:"computed"`
	Rules    []Rule            `json:"rules"`
}

// Rule is a CSS rule that matches the element.
type Rule struct {
	Selector     string   `json:"selector"`
	Media        string   `json:"media"`
	Source       string   `json:"source"`
	Declarations []string `json:"declarations"`
}

// Styles reads el's matched rules and computed values, of properties when
// given.
func (a *Agent) Styles(el Element, properties ...string) Styles {
	a.t.Helper()
	var s Styles
	a.call("styles", stylesArgs(el, properties), &s)
	return s
}

// TryStyles is Styles for a call expected to fail; it returns the failure.
func (a *Agent) TryStyles(el Element) error {
	a.t.Helper()
	return a.try("styles", stylesArgs(el, nil))
}

func stylesArgs(el Element, properties []string) map[string]any {
	args := elementArgs(el)
	if len(properties) > 0 {
		args["properties"] = properties
	}
	return args
}
