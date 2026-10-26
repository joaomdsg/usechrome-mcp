package dsl

// Element names an element by its ref from a snapshot or by a CSS selector.
type Element struct {
	Ref      string `json:"ref,omitempty"`
	Selector string `json:"selector,omitempty"`
}

// Ref names the element a snapshot gave ref.
func Ref(ref string) Element { return Element{Ref: ref} }

// CSS names the element selector picks.
func CSS(selector string) Element { return Element{Selector: selector} }

// After is the page after an action.
type After struct {
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	Navigated bool     `json:"navigated"`
	Dialogs   []string `json:"dialogs"`
	Opened    []Opened `json:"opened"`
}

// Opened is a tab an action opened.
type Opened struct {
	Tab string `json:"tab"`
	URL string `json:"url"`
}

// Click clicks el with the mouse.
func (a *Agent) Click(el Element) After {
	a.t.Helper()
	return a.interact(el, "click", "", "")
}

// ClickAt clicks with the mouse at x, y in CSS pixels from the viewport's
// top-left corner, on whatever is there.
func (a *Agent) ClickAt(x, y float64) After {
	a.t.Helper()
	var after After
	a.call("interact", map[string]any{"action": "click", "x": x, "y": y}, &after)
	return after
}

// TryClickAt is ClickAt for a click expected to fail; it returns the failure.
func (a *Agent) TryClickAt(x, y float64) error {
	a.t.Helper()
	return a.try("interact", map[string]any{"action": "click", "x": x, "y": y})
}

// TryClick is Click for a click expected to fail; it returns the failure.
func (a *Agent) TryClick(el Element) error {
	a.t.Helper()
	return a.try("interact", interactArgs(el, "click", "", ""))
}

// Type types text into el in place of its content.
func (a *Agent) Type(el Element, text string) After {
	a.t.Helper()
	return a.interact(el, "type", "text", text)
}

// Press presses key, such as Enter or Control+a, on the focused element.
func (a *Agent) Press(key string) After {
	a.t.Helper()
	return a.interact(Element{}, "press", "key", key)
}

// Select picks the option of el with value or label option.
func (a *Agent) Select(el Element, option string) After {
	a.t.Helper()
	return a.interact(el, "select", "option", option)
}

func (a *Agent) interact(el Element, action, arg, value string) After {
	a.t.Helper()
	var after After
	a.call("interact", interactArgs(el, action, arg, value), &after)
	return after
}

func interactArgs(el Element, action, arg, value string) map[string]any {
	args := elementArgs(el)
	args["action"] = action
	if arg != "" {
		args[arg] = value
	}
	return args
}

func elementArgs(el Element) map[string]any {
	args := map[string]any{}
	if el.Ref != "" {
		args["ref"] = el.Ref
	}
	if el.Selector != "" {
		args["selector"] = el.Selector
	}
	return args
}
