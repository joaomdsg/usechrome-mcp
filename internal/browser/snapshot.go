package browser

// SnapshotRequest reads the accessibility tree of a tab's page, or of the
// element Selector picks.
type SnapshotRequest struct {
	Tab      string
	Selector string
}

// Snapshot is a page as assistive technology sees it.
type Snapshot struct {
	URL   string
	Title string
	Nodes []*Node
}

// Node is one meaningful element or run of text. Ref names the element for
// tools that act on it; text has none.
type Node struct {
	Ref      string
	Role     string
	Name     string
	Value    string
	Props    []Prop
	Children []*Node
}

// Prop is a state or attribute worth showing, such as level=2 or
// checked=true.
type Prop struct {
	Key   string
	Value string
}
