package chrome

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"

	"github.com/joaomdsg/usechrome-mcp/internal/browser"
)

func val(v any) *accessibility.Value {
	b, _ := json.Marshal(v)
	return &accessibility.Value{Value: b}
}

func prop(name accessibility.PropertyName, v any) *accessibility.Property {
	return &accessibility.Property{Name: name, Value: val(v)}
}

// axNode builds a node whose id is also its backend DOM id.
func axNode(id int, role, name string, children ...int) *accessibility.Node {
	n := &accessibility.Node{NodeID: accessibility.NodeID(itoa(id)), BackendDOMNodeID: cdp.BackendNodeID(id), Role: val(role)}
	if name != "" {
		n.Name = val(name)
	}
	for _, c := range children {
		n.ChildIDs = append(n.ChildIDs, accessibility.NodeID(itoa(c)))
	}
	return n
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestOutlineHoistsGroupingAndDropsRepeatedText(t *testing.T) {
	ignored := axNode(6, "generic", "", 7)
	ignored.Ignored = true
	heading := axNode(3, "heading", "Title", 4)
	heading.Properties = []*accessibility.Property{prop(accessibility.PropertyNameLevel, 1), prop(accessibility.PropertyNameFocused, false)}
	nodes := []*accessibility.Node{
		axNode(1, "RootWebArea", "Doc title", 2, 6),
		axNode(2, "generic", "", 3, 5),
		heading,
		axNode(4, "StaticText", "Title"),
		axNode(5, "StaticText", "  free \n text  ", 8),
		ignored,
		axNode(7, "button", "Go", 9),
		axNode(8, "InlineTextBox", "free text"),
		axNode(9, "StaticText", "Go"),
	}
	got, err := outline(nodes, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []*browser.Node{
		{Ref: "e3", Role: "heading", Name: "Title", Props: []browser.Prop{{Key: "level", Value: "1"}}},
		{Role: "text", Name: "free text"},
		{Ref: "e7", Role: "button", Name: "Go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
}

func TestOutlineKeepsOffTogglesAndValues(t *testing.T) {
	box := axNode(2, "checkbox", "Agree")
	box.Properties = []*accessibility.Property{
		prop(accessibility.PropertyNameDisabled, false),
		prop(accessibility.PropertyNameChecked, "false"),
		prop(accessibility.PropertyNameRequired, true),
	}
	field := axNode(3, "textbox", "Email")
	field.Value = val("a@b.c")
	link := axNode(4, "link", "Home")
	link.Properties = []*accessibility.Property{prop(accessibility.PropertyNameURL, "http://x/")}
	got, err := outline([]*accessibility.Node{axNode(1, "RootWebArea", "", 2, 3, 4), box, field, link}, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []*browser.Node{
		{Ref: "e2", Role: "checkbox", Name: "Agree", Props: []browser.Prop{{Key: "checked", Value: "false"}, {Key: "required", Value: "true"}}},
		{Ref: "e3", Role: "textbox", Name: "Email", Value: "a@b.c"},
		{Ref: "e4", Role: "link", Name: "Home", Props: []browser.Prop{{Key: "url", Value: "http://x/"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
}

func TestOutlineStartsAtPickedElementEvenIfItOnlyGroups(t *testing.T) {
	nodes := []*accessibility.Node{
		axNode(1, "RootWebArea", "", 2, 4),
		axNode(2, "generic", "", 3),
		axNode(3, "button", "Inside"),
		axNode(4, "button", "Outside"),
	}
	got, err := outline(nodes, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []*browser.Node{{Ref: "e2", Role: "generic", Children: []*browser.Node{{Ref: "e3", Role: "button", Name: "Inside"}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
	if _, err := outline(nodes, 99); err == nil {
		t.Error("outline from an element missing from the tree succeeded")
	}
}

func TestOutlineDropsLabelAndValueEchoes(t *testing.T) {
	field := axNode(4, "textbox", "Email", 5)
	field.Value = val("a@b.c")
	nodes := []*accessibility.Node{
		axNode(1, "RootWebArea", "", 2),
		axNode(2, "LabelText", "", 3, 4),
		axNode(3, "StaticText", "Email"),
		field,
		axNode(5, "StaticText", "a@b.c"),
	}
	got, err := outline(nodes, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []*browser.Node{{Ref: "e4", Role: "textbox", Name: "Email", Value: "a@b.c"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
}

func TestOutlineOfEmptyTreeIsEmpty(t *testing.T) {
	if got, err := outline(nil, 0); got != nil || err != nil {
		t.Errorf("got %v, %v; want nothing", got, err)
	}
}

func dump(nodes []*browser.Node) string {
	b, _ := json.Marshal(nodes)
	return string(b)
}

func TestOutlineStartsAtFirstNodeWhenPicked(t *testing.T) {
	got, err := outline([]*accessibility.Node{axNode(1, "button", "Go"), axNode(2, "RootWebArea", "", 1)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := []*browser.Node{{Ref: "e1", Role: "button", Name: "Go"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
}

func TestOutlineHoistsIgnoredNodesButKeepsNamedGroups(t *testing.T) {
	ignored := axNode(2, "button", "Hidden", 3)
	ignored.Ignored = true
	nodes := []*accessibility.Node{
		axNode(1, "RootWebArea", "", 2, 4),
		ignored,
		axNode(3, "link", "Shown"),
		axNode(4, "generic", "Card", 5),
		axNode(5, "button", "Open"),
	}
	got, err := outline(nodes, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []*browser.Node{
		{Ref: "e3", Role: "link", Name: "Shown"},
		{Ref: "e4", Role: "generic", Name: "Card", Children: []*browser.Node{{Ref: "e5", Role: "button", Name: "Open"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
}

func TestOutlineGivesNoRefToNodeWithoutDOMNode(t *testing.T) {
	pseudo := axNode(2, "button", "Pseudo")
	pseudo.BackendDOMNodeID = 0
	got, err := outline([]*accessibility.Node{axNode(1, "RootWebArea", "", 2), pseudo}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []*browser.Node{{Role: "button", Name: "Pseudo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
}

func TestOutlineDropsEmptyPropsAndBlankText(t *testing.T) {
	field := axNode(2, "textbox", "Email")
	field.Properties = []*accessibility.Property{prop(accessibility.PropertyNameInvalid, ""), prop(accessibility.PropertyNameChecked, "")}
	got, err := outline([]*accessibility.Node{axNode(1, "RootWebArea", "", 2, 3), field, axNode(3, "StaticText", " \n ")}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []*browser.Node{{Ref: "e2", Role: "textbox", Name: "Email"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("outline:\n got %s\nwant %s", dump(got), dump(want))
	}
}
