package browser

// Gesture names what a person does to an element.
type Gesture string

// The gestures a tab supports.
const (
	Click  Gesture = "click"
	Tap    Gesture = "tap"
	Hover  Gesture = "hover"
	Type   Gesture = "type"
	Press  Gesture = "press"
	Select Gesture = "select"
)

// InteractRequest asks a tab to act on an element as a person would. The
// element is Ref, a ref from a snapshot, or the element Selector picks. With
// neither, Type and Press go to the focused element, and Click and Tap go to
// At, a point in the viewport, whatever is there. Text is what Type types, the key
// Press presses, such as Enter or Control+a, or the option Select picks by
// value or label.
type InteractRequest struct {
	Tab      string
	Action   Gesture
	Ref      string
	Selector string
	At       *Point
	Text     string
}

// Point is a spot in the viewport, in CSS pixels from its top-left corner.
type Point struct {
	X, Y float64
}

// Interaction reports the page after an action. Navigated is true when the
// action moved the tab to another document or within its document. Dialogs
// lists the JavaScript dialogs the action opened, as "type: message"; the
// browser accepts every dialog. Opened lists the tabs the action opened,
// each with the URL the page asked it to load; the current tab stays the
// same.
type Interaction struct {
	URL       string
	Title     string
	Navigated bool
	Dialogs   []string
	Opened    []Tab
}
