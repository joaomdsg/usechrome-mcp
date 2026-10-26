package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Interacter acts on a tab's elements.
type Interacter interface {
	Interact(ctx context.Context, req browser.InteractRequest) (browser.Interaction, error)
}

// InteractInput is the arguments of the interact tool.
type InteractInput struct {
	Action   string   `json:"action" jsonschema:"what to do: click, hover, type, press or select"`
	Ref      string   `json:"ref,omitempty" jsonschema:"ref of the element from the snapshot tool, such as e12"`
	Selector string   `json:"selector,omitempty" jsonschema:"CSS selector of the element, instead of a ref"`
	X        *float64 `json:"x,omitempty" jsonschema:"for click: CSS pixels from the viewport's left edge, with y and instead of an element"`
	Y        *float64 `json:"y,omitempty" jsonschema:"for click: CSS pixels from the viewport's top edge, with x and instead of an element"`
	Text     string   `json:"text,omitempty" jsonschema:"for type: the text, which replaces the element's content; empty clears it"`
	Key      string   `json:"key,omitempty" jsonschema:"for press: a key such as Enter, Tab, ArrowDown or Control+a"`
	Option   string   `json:"option,omitempty" jsonschema:"for select: the value or label of the option to pick"`
	Target
}

// InteractOutput describes the page after the action.
type InteractOutput struct {
	URL       string      `json:"url"`
	Title     string      `json:"title"`
	Navigated bool        `json:"navigated" jsonschema:"whether the action moved the tab to another page or within its page"`
	Dialogs   []string    `json:"dialogs,omitempty" jsonschema:"JavaScript dialogs the action opened, as type: message; all were accepted"`
	Opened    []OpenedTab `json:"opened,omitempty" jsonschema:"tabs the action opened, as a link with target=_blank does; the current tab stays the same"`
}

// OpenedTab is a tab an action opened.
type OpenedTab struct {
	Tab string `json:"tab" jsonschema:"tab id, to select it with the tabs tool or pass it to another tool"`
	URL string `json:"url" jsonschema:"the page the tab was asked to load"`
}

// gestureArg names, for each action, the argument it takes, if any,
// whether it needs an element and whether a point can stand in for it.
var gestureArg = map[browser.Gesture]struct {
	name         string
	needsElement bool
	takesPoint   bool
}{
	browser.Click:  {"", true, true},
	browser.Hover:  {"", true, false},
	browser.Type:   {"text", false, false},
	browser.Press:  {"key", false, false},
	browser.Select: {"option", true, false},
}

var (
	errBothTargets     = errors.New("give a ref or a selector, not both")
	errHalfPoint       = errors.New("give x and y together")
	errPointAndElement = errors.New("give an element or x and y, not both")
)

// AddInteract registers the interact tool, which drives in.
func AddInteract(s *mcp.Server, in Interacter) {
	mcp.AddTool(s, interactTool(), func(ctx context.Context, _ *mcp.CallToolRequest, args InteractInput) (*mcp.CallToolResult, InteractOutput, error) {
		req, err := args.request()
		if err != nil {
			return nil, InteractOutput{}, err
		}
		r, err := in.Interact(ctx, req)
		if err != nil {
			return nil, InteractOutput{}, err
		}
		out := InteractOutput{URL: r.URL, Title: r.Title, Navigated: r.Navigated, Dialogs: r.Dialogs}
		for _, t := range r.Opened {
			out.Opened = append(out.Opened, OpenedTab{Tab: t.ID, URL: t.URL})
		}
		return nil, out, nil
	})
}

func (in InteractInput) request() (browser.InteractRequest, error) {
	g := browser.Gesture(in.Action)
	at, err := in.point(g)
	if err != nil {
		return browser.InteractRequest{}, err
	}
	if err := in.checkElement(g, at != nil); err != nil {
		return browser.InteractRequest{}, err
	}
	value, err := in.argument(g)
	if err != nil {
		return browser.InteractRequest{}, err
	}
	return browser.InteractRequest{Tab: in.Tab, Action: g, Ref: in.Ref, Selector: in.Selector, At: at, Text: value}, nil
}

// point returns the point x and y name, if any, for an action that takes
// one in place of an element.
func (in InteractInput) point(g browser.Gesture) (*browser.Point, error) {
	switch {
	case in.X == nil && in.Y == nil:
		return nil, nil
	case in.X == nil || in.Y == nil:
		return nil, errHalfPoint
	case !gestureArg[g].takesPoint:
		return nil, fmt.Errorf("%s takes no x and y", g)
	case in.hasElement():
		return nil, errPointAndElement
	}
	return &browser.Point{X: *in.X, Y: *in.Y}, nil
}

// checkElement rejects two elements, and no element for an action that
// needs one, unless a point stands in for it.
func (in InteractInput) checkElement(g browser.Gesture, hasPoint bool) error {
	switch {
	case in.Ref != "" && in.Selector != "":
		return errBothTargets
	case in.hasElement() || hasPoint || !gestureArg[g].needsElement:
		return nil
	case gestureArg[g].takesPoint:
		return fmt.Errorf("%s needs a ref or a selector, or x and y", g)
	}
	return fmt.Errorf("%s needs a ref or a selector", g)
}

func (in InteractInput) hasElement() bool { return in.Ref != "" || in.Selector != "" }

// argument returns the one argument g takes and rejects the others. Only
// type may leave it empty, to clear its element.
func (in InteractInput) argument(g browser.Gesture) (string, error) {
	want := gestureArg[g].name
	var value string
	for _, arg := range []struct{ name, value string }{{"text", in.Text}, {"key", in.Key}, {"option", in.Option}} {
		if arg.name == want {
			value = arg.value
		} else if arg.value != "" {
			return "", fmt.Errorf("%s takes no %s", g, arg.name)
		}
	}
	if value == "" && want != "" && (g != browser.Type || !in.hasElement()) {
		return "", fmt.Errorf("%s needs %s", g, want)
	}
	return value, nil
}

func interactTool() *mcp.Tool {
	in := inputSchema[InteractInput]()
	in.Properties["action"].Enum = []any{
		string(browser.Click), string(browser.Hover), string(browser.Type), string(browser.Press), string(browser.Select),
	}
	return &mcp.Tool{
		Name: "interact",
		Description: "Act on an element as a person would: click, hover, type, press a key or select an option. " +
			"Name the element by its ref from the snapshot tool, such as e12, or by a CSS selector. " +
			"click and hover scroll the element into view and use the mouse at its centre; they fail if it is disabled, hidden or covered. " +
			"click can instead take x and y, CSS pixels from the viewport's top-left corner, and clicks whatever is there; it fails if the point is outside the viewport. " +
			"type replaces the element's content key by key, or types at the focus when no element is named. " +
			"press sends a key to the element, or else to the focus. select picks an option of a <select> by value or label. " +
			"When the action starts a navigation, the call waits for the new page to load. JavaScript dialogs are accepted and listed, and so are the tabs the action opened.",
		InputSchema: in,
	}
}
