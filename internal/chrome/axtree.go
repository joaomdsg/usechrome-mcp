package chrome

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

// Roles that only group or lay out content: their children take their place.
var hoistedRoles = []string{"generic", "none", "presentation", "LabelText"}

// Roles that carry nothing a reader needs.
var droppedRoles = []string{"InlineTextBox", "LineBreak"}

// shownProps are the states and attributes a snapshot keeps, in the order it
// prints them. A false value prints only for the toggles, where it means off
// rather than not applicable.
var shownProps = []accessibility.PropertyName{
	accessibility.PropertyNameLevel,
	accessibility.PropertyNameChecked,
	accessibility.PropertyNamePressed,
	accessibility.PropertyNameExpanded,
	accessibility.PropertyNameSelected,
	accessibility.PropertyNameDisabled,
	accessibility.PropertyNameRequired,
	accessibility.PropertyNameReadonly,
	accessibility.PropertyNameInvalid,
	accessibility.PropertyNameFocused,
	accessibility.PropertyNameURL,
}

var toggles = []accessibility.PropertyName{
	accessibility.PropertyNameChecked,
	accessibility.PropertyNamePressed,
	accessibility.PropertyNameExpanded,
}

var errNotInAXTree = errors.New("the element is not in the accessibility tree, as when hidden")

// outline turns Chrome's flat accessibility node list, whose first node is
// the document, into the tree a snapshot shows. A nonzero root starts it at
// that element instead.
func outline(nodes []*accessibility.Node, root cdp.BackendNodeID) ([]*browser.Node, error) {
	t := axTree{byID: make(map[accessibility.NodeID]*accessibility.Node, len(nodes))}
	for _, n := range nodes {
		t.byID[n.NodeID] = n
	}
	if root == 0 {
		if len(nodes) == 0 {
			return nil, nil
		}
		return t.convert(nodes[0], nil), nil
	}
	i := slices.IndexFunc(nodes, func(n *accessibility.Node) bool { return n.BackendDOMNodeID == root })
	if i < 0 {
		return nil, errNotInAXTree
	}
	// The picked element shows even when its role would only group.
	n := t.element(nodes[i])
	n.Children = t.children(nodes[i], n)
	return []*browser.Node{n}, nil
}

type axTree struct {
	byID map[accessibility.NodeID]*accessibility.Node
}

// convert returns what n becomes in the outline: itself, its children in
// its place, or nothing. parent, the nearest element above, lets text that
// repeats its name or value drop out.
func (t axTree) convert(n *accessibility.Node, parent *browser.Node) []*browser.Node {
	role := str(n.Role)
	switch {
	case slices.Contains(droppedRoles, role):
		return nil
	case role == "StaticText":
		return textNode(str(n.Name), parent)
	case n.Ignored || role == "RootWebArea" || (slices.Contains(hoistedRoles, role) && str(n.Name) == ""):
		return t.children(n, parent)
	}
	node := t.element(n)
	node.Children = t.children(n, node)
	return []*browser.Node{node}
}

// children converts n's children, dropping text that repeats a sibling's
// name, as a label's text does beside its field.
func (t axTree) children(n *accessibility.Node, parent *browser.Node) []*browser.Node {
	var out []*browser.Node
	for _, id := range n.ChildIDs {
		if c, ok := t.byID[id]; ok {
			out = append(out, t.convert(c, parent)...)
		}
	}
	names := make(map[string]bool)
	for _, c := range out {
		if c.Role != textRole {
			names[c.Name] = true
		}
	}
	return slices.DeleteFunc(out, func(c *browser.Node) bool { return c.Role == textRole && names[c.Name] })
}

func (t axTree) element(n *accessibility.Node) *browser.Node {
	node := &browser.Node{Role: str(n.Role), Name: str(n.Name), Value: str(n.Value)}
	if n.BackendDOMNodeID != 0 {
		node.Ref = ref(n.BackendDOMNodeID)
	}
	for _, name := range shownProps {
		i := slices.IndexFunc(n.Properties, func(p *accessibility.Property) bool { return p.Name == name })
		if i < 0 {
			continue
		}
		v := str(n.Properties[i].Value)
		if v == "" || (v == "false" && !slices.Contains(toggles, name)) {
			continue
		}
		node.Props = append(node.Props, browser.Prop{Key: string(name), Value: v})
	}
	return node
}

const textRole = "text"

// textNode drops text that is blank or that the parent already shows: a
// link's label as its name, a field's content as its value.
func textNode(text string, parent *browser.Node) []*browser.Node {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" || (parent != nil && (strings.Contains(parent.Name, text) || text == parent.Value)) {
		return nil
	}
	return []*browser.Node{{Role: textRole, Name: text}}
}

// str renders an accessibility value as plain text: strings unquoted,
// numbers and booleans as written.
func str(v *accessibility.Value) string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(v.Value, &s); err == nil {
		return s
	}
	return string(v.Value)
}

func ref(id cdp.BackendNodeID) string {
	return "e" + strconv.FormatInt(int64(id), 10)
}
